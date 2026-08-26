package router

import (
	"github.com/gin-gonic/gin"

	"video-captions/internal/controller"
)

// RegisterDownloadRouter 注册下载任务管理路由（需在鉴权分组下调用）
func RegisterDownloadRouter(rg *gin.RouterGroup) {
	dlCtl := controller.NewDownloadController()

	api := rg.Group("/downloads")
	{
		api.POST("", dlCtl.Create)
		api.GET("", dlCtl.List)
		api.POST("/:id/cancel", dlCtl.Cancel)
		api.DELETE("/:id", dlCtl.Delete)
	}
}
