package controller

import (
	"github.com/gin-gonic/gin"

	"video-captions/enum"
	"video-captions/internal/dto/req"
	"video-captions/internal/logic"
	"video-captions/utils/response"
)

// VideoController 视频管理控制器
type VideoController struct {
	videoLogic *logic.VideoLogic
}

// NewVideoController 创建视频管理控制器
func NewVideoController() *VideoController {
	return &VideoController{
		videoLogic: logic.NewVideoLogic(),
	}
}

// Scan 扫描本地视频目录
// POST /api/v1/videos/scan
func (ctl *VideoController) Scan(c *gin.Context) {
	var scanReq req.VideoScanReq
	if err := c.ShouldBindJSON(&scanReq); err != nil {
		response.FailByBizError(c, enum.ErrInvalidParam)
		return
	}

	// path 为空时由 logic 层回退到配置的视频目录
	res, err := ctl.videoLogic.ScanDir(c.Request.Context(), &scanReq)
	if err != nil {
		HandleError(c, err)
		return
	}
	response.Success(c, res)
}

// List 分页查询视频列表
// GET /api/v1/videos
func (ctl *VideoController) List(c *gin.Context) {
	var listReq req.VideoListReq
	if err := c.ShouldBindQuery(&listReq); err != nil {
		response.FailByBizError(c, enum.ErrInvalidParam)
		return
	}

	res, err := ctl.videoLogic.List(c.Request.Context(), &listReq)
	if err != nil {
		HandleError(c, err)
		return
	}
	response.Success(c, res)
}

// Update 更新视频信息
// PUT /api/v1/videos/:id
func (ctl *VideoController) Update(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.FailByBizError(c, enum.ErrInvalidParam)
		return
	}

	var updateReq req.VideoUpdateReq
	if err := c.ShouldBindJSON(&updateReq); err != nil {
		response.FailByBizError(c, enum.ErrInvalidParam)
		return
	}

	res, err := ctl.videoLogic.Update(c.Request.Context(), id, &updateReq)
	if err != nil {
		HandleError(c, err)
		return
	}
	response.Success(c, res)
}

// Delete 删除视频记录
// DELETE /api/v1/videos/:id
func (ctl *VideoController) Delete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.FailByBizError(c, enum.ErrInvalidParam)
		return
	}

	if err := ctl.videoLogic.Delete(c.Request.Context(), id); err != nil {
		HandleError(c, err)
		return
	}
	response.Success(c, nil)
}

// BatchDelete 批量删除视频记录，可选同时删除输出目录
// POST /api/v1/videos/batch-delete
func (ctl *VideoController) BatchDelete(c *gin.Context) {
	var deleteReq req.VideoBatchDeleteReq
	if err := c.ShouldBindJSON(&deleteReq); err != nil {
		response.FailByBizError(c, enum.ErrInvalidParam)
		return
	}

	res, err := ctl.videoLogic.BatchDelete(c.Request.Context(), &deleteReq)
	if err != nil {
		HandleError(c, err)
		return
	}
	response.Success(c, res)
}

// DirFiles 获取视频输出目录下所有视频文件的详细信息，包括分辨率
// GET /api/v1/videos/:id/dir-files
func (ctl *VideoController) DirFiles(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.FailByBizError(c, enum.ErrInvalidParam)
		return
	}

	res, err := ctl.videoLogic.DirFiles(c.Request.Context(), id)
	if err != nil {
		HandleError(c, err)
		return
	}
	response.Success(c, res)
}
