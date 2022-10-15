package server

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hktalent/go4Hacker/cache"
	"github.com/hktalent/go4Hacker/models"

	"github.com/chennqqi/goutils/ginutils"
	"github.com/dchest/captcha"
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"net/http"
	// "xorm.io/builder"
)

type MyClaims struct {
	Seed string `json:"seed"`
	jwt.StandardClaims
}

//==============================================================================
// ui standard api
//==============================================================================
func (self *WebServer) respData(c *gin.Context, status, code int,
	message string, data interface{}) {
	c.JSON(status, &CR{
		Message:   message,
		Code:      code,
		Timestamp: time.Now().Unix(),
	})
}

func (self *WebServer) resp(c *gin.Context, status int, cr *CR) {
	cr.Timestamp = time.Now().Unix()
	c.JSON(status, cr)
}

func (self *WebServer) initDatabase() error {
	orm := self.orm
	orm.SetTZDatabase(time.Local)
	orm.SetTZLocation(time.Local)

	err := orm.Sync(&models.TblDns{},
		&models.TblHttp{},
		&models.TblUser{},
		&models.TblResolve{})
	if err != nil {
		logrus.Errorf("[webui.go::initDatabase] orm.Sync: %v", err)
		return err
	}

	// check superUser
	{
		count, err := orm.Count(&models.TblUser{})
		if err != nil {
			logrus.Errorf("[webui.go::initDatabase] orm.Count(user): %v", err)
			return err
		}
		// if there is no supser user when system first init
		if count == 0 {
			randomPass := genRandomString(12)
			_, err = orm.InsertOne(&models.TblUser{
				Name:          "admin",
				Email:         "admin@dnslog.com",
				Company:       self.WebServerConfig.Author,
				FullName:      "admin",
				ShortId:       genShortId(),
				Pass:          makePassword(randomPass),
				Token:         genRandomToken(),
				Role:          roleSuper,
				Lang:          self.DefaultLanguage,
				CleanInterval: self.DefaultCleanInterval,
			})
			if err != nil {
				logrus.Errorf("[webui.go::initDatabase] orm.InsertOne(user): %v", err)
				return err
			}
			fmt.Printf("Init super admin user with password: %v\n", randomPass)
		}
	}

	// check and add guest user
	if self.WithGuest {
		var guestUser models.TblUser
		exist, err := orm.Where(`name=?`, "guest").Get(&guestUser)
		if err != nil {
			logrus.Errorf("[webui.go::initDatabase] orm.Get(user.name='guest'): %v", err)
			return err
		} else if !exist {
			guestUser.Name = "guest"
			guestUser.Email = "guest@godnslog.com"
			guestUser.Pass = makePassword("guest123")
			guestUser.Company = "guest"
			guestUser.FullName = "guest"
			guestUser.Token = genRandomToken()
			guestUser.Role = roleGuest
			guestUser.Lang = self.DefaultLanguage
			guestUser.CleanInterval = self.DefaultCleanInterval
			_, err = orm.InsertOne(&guestUser)
			if err != nil {
				logrus.Errorf("[webui.go::initDatabase] orm.InsertOne(user.name=guest): %v", err)
				return err
			}
		}
	}

	var wwwRcd models.TblResolve
	exist, err := orm.Where(`host=?`, `www`).And(`type=?`, `A`).Get(&wwwRcd)
	if err != nil {
		logrus.Errorf("[webui.go::initDatabase] orm.Get(resolve): %v", err)
		return err
	} else if !exist {
		wwwRcd.Host = "www"
		wwwRcd.Value = self.IP
		wwwRcd.Type = "A"
		wwwRcd.Ttl = 600 // default 600s
		orm.InsertOne(&wwwRcd)
	} else if wwwRcd.Value != self.IP {
		wwwRcd.Value = self.IP
		orm.Update(&wwwRcd)
	}

	store := self.store
	// sync user
	orm.Iterate(new(models.TblUser), func(idx int, bean interface{}) error {
		user := bean.(*models.TblUser)
		userKey := fmt.Sprintf("%v.user", user.Id)
		store.Set(userKey, user, cache.NoExpiration)
		domainKey := fmt.Sprintf("%v.suser", user.ShortId)
		store.Set(domainKey, user, cache.NoExpiration)
		return nil
	})

	// sync standard dns service data
	var hosts []string
	err = orm.Table(&models.TblResolve{}).GroupBy("host").Cols("host").Find(&hosts)
	if err != nil {
		logrus.Panicf("[webui.go::initDatabase] orm.GroupBy: %v", err)
	}

	for i := 0; i < len(hosts); i++ {
		var all []models.TblResolve
		err := orm.Where(`host=?`, hosts[i]).Find(&all)
		if err != nil {
			logrus.Panicf("[webui.go::initDatabase] orm.Find(%v): %v", hosts[i], err)
		}
		self.updateResolveCache(hosts[i], "", all)
	}

	return nil
}

func (self *WebServer) authHandler(c *gin.Context) {
	T := getTranslateFunc(c)
	tokenString := c.GetHeader("Access-Token")
	if tokenString == "" {
		c.JSON(401, CR{
			Message: T("Token Required"),
			Code:    CodeNoAuth,
		})
		c.Abort()
		return
	}
	var claim MyClaims
	token, err := jwt.ParseWithClaims(tokenString, &claim, func(token *jwt.Token) (interface{}, error) {
		// since we only use the one private key to sign the tokens,
		// we also only use its public counter part to verify
		return []byte(self.verifyKey), nil
	})
	if token.Valid {
		store := self.store
		key := fmt.Sprintf("%v.seed", claim.Id)
		realSeed, exist := store.Get(key)
		if !exist {
			logrus.Infof("That's not even a token")
			c.JSON(401, CR{
				Message: T("not login"),
				Code:    CodeNoAuth,
			})
			c.Abort()
			return
		} else if realSeed.(string) != claim.Seed {
			logrus.Infof("That's not even a token")
			c.JSON(401, CR{
				Message: T("Token Expire"),
				Code:    CodeNoAuth,
			})
			c.Abort()
			return
		}
		u, exist := store.Get(fmt.Sprintf("%v.user", claim.Id))
		if !exist {
			logrus.Infof("[webui.go::authHandler] cache.Get(user), not exist")
			c.JSON(401, CR{
				Message: T("not login"),
				Code:    CodeNoAuth,
			})
			c.Abort()
			return
		}

		var uid int64
		fmt.Sscanf(claim.Id, "%d", &uid)
		c.Set("id", uid)
		c.Set("username", claim.Audience)
		c.Set("email", claim.Subject)
		c.Set("seed", claim.Seed)
		c.Set("role", u.(*models.TblUser).Role)

		//TODO: permission
		return
	} else if ve, ok := err.(*jwt.ValidationError); ok {
		if ve.Errors&jwt.ValidationErrorMalformed != 0 {
			logrus.Infof("That's not even a token")
			c.JSON(401, CR{
				Message: T("Token invalid"),
				Code:    CodeNoAuth,
			})
			c.Abort()
			return
		} else if ve.Errors&(jwt.ValidationErrorExpired|jwt.ValidationErrorNotValidYet) != 0 {
			// Token is either expired or not active yet
			logrus.Infof("Timing is everything")
			c.JSON(401, CR{
				Message: T("Token Expired or not active yet"),
				Code:    CodeNoAuth,
			})
			c.Abort()
			return
		} else {
			logrus.Warnf("Couldn't handle this token: %v", err)
			c.JSON(401, CR{
				Message: T("Can't handle this token"),
				Code:    0,
			})
			c.Abort()
			return
		}
	}
}

func (self *WebServer) verifyAdminPermission(c *gin.Context) {
	T := getTranslateFunc(c)

	role := c.GetInt("role")
	switch role {
	case roleAdmin, roleSuper:
		return
	default:
		self.resp(c, 403, &CR{
			Message: T("bad permission"),
			Code:    CodeNoPermission,
		})
		c.Abort()
		return
	}
}

//==============================================================================
//									user auth
//==============================================================================

