package router

import (
	"github.com/gin-gonic/gin"

	"video-captions/internal/controller"
)

// RegisterVideoRouter 注册视频管理路由（需在鉴权分组下调用）
func RegisterVideoRouter(rg *gin.RouterGroup) {
	videoCtl := controller.NewVideoController()

	api := rg.Group("/videos")
	{
		api.GET("", videoCtl.List)
		api.POST("/scan", videoCtl.Scan)
		api.POST("/batch-delete", videoCtl.BatchDelete)
		api.PUT("/:id", videoCtl.Update)
		api.DELETE("/:id", videoCtl.Delete)
		api.GET("/:id/dir-files", videoCtl.DirFiles)
	}
}
