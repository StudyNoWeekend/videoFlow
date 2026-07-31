package controller

import (
	"errors"

	"github.com/gin-gonic/gin"

	"video-captions/enum"
	"video-captions/internal/logic"
	"video-captions/utils/response"
)

// HealthController 健康检查控制器
type HealthController struct {
	healthLogic *logic.HealthLogic
}

// NewHealthController 创建健康检查控制器
func NewHealthController() *HealthController {
	return &HealthController{
		healthLogic: logic.NewHealthLogic(),
	}
}

// Health 存活检查接口
func (ctl *HealthController) Health(c *gin.Context) {
	res := ctl.healthLogic.Health()
	response.Success(c, res)
}

// Ready 就绪检查接口
func (ctl *HealthController) Ready(c *gin.Context) {
	res, err := ctl.healthLogic.Ready(c.Request.Context())
	if err != nil {
		response.Fail(c, enum.ErrDatabase.Code, "服务未就绪：数据库不可用", 503)
		return
	}
	response.Success(c, res)
}

// HandleError 统一错误处理（示例方法，供其他 controller 参考）
func HandleError(c *gin.Context, err error) {
	var bizErr *enum.BizError
	if errors.As(err, &bizErr) {
		response.Fail(c, bizErr.Code, bizErr.Msg, bizErr.HttpCode)
		return
	}
	response.Fail(c, enum.ErrInternalServer.Code, enum.ErrInternalServer.Msg, enum.ErrInternalServer.HttpCode)
}
