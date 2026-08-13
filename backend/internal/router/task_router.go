package router

import (
	"github.com/gin-gonic/gin"

	"video-captions/internal/controller"
)

// RegisterTaskRouter 注册任务管理路由
func RegisterTaskRouter(r *gin.Engine) {
	taskCtl := controller.NewTaskController()

	api := r.Group("/api/v1/tasks")
	{
		api.GET("", taskCtl.List)
		api.POST("", taskCtl.Create)
		api.POST("/:id/retry", taskCtl.Retry)
		api.POST("/:id/cancel", taskCtl.Cancel)
		api.DELETE("/:id", taskCtl.Delete)
	}
}
