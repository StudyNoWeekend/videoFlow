package router

import (
	"github.com/gin-gonic/gin"

	"video-captions/internal/controller"
)

// RegisterHealthRouter 注册健康检查路由，必须不被鉴权中间件拦截
func RegisterHealthRouter(r *gin.Engine) {
	healthCtl := controller.NewHealthController()
	// 根路径健康检查
	r.GET("/health", healthCtl.Health)
	r.GET("/ready", healthCtl.Ready)

	// API 路径下的健康检查（可选，保持兼容性）
	api := r.Group("/api/v1")
	{
		api.GET("/health", healthCtl.Health)
		api.GET("/ready", healthCtl.Ready)
	}
}
