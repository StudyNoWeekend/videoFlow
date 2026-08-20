package router

import (
	"github.com/gin-gonic/gin"

	"video-captions/internal/controller"
)

// RegisterSettingRouter 注册运行时配置路由（需在鉴权分组下调用）
func RegisterSettingRouter(rg *gin.RouterGroup) {
	settingCtl := controller.NewSettingController()

	api := rg.Group("/settings")
	{
		api.GET("", settingCtl.GetSettings)
		api.PUT("", settingCtl.UpdateSettings)
	}
}
