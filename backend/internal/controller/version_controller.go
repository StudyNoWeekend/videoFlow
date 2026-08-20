package controller

import (
	"github.com/gin-gonic/gin"

	"video-captions/internal/logic"
	"video-captions/utils/response"
)

// VersionController 版本号控制器
type VersionController struct {
	versionLogic *logic.VersionLogic
}

// NewVersionController 创建版本号控制器
func NewVersionController() *VersionController {
	return &VersionController{
		versionLogic: logic.NewVersionLogic(),
	}
}

// GetVersion 获取应用版本号
// GET /api/v1/version
func (ctl *VersionController) GetVersion(c *gin.Context) {
	res := ctl.versionLogic.GetVersion(c.Request.Context())
	response.Success(c, res)
}
