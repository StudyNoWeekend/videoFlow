package router

import (
	"github.com/gin-gonic/gin"

	"video-captions/internal/controller"
)

// RegisterTaskRouter 注册任务管理路由（需在鉴权分组下调用）
func RegisterTaskRouter(rg *gin.RouterGroup) {
	taskCtl := controller.NewTaskController()

	api := rg.Group("/tasks")
	{
		api.GET("", taskCtl.List)
		api.POST("", taskCtl.Create)
		api.POST("/:id/retry", taskCtl.Retry)
		api.POST("/:id/cancel", taskCtl.Cancel)
		api.DELETE("/:id", taskCtl.Delete)
	}
}
