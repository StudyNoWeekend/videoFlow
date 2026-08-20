package router

import (
	"github.com/gin-gonic/gin"

	"video-captions/internal/controller"
)

// RegisterVersionRouter 注册版本号路由（公开接口，无需认证）
func RegisterVersionRouter(r *gin.Engine) {
	versionCtl := controller.NewVersionController()

	api := r.Group("/api/v1")
	{
		api.GET("/version", versionCtl.GetVersion)
	}
}
