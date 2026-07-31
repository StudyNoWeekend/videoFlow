package router

import (
	"github.com/gin-gonic/gin"

	"video-captions/internal/controller"
)

// RegisterVideoRouter 注册视频管理路由
func RegisterVideoRouter(r *gin.Engine) {
	videoCtl := controller.NewVideoController()

	api := r.Group("/api/v1/videos")
	{
		api.GET("", videoCtl.List)
		api.POST("/scan", videoCtl.Scan)
		api.PUT("/:id", videoCtl.Update)
		api.DELETE("/:id", videoCtl.Delete)
	}
}
