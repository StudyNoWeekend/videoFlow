package req

// InitReq 系统初始化请求
// 用户名：3-64 位，仅限字母+数字；密码：6-64 位，须含字母+数字（组合规则在 logic 层校验）
type InitReq struct {
	Username        string `json:"username" binding:"required,min=3,max=64,alphanum"`
	Password        string `json:"password" binding:"required,min=6,max=64"`
	ConfirmPassword string `json:"confirm_password" binding:"required,eqfield=Password"`
}

// LoginPwdReq 密码登录请求。
// 登录不校验用户名 min（兼容历史账号），仅限制 max 防止超长输入打满 bcrypt
type LoginPwdReq struct {
	Username string `json:"username" binding:"required,max=64"`
	Password string `json:"password" binding:"required,max=64"`
}

// ResetPasswordReq 重置密码请求（通过重置令牌）
type ResetPasswordReq struct {
	Token           string `json:"token" binding:"required,max=128"`
	NewPassword     string `json:"new_password" binding:"required,min=6,max=64"`
	ConfirmPassword string `json:"confirm_password" binding:"required,eqfield=NewPassword"`
}

// ChangePwdReq 修改密码请求（需登录）
type ChangePwdReq struct {
	OldPassword     string `json:"old_password" binding:"required,max=64"`
	NewPassword     string `json:"new_password" binding:"required,min=6,max=64"`
	ConfirmPassword string `json:"confirm_password" binding:"required,eqfield=NewPassword"`
}

// GenerateResetTokenReq 触发生成重置令牌请求（通过接口）
type GenerateResetTokenReq struct {
	Username string `json:"username" binding:"required,max=64"`
}