// @Summary userLogin
// @Description get Dns Record by user query
// @Accept  json
// @Produce  json
// @Param   some_id path int true "Some ID"
// @Success 200 {string} CR	"OK"
// @Failure 502 {object} CR "BadService"
// @Failure 403 {object} CR "Forbidden"
// @Failure 401 {object} CR "Unauthorized"
// @Router /user/login [post]
func (self *WebServer) userLogin(c *gin.Context) {
	T := getTranslateFunc(c)

	var req LoginRequest
	err := c.BindJSON(&req)
	if err != nil {
		logrus.Infof("[webui.go::userLogin] bad input param")
		self.resp(c, 400, &CR{
			Code:    CodeBadData,
			Message: T("bad input"),
		})
		return
	}

	if !captcha.VerifyString(req.CaptchaId, req.Verifycode) {
		self.resp(c, 400, &CR{
			Code:    VerifycodeErr,
			Message: T("验证码错误"),
		})
		return
	}

	session := self.orm.NewSession()
	defer session.Close()
	var user = new(models.TblUser)
	exist, err := session.Where(`name=?`, req.Username).
		Or(`email=?`, req.Email).Get(user)

	if err != nil {
		logrus.Errorf("[webui.go::userLogin] orm.Get: %v", err)
		self.respData(c, 502, CodeServerInternal, T("bad service"), nil)
		return
	} else if !exist {
		logrus.Infof("[webui.go::userLogin] not found: %v", req)
		self.respData(c, 401, CodeBadData, T("bad request"), nil)
		return
	}
	err = comparePassword(req.Password, user.Pass)
	if err != nil {
		logrus.Infof("[webui.go::userLogin] password not match")
		self.respData(c, 401, CodeBadData, T("bad request"), nil)
		return
	}

	now := time.Now()
	seed := getSecuritySeed()
	token := jwt.NewWithClaims(jwt.SigningMethodHS384, MyClaims{
		seed,
		jwt.StandardClaims{
			Id:        fmt.Sprintf("%v", user.Id),
			Audience:  user.Name,
			Subject:   user.Email,
			ExpiresAt: now.Add(3600 * 24 * time.Second).Unix(),
			IssuedAt:  now.Unix(),
			Issuer:    self.Domain,
		},
	})

	tokenString, err := token.SignedString([]byte(self.verifyKey))
	if err != nil {
		logrus.Errorf("[webui.go::userLogin] token.SignedString: %v", err)
		self.respData(c, 502, CodeServerInternal, T("bad service"), nil)
		return
	}
	store := self.store

	store.Set(fmt.Sprintf("%v.seed", user.Id), seed, self.AuthExpire)
	store.Set(fmt.Sprintf("%v.user", user.Id), user, cache.NoExpiration)

	self.resp(c, 200, &CR{
		Message: T("OK"),
		Result: LoginResponse{
			Islogin: true,
			Token:   tokenString,
		},
	})
}

func Serve(w http.ResponseWriter, r *http.Request, id, ext, lang string, download bool, width, height int) error {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	var content bytes.Buffer
	switch ext {
	case ".png":
		w.Header().Set("Content-Type", "image/png")
		_ = captcha.WriteImage(&content, id, width, height)
	case ".wav":
		w.Header().Set("Content-Type", "audio/x-wav")
		_ = captcha.WriteAudio(&content, id, lang)
	default:
		return captcha.ErrNotFound
	}

	if download {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	http.ServeContent(w, r, id+ext, time.Time{}, bytes.NewReader(content.Bytes()))
	return nil
}

func (self *WebServer) getCaptchaImg(c *gin.Context) {
	captchaId := c.Param("captchaId")
	captchaId = strings.Replace(captchaId, "/", "", 1)
	w, h := 100, 40
	_ = Serve(c.Writer, c.Request, captchaId, ".png", "zh", false, w, h)
}

func (self *WebServer) getCaptcha(c *gin.Context) {
	T := getTranslateFunc(c)
	captchaId := captcha.NewLen(4)

	self.resp(c, 200, &CR{
		Message: T("OK"),
		Code:    CodeOK,
		Result:  captchaId,
	})
}

// @Summary userLogout
// @Description get Dns Record by user query
// @Accept  json
// @Produce  json
// @Param   some_id     path    int     true        "Some ID"
// @Success 200 {string} CR	"OK"
// @Failure 502 {object} CR "BadService"
// @Failure 403 {object} CR "Forbidden"
// @Failure 401 {object} CR "Unauthorized"
// @Router /user/logout [post]
func (self *WebServer) userLogout(c *gin.Context) {
	T := getTranslateFunc(c)

	store := self.store
	id := c.GetInt64("id")
	store.Delete(fmt.Sprintf("%v.seed", id))
	store.Delete(fmt.Sprintf("%v.user", id))
	self.resp(c, 200, &CR{
		Message: T("OK"),
	})
}

func parseRole(roleValue int) models.Role {
	var role models.Role
	role.Id = "normal"
	role.Name = "普通用户"
	role.Permissions = []models.Permission{
		models.Permission{
			RoleId:         roleNormal,
			PermissionId:   "document",
			PermissionName: "文档",
		},
	}
	switch roleValue {
	case roleAdmin, roleSuper:
		role.Id = "admin"
		role.Name = "管理员"
		role.Permissions = append(role.Permissions, []models.Permission{
			models.Permission{
				RoleId:         roleNormal,
				PermissionId:   "setting",
				PermissionName: "设置",
			},
			models.Permission{
				RoleId:         roleAdmin,
				PermissionId:   "manage",
				PermissionName: "管理用户",
			},
			models.Permission{
				RoleId:         roleNormal,
				PermissionId:   "record",
				PermissionName: "记录",
			},
		}...)

	case roleNormal:
		role.Permissions = append(role.Permissions, []models.Permission{
			models.Permission{
				RoleId:         roleNormal,
				PermissionId:   "setting",
				PermissionName: "设置",
			},
			models.Permission{
				RoleId:         roleNormal,
				PermissionId:   "record",
				PermissionName: "记录",
			},
		}...)
	case roleGuest:
		role.Id = "guest"
		role.Name = "访客"
	}
	return role
}

// @Summary userInfo
// @Description get Dns Record by user query
// @Accept  json
// @Produce  json
// @Param   some_id     path    int     true        "Some ID"
// @Success 200 {string} string	"ok"
// @Failure 400 {object} CR "We need ID!!"
// @Failure 404 {object} CR "Can not find ID"
// @Failure 401 {object} CR "Can not find ID"
// @Router /user/info [get]
func (self *WebServer) userInfo(c *gin.Context) {
	T := getTranslateFunc(c)
	id := c.GetInt64("id")
	session := self.orm.NewSession()
	defer session.Close()

	store := self.store
	userKey := fmt.Sprintf("%v.user", id)
	v, exist := store.Get(userKey)
	var user *models.TblUser
	if !exist {
		user = new(models.TblUser)
		exist, err := session.ID(id).Get(user)
		if err != nil {
			logrus.Errorf("[webui.go::userInfo] orm.Get: %v", err)
			self.resp(c, 502, &CR{
				Message: T("Failed"),
				Code:    CodeServerInternal,
			})
			return
		}
		if !exist {
			logrus.Errorf("[webui.go::userInfo] No such user")
			self.resp(c, 400, &CR{
				Message: T("No such user"),
				Code:    CodeBadData,
			})
			return
		}
		store.Set(userKey, user, cache.NoExpiration)
		domainKey := fmt.Sprintf("%v.suser", user.ShortId)
		store.Set(domainKey, user, cache.NoExpiration)
	} else {
		user = v.(*models.TblUser)
	}

	//TODO: UserInfo from cache, role & permissions
	self.resp(c, 200, &CR{
		Message: T("OK"),
		Code:    CodeOK,
		Result: UserInfo{
			Id:       user.Id,
			Name:     user.Name,
			FullName: user.FullName,
			Company:  user.Company,
			Email:    user.Email,
			Role:     parseRole(user.Role),
		},
	})
}

// @Summary userInfo
// @Description get Dns Record by user query
// @Accept  json
// @Produce  json
// @Param   some_id     path    int     true        "Some ID"
// @Success 200 {string} string	"ok"
// @Failure 400 {object} CR "We need ID!!"
// @Failure 404 {object} CR "Can not find ID"
// @Failure 401 {object} CR "Can not find ID"
// @Router /admin/user/list [get]
func (self *WebServer) userList(c *gin.Context) {
	T := getTranslateFunc(c)

	pageNo, pageNoErr := ginutils.GetQueryInt(c, "pageNo")
	if pageNoErr != nil || pageNo <= 0 {
		pageNo = 1
	}
	pageSize, pageSizeErr := ginutils.GetQueryInt(c, "pageSize")
	if pageSizeErr != nil {
		pageSize = 10
	}

	session := self.orm.NewSession()
	defer session.Close()

	session = session.Where(`id>1`)
	var items []models.TblUser
	count, err := session.Limit(pageSize, (pageNo-1)*pageSize).FindAndCount(&items)
	if err != nil {
		self.resp(c, 502, &CR{
			Code:    CodeServerInternal,
			Message: T("Failed"),
		})
		return
	}

	var resp UserListResp
	resp.TotalCount = int(count)
	resp.PageSize = pageSize
	resp.PageNo = pageNo
	resp.TotalPage = (resp.TotalCount + (pageSize - 1)) / pageSize
	resp.Data = make([]models.UserInfo, len(items))
	for i := 0; i < len(items); i++ {
		rcd := &resp.Data[i]
		item := &items[i]
		rcd.Id = item.Id
		rcd.Name = item.Name
		rcd.ShortId = item.ShortId
		rcd.Email = item.Email
		rcd.Company = item.Company
		rcd.FullName = item.FullName
		rcd.Utime = item.Utime
		rcd.Role = parseRole(item.Role)
	}

	self.resp(c, 200, &CR{
		Message: T("OK"),
		Result:  &resp,
	})
}

func (self *WebServer) allUserNameList(c *gin.Context) {
	T := getTranslateFunc(c)
	session := self.orm.NewSession()
	defer session.Close()

	session = session.Where(`id>0`)
	var items []models.TblUser
	err := session.Find(&items)
	if err != nil {
		self.resp(c, 502, &CR{
			Code:    CodeServerInternal,
			Message: T("Failed"),
		})
		return
	}

	var allUsers = make([]string, len(items))

	for i := 0; i < len(items); i++ {
		item := &items[i]
		allUsers[i] = item.Name
	}

	self.resp(c, 200, &CR{
		Message: T("OK"),
		Result:  allUsers,
	})
}

// @Summary userNav
// @Description get Dns Record by user query
// @Accept  json
// @Produce  json
// @Param   some_id     path    int     true        "Some ID"
// @Success 200 {string} string	"ok"
// @Failure 400 {object} CR "We need ID!!"
// @Failure 404 {object} CR "Can not find ID"
// @Failure 401 {object} CR "Can not find ID"
// @Router /user/nav [get]
func (self *WebServer) userNav(c *gin.Context) {
}

//==============================================================================
//							user manage
//==============================================================================

// @Summary userNav
// @Description get Dns Record by user query
// @Accept  json
// @Produce  json
// @Param   some_id     path    int     true        "Some ID"
// @Success 200 {string} string	"ok"
// @Failure 400 {object} CR "We need ID!!"
// @Failure 404 {object} CR "Can not find ID"
// @Failure 401 {object} CR "Can not find ID"
// @Router /user/nav [get]
func (self *WebServer) delUser(c *gin.Context) {
	T := getTranslateFunc(c)

	var req DeleteRecordRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		logrus.Infof("[webapi.go::delUser] parameter required")
		self.resp(c, 400, &CR{
			Message: T("param required"),
			Code:    CodeBadData,
		})
		return
	}
	var ids = make([]interface{}, len(req.Ids))
	for i := 0; i < len(req.Ids); i++ {
		ids[i] = req.Ids[i]
	}

	session := self.orm.NewSession()
	defer session.Close()

	var users []*models.TblUser
	err = session.In("id", ids...).Find(&users)
	if err != nil {
		logrus.Errorf("[webapi.go::delUser] orm.Find: %v", err)
		self.resp(c, 502, &CR{
			Message: T("failed"),
			Code:    CodeServerInternal,
		})
		return
	}

	//do not delete super user
	_, err = session.In("id", ids...).Delete(&models.TblUser{})
	if err != nil {
		logrus.Errorf("[webapi.go::delUser] orm.Delete: %v", err)
		self.resp(c, 502, &CR{
			Message: T("failed"),
			Code:    CodeServerInternal,
		})
		return
	}
	session.In("uid", ids).Delete(&models.TblDns{})
	session.In("uid", ids).Delete(&models.TblHttp{})

	cache := self.store

	for _, user := range users {
		domainKey := fmt.Sprintf("%v.suser", user.ShortId)
		cache.Delete(domainKey)
	}

	for i := 0; i < len(req.Ids); i++ {
		seedKey := fmt.Sprintf("%v.seed", req.Ids[i])
		userKey := fmt.Sprintf("%v.user", req.Ids[i])
		// v, exist := cache.Get(userKey)
		// if exist {
		// 	domainKey := fmt.Sprintf("%v.suser", v.(*models.TblUser).ShortId)
		// 	cache.Delete(domainKey)
		// }

		//logout these users
		cache.Delete(seedKey)
		cache.Delete(userKey)
	}

	self.resp(c, 200, &CR{
		Message: T("OK"),
	})
}

