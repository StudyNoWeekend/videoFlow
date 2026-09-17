package res

// TelegramStatusRes Telegram 连接与登录状态
type TelegramStatusRes struct {
	// Configured 是否已配置 api_id/app_hash
	Configured bool `json:"configured"`
	// Ready 是否已与 Telegram 建立连接
	Ready bool `json:"ready"`
	// Authenticated 是否已登录账号
	Authenticated bool `json:"authenticated"`
	// Account 已登录账号的展示名
	Account string `json:"account,omitempty"`
	// LoginStatus 扫码登录状态：空串/pending/need_2fa/success/error
	LoginStatus string `json:"login_status"`
	// QRURL 扫码登录二维码内容（tg://login?token=...）
	QRURL string `json:"qr_url,omitempty"`
	// Error 登录过程中的错误信息
	Error string `json:"error,omitempty"`
}

// TelegramQRRes 扫码登录二维码
type TelegramQRRes struct {
	// QRURL 二维码承载的登录链接（tg://login?token=...）
	QRURL string `json:"qr_url"`
	// QRImage 二维码图片，data URI 形式，可直接用于 img 标签
	QRImage string `json:"qr_image"`
}
