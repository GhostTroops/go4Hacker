package models

import (
	"time"
)

//==============================================================================
// api models
//==============================================================================
const (
	CodeOK             = 0
	CodeBadPermission  = 1
	CodeBadData        = 2
	CodeNoAuth         = 3
	CodeNoPermission   = 4
	CodeServerInternal = 5
	CodeNoData         = 6
	CodeExpire         = 7
	VerifycodeErr      = 8

	RoleSuper  = 0
	RoleAdmin  = 1
	RoleNormal = 2
	RoleGuest  = 3

	GODNS_RFI_KEY   = "GODNSLOG"
	GODNS_RFI_VALUE = "694ef536e5d0245f203a1bcf8cbf3294" // md5sum($GODNS_RFI_KEY)
)

// 登陆请求
type LoginRequest struct {
	Email      string `json:"email"`      // 信箱
	Username   string `json:"username"`   // 用户名
	Password   string `json:"password"`   // 密码
	CaptchaId  string `json:"captcha_id"` // 验证码
	Verifycode string `json:"verifycode"` // 校验码
}

// 登陆响应
type LoginResponse struct {
	Islogin  bool   `json:"isLogin"`  // 是否成功登录
	Token    string `json:"token"`    // token
	Username string `json:"username"` // 用户名
	RoleId   string `json:"roleId"`   // 角色id
	Lang     string `json:"lang"`     // 语言
}

// 角色
type Role struct {
	Id          string       `json:"id"`          // 角色id
	Name        string       `json:"name"`        // 角色名
	Description string       `json:"description"` // 角色描述
	Permissions []Permission `json:"permissions"` //角色权限
}

// 授权设置
type PermissionActionSet struct {
	Action       string `json:"action"`       // 授权内容、操作、行为
	Description  string `json:"description"`  //描述
	DefaultCheck bool   `json:"defaultCheck"` // 默认选择
}

// 授权
type Permission struct {
	RoleId          int                   `json:"roleId"`          // 角色id、
	PermissionId    string                `json:"permissionId"`    // 授权id
	PermissionName  string                `json:"permissionName"`  // 授权名
	ActionEntitySet []PermissionActionSet `json:"ActionEntitySet"` // 授权、行为、动作
}

// 用户信息
type UserInfo struct {
	Id        int64     `json:"id"`         // 唯一ID、
	Name      string    `json:"username"`   // 用户名
	Email     string    `json:"email"`      // 信箱
	Company   string    `json:"company"`    // 公司名
	FullName  string    `json:"full_name"`  // 全名
	ShortId   string    `json:"short_id"`   // 短名
	Avatar    string    `json:"avatar"`     // 头像
	Language  string    `json:"lang"`       // 语言
	Role      Role      `json:"role"`       // 角色
	Utime     time.Time `json:"utime"`      // 时间
	DnsCount  int64     `json:"dns_count"`  // dns 次数
	HttpCount int64     `json:"http_count"` // http请求次数
}

// 用户请求
type UserRequest struct {
	Id       int64  `json:"id"`
	Name     string `json:"username"`
	Email    string `json:"email"`
	Company  string `json:"company"`
	FullName string `json:"full_name"`
	ShortId  string `json:"short_id"`
	Password string `json:"password"`
	Role     int    `json:"role"`
	Language string `json:"lang"`
}

// dns记录
type DnsRecordResp struct {
	Pagination
	Data []DnsRecord `json:"data"`
}

// http记录
type HttpRecordResp struct {
	Pagination
	Data []HttpRecord `json:"data"`
}

// 用户列表
type UserListResp struct {
	Pagination
	Data []UserInfo `json:"data"`
}

// 应用配置
type AppSetting struct {
	Callback  string   `json:"callback"`  // XSS 回调
	CleanHour int64    `json:"cleanHour"` // 清除时间
	Rebind    []string `json:"rebind"`    // 重新绑定
}

// 删除记录请求
type DeleteRecordRequest struct {
	Ids []int64 `json:"ids"`
}

// 应用安全
type AppSecurity struct {
	Token    string `json:"token"`     // token
	DnsAddr  string `json:"dns_addr"`  //dns地址
	HttpAddr string `json:"http_addr"` // http 地址
}

// 应用安全设置
type AppSecuritySet struct {
	Password string `json:"password"` // 密码
}

// dns记录
type DnsRecord struct {
	Id       int64     `json:"id,omitempty"`
	Uid      int64     `json:"-"`
	Callback string    `json:"-"`
	Var      string    `json:"-"`
	Domain   string    `json:"domain"`
	Ip       string    `json:"addr"`
	Ctime    time.Time `json:"ctime"`
	Username string    `json:"username"`
	Company  string    `json:"company"`
	FullName string    `json:"full_name"`
}

// http 记录
type HttpRecord struct {
	Id       int64     `json:"id,omitempty"`
	Uid      int64     `json:"-"`
	Callback string    `json:"-"`
	Path     string    `json:"path"`
	Ip       string    `json:"addr"`
	Method   string    `json:"method"`
	Data     string    `json:"data"`
	Ctype    string    `json:"ctype"`
	Ua       string    `json:"ua"`
	Ctime    time.Time `json:"ctime"`
	Username string    `json:"username"`
	Company  string    `json:"company"`
	FullName string    `json:"full_name"`
}

// 公共的响应数据 commone response
type CR struct {
	Message   string      `json:"message"`          // 消息
	Code      int         `json:"code"`             // 状态码
	Error     error       `json:"error,omitempty"`  // 错误消息
	Timestamp int64       `json:"timestemp"`        // 时间戳
	Result    interface{} `json:"result,omitempty"` // 结果
}

// 分页
type Pagination struct {
	PageNo     int `json:"pageNo"`     // 分页号
	PageSize   int `json:"pageSize"`   // 每页size
	TotalCount int `json:"totalCount"` // 总记录数据
	TotalPage  int `json:"totalPage"`  // 总页数
}

// 解决, 解析数据
type Resolve struct {
	Id         int64  `json:"id,omitempty"`
	Host       string `json:"host"` // host record, eg. www
	Type       string `json:"type"` // record type, eg. CNAME/A/MX/TXT/SRV/NS.
	Value      string `json:"value"`
	Ttl        uint32 `json:"ttl"`
	Utimestamp int64  `json:"timestamp"`
}