// @Summary addUser
// @Description add a new User
// @Accept  json
// @Produce  json
// @Param   some_id     path    int     true        "Some ID"
// @Success 200 {string} CR	"OK"
// @Failure 502 {object} CR "BadService"
// @Failure 403 {object} CR "Forbidden"
// @Failure 401 {object} CR "Unauthorized"
// @Router /user/logout [post]
func (self *WebServer) addUser(c *gin.Context) {
	T := getTranslateFunc(c)

	var req UserRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		logrus.Infof("[webui.go::addUser] parameter format invalid")
		self.resp(c, 400, &CR{
			Message: T("Bad param"),
			Code:    CodeBadData,
		})
		return
	}

	if isWeakPass(req.Password) {
		self.resp(c, 400, &CR{
			Message: T("password too weak"),
			Code:    CodeBadData,
		})
		return
	}

	// TODO: other checks

	//random api Token
	session := self.orm.NewSession()
	defer session.Close()

	var item = models.TblUser{
		Name:          req.Name,
		FullName:      req.FullName,
		Email:         req.Email,
		Company:       req.Company,
		Role:          req.Role,
		Token:         genRandomToken(),
		ShortId:       req.ShortId,
		Lang:          self.DefaultLanguage,
		Pass:          makePassword(req.Password),
		CleanInterval: self.DefaultCleanInterval,
	}
	_, err = session.InsertOne(&item)
	if self.IsDuplicate(err) {
		self.resp(c, 400, &CR{
			Message: T("Failed"),
			Code:    CodeBadData,
		})
		return
	} else if err != nil {
		self.resp(c, 502, &CR{
			Message: T("Failed"),
			Code:    CodeServerInternal,
		})
		return
	}

	store := self.store
	domainKey := fmt.Sprintf("%v.suser", item.ShortId)
	store.Set(domainKey, &item, cache.NoExpiration)

	self.resp(c, 200, &CR{
		Message: T("OK"),
	})
}

func (self *WebServer) getUserById(id int64) *models.TblUser {
	session := self.orm.NewSession()
	defer session.Close()

	var user *models.TblUser
	user = new(models.TblUser)
	exist, err := session.ID(id).Get(user)
	if err != nil {
		sql, _ := session.LastSQL()
		logrus.Errorf("[webapi.go::setUser] orm.Get error: %v, sql:%v", err, sql)
		return nil
	} else if !exist {
		return nil
	}
	return user
}

func (self *WebServer) setUser(c *gin.Context) {
	T := getTranslateFunc(c)

	var req UserRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		logrus.Infof("[webapi.go::setUser] parameter required")
		self.resp(c, 400, &CR{
			Message: T("param invaid: ") + err.Error(),
			Code:    CodeBadData,
		})
		return
	}
	if req.Id < 1 {
		self.resp(c, 400, &CR{
			Message: T("Can't change"),
			Code:    CodeBadData,
		})
		return
	}

	store := self.store
	id := c.GetInt64("id")
	role := c.GetInt("role")
	session := self.orm.NewSession()
	defer session.Close()

	var user *models.TblUser

	switch role {
	case roleSuper, roleAdmin:
		//change other user
		var oldUser *models.TblUser

		session = session.ID(req.Id)
		if req.Password != "" {
			newPass := makePassword(req.Password)
			session = session.SetExpr(`pass`, customQuote(newPass))
		}
		if req.Email != "" {
			session = session.SetExpr(`email`, customQuote(req.Email))
		}
		if req.Name != "" {
			session = session.SetExpr(`name`, customQuote(req.Name))
		}

		if req.Company != "" {
			session = session.SetExpr(`company`, customQuote(req.Company))
		}

		if req.FullName != "" {
			session = session.SetExpr(`full_name`, customQuote(req.FullName))
		}

		if req.ShortId != "" {
			oldUser = self.getUserById(req.Id)
			session = session.SetExpr(`short_id`, customQuote(req.ShortId))
		}

		_, err = session.Update(&models.TblUser{})
		if err != nil {
			sql, _ := session.LastSQL()
			logrus.Errorf("[webapi.go::setUser] orm.Update error: %v, sql:%v", err, sql)
			self.resp(c, 400, &CR{
				Message: "failed",
				Code:    CodeServerInternal,
			})
			return
		}

		//logout req.Id
		// cache := self.store
		if req.ShortId != "" {
			//更新domainKey
			if oldUser != nil {
				oldDomainKey := fmt.Sprintf("%v.suser", oldUser.ShortId)
				store.Delete(oldDomainKey)
			}
			newUser := self.getUserById(req.Id)
			newDomainKey := fmt.Sprintf("%v.suser", newUser.ShortId)
			store.Set(newDomainKey, newUser, cache.NoExpiration)
		}
		store.Delete(fmt.Sprintf("%v.seed", req.Id))
		store.Delete(fmt.Sprintf("%v.user", req.Id))
		self.resp(c, 200, &CR{
			Message: T("OK"),
		})

	case roleNormal:
		//allow change language only
		userKey := fmt.Sprintf("%v.user", id)
		v, exist := store.Get(userKey)
		if !exist {
			user = new(models.TblUser)
			exist, err := session.ID(id).Get(user)
			if err != nil {
				sql, _ := session.LastSQL()
				logrus.Errorf("[webapi.go::setUser] orm.Get error: %v, sql:%v", err, sql)
				self.resp(c, 502, &CR{
					Message: T("failed"),
					Code:    CodeServerInternal,
				})
				return
			} else if !exist {
				//this should not happend
				self.resp(c, 400, &CR{
					Message: T("Failed"),
					Code:    CodeBadData,
				})
				return
			}

		} else {
			user = v.(*models.TblUser)
		}
		dupUser := new(models.TblUser)
		*dupUser = *user

		_, err := session.ID(id).Cols("lang").Update(dupUser)
		if err != nil {
			sql, _ := session.LastSQL()
			logrus.Errorf("[webapi.go::setUser] orm.Update error: %v, sql:%v", err, sql)
			self.resp(c, 400, &CR{
				Message: T("failed"),
				Code:    CodeServerInternal,
			})
			return
		}
		store.Set(userKey, dupUser, cache.NoExpiration)
		domainKey := fmt.Sprintf("%v.suser", dupUser.ShortId)
		store.Set(domainKey, dupUser, cache.NoExpiration)
		self.resp(c, 200, &CR{
			Message: T("OK"),
		})
	}
}

