package controller

import (
	"github.com/gin-gonic/gin"

	"video-captions/enum"
	"video-captions/internal/dto/req"
	"video-captions/internal/logic"
	"video-captions/utils/response"
)

// TaskController 任务管理控制器
type TaskController struct {
	taskLogic *logic.TaskLogic
}

// NewTaskController 创建任务管理控制器
func NewTaskController() *TaskController {
	return &TaskController{
		taskLogic: logic.NewTaskLogic(),
	}
}

// Create 创建字幕任务
// POST /api/v1/tasks
func (ctl *TaskController) Create(c *gin.Context) {
	var createReq req.TaskCreateReq
	if err := c.ShouldBindJSON(&createReq); err != nil {
		response.FailByBizError(c, enum.ErrInvalidParam)
		return
	}

	res, err := ctl.taskLogic.CreateTask(c.Request.Context(), &createReq)
	if err != nil {
		HandleError(c, err)
		return
	}
	response.Success(c, res)
}

// List 分页查询任务列表
// GET /api/v1/tasks
func (ctl *TaskController) List(c *gin.Context) {
	var listReq req.TaskListReq
	if err := c.ShouldBindQuery(&listReq); err != nil {
		response.FailByBizError(c, enum.ErrInvalidParam)
		return
	}

	res, err := ctl.taskLogic.ListTasks(c.Request.Context(), &listReq)
	if err != nil {
		HandleError(c, err)
		return
	}
	response.Success(c, res)
}

// Retry 重试失败任务
// POST /api/v1/tasks/:id/retry
func (ctl *TaskController) Retry(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.FailByBizError(c, enum.ErrInvalidParam)
		return
	}

	res, err := ctl.taskLogic.RetryTask(c.Request.Context(), id)
	if err != nil {
		HandleError(c, err)
		return
	}
	response.Success(c, res)
}

// Delete 删除任务
// DELETE /api/v1/tasks/:id
func (ctl *TaskController) Delete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.FailByBizError(c, enum.ErrInvalidParam)
		return
	}

	if err := ctl.taskLogic.DeleteTask(c.Request.Context(), id); err != nil {
		HandleError(c, err)
		return
	}
	response.Success(c, nil)
}
