package router

import (
	"github.com/gin-gonic/gin"

	"video-captions/internal/controller"
	"video-captions/internal/logic"
	"video-captions/internal/middleware"
)

// RegisterAuthRouter 注册认证相关路由
func RegisterAuthRouter(r *gin.Engine) {
	authCtl := controller.NewAuthController()
	authLogic := logic.NewAuthLogic()

	// 公开路由（无需认证）
	auth := r.Group("/api/v1/auth")
	{
		auth.GET("/status", authCtl.Status)
		auth.POST("/init", authCtl.Init)
		auth.POST("/login/password", authCtl.LoginByPassword)
		auth.POST("/reset-password", authCtl.ResetPassword)
		auth.POST("/reset-token", authCtl.GenerateResetToken)
	}

	// 修改密码需要认证
	authed := r.Group("/api/v1/auth")
	authed.Use(middleware.AuthRequired(authLogic.ValidateToken))
	{
		authed.POST("/change-password", authCtl.ChangePassword)
	}
}