func (self *WebServer) getAppSetting(c *gin.Context) {
	T := getTranslateFunc(c)
	id := c.GetInt64("id")
	store := self.store
	userKey := fmt.Sprintf("%v.user", id)
	v, exist := store.Get(fmt.Sprintf(userKey, id))
	var user *models.TblUser
	if !exist {
		session := self.orm.NewSession()
		defer session.Close()

		user = new(models.TblUser)
		exist, err := session.ID(id).Get(user)
		if err != nil {
			sql, _ := session.LastSQL()
			logrus.Errorf("[webui.go::getSecuritySetting] orm.Get error: %v, sql: %v", err, sql)
			self.resp(c, 502, &CR{
				Message: T("Failed"),
				Code:    CodeServerInternal,
			})
			return
		} else if !exist {
			logrus.Errorf("[webui.go::getSecuritySetting] not found user(id=%v), this should not happend", id)
			self.resp(c, 502, &CR{
				Message: T("Failed"),
				Code:    CodeServerInternal,
			})
			return
		}
		store.Set(userKey, user, cache.NoExpiration)
		domainKey := fmt.Sprintf("%v.suser", user.ShortId)
		store.Set(domainKey, user, cache.NoExpiration)
	} else {
		user = v.(*models.TblUser)
	}

	self.resp(c, 200, &CR{
		Message: "OK",
		Result: AppSetting{
			Rebind:    user.Rebind,
			Callback:  user.Callback,
			CleanHour: user.CleanInterval / 3600,
		},
	})
}

func (self *WebServer) setAppSetting(c *gin.Context) {
	T := getTranslateFunc(c)
	var req AppSetting
	err := c.ShouldBindJSON(&req)
	if err != nil {
		logrus.Infof("[webui.go::setAppSetting] parameter format invalid")
		self.resp(c, 400, &CR{
			Message: T("Bad param"),
			Code:    CodeBadData,
		})
		return
	}

	id := c.GetInt64("id")
	store := self.store
	userKey := fmt.Sprintf("%v.user", id)
	v, exist := store.Get(userKey)
	session := self.orm.NewSession()
	defer session.Close()

	var user *models.TblUser
	if !exist {
		user = new(models.TblUser)
		exist, err := session.ID(id).Get(user)
		if err != nil {
			logrus.Errorf("[webuig.go::setAppSetting] orm.Get error: %v", err)
			self.resp(c, 502, &CR{
				Message: T("Failed"),
				Code:    CodeServerInternal,
			})
			return
		} else if !exist {
			logrus.Errorf("[webuig.go::setAppSetting] not found user(id=%v), this should not happend", id)
			self.resp(c, 502, &CR{
				Message: T("Failed"),
				Code:    CodeServerInternal,
			})
			return
		}
		store.Set(userKey, user, cache.NoExpiration)
		domainkey := fmt.Sprintf("%v.suser", user.ShortId)
		store.Set(domainkey, user, cache.NoExpiration)
	} else {
		user = v.(*models.TblUser)
	}

	dupUser := new(models.TblUser)
	*dupUser = *user
	dupUser.Rebind = req.Rebind
	dupUser.Callback = req.Callback
	dupUser.CleanInterval = req.CleanHour * 3600
	_, err = session.ID(id).Cols("rebind", "callback", "clean_interval").Update(dupUser)
	if err != nil {
		logrus.Errorf("[webui.go::setAppSetting] orm.Update error: %v", err)
		self.resp(c, 502, &CR{
			Message: T("Failed"),
			Code:    CodeServerInternal,
		})
		return
	}

	//update cache
	{
		domainKey := fmt.Sprintf("%v.suser", user.ShortId)
		userKey := fmt.Sprintf("%v.user", user.Id)
		store.Set(userKey, dupUser, cache.NoExpiration)
		store.Set(domainKey, dupUser, cache.NoExpiration)
	}

	self.resp(c, 200, &CR{
		Message: T("OK"),
	})
}

//change self password
func (self *WebServer) getSecuritySetting(c *gin.Context) {
	T := getTranslateFunc(c)
	id := c.GetInt64("id")
	store := self.store
	userKey := fmt.Sprintf("%v.user", id)
	v, exist := store.Get(userKey)
	var user *models.TblUser
	if !exist {
		session := self.orm.NewSession()
		defer session.Close()
		user = new(models.TblUser)
		exist, err := session.ID(id).Get(user)
		if err != nil {
			sql, _ := session.LastSQL()
			logrus.Errorf("[webuig.go::getSecuritySetting] orm.Get error: %v, sql: %v", err, sql)
			self.resp(c, 502, &CR{
				Message: T("Failed"),
				Code:    CodeServerInternal,
			})
			return
		} else if !exist {
			logrus.Errorf("[webuig.go::getSecuritySetting] not found user(id=%v), this should not happend", id)
			self.resp(c, 502, &CR{
				Message: T("Failed"),
				Code:    CodeServerInternal,
			})
			return
		}
		store.Set(userKey, user, cache.NoExpiration)
		domainkey := fmt.Sprintf("%v.suser", user.ShortId)
		store.Set(domainkey, user, cache.NoExpiration)
	} else {
		user = v.(*models.TblUser)
	}

	self.resp(c, 200, &CR{
		Message: T("OK"),
		Result: AppSecurity{
			HttpAddr: fmt.Sprintf("http://%v/log/%v/", self.IP, user.ShortId),
			DnsAddr:  user.ShortId + "." + self.Domain,
			Token:    user.Token,
		},
	})
}

//change self password
func (self *WebServer) setSecuritySetting(c *gin.Context) {
	T := getTranslateFunc(c)
	var req AppSecuritySet
	err := c.ShouldBindJSON(&req)
	if err != nil {
		logrus.Infof("[webuig.go::setSecuritySetting] bad data")
		self.resp(c, 400, &CR{
			Message: T("bad param"),
			Code:    CodeBadData,
		})
		return
	}
	if isWeakPass(req.Password) {
		logrus.Warnf("[webuig.go::setSecuritySetting] weak password data")
		self.resp(c, 400, &CR{
			Message: T("password too weak"),
			Code:    CodeBadData,
		})
		return
	}

	id := c.GetInt64("id")
	session := self.orm.NewSession()
	defer session.Close()

	newPass := makePassword(req.Password)
	//logrus.Debugf("password:%v, hashpass=%v", req.Password, string(newPass))
	_, err = session.ID(id).SetExpr(`pass`, customQuote(newPass)).Update(&models.TblUser{})
	if err != nil {
		sql, _ := session.LastSQL()
		logrus.Errorf("[webuig.go::setSecuritySetting] orm.Update(%v), last SQL: %v", err, sql)
		self.resp(c, 502, &CR{
			Message: T("update Failed"),
			Code:    CodeServerInternal,
		})
		return
	}

	//logout & resp success
	self.userLogout(c)
}

//==============================================================================
// data api
//==============================================================================

