package controller

import (
	"github.com/gin-gonic/gin"

	"video-captions/enum"
	"video-captions/internal/dto/req"
	"video-captions/internal/logic"
	"video-captions/utils/response"
)

// DownloadController 下载管理控制器
type DownloadController struct {
	downloadLogic *logic.DownloadLogic
}

// NewDownloadController 创建下载管理控制器
func NewDownloadController() *DownloadController {
	return &DownloadController{
		downloadLogic: logic.NewDownloadLogic(),
	}
}

// Create 创建下载任务
// POST /api/v1/downloads
func (ctl *DownloadController) Create(c *gin.Context) {
	var createReq req.DownloadCreateReq
	if err := c.ShouldBindJSON(&createReq); err != nil {
		response.FailByBizError(c, enum.ErrInvalidParam)
		return
	}

	res, err := ctl.downloadLogic.CreateDownload(c.Request.Context(), &createReq)
	if err != nil {
		HandleError(c, err)
		return
	}
	response.Success(c, res)
}

// List 分页查询下载任务列表
// GET /api/v1/downloads
func (ctl *DownloadController) List(c *gin.Context) {
	page := 1
	pageSize := 20

	listReq := struct {
		Page     int    `form:"page"`
		PageSize int    `form:"page_size"`
		SortBy   string `form:"sort_by"`
		Order    string `form:"order"`
	}{
		Page:     page,
		PageSize: pageSize,
	}
	if err := c.ShouldBindQuery(&listReq); err != nil {
		response.FailByBizError(c, enum.ErrInvalidParam)
		return
	}

	res, err := ctl.downloadLogic.ListDownloads(c.Request.Context(), &listReq)
	if err != nil {
		HandleError(c, err)
		return
	}
	response.Success(c, res)
}

// Cancel 取消进行中的下载任务
// POST /api/v1/downloads/:id/cancel
func (ctl *DownloadController) Cancel(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.FailByBizError(c, enum.ErrInvalidParam)
		return
	}

	res, err := ctl.downloadLogic.CancelDownload(c.Request.Context(), id)
	if err != nil {
		HandleError(c, err)
		return
	}
	response.Success(c, res)
}

// Delete 删除下载记录
// DELETE /api/v1/downloads/:id?delete_file=1
func (ctl *DownloadController) Delete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.FailByBizError(c, enum.ErrInvalidParam)
		return
	}

	deleteFile := c.Query("delete_file") == "1"

	if err := ctl.downloadLogic.DeleteDownload(c.Request.Context(), id, deleteFile); err != nil {
		HandleError(c, err)
		return
	}
	response.Success(c, nil)
}
