package router

import (
	"github.com/gin-gonic/gin"

	"video-captions/internal/controller"
)

// RegisterSettingRouter 注册运行时配置路由
func RegisterSettingRouter(r *gin.Engine) {
	settingCtl := controller.NewSettingController()

	api := r.Group("/api/v1/settings")
	{
		api.GET("", settingCtl.GetSettings)
		api.PUT("", settingCtl.UpdateSettings)
	}
}