// @Summary getDnsRecord
// @Description get Dns Record by user query
// @Accept  json
// @Produce  json
// @Param   some_id     path    int     true        "Some ID"
// @Success 200 {string} string	"ok"
// @Failure 400 {object} CR "We need ID!!"
// @Failure 404 {object} CR "Can not find ID"
// @Failure 401 {object} CR "Can not find ID"
// @Router /testapi/get-string-by-int/{some_id} [get]
func (self *WebServer) getDnsRecord(c *gin.Context) {
	T := getTranslateFunc(c)
	ip, ipExist := c.GetQuery("ip")
	domain, domainExist := c.GetQuery("domain")
	date, dateExist := c.GetQuery("date")

	pageNo, pageNoErr := ginutils.GetQueryInt(c, "pageNo")
	if pageNoErr != nil || pageNo <= 0 {
		pageNo = 1
	}
	pageSize, pageSizeErr := ginutils.GetQueryInt(c, "pageSize")
	if pageSizeErr != nil {
		pageSize = 10
	}

	session := self.orm.NewSession()
	defer session.Close()

	role := c.GetInt("role")
	id := c.GetInt64("id")
	switch role {
	case roleAdmin, roleSuper:
		session = session.In("uid", 0, id)
	default:
		session = session.Where(`uid=?`, id)
	}

	if domainExist {
		session = session.And(`domain like ?`, "%"+domain+"%")
	}
	if ipExist {
		session = session.And(`ip like ?`, "%"+ip+"%")
	}
	if dateExist {
		t, _ := time.Parse(time.RFC3339, strings.Trim(date, `"`))
		if self.orm.DriverName() == "sqlite3" { //sqlite not support timezone
			t = t.Local()
		}
		session = session.And(`ctime > ?`, t)
		// fmt.Println("QUERYDATE=[", date, "] = ", t)
	}

	session = session.And(`hidden=false`)

	var items []models.TblDns
	count, err := session.Desc("id").Limit(pageSize, (pageNo-1)*pageSize).FindAndCount(&items)
	if err != nil {
		logrus.Errorf("[webui.go::getDnsRecord] orm.FindAndCount: %v", err)
		self.resp(c, 502, &CR{
			Message: T("Failed"),
			Code:    CodeServerInternal,
		})
		return
	}

	var resp DnsRecordResp
	resp.TotalCount = int(count)
	resp.PageSize = pageSize
	resp.PageNo = pageNo
	resp.TotalPage = (resp.TotalCount + (pageSize - 1)) / pageSize
	resp.Data = make([]models.DnsRecord, len(items))
	for i := 0; i < len(items); i++ {
		rcd := &resp.Data[i]
		item := &items[i]
		rcd.Id = item.Id
		rcd.Domain = item.Domain
		rcd.Ip = item.Ip
		rcd.Ctime = item.Ctime
	}

	self.resp(c, 200, &CR{
		Message: T("OK"),
		Result:  &resp,
	})
}

// @Summary delDnsRecord
// @Description del Dns Record by query ids
// @Accept  json
// @Produce  json
// @Param   some_id     path    int     true        "Some ID"
// @Success 200 {string} string	"ok"
// @Failure 400 {object} CR "We need ID!!"
// @Failure 404 {object} CR "Can not find ID"
// @Router /testapi/get-string-by-int/{some_id} [get]
func (self *WebServer) delDnsRecord(c *gin.Context) {
	T := getTranslateFunc(c)
	var req DeleteRecordRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		logrus.Errorf("[webui.go::delDnsRecord] orm.Delete: %v", err)
		self.resp(c, 400, &CR{
			Message: T("invalid Param"),
			Code:    CodeServerInternal,
			Error:   err,
		})
		return
	}

	var newDns models.TblDns
	newDns.Hidden = true

	session := self.orm.NewSession()
	defer session.Close()

	role := c.GetInt("role")
	id := c.GetInt64("id")

	switch role {
	case roleAdmin, roleSuper:
		if len(req.Ids) == 0 {
			_, err := session.In(`uid`, id, 0).Cols("hidden").Update(&newDns)
			if err != nil {
				logrus.Errorf("[webui.go::delDnsRecord] orm.Delete: %v", err)
				self.resp(c, 502, &CR{
					Message: T("Failed"),
					Code:    CodeServerInternal,
				})
				return
			}
			self.resp(c, 200, &CR{
				Message: T("OK"),
			})
			return
		} else {
			params := make([]interface{}, len(req.Ids))
			for i := 0; i < len(req.Ids); i++ {
				params[i] = req.Ids[i]
			}
			_, err := session.In(`uid`, id, 0).In("id", params...).Cols("hidden").Update(&newDns)
			if err != nil {
				logrus.Errorf("[webui.go::delDnsRecord] orm.Delete: %v", err)
				self.resp(c, 502, &CR{
					Message: T("Failed"),
					Code:    CodeServerInternal,
				})
				return
			}
			self.resp(c, 200, &CR{
				Message: T("OK"),
			})
			return
		}
	default:
		if len(req.Ids) == 0 {
			_, err := session.Where(`uid=?`, id).Cols("hidden").Update(&newDns)
			if err != nil {
				logrus.Errorf("[webui.go::delDnsRecord] orm.Delete: %v", err)
				self.resp(c, 502, &CR{
					Message: T("Failed"),
					Code:    CodeServerInternal,
				})
				return
			}
			self.resp(c, 200, &CR{
				Message: T("OK"),
			})
			return
		} else {
			params := make([]interface{}, len(req.Ids))
			for i := 0; i < len(req.Ids); i++ {
				params[i] = req.Ids[i]
			}
			_, err := session.Where(`uid=?`, id).In("id", params...).Cols("hidden").Update(&newDns)
			if err != nil {
				logrus.Errorf("[webui.go::delDnsRecord] orm.Delete: %v", err)
				self.resp(c, 502, &CR{
					Message: T("Failed"),
					Code:    CodeServerInternal,
				})
				return
			}
			self.resp(c, 200, &CR{
				Message: T("OK"),
			})
			return
		}
	}
}

func (self *WebServer) getHttpRecord(c *gin.Context) {
	T := getTranslateFunc(c)
	ip, ipExist := c.GetQuery("ip")
	domain, domainExist := c.GetQuery("domain")
	date, dateExist := c.GetQuery("date")

	pageNo, pageNoErr := ginutils.GetQueryInt(c, "pageNo")
	if pageNoErr != nil || pageNo <= 0 {
		pageNo = 1
	}
	pageSize, pageSizeErr := ginutils.GetQueryInt(c, "pageSize")
	if pageSizeErr != nil || pageSize <= 0 {
		pageSize = 10
	}

	ctype, ctypeExist := c.GetQuery("ctype")
	data, dataExist := c.GetQuery("data")
	method, methodExist := c.GetQuery("method")

	session := self.orm.NewSession()
	defer session.Close()

	role := c.GetInt("role")
	id := c.GetInt64("id")
	switch role {
	case roleAdmin, roleSuper:
		session = session.Where(`id>0`)
	default:
		session = session.Where(`uid=?`, id)
	}

	if domainExist {
		session = session.And(`domain like ?`, "%"+domain+"%")
	}
	if ipExist {
		session = session.And(`ip like ?`, "%"+ip+"%")
	}
	if dateExist {
		t, _ := time.Parse(time.RFC3339, strings.Trim(date, `"`))
		if self.orm.DriverName() == "sqlite3" { //sqlite不支持时区
			t = t.Local()
		}
		session = session.And(`ctime > ?`, t)
	}
	if ctypeExist {
		session = session.And(`ctype like ?`, "%"+ctype+"%")
	}
	if dataExist {
		session = session.And(`data like ?`, "%"+data+"%")
	}
	if methodExist {
		session = session.And(`method = ?`, method)
	}

	session = session.And(`hidden=false`)

	var items []models.TblHttp
	count, err := session.Desc("id").Limit(pageSize, (pageNo-1)*pageSize).FindAndCount(&items)
	if err != nil {
		logrus.Errorf("[webui.go::getHttpRecord] orm.FindAndCount: %v", err)
		self.resp(c, 502, &CR{
			Code:    CodeServerInternal,
			Message: T("Failed"),
		})
		return
	}

	var resp HttpRecordResp
	resp.TotalCount = int(count)
	resp.PageSize = pageSize
	resp.PageNo = pageNo
	resp.TotalPage = (resp.TotalCount + (pageSize - 1)) / pageSize
	resp.Data = make([]models.HttpRecord, len(items))

	for i := 0; i < len(items); i++ {
		rcd := &resp.Data[i]
		item := &items[i]
		rcd.Id = item.Id
		rcd.Path = item.Path
		rcd.Ip = item.Ip
		rcd.Ctime = item.Ctime
		rcd.Ctype = item.Ctype
		rcd.Data = item.Data
		rcd.Method = item.Method
		rcd.Ua = item.Ua
	}
	self.resp(c, 200, &CR{
		Message: T("OK"),
		Result:  &resp,
	})
}

