//go:gernerate swag init
package server

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/hktalent/go4Hacker/cache"
	"github.com/hktalent/go4Hacker/models"
	"github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
	"xorm.io/xorm"
)

type WebServerConfig struct {
	Driver                       string        `json:"driver"`      // 驱动名
	Dsn                          string        `json:"dsn"`         // 驱动链接信息
	Domain                       string        `json:"domain"`      // 域名
	Author                       string        `json:"author"`      //
	IP                           string        `json:"ip"`          // ip地址
	Listen                       string        `json:"listen"`      // 监听地址，默认 0:8081
	Swagger                      bool          `json:"swagger"`     // 默认 false
	WithGuest                    bool          `json:"withGuest"`   // 默认false
	AuthExpire                   time.Duration `json:"auth_expire"` // 登陆有效期
	DefaultCleanInterval         int64         `json:"default_clean_interval"`
	DefaultQueryApiMaxItem       int           `json:"default_query_api_max_item"`
	DefaultMaxCallbackErrorCount int64         `json:"default_max_callback_error_count"`
	DefaultLanguage              string        `json:"default_language"`
	ServerPem                    string        `json:"server_pem"`
	ServerKey                    string        `json:"server_key"`
}

type WebServer struct {
	WebServerConfig
	engine  *gin.Engine
	orm     *xorm.Engine
	store   *cache.Cache
	uiMutex sync.Mutex

	//internal
	s         *http.Server
	client    *http.Client
	storeQuit chan struct{}
	wg        sync.WaitGroup
	verifyKey string //random generate
}

func NewWebServer(cfg *WebServerConfig, store *cache.Cache) (*WebServer, error) {
	app := &WebServer{}
	app.WebServerConfig = *cfg

	orm, err := xorm.NewEngine(cfg.Driver, cfg.Dsn)
	if err != nil {
		return nil, err
	}
	err = orm.Ping()
	if err != nil {
		return nil, err
	}
	app.orm = orm
	app.store = store

	err = app.initDatabase()
	if err != nil {
		logrus.Errorf("[webserver.go::NewWebServer] initDatabase: %v", err)
		return nil, err
	}

	app.verifyKey = genRandomString(16)
	app.storeQuit = make(chan struct{})
	return app, nil
}

func (self *WebServer) doClean() {
	cache := self.store
	session := self.orm.NewSession()
	defer session.Close()

	var ids []int64
	err := session.Table(&models.TblUser{}).Cols("id").Find(&ids)
	if err != nil {
		logrus.Errorf("[webserver.go::doClean] orm.Find: %v", err)
		return
	}
	now := time.Now()
	if self.orm.DriverName() == "sqlite3" {
		now = now.Local()
	}

	for _, id := range ids {
		userKey := fmt.Sprintf("%v.user", id)
		v, exist := cache.Get(userKey)
		if exist {
			user := v.(*models.TblUser)
			t := now.Add(time.Duration(-1) * time.Duration(user.CleanInterval) * time.Second)
			session.Where(`uid=?`, id).And(`ctime<?`, t).Delete(&models.TblDns{})
			session.Where(`uid=?`, id).And(`ctime<?`, t).Delete(&models.TblHttp{})
		}
	}
}

func (self *WebServer) RunStoreRoutine() {
	store := self.store
	session := self.orm.NewSession()
	defer session.Close()
	ticker := time.NewTicker(1800 * time.Second)
	defer ticker.Stop()

	var client = retryablehttp.NewClient()
	client.RetryMax = 3
	client.RetryWaitMin = 5 * time.Second
	client.RetryWaitMax = 60 * time.Second

	dnsCallBack := func(rcd *DnsRecord) {
		defer self.wg.Done()
		req, err := retryablehttp.NewRequest("POST", rcd.Callback, nil)
		resp, err := client.Do(req)
		errorCountKey := fmt.Sprintf("%v.errcount", rcd.Uid)
		if err != nil {
			store.IncrementInt64(errorCountKey, 1)
			logrus.Infof("[webserver.go::RunStoreRoutine] dns callback: %v", err)
			return
		}
		store.Delete(errorCountKey)
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}

	// httpCallBack := func(rcd *HttpRecord) {
	// 	defer self.wg.Done()
	// 	req, err := retryablehttp.NewRequest("POST", rcd.Callback, nil)
	// 	resp, err := client.Do(req)
	// 	if err != nil {
	// 		logrus.Infof("[webserver.go::RunStoreRoutine] http callback:", err)
	// 		return
	// 	}
	// 	io.Copy(ioutil.Discard, resp.Body)
	// 	resp.Body.Close()
	// }

FOR_LOOP:
	for {
		select {
		case <-ticker.C:
			self.wg.Add(1)
			go func() {
				defer self.wg.Done()
				// self.doClean()
			}()

		case rcd, ok := <-store.Output():
			if !ok {
				break FOR_LOOP
			}
			switch rcd.(type) {
			case *DnsRecord:
				d := rcd.(*DnsRecord)
				_, err := session.InsertOne(&models.TblDns{
					Uid:    d.Uid,
					Domain: d.Domain,
					Var:    d.Var,
					Ip:     d.Ip,
					Ctime:  d.Ctime,
				})
				if err != nil {
					logrus.Fatalf("[web.go::storeRoutine] orm.InsertOne: %v", err)
				}
				if d.Callback != "" && d.Uid > 0 {
					errorCountKey := fmt.Sprintf("%v.errcount", d.Uid)
					v, exist := store.Get(errorCountKey)
					if exist {
						if v.(int64) >= self.DefaultMaxCallbackErrorCount {
							break
						}
					}
					self.wg.Add(1)
					go dnsCallBack(d)
				}
			case *HttpRecord:
				// logged in `record` function
				// 	h := rcd.(*HttpRecord)
				// 	_, err := session.InsertOne(&models.TblHttp{
				// 		Uid:    h.Uid,
				// 		Url:    h.Url,
				// 		Ip:     h.Ip,
				// 		Ua:     h.Ua,
				// 		Data:   h.Data,
				// 		Ctype:  h.Ctype,
				// 		Method: h.Method,
				// 		Ctime:  h.Ctime,
				// 	})

				// 	if err != nil {
				// 		logrus.Fatalf("[web.go::storeRoutine] orm.InsertOne: %v", err)
				// 	}

				// 	//async callback
				// 	if h.Callback != "" && h.Uid > 0 {
				// 		self.wg.Add(1)
				// 		go httpCallBack(h)
				// 	}
			}
		}
	}
	close(self.storeQuit)
}

