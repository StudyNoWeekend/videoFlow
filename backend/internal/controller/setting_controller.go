package controller

import (
	"github.com/gin-gonic/gin"

	"video-captions/enum"
	"video-captions/internal/dto/req"
	"video-captions/internal/logic"
	"video-captions/utils/response"
)

// SettingController 运行时配置控制器
type SettingController struct {
	settingLogic *logic.SettingLogic
}

// NewSettingController 创建运行时配置控制器
func NewSettingController() *SettingController {
	return &SettingController{
		settingLogic: logic.NewSettingLogic(),
	}
}

// GetSettings 获取统一设置
// GET /api/v1/settings
func (ctl *SettingController) GetSettings(c *gin.Context) {
	res, err := ctl.settingLogic.GetSettings(c.Request.Context())
	if err != nil {
		HandleError(c, err)
		return
	}
	response.Success(c, res)
}

// UpdateSettings 更新统一设置
// PUT /api/v1/settings
func (ctl *SettingController) UpdateSettings(c *gin.Context) {
	var updateReq req.SettingReq
	if err := c.ShouldBindJSON(&updateReq); err != nil {
		response.FailByBizError(c, enum.ErrInvalidParam)
		return
	}

	if err := ctl.settingLogic.UpdateSettings(c.Request.Context(), &updateReq); err != nil {
		HandleError(c, err)
		return
	}
	response.Success(c, nil)
}
