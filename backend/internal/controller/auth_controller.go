package controller

import (
	"github.com/gin-gonic/gin"

	"video-captions/enum"
	"video-captions/internal/dto/req"
	"video-captions/internal/logic"
	"video-captions/utils/response"
)

// AuthController 认证控制器
type AuthController struct {
	authLogic *logic.AuthLogic
}

// NewAuthController 创建认证控制器
func NewAuthController() *AuthController {
	return &AuthController{
		authLogic: logic.NewAuthLogic(),
	}
}

// Status 查询系统初始化状态
// GET /api/v1/auth/status
func (ctl *AuthController) Status(c *gin.Context) {
	res := ctl.authLogic.CheckInitStatus(c.Request.Context())
	response.Success(c, res)
}

// Init 系统初始化（创建管理员账号）
// POST /api/v1/auth/init
func (ctl *AuthController) Init(c *gin.Context) {
	var initReq req.InitReq
	if err := c.ShouldBindJSON(&initReq); err != nil {
		response.FailByBizError(c, enum.ErrInvalidParam.WithMsg("请检查输入参数"))
		return
	}

	res, err := ctl.authLogic.Init(c.Request.Context(), &initReq)
	if err != nil {
		HandleError(c, err)
		return
	}
	response.Success(c, res)
}

// LoginByPassword 密码登录
// POST /api/v1/auth/login/password
func (ctl *AuthController) LoginByPassword(c *gin.Context) {
	var loginReq req.LoginPwdReq
	if err := c.ShouldBindJSON(&loginReq); err != nil {
		response.FailByBizError(c, enum.ErrInvalidParam.WithMsg("请输入用户名和密码"))
		return
	}

	res, err := ctl.authLogic.LoginByPassword(c.Request.Context(), &loginReq)
	if err != nil {
		HandleError(c, err)
		return
	}
	response.Success(c, res)
}

// GenerateResetToken 触发生成重置令牌（令牌输出到服务端日志，不返回给前端）
// POST /api/v1/auth/reset-token
func (ctl *AuthController) GenerateResetToken(c *gin.Context) {
	var genReq req.GenerateResetTokenReq
	if err := c.ShouldBindJSON(&genReq); err != nil {
		response.FailByBizError(c, enum.ErrInvalidParam.WithMsg("请输入用户名"))
		return
	}

	if err := ctl.authLogic.GenerateResetTokenForAPI(c.Request.Context(), genReq.Username); err != nil {
		HandleError(c, err)
		return
	}
	response.Success(c, nil)
}

// ResetPassword 通过重置令牌重置密码
// POST /api/v1/auth/reset-password
func (ctl *AuthController) ResetPassword(c *gin.Context) {
	var resetReq req.ResetPasswordReq
	if err := c.ShouldBindJSON(&resetReq); err != nil {
		response.FailByBizError(c, enum.ErrInvalidParam.WithMsg("请检查输入参数"))
		return
	}

	if err := ctl.authLogic.ResetPassword(c.Request.Context(), &resetReq); err != nil {
		HandleError(c, err)
		return
	}
	response.Success(c, nil)
}

// ChangePassword 修改密码（需已登录）
// POST /api/v1/auth/change-password
func (ctl *AuthController) ChangePassword(c *gin.Context) {
	var changeReq req.ChangePwdReq
	if err := c.ShouldBindJSON(&changeReq); err != nil {
		response.FailByBizError(c, enum.ErrInvalidParam.WithMsg("请检查输入参数"))
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		response.FailByBizError(c, enum.ErrUnauthorized)
		return
	}

	if err := ctl.authLogic.ChangePassword(c.Request.Context(), userID, &changeReq); err != nil {
		HandleError(c, err)
		return
	}
	response.Success(c, nil)
}