func (self *WebServer) Run() error {
	r := gin.Default()

	//if self.Swagger {
	//	// use localhost
	//	url := ginSwagger.URL("http://localhost:8080/swagger/doc.json") // The url pointing to API definition
	//	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, url))
	//}

	//static handler
	r.Use(static.Serve("/", static.LocalFile("dist", false)))
	r.NoRoute(func(c *gin.Context) {
		c.File("dist/index.html")
	})

	//api handler
	api := r.Group("/api")

	r.GET("/Auth", self.getAuthorizationInfo)

	//auth group
	auth := api.Group("auth")
	{
		auth.POST("/login", self.userLogin)
		auth.POST("/logout", self.authHandler, self.userLogout)
		auth.GET("/info", self.authHandler, self.userInfo)
		auth.GET("/nav", self.authHandler, self.userNav)
		auth.GET("/captcha", self.getCaptcha)
	}

	r.GET("/auth/show/*captchaId", self.getCaptchaImg)

	//data group
	data := api.Group("/record", self.authHandler)
	{
		data.GET("/dns", self.getDnsRecord)
		data.GET("/http", self.getHttpRecord)
		data.DELETE("/dns", self.delDnsRecord)
		data.DELETE("/http", self.delHttpRecord)
	}

	setting := api.Group("/setting", self.authHandler)
	{
		setting.GET("/app", self.getAppSetting)
		setting.POST("/app", self.setAppSetting)

		setting.GET("/security", self.getSecuritySetting)
		setting.POST("/security", self.setSecuritySetting)

		setting.GET("/resolve", self.verifyAdminPermission, self.getResolveRecord)
		setting.POST("/resolve", self.verifyAdminPermission, self.setResolveRecord)
		setting.DELETE("/resolve", self.verifyAdminPermission, self.delResolveRecord)
	}

	//admin
	admin := api.Group("admin", self.authHandler, self.verifyAdminPermission)
	{
		admin.DELETE("/user", self.delUser)
		admin.PUT("/user", self.addUser)
		admin.POST("/user", self.setUser)
		admin.GET("/user/list", self.userList)

		admin.GET("/allusers", self.allUserNameList)
		admin.GET("/allrecord/dns", self.getDnsAllRecord)
		admin.GET("/allrecord/http", self.getAllHttpRecord)
		admin.DELETE("/allrecord/dns", self.delAllDnsRecord)
		admin.DELETE("/allrecord/http", self.delAllHttpRecord)

		admin.GET("/allrecord/count", self.countAllRecord)
	}

	//record handler
	dataApi := r.Group("/data", self.dataPreHandler, self.dataAuthHandler)
	{
		dataApi.GET("/dns", self.queryDnsRecord)
		dataApi.GET("/http", self.queryHttpRecord)
	}
	//http log
	r.Any("/log/:shortId/*any", self.record)

	payload := r.Group("/payload")
	{
		payload.GET("/xss", self.xss)
		payload.GET("/phprfi", self.phpRFI)
	}

	s := &http.Server{
		Handler: r,
	}
	l, err := net.Listen("tcp", self.Listen)
	if err != nil {
		return err
	}
	self.s = s
	return s.Serve(l)
}

func (self *WebServer) Shutdown(ctx context.Context) error {
	err := self.s.Shutdown(ctx)
	//important: stop input then call shutdown

	<-self.storeQuit
	self.orm.Close()
	return err
}

func (self *WebServer) IsDuplicate(err error) bool {
	if err == nil {
		return false
	}

	orm := self.orm
	switch orm.DriverName() {
	case "sqlite3":
		e, ok := err.(sqlite3.Error)
		if !ok {
			logrus.Printf("[IsDuplicate] convert sqlite error: typeof(err)")
		}
		if e.Code == sqlite3.ErrConstraint {
			return true
		}
	case "mysql":
		e := err.(*mysql.MySQLError)
		if e.Number == 1062 || e.Number == 1169 || e.Number == 1022 {
			return true
		}
	}
	return false
}

func (self *WebServer) ResetPassword(user, password string) error {
	if isWeakPass(password) {
		return fmt.Errorf("Password(%v) too weak!", password)
	}

	orm := self.orm
	session := orm.NewSession()
	defer session.Close()

	_, err := session.Where(`role = ?`, roleSuper).Cols("pass").
		Update(&models.TblUser{
			Pass: makePassword(password),
		})
	return err
}
