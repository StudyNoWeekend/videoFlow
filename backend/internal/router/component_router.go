package router

import (
	"github.com/gin-gonic/gin"

	"video-captions/internal/controller"
)

// RegisterComponentRouter 注册组件管理路由（需在鉴权分组下调用）
func RegisterComponentRouter(rg *gin.RouterGroup) {
	componentCtl := controller.NewComponentController()

	api := rg.Group("/components")
	{
		api.GET("", componentCtl.ListComponents)
		api.GET("/active-session/:component_type", componentCtl.GetActiveSession)
		api.GET("/install/history/:component_type", componentCtl.GetInstallHistory)
		api.POST("/install", componentCtl.InstallComponent)
		api.POST("/reinstall", componentCtl.ReinstallComponent)
		api.POST("/uninstall", componentCtl.UninstallComponent)
	}
}

// RegisterComponentProgressRouter 注册组件安装进度 SSE 路由（公开接口，EventSource 不支持自定义 header）
func RegisterComponentProgressRouter(r *gin.Engine) {
	componentCtl := controller.NewComponentController()

	r.GET("/api/v1/components/install/progress/:session_id", componentCtl.InstallProgress)
}
