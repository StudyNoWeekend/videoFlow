package res

// AuthStatusRes 认证状态响应
type AuthStatusRes struct {
	Initialized bool `json:"initialized"`
}

// UserInfo 用户基本信息
type UserInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

// LoginRes 登录/初始化成功响应
type LoginRes struct {
	Token string   `json:"token"`
	User  UserInfo `json:"user"`
}