func (self *WebServer) delHttpRecord(c *gin.Context) {
	T := getTranslateFunc(c)
	var req DeleteRecordRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		logrus.Errorf("[webui.go::delHttpRecord] orm.Delete: %v", err)
		self.resp(c, 400, &CR{
			Message: T(`invalid Param`),
			Code:    CodeServerInternal,
			Error:   err,
		})
		return
	}

	var newHttp models.TblHttp
	newHttp.Hidden = true

	session := self.orm.NewSession()
	defer session.Close()

	role := c.GetInt("role")
	id := c.GetInt64("id")

	switch role {
	case roleAdmin, roleSuper:
		if len(req.Ids) == 0 {
			_, err := session.In(`uid`, id, 0).Cols("hidden").Update(&newHttp)
			if err != nil {
				//TODO:
				logrus.Errorf("[webui.go::delHttpRecord] orm.Delete: %v", err)
				self.resp(c, 502, &CR{
					Message: T("Failed"),
					Code:    CodeServerInternal,
				})
				return
			}
			self.resp(c, 200, &CR{
				Message: T("OK"),
			})
			return
		} else {
			params := make([]interface{}, len(req.Ids))
			for i := 0; i < len(req.Ids); i++ {
				params[i] = req.Ids[i]
			}
			_, err := session.In(`uid`, id, 0).In("id", params...).Cols("hidden").Update(&newHttp)
			if err != nil {
				logrus.Errorf("[webui.go::delHttpRecord] orm.Delete: %v", err)
				self.resp(c, 502, &CR{
					Message: T("Failed"),
					Code:    CodeServerInternal,
				})
				return
			}
			self.resp(c, 200, &CR{
				Message: T("OK"),
			})
			return
		}
	default:
		if len(req.Ids) == 0 {
			_, err := session.Where(`uid=?`, id).Cols("hidden").Update(&newHttp)
			if err != nil {
				//TODO:
				logrus.Errorf("[webui.go::delHttpRecord] orm.Delete: %v", err)
				self.resp(c, 502, &CR{
					Message: fmt.Sprintf(T("Delete Record: %v"), err),
					Code:    CodeServerInternal,
				})
				return
			}
			self.resp(c, 200, &CR{
				Message: T("OK"),
			})
			return
		} else {
			params := make([]interface{}, len(req.Ids))
			for i := 0; i < len(req.Ids); i++ {
				params[i] = req.Ids[i]
			}
			_, err := session.Where(`uid=?`, id).In("id", params...).Cols("hidden").Update(&newHttp)
			if err != nil {
				logrus.Errorf("[webui.go::delHttpRecord] orm.Delete: %v", err)
				self.resp(c, 502, &CR{
					Message: T("Failed"),
					Code:    CodeServerInternal,
				})
				return
			}
			self.resp(c, 200, &CR{
				Message: T("OK"),
			})
			return
		}
	}
}

func (self *WebServer) getDnsAllRecord(c *gin.Context) {
	T := getTranslateFunc(c)
	user, userExist := c.GetQuery("user")
	ip, ipExist := c.GetQuery("ip")
	domain, domainExist := c.GetQuery("domain")
	startTime, startTimeExist := c.GetQuery("startTime")
	endTime, endTimeExist := c.GetQuery("endTime")

	pageNo, pageNoErr := ginutils.GetQueryInt(c, "pageNo")
	if pageNoErr != nil || pageNo <= 0 {
		pageNo = 1
	}
	pageSize, pageSizeErr := ginutils.GetQueryInt(c, "pageSize")
	if pageSizeErr != nil {
		pageSize = 10
	}

	session := self.orm.NewSession()
	defer session.Close()

	session = session.Table("tbl_user").Join("INNER", "tbl_dns", "tbl_user.id=tbl_dns.uid")

	if !userExist || "ALL" == user {
		// session = session.Where(`uid>0`)
		session = session.Where("tbl_user.id>0")
	} else {
		// session = session.Where(builder.Eq{
		// 	"uid": builder.Select("id").Where(builder.Eq{"name":user}).From("tbl_user"),
		// })
		session = session.Where(`tbl_user.name = ?`, user)
	}

	if domainExist {
		session = session.And(`tbl_dns.domain like ?`, "%"+domain+"%")
	}
	if ipExist {
		session = session.And(`tbl_dns.ip like ?`, "%"+ip+"%")
	}
	if startTimeExist {
		t, _ := time.Parse(time.RFC3339, strings.Trim(startTime, `"`))
		if self.orm.DriverName() == "sqlite3" { //sqlite not support timezone
			t = t.Local()
		}
		session = session.And(`tbl_dns.ctime > ?`, t)
		// fmt.Println("QUERYDATE=[", date, "] = ", t)
	}

	if endTimeExist {
		t, _ := time.Parse(time.RFC3339, strings.Trim(endTime, `"`))
		if self.orm.DriverName() == "sqlite3" { //sqlite not support timezone
			t = t.Local()
		}
		session = session.And(`tbl_dns.ctime < ?`, t)
		// fmt.Println("QUERYDATE=[", date, "] = ", t)
	}

	type UserDns struct {
		models.TblDns  `xorm:"extends"`
		models.TblUser `xorm:"extends"`
	}

	var items []UserDns
	count, err := session.Desc("tbl_dns.id").Limit(pageSize, (pageNo-1)*pageSize).FindAndCount(&items)
	if err != nil {
		logrus.Errorf("[webui.go::getDnsAllRecord] orm.FindAndCount: %v", err)
		self.resp(c, 502, &CR{
			Message: T("Failed"),
			Code:    CodeServerInternal,
		})
		return
	}

	// logrus.Errorf("getDnsAllRecord:-----: %v", count)

	var resp DnsRecordResp
	resp.TotalCount = int(count)
	resp.PageSize = pageSize
	resp.PageNo = pageNo
	resp.TotalPage = (resp.TotalCount + (pageSize - 1)) / pageSize
	resp.Data = make([]models.DnsRecord, len(items))
	for i := 0; i < len(items); i++ {
		rcd := &resp.Data[i]
		item := &items[i]
		rcd.Id = item.TblDns.Id
		rcd.Domain = item.TblDns.Domain
		rcd.Ip = item.TblDns.Ip
		rcd.Ctime = item.TblDns.Ctime
		rcd.Username = item.TblUser.Name
		rcd.Company = item.TblUser.Company
		rcd.FullName = item.TblUser.FullName
	}

	self.resp(c, 200, &CR{
		Message: T("OK"),
		Result:  &resp,
	})
}

func (self *WebServer) delAllDnsRecord(c *gin.Context) {
	T := getTranslateFunc(c)
	var req DeleteRecordRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		logrus.Errorf("[webui.go::delAllDnsRecord] orm.Delete: %v", err)
		self.resp(c, 400, &CR{
			Message: T("invalid Param"),
			Code:    CodeServerInternal,
			Error:   err,
		})
		return
	}

	session := self.orm.NewSession()
	defer session.Close()

	if len(req.Ids) == 0 {
		_, err := session.Where(`uid>0`).Delete(&models.TblDns{})
		if err != nil {
			logrus.Errorf("[webui.go::delAllDnsRecord] orm.Delete: %v", err)
			self.resp(c, 502, &CR{
				Message: T("Failed"),
				Code:    CodeServerInternal,
			})
			return
		}
		self.resp(c, 200, &CR{
			Message: T("OK"),
		})
		return
	} else {
		params := make([]interface{}, len(req.Ids))
		for i := 0; i < len(req.Ids); i++ {
			params[i] = req.Ids[i]
		}
		_, err := session.Where(`uid>0`).In("id", params...).Delete(&models.TblDns{})
		if err != nil {
			logrus.Errorf("[webui.go::delAllDnsRecord] orm.Delete: %v", err)
			self.resp(c, 502, &CR{
				Message: T("Failed"),
				Code:    CodeServerInternal,
			})
			return
		}
		self.resp(c, 200, &CR{
			Message: T("OK"),
		})
		return
	}

}

