package req

// TelegramLogin2FAReq 提交两步验证密码请求
type TelegramLogin2FAReq struct {
	Password string `json:"password" binding:"required,max=256"`
}
