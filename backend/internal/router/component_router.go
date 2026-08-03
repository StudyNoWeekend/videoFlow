package router

import (
	"github.com/gin-gonic/gin"

	"video-captions/internal/controller"
)

// RegisterComponentRouter 注册组件管理路由
func RegisterComponentRouter(r *gin.Engine) {
	componentCtl := controller.NewComponentController()

	api := r.Group("/api/v1/components")
	{
		api.GET("", componentCtl.ListComponents)
		api.GET("/active-session/:component_type", componentCtl.GetActiveSession)
		api.GET("/install/history/:component_type", componentCtl.GetInstallHistory)
		api.POST("/install", componentCtl.InstallComponent)
		api.POST("/reinstall", componentCtl.ReinstallComponent)
		api.POST("/uninstall", componentCtl.UninstallComponent)
		api.GET("/install/progress/:session_id", componentCtl.InstallProgress)
	}
}