func (self *WebServer) getAllHttpRecord(c *gin.Context) {
	T := getTranslateFunc(c)
	user, userExist := c.GetQuery("user")
	ip, ipExist := c.GetQuery("ip")
	domain, domainExist := c.GetQuery("domain")
	startTime, startTimeExist := c.GetQuery("startTime")
	endTime, endTimeExist := c.GetQuery("endTime")

	pageNo, pageNoErr := ginutils.GetQueryInt(c, "pageNo")
	if pageNoErr != nil || pageNo <= 0 {
		pageNo = 1
	}
	pageSize, pageSizeErr := ginutils.GetQueryInt(c, "pageSize")
	if pageSizeErr != nil || pageSize <= 0 {
		pageSize = 10
	}

	ctype, ctypeExist := c.GetQuery("ctype")
	data, dataExist := c.GetQuery("data")
	method, methodExist := c.GetQuery("method")

	session := self.orm.NewSession()
	defer session.Close()

	session = session.Table("tbl_user").Join("INNER", "tbl_http", "tbl_user.id=tbl_http.uid")

	if !userExist || "ALL" == user {
		// session = session.Where(`uid>0`)
		session = session.Where("tbl_user.id>0")
	} else {
		// session = session.Where(builder.Eq{
		// 	"uid": builder.Select("id").Where(builder.Eq{"name":user}).From("tbl_user"),
		// })
		session = session.Where(`tbl_user.name = ?`, user)
	}

	if domainExist {
		session = session.And(`tbl_http.domain like ?`, "%"+domain+"%")
	}
	if ipExist {
		session = session.And(`tbl_http.ip like ?`, "%"+ip+"%")
	}
	if startTimeExist {
		t, _ := time.Parse(time.RFC3339, strings.Trim(startTime, `"`))
		if self.orm.DriverName() == "sqlite3" { //sqlite不支持时区
			t = t.Local()
		}
		session = session.And(`tbl_http.ctime > ?`, t)
	}

	if endTimeExist {
		t, _ := time.Parse(time.RFC3339, strings.Trim(endTime, `"`))
		if self.orm.DriverName() == "sqlite3" { //sqlite不支持时区
			t = t.Local()
		}
		session = session.And(`tbl_http.ctime < ?`, t)
	}

	if ctypeExist {
		session = session.And(`tbl_http.ctype like ?`, "%"+ctype+"%")
	}
	if dataExist {
		session = session.And(`tbl_http.data like ?`, "%"+data+"%")
	}
	if methodExist {
		session = session.And(`tbl_http.method = ?`, method)
	}

	type UserHttp struct {
		models.TblHttp `xorm:"extends"`
		models.TblUser `xorm:"extends"`
	}

	var items []UserHttp
	count, err := session.Desc("tbl_http.id").Limit(pageSize, (pageNo-1)*pageSize).FindAndCount(&items)
	if err != nil {
		logrus.Errorf("[webui.go::getAllHttpRecord] orm.FindAndCount: %v", err)
		self.resp(c, 502, &CR{
			Code:    CodeServerInternal,
			Message: T("Failed"),
		})
		return
	}

	var resp HttpRecordResp
	resp.TotalCount = int(count)
	resp.PageSize = pageSize
	resp.PageNo = pageNo
	resp.TotalPage = (resp.TotalCount + (pageSize - 1)) / pageSize
	resp.Data = make([]models.HttpRecord, len(items))

	for i := 0; i < len(items); i++ {
		rcd := &resp.Data[i]
		item := &items[i]
		rcd.Id = item.TblHttp.Id
		rcd.Path = item.TblHttp.Path
		rcd.Ip = item.TblHttp.Ip
		rcd.Ctime = item.TblHttp.Ctime
		rcd.Ctype = item.TblHttp.Ctype
		rcd.Data = item.TblHttp.Data
		rcd.Method = item.TblHttp.Method
		rcd.Ua = item.TblHttp.Ua
		rcd.Username = item.TblUser.Name
		rcd.Company = item.TblUser.Company
		rcd.FullName = item.TblUser.FullName
	}
	self.resp(c, 200, &CR{
		Message: T("OK"),
		Result:  &resp,
	})
}

func (self *WebServer) delAllHttpRecord(c *gin.Context) {
	T := getTranslateFunc(c)
	var req DeleteRecordRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		logrus.Errorf("[webui.go::delAllHttpRecord] orm.Delete: %v", err)
		self.resp(c, 400, &CR{
			Message: T(`invalid Param`),
			Code:    CodeServerInternal,
			Error:   err,
		})
		return
	}

	session := self.orm.NewSession()
	defer session.Close()

	if len(req.Ids) == 0 {
		_, err := session.Where(`uid>0`).Delete(&models.TblHttp{})
		if err != nil {
			//TODO:
			logrus.Errorf("[webui.go::delAllHttpRecord] orm.Delete: %v", err)
			self.resp(c, 502, &CR{
				Message: fmt.Sprintf(T("Delete Record: %v"), err),
				Code:    CodeServerInternal,
			})
			return
		}
		self.resp(c, 200, &CR{
			Message: T("OK"),
		})
		return
	} else {
		params := make([]interface{}, len(req.Ids))
		for i := 0; i < len(req.Ids); i++ {
			params[i] = req.Ids[i]
		}
		_, err := session.Where(`uid>0`).In("id", params...).Delete(&models.TblHttp{})
		if err != nil {
			logrus.Errorf("[webui.go::delAllHttpRecord] orm.Delete: %v", err)
			self.resp(c, 502, &CR{
				Message: T("Failed"),
				Code:    CodeServerInternal,
			})
			return
		}
		self.resp(c, 200, &CR{
			Message: T("OK"),
		})
		return
	}
}

type countRecordResult struct {
	Count int64 `xorm:"count"`
	Uid   int64 `xorm:"uid"`
}

func (self *WebServer) countRecord(recordType string) map[int64]int64 {
	sessionCount := self.orm.NewSession()

	item := &countRecordResult{}

	var sql string
	if recordType == "DNS" {
		sql = "select `uid`,count(*) as count from tbl_dns group by `uid`"
	} else {
		sql = "select `uid`,count(*) as count from tbl_http group by `uid`"
	}

	items, err := sessionCount.SQL(sql).Rows(item)
	defer items.Close()

	if err != nil {
		logrus.Errorf("err:", err.Error())
	}

	var result map[int64]int64

	result = make(map[int64]int64)

	for items.Next() {
		err := items.Scan(item)
		if err != nil {
			logrus.Errorf("err:", err.Error())
		} else {
			result[item.Uid] = item.Count
		}
	}

	return result
}

func (self *WebServer) countAllRecord(c *gin.Context) {
	T := getTranslateFunc(c)

	pageNo, pageNoErr := ginutils.GetQueryInt(c, "pageNo")
	if pageNoErr != nil || pageNo <= 0 {
		pageNo = 1
	}
	pageSize, pageSizeErr := ginutils.GetQueryInt(c, "pageSize")
	if pageSizeErr != nil {
		pageSize = 10
	}

	session := self.orm.NewSession()
	defer session.Close()

	session = session.Where(`id>0`)
	var items []models.TblUser
	count, err := session.Limit(pageSize, (pageNo-1)*pageSize).FindAndCount(&items)
	if err != nil {
		self.resp(c, 502, &CR{
			Code:    CodeServerInternal,
			Message: T("Failed"),
		})
		return
	}

	countDnsItem := self.countRecord("DNS")
	countHttpItem := self.countRecord("HTTP")

	var resp UserListResp
	resp.TotalCount = int(count)
	resp.PageSize = pageSize
	resp.PageNo = pageNo
	resp.TotalPage = (resp.TotalCount + (pageSize - 1)) / pageSize
	resp.Data = make([]models.UserInfo, len(items))
	for i := 0; i < len(items); i++ {
		rcd := &resp.Data[i]
		item := &items[i]
		rcd.Id = item.Id
		rcd.Name = item.Name
		rcd.ShortId = item.ShortId
		rcd.Email = item.Email
		rcd.Company = item.Company
		rcd.FullName = item.FullName
		rcd.Utime = item.Utime
		rcd.Role = parseRole(item.Role)
		if value, ok := countDnsItem[item.Id]; ok {
			rcd.DnsCount = value
		} else {
			rcd.DnsCount = 0
		}

		if value, ok := countHttpItem[item.Id]; ok {
			rcd.HttpCount = value
		} else {
			rcd.HttpCount = 0
		}
	}

	self.resp(c, 200, &CR{
		Message: T("OK"),
		Result:  &resp,
	})
}

// POST
// TYPE=[CNAME/A/MX/TXT], HOST={}, RECORD={}, TTL={}
func (self *WebServer) getResolveRecord(c *gin.Context) {
	T := getTranslateFunc(c)
	pageNo, pageNoErr := ginutils.GetQueryInt(c, "pageNo")
	if pageNoErr != nil || pageNo <= 0 {
		pageNo = 1
	}
	pageSize, pageSizeErr := ginutils.GetQueryInt(c, "pageSize")
	if pageSizeErr != nil || pageSize <= 0 {
		pageSize = 10
	}
	field, fieldExist := c.GetQuery("sortField")
	order, _ := c.GetQuery("sortOrder")

	host, hostExist := c.GetQuery("keyword")
	orm := self.orm
	session := orm.NewSession()
	defer session.Close()
	session = session.Where(`id>0`)
	if hostExist {
		session = session.And(`host like ?`, "%"+host+"%")
	}
	if fieldExist {
		if field == "timestamp" {
			field = "utime"
		}

		if order != "ascend" {
			session = session.Desc(field)
		} else {
			session = session.Asc(field)
		}
	}

	var resolves []models.TblResolve
	count, err := session.Limit(pageSize, (pageNo-1)*pageSize).FindAndCount(&resolves)
	if err != nil {
		logrus.Errorf("[webui.go::delResolveRecord] orm.Delete: %v", err)
		self.resp(c, 502, &CR{
			Message: T("Failed"),
			Code:    CodeServerInternal,
		})
		return
	}
	var ret ResolveResult
	ret.Pagination.PageNo = pageNo
	ret.Pagination.PageSize = pageSize
	ret.Pagination.TotalCount = int(count)
	ret.Pagination.TotalPage = (int(count) + pageSize - 1) / pageSize
	for i := 0; i < len(resolves); i++ {
		r := &resolves[i]
		ret.Data = append(ret.Data, &ResolveItem{
			Id:         r.Id,
			Type:       r.Type,
			Host:       r.Host,
			Value:      r.Value,
			Ttl:        r.Ttl,
			Utimestamp: r.Utime.Unix(),
		})
	}
	c.JSON(200, &ret)
}

