package logic

import (
	"context"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"

	"video-captions/enum"
	"video-captions/internal/asr"
	"video-captions/internal/dto/req"
	"video-captions/internal/dto/res"
	"video-captions/internal/model"
)

// TaskLogic 任务管理业务逻辑
type TaskLogic struct{}

// NewTaskLogic 创建任务管理 logic 实例
func NewTaskLogic() *TaskLogic {
	return &TaskLogic{}
}

// CreateTask 为指定视频创建 pending 状态字幕任务
func (l *TaskLogic) CreateTask(ctx context.Context, createReq *req.TaskCreateReq) (*res.TaskRes, error) {
	if createReq.VideoID == "" {
		return nil, enum.ErrInvalidParam
	}

	video, err := model.VideoGetByID(ctx, createReq.VideoID)
	if err != nil {
		return nil, enum.ErrDatabase.WithMsg(fmt.Sprintf("查询视频失败: %v", err))
	}
	if video == nil {
		return nil, enum.ErrNotFound.WithMsg("视频不存在")
	}

	// 如果是翻译任务，需要校验字幕任务状态
	if createReq.TaskType == model.TaskTypeTranslate {
		if err := l.validateTranslateTask(ctx, createReq.VideoID); err != nil {
			return nil, err
		}
	}

	task := &model.Task{
		VideoID:  video.ID,
		TaskType: createReq.TaskType,
		Status:   model.TaskStatusPending,
	}
	if err := model.TaskCreate(ctx, task); err != nil {
		return nil, enum.ErrDatabase.WithMsg(fmt.Sprintf("创建任务失败: %v", err))
	}

	return taskModelToResWithVideo(task, video), nil
}

// validateTranslateTask 校验翻译任务的前置条件：视频必须已有完成的字幕生成任务
func (l *TaskLogic) validateTranslateTask(ctx context.Context, videoID string) error {
	subtitleTask, err := model.TaskGetLatestByVideoIDAndType(ctx, videoID, model.TaskTypeSubtitle)
	if err != nil {
		return enum.ErrDatabase.WithMsg(fmt.Sprintf("查询字幕任务失败: %v", err))
	}
	if subtitleTask == nil {
		return enum.ErrSubtitleNotCompleted
	}
	if subtitleTask.Status != model.TaskStatusCompleted {
		return enum.ErrSubtitleNotCompleted
	}
	return nil
}

// RetryTask 重试失败任务，将其状态重置为 pending
func (l *TaskLogic) RetryTask(ctx context.Context, taskID string) (*res.TaskRes, error) {
	if taskID == "" {
		return nil, enum.ErrInvalidParam
	}

	err := model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		task, err := model.TaskGetByIDTx(tx, taskID)
		if err != nil {
			return err
		}
		if task == nil {
			return gorm.ErrRecordNotFound
		}
		if task.Status != model.TaskStatusFailed {
			return enum.ErrTaskNotFailed
		}
		return model.TaskResetFailedTx(tx, taskID)
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, enum.ErrTaskNotFound
		}
		if err == enum.ErrTaskNotFailed {
			return nil, err
		}
		return nil, enum.ErrDatabase.WithMsg(fmt.Sprintf("重试任务失败: %v", err))
	}

	task, err := model.TaskGetByID(ctx, taskID)
	if err != nil {
		return nil, enum.ErrDatabase.WithMsg(fmt.Sprintf("查询任务失败: %v", err))
	}
	video, err := model.VideoGetByID(ctx, task.VideoID)
	if err != nil {
		return nil, enum.ErrDatabase.WithMsg(fmt.Sprintf("查询视频失败: %v", err))
	}

	return taskModelToResWithVideo(task, video), nil
}

// ListTasks 分页查询任务列表，并关联视频信息
func (l *TaskLogic) ListTasks(ctx context.Context, listReq *req.TaskListReq) (*res.TaskListRes, error) {
	page := listReq.Page
	pageSize := listReq.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	items, total, err := model.TaskList(ctx, &model.TaskListQuery{
		Page:     page,
		PageSize: pageSize,
		TaskType: listReq.TaskType,
	})
	if err != nil {
		return nil, enum.ErrDatabase.WithMsg(fmt.Sprintf("查询任务列表失败: %v", err))
	}

	list := make([]*res.TaskRes, 0, len(items))
	for _, item := range items {
		list = append(list, taskWithVideoToRes(item))
	}

	return &res.TaskListRes{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// DeleteTask 删除指定任务，运行中的任务不允许删除
func (l *TaskLogic) DeleteTask(ctx context.Context, taskID string) error {
	if taskID == "" {
		return enum.ErrInvalidParam
	}

	task, err := model.TaskGetByID(ctx, taskID)
	if err != nil {
		return enum.ErrDatabase.WithMsg(fmt.Sprintf("查询任务失败: %v", err))
	}
	if task == nil {
		return enum.ErrTaskNotFound
	}
	if task.Status == model.TaskStatusRunning {
		return enum.ErrTaskRunningCannotDelete
	}

	if err := model.TaskDelete(ctx, taskID); err != nil {
		if err == gorm.ErrRecordNotFound {
			return enum.ErrTaskNotFound
		}
		return enum.ErrDatabase.WithMsg(fmt.Sprintf("删除任务失败: %v", err))
	}
	return nil
}

// taskModelToResWithVideo 将任务模型与视频模型转换为响应结构
func taskModelToResWithVideo(task *model.Task, video *model.Video) *res.TaskRes {
	var result interface{}
	if task.ResultJSON != "" {
		var segments []asr.Segment
		if err := json.Unmarshal([]byte(task.ResultJSON), &segments); err == nil {
			result = segments
		}
	}

	return &res.TaskRes{
		ID:          task.ID,
		VideoID:     task.VideoID,
		TaskType:    task.TaskType,
		Status:      task.Status,
		Progress:    task.Progress,
		ProgressMsg: task.ProgressMsg,
		Result:      result,
		ErrorMsg:    task.ErrorMsg,
		RetryCount:  task.RetryCount,
		CreatedAt:   task.CreatedAt,
		UpdatedAt:   task.UpdatedAt,
		Video:       videoModelToRes(video),
	}
}

// taskWithVideoToRes 将关联查询结果转换为响应结构
func taskWithVideoToRes(item *model.TaskWithVideo) *res.TaskRes {
	var result interface{}
	if item.ResultJSON != "" {
		var segments []asr.Segment
		if err := json.Unmarshal([]byte(item.ResultJSON), &segments); err == nil {
			result = segments
		}
	}

	var videoRes *res.VideoRes
	if item.VideoPath != "" {
		videoRes = &res.VideoRes{
			ID:        item.VideoID,
			Path:      item.VideoPath,
			Name:      item.VideoName,
			Duration:  item.VideoDuration,
			Size:      item.VideoSize,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		}
	}

	return &res.TaskRes{
		ID:          item.ID,
		VideoID:     item.VideoID,
		TaskType:    item.TaskType,
		Status:      item.Status,
		Progress:    item.Progress,
		ProgressMsg: item.ProgressMsg,
		Result:      result,
		ErrorMsg:    item.ErrorMsg,
		RetryCount:  item.RetryCount,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
		Video:       videoRes,
	}
}
