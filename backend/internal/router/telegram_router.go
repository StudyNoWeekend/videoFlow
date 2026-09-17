package router

import (
	"github.com/gin-gonic/gin"

	"video-captions/internal/controller"
)

// RegisterTelegramRouter 注册 Telegram 登录与状态路由（需在鉴权分组下调用）
func RegisterTelegramRouter(rg *gin.RouterGroup) {
	tgCtl := controller.NewTelegramController()

	api := rg.Group("/telegram")
	{
		api.GET("/status", tgCtl.Status)
		api.POST("/login/qr", tgCtl.StartQRLogin)
		api.GET("/login/qr", tgCtl.QRCode)
		api.POST("/login/2fa", tgCtl.Submit2FA)
		api.POST("/logout", tgCtl.Logout)
	}
}