func (self *WebServer) updateResolveCache(host, rType string, all []models.TblResolve) {
	if rType != "" {
		var rr []*Resolve
		for i := 0; i < len(all); i++ {
			rcd := &all[i]
			r := &Resolve{
				Type:  rcd.Type,
				Name:  rcd.Host,
				Value: rcd.Value,
				Ttl:   rcd.Ttl,
			}
			rr = append(rr, r)
		}
		key := fmt.Sprintf("%v#%v", host, rType)
		self.store.Delete(key)

		if len(rr) > 0 {
			self.store.Set(key, rr, cache.NoExpiration)
		}
		return
	}

	var rrMap = make(map[string][]*Resolve)
	for i := 0; i < len(all); i++ {
		rcd := &all[i]
		r := &Resolve{
			Type:  rcd.Type,
			Name:  rcd.Host,
			Value: rcd.Value,
			Ttl:   rcd.Ttl,
		}
		if rr, exist := rrMap[rcd.Type]; exist {
			rrMap[rcd.Type] = append(rr, r)
		} else {
			rrMap[rcd.Type] = []*Resolve{r}
		}
	}
	// clear old cache
	for _, t := range []string{"A", "TXT", "CNAME", "SRV", "NS", "MX"} {
		key := fmt.Sprintf("%v#%v", host, t)
		self.store.Delete(key)
	}

	for k, v := range rrMap {
		key := fmt.Sprintf("%v#%v", host, k)
		self.store.Set(key, v, cache.NoExpiration)
	}
	return
}

func (self *WebServer) setResolveRecord(c *gin.Context) {
	T := getTranslateFunc(c)

	self.uiMutex.Lock()
	defer self.uiMutex.Unlock()

	var rcd models.Resolve
	err := c.ShouldBindJSON(&rcd)
	if err != nil {
		logrus.Errorf("[webui.go::setResolveRecord] orm.Delete: %v", err)
		self.resp(c, 400, &CR{
			Message: T("Failed"),
			Code:    CodeServerInternal,
		})
		return
	}
	session := self.orm.NewSession()
	defer session.Close()

	// verify input
	if rcd.Host == "" || len(rcd.Host) > 128 ||
		rcd.Value == "" || len(rcd.Value) > 128 {
		self.resp(c, 400, &CR{
			Message: T("Bad Resolve Data"),
			Code:    CodeServerInternal,
		})
		return
	}

	// branch 1: create new
	if rcd.Id == 0 {
		// check confilct
		var rcds []models.TblResolve
		err = session.Where(`host = ?`, rcd.Host).Find(&rcds)
		if err != nil {
			logrus.Errorf("[webui.go::setResolveRecord] orm.Find: %v", err)
			self.resp(c, 502, &CR{
				Message: T("Failed"),
				Code:    CodeServerInternal,
			})
			return
		}

		// check conflict by type
		conflict, groups := models.Resolves(rcds).GetTypeConflict(&rcd)
		if conflict != nil {
			logrus.Errorf("[webui.go::setResolveRecord] record(%v) conflict with (%v)",
				rcd.Type, conflict.Type)
			self.resp(c, 400, &CR{
				Message: fmt.Sprintf(T("Type:(%v) conflict with %v"),
					rcd.Type, conflict.Type),
				Code: CodeServerInternal,
			})
			return
		} //1.24  3.30

		// check conflict by value, 不允许插入相同值
		if len(groups) >= 1 {
			sort.Sort(models.Resolves(groups))
			conflictV := groups.GetValueConflict(&rcd)
			if conflictV != nil {
				self.resp(c, 400, &CR{
					Message: fmt.Sprintf(T("Value(%v) already exists"), rcd.Value),
					Code:    CodeServerInternal,
				})
				return
			}
		}

		// insert into orm
		_, err = session.InsertOne(&models.TblResolve{
			Type:  rcd.Type,
			Host:  rcd.Host,
			Value: rcd.Value,
			Ttl:   rcd.Ttl,
		})
		if err != nil {
			logrus.Errorf("[webui.go::setResolveRecord] orm.InsertOne: %v", err)
			self.resp(c, 502, &CR{
				Message: fmt.Sprintf(T("Insert record: %v"), err),
				Code:    CodeServerInternal,
			})
			return
		}
		// update cache by type
		groups = append(groups, models.TblResolve{
			Type:  rcd.Type,
			Host:  rcd.Host,
			Value: rcd.Value,
			Ttl:   rcd.Ttl,
		})
		self.updateResolveCache(rcd.Host, rcd.Type, groups)

		c.JSON(200, CR{
			Message: T("OK"),
		})
		return
	}

	// branch 2: modify exist
	// check confilct
	var oldRcd models.TblResolve
	exist, err := session.ID(rcd.Id).Get(&oldRcd)
	if err != nil {
		logrus.Errorf("[webui.go::setResolveRecord] orm.Get: %v", err)
		self.resp(c, 502, &CR{
			Message: fmt.Sprintf(T("Query record: %v"), err),
			Code:    CodeServerInternal,
		})
		return
	} else if !exist {
		self.resp(c, 400, &CR{
			Message: fmt.Sprintf(T("Record not exist"), err),
			Code:    CodeServerInternal,
		})
		return
	} else if oldRcd.Host != rcd.Host || oldRcd.Type != rcd.Type {
		self.resp(c, 400, &CR{
			Message: fmt.Sprintf(T("Can't Change Host/Type"), err),
			Code:    CodeServerInternal,
		})
		return
	}

	var rcds []models.TblResolve
	err = session.Where(`host = ?`, rcd.Host).
		And(`type = ?`, rcd.Type).
		And(`id != ?`, oldRcd.Id).
		Find(&rcds)
	if err != nil {
		logrus.Errorf("[webui.go::setResolveRecord] orm.Find: %v", err)
		self.resp(c, 502, &CR{
			Message: fmt.Sprintf(T("Query Record: %v"), err),
			Code:    CodeServerInternal,
		})
		return
	} else if len(rcds) > 0 { // not exist groups
		// 1. conflict check by type
		// ... skip

		// 2. conflict check by value, distinct, 不允许将记录修改为已有值
		sort.Sort(models.Resolves(rcds))

		conflictV := models.Resolves(rcds).GetValueConflict(&rcd)
		if conflictV != nil {
			self.resp(c, 400, &CR{
				Message: fmt.Sprintf(T("Value(%v) already exists"), rcd.Value),
				Code:    CodeServerInternal,
			})
			return
		}
	}

	oldRcd.Ttl = rcd.Ttl
	oldRcd.Value = rcd.Value
	_, err = session.ID(oldRcd.Id).Cols("ttl", "value").Update(&oldRcd)
	if err != nil {
		logrus.Errorf("[webui.go::setResolveRecord] orm.InsertOne: %v", err)
		self.resp(c, 502, &CR{
			Message: fmt.Sprintf(T("Update record: %v"), err),
			Code:    CodeServerInternal,
		})
		return
	}

	// update cache by type
	rcds = append(rcds, models.TblResolve{
		Type:  rcd.Type,
		Host:  rcd.Host,
		Value: rcd.Value,
		Ttl:   rcd.Ttl,
	})

	// update cache
	self.updateResolveCache(rcd.Host, rcd.Type, rcds)

	c.JSON(200, CR{
		Message: T("OK"),
	})
}

// DELETE
func (self *WebServer) delResolveRecord(c *gin.Context) {
	T := getTranslateFunc(c)
	id, err := ginutils.GetQueryInt64(c, "id")
	if err != nil {
		logrus.Errorf("[webui.go::delResolveRecord] not input id")
		self.resp(c, 502, &CR{
			Message: T("parameter id is required"),
			Code:    CodeServerInternal,
		})
		return
	}
	orm := self.orm
	session := orm.NewSession()
	defer session.Close()

	var rcd models.TblResolve
	exist, err := session.ID(id).Get(&rcd)
	if err != nil {
		logrus.Errorf("[webui.go::delResolveRecord] orm.Delete: %v", err)
		self.resp(c, 502, &CR{
			Message: fmt.Sprintf(T("delete: %v"), err),
			Code:    CodeServerInternal,
		})
		return
	} else if !exist {
		logrus.Errorf("[webui.go::delResolveRecord] orm.Delete: %v", err)
		self.resp(c, 400, &CR{
			Message: fmt.Sprintf(T("Delete resolve(%v), but not exist"), rcd.Id),
			Code:    CodeServerInternal,
		})
		return
	}

	_, err = session.ID(id).Delete(&models.TblResolve{})
	if err != nil {
		logrus.Errorf("[webui.go::delResolveRecord] orm.Delete: %v", err)
		self.resp(c, 502, &CR{
			Message: fmt.Sprintf(T("delete: %v"), err),
			Code:    CodeServerInternal,
		})
		return
	}

	// delete -> update cache
	var all []models.TblResolve
	session.Where(`host=?`, rcd.Host).Find(&all)
	self.updateResolveCache(rcd.Host, rcd.Type, all)

	c.JSON(200, CR{
		Message: T("OK"),
	})
}

func getTranslateFunc(c *gin.Context) func(id string) string {
	lang := c.GetHeader("Language")
	return func(id string) string {
		return translateByLang(lang, id)
	}
}
