package controller

import (
	"github.com/gin-gonic/gin"

	"video-captions/enum"
	"video-captions/internal/dto/req"
	"video-captions/internal/logic"
	"video-captions/utils/response"
)

// TelegramController Telegram 登录与状态控制器
type TelegramController struct {
	telegramLogic *logic.TelegramLogic
}

// NewTelegramController 创建 Telegram 控制器
func NewTelegramController() *TelegramController {
	return &TelegramController{
		telegramLogic: logic.NewTelegramLogic(),
	}
}

// Status 查询 Telegram 连接与登录状态
// GET /api/v1/telegram/status
func (ctl *TelegramController) Status(c *gin.Context) {
	response.Success(c, ctl.telegramLogic.Status(c.Request.Context()))
}

// StartQRLogin 发起扫码登录
// POST /api/v1/telegram/login/qr
func (ctl *TelegramController) StartQRLogin(c *gin.Context) {
	res, err := ctl.telegramLogic.StartQRLogin(c.Request.Context())
	if err != nil {
		HandleError(c, err)
		return
	}
	response.Success(c, res)
}

// QRCode 获取当前登录二维码图片
// GET /api/v1/telegram/login/qr
func (ctl *TelegramController) QRCode(c *gin.Context) {
	res, err := ctl.telegramLogic.QRCode(c.Request.Context())
	if err != nil {
		HandleError(c, err)
		return
	}
	response.Success(c, res)
}

// Submit2FA 提交两步验证密码
// POST /api/v1/telegram/login/2fa
func (ctl *TelegramController) Submit2FA(c *gin.Context) {
	var submitReq req.TelegramLogin2FAReq
	if err := c.ShouldBindJSON(&submitReq); err != nil {
		response.FailByBizError(c, enum.ErrInvalidParam)
		return
	}

	if err := ctl.telegramLogic.Submit2FA(c.Request.Context(), submitReq.Password); err != nil {
		HandleError(c, err)
		return
	}
	response.Success(c, nil)
}

// Logout 退出登录
// POST /api/v1/telegram/logout
func (ctl *TelegramController) Logout(c *gin.Context) {
	if err := ctl.telegramLogic.Logout(c.Request.Context()); err != nil {
		HandleError(c, err)
		return
	}
	response.Success(c, nil)
}
