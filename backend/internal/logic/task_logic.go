package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/gorm"

	"video-captions/enum"
	"video-captions/internal/asr"
	"video-captions/internal/dto/req"
	"video-captions/internal/dto/res"
	"video-captions/internal/model"
	"video-captions/internal/scheduler"
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

	// 校验可选的实际处理源文件路径
	sourcePath, err := l.validateSourcePath(video, createReq.SourcePath)
	if err != nil {
		return nil, err
	}

	task := &model.Task{
		VideoID:           video.ID,
		TaskType:          createReq.TaskType,
		Status:            model.TaskStatusPending,
		SourcePath:        sourcePath,
		TargetWidth:       createReq.TargetWidth,
		TargetHeight:      createReq.TargetHeight,
		UpscaleProcessor:  createReq.UpscaleProcessor,
		UpscaleModel:      createReq.UpscaleModel,
		UpscaleNoiseLevel: createReq.UpscaleNoiseLevel,
	}
	if err := model.TaskCreate(ctx, task); err != nil {
		return nil, enum.ErrDatabase.WithMsg(fmt.Sprintf("创建任务失败: %v", err))
	}

	return taskModelToResWithVideo(task, video), nil
}

// validateSourcePath 校验实际处理源文件路径。
// 为空或等于关联视频本身时视为使用原视频；否则必须为存在且位于该视频
// 同名输出目录内的视频文件（即由该视频生成的衍生视频，如字幕合成视频）。
func (l *TaskLogic) validateSourcePath(video *model.Video, sourcePath string) (string, error) {
	if sourcePath == "" || sourcePath == video.Path {
		return "", nil
	}

	if !model.IsVideoFile(sourcePath) {
		return "", enum.ErrInvalidParam.WithMsg("处理源必须是视频文件")
	}
	if _, err := os.Stat(sourcePath); err != nil {
		return "", enum.ErrInvalidParam.WithMsg("处理源文件不存在")
	}

	// 仅允许原视频同名输出目录内的衍生视频，避免把任务指向任意路径
	outputDir := filepath.Join(filepath.Dir(video.Path), strings.TrimSuffix(video.Name, filepath.Ext(video.Name)))
	if filepath.Dir(sourcePath) != outputDir {
		return "", enum.ErrInvalidParam.WithMsg("处理源文件必须是原视频的衍生视频")
	}
	return sourcePath, nil
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
		if task.Status != model.TaskStatusFailed && task.Status != model.TaskStatusCancelled {
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

// CancelTask 取消指定任务：等待中的任务直接取消，运行中的任务会中断正在执行的逻辑
func (l *TaskLogic) CancelTask(ctx context.Context, taskID string) (*res.TaskRes, error) {
	if taskID == "" {
		return nil, enum.ErrInvalidParam
	}
	if scheduler.Default == nil {
		return nil, enum.ErrInternalServer.WithMsg("任务调度器未初始化")
	}

	if err := scheduler.Default.CancelByID(ctx, taskID); err != nil {
		var bizErr *enum.BizError
		if errors.As(err, &bizErr) {
			return nil, bizErr
		}
		return nil, enum.ErrInternalServer.WithMsg(fmt.Sprintf("取消任务失败: %v", err))
	}

	task, err := model.TaskGetByID(ctx, taskID)
	if err != nil {
		return nil, enum.ErrDatabase.WithMsg(fmt.Sprintf("查询任务失败: %v", err))
	}
	if task == nil {
		return nil, enum.ErrTaskNotFound
	}
	video, err := model.VideoGetByID(ctx, task.VideoID)
	if err != nil {
		return nil, enum.ErrDatabase.WithMsg(fmt.Sprintf("查询视频失败: %v", err))
	}

	return taskModelToResWithVideo(task, video), nil
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
		ID:                task.ID,
		VideoID:           task.VideoID,
		TaskType:          task.TaskType,
		Status:            task.Status,
		SourcePath:        task.SourcePath,
		TargetWidth:       task.TargetWidth,
		TargetHeight:      task.TargetHeight,
		UpscaleProcessor:  task.UpscaleProcessor,
		UpscaleModel:      task.UpscaleModel,
		UpscaleNoiseLevel: task.UpscaleNoiseLevel,
		Progress:          task.Progress,
		ProgressMsg:       task.ProgressMsg,
		Result:            result,
		ErrorMsg:          task.ErrorMsg,
		RetryCount:        task.RetryCount,
		CreatedAt:         task.CreatedAt,
		UpdatedAt:         task.UpdatedAt,
		Video:             videoModelToRes(video),
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
		ID:                item.ID,
		VideoID:           item.VideoID,
		TaskType:          item.TaskType,
		Status:            item.Status,
		SourcePath:        item.SourcePath,
		TargetWidth:       item.TargetWidth,
		TargetHeight:      item.TargetHeight,
		UpscaleProcessor:  item.UpscaleProcessor,
		UpscaleModel:      item.UpscaleModel,
		UpscaleNoiseLevel: item.UpscaleNoiseLevel,
		Progress:          item.Progress,
		ProgressMsg:       item.ProgressMsg,
		Result:            result,
		ErrorMsg:          item.ErrorMsg,
		RetryCount:        item.RetryCount,
		CreatedAt:         item.CreatedAt,
		UpdatedAt:         item.UpdatedAt,
		Video:             videoRes,
	}
}
