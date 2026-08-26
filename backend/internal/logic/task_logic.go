package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"video-captions/enum"
	"video-captions/internal/asr"
	"video-captions/internal/component"
	"video-captions/internal/dto/req"
	"video-captions/internal/dto/res"
	"video-captions/internal/model"
	"video-captions/internal/scheduler"
	"video-captions/utils/logger"
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
	sourcePath, err := l.validateSourcePath(ctx, video, createReq.SourcePath)
	if err != nil {
		return nil, err
	}

	// 校验任务依赖的组件是否就绪，避免任务创建后执行时才失败
	if missing := component.CheckTaskDependencies(ctx, createReq.TaskType); len(missing) > 0 {
		return nil, componentMissingErr(missing)
	}

	// 防重复提交：同一视频不允许同时存在同类型的 pending/running/cancelling 任务
	if exists, err := model.TaskExistsPendingOrRunningByVideoAndType(ctx, createReq.VideoID, createReq.TaskType); err != nil {
		return nil, enum.ErrDatabase.WithMsg(fmt.Sprintf("查重失败: %v", err))
	} else if exists {
		return nil, enum.ErrTaskDuplicate
	}

	task := &model.Task{
		VideoID:    video.ID,
		TaskType:   createReq.TaskType,
		Status:     model.TaskStatusPending,
		SourcePath: sourcePath,
		// 覆盖模式仅对衍生视频（SourcePath 非原视频）有意义，原视频时强制为 false
		Overwrite:         createReq.Overwrite && sourcePath != "" && sourcePath != video.Path,
		TargetWidth:       createReq.TargetWidth,
		TargetHeight:      createReq.TargetHeight,
		UpscaleProcessor:  createReq.UpscaleProcessor,
		UpscaleModel:      createReq.UpscaleModel,
		UpscaleNoiseLevel: createReq.UpscaleNoiseLevel,
	}
	// 创建任务的同时同步视频对应任务类型的状态为 pending
	if err := model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := model.TaskCreateTx(tx, task); err != nil {
			return err
		}
		return model.VideoSetTaskStatusTx(tx, video.ID, task.TaskType, model.TaskStatusPending)
	}); err != nil {
		return nil, enum.ErrDatabase.WithMsg(fmt.Sprintf("创建任务失败: %v", err))
	}

	return taskModelToResWithVideo(task, video), nil
}

// validateSourcePath 校验实际处理源文件路径。
// 为空或等于关联视频本身时视为使用原视频；否则必须为存在且位于该视频
// 同名输出目录内的视频文件（即由该视频生成的衍生视频，如字幕合成视频）。
func (l *TaskLogic) validateSourcePath(ctx context.Context, video *model.Video, sourcePath string) (string, error) {
	cleaned := filepath.Clean(sourcePath)
	if cleaned == "" || cleaned == filepath.Clean(video.Path) {
		return "", nil
	}

	if !model.IsVideoFile(cleaned) {
		return "", enum.ErrInvalidParam.WithMsg("处理源必须是视频文件")
	}
	if _, err := os.Stat(cleaned); err != nil {
		return "", enum.ErrInvalidParam.WithMsg("处理源文件不存在")
	}

	// 仅允许原视频输出目录内的衍生视频，避免把任务指向任意路径
	outputDir := filepath.Clean(model.VideoOutputDir(ctx, video))
	if filepath.Dir(cleaned) != outputDir {
		return "", enum.ErrInvalidParam.WithMsg("处理源文件必须是原视频的衍生视频")
	}
	return cleaned, nil
}

// componentMissingErr 将未就绪组件列表转换为业务错误，提示用户先到组件管理安装/配置
func componentMissingErr(missing []component.ComponentInfo) *enum.BizError {
	names := make([]string, 0, len(missing))
	for _, info := range missing {
		names = append(names, info.Name)
	}
	return enum.ErrTaskComponentMissing.WithMsg(fmt.Sprintf(
		"任务依赖组件未就绪：%s，请先到组件管理安装/配置后再试",
		strings.Join(names, "、"),
	))
}

// RetryTask 重试失败任务，将其状态重置为 pending
func (l *TaskLogic) RetryTask(ctx context.Context, taskID string) (*res.TaskRes, error) {
	if taskID == "" {
		return nil, enum.ErrInvalidParam
	}

	// 重试前同样校验任务依赖组件是否就绪，避免组件未就绪时重置后再次失败
	task, err := model.TaskGetByID(ctx, taskID)
	if err != nil {
		return nil, enum.ErrDatabase.WithMsg(fmt.Sprintf("查询任务失败: %v", err))
	}
	if task == nil {
		return nil, enum.ErrTaskNotFound
	}
	if missing := component.CheckTaskDependencies(ctx, task.TaskType); len(missing) > 0 {
		return nil, componentMissingErr(missing)
	}

	err = model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
		if err := model.TaskResetFailedTx(tx, taskID); err != nil {
			return err
		}
		// 重试后该类型最新任务回到 pending，同步视频状态字段
		return model.VideoResyncTaskStatusTx(tx, task.VideoID, task.TaskType)
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

	task, err = model.TaskGetByID(ctx, taskID)
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
		SortBy:   listReq.SortBy,
		Order:    listReq.Order,
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

// DeleteTask 删除指定任务，运行中的任务不允许删除；可选同时删除任务对应的输出文件
func (l *TaskLogic) DeleteTask(ctx context.Context, taskID string, deleteFiles bool) error {
	if taskID == "" {
		return enum.ErrInvalidParam
	}

	var delTask *model.Task
	err := model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		task, err := model.TaskGetByIDTx(tx, taskID)
		if err != nil {
			return err
		}
		if task == nil {
			return gorm.ErrRecordNotFound
		}
		if task.Status == model.TaskStatusRunning || task.Status == model.TaskStatusCancelling {
			return enum.ErrTaskRunningCannotDelete
		}
		if err := model.TaskDeleteTx(tx, taskID); err != nil {
			return err
		}
		// 删除任务后重新对齐该类型状态字段（最新任务回退为旧任务，或清空为未开始）
		if err := model.VideoResyncTaskStatusTx(tx, task.VideoID, task.TaskType); err != nil {
			return err
		}
		delTask = task
		return nil
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return enum.ErrTaskNotFound
		}
		if bizErr, ok := err.(*enum.BizError); ok {
			return bizErr
		}
		return enum.ErrDatabase.WithMsg(fmt.Sprintf("删除任务失败: %v", err))
	}

	if deleteFiles && delTask != nil {
		// 构造一个含 VideoID/TaskType/Overwrite/SourcePath 的任务用于删除文件
		deleteTaskOutputFiles(ctx, delTask)
	}
	return nil
}

// BatchDelete 批量删除任务记录，运行中的任务跳过（沿用单条删除的规则），
// 不存在的记录跳过，部分失败不中断整体流程。可选同时删除任务对应的输出文件。
// 任务删除后其对应任务类型在视频上的状态回退（改为旧任务状态或清空为未开始）。
func (l *TaskLogic) BatchDelete(ctx context.Context, deleteReq *req.TaskBatchDeleteReq) (*res.BatchDeleteRes, error) {
	seen := make(map[string]struct{}, len(deleteReq.IDs))
	result := &res.BatchDeleteRes{}
	for _, id := range deleteReq.IDs {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}

		var delTask *model.Task
		err := model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			task, err := model.TaskGetByIDTx(tx, id)
			if err != nil {
				return err
			}
			if task == nil {
				return gorm.ErrRecordNotFound
			}
			if task.Status == model.TaskStatusRunning || task.Status == model.TaskStatusCancelling {
				return enum.ErrTaskRunningCannotDelete
			}
			if err := model.TaskDeleteTx(tx, id); err != nil {
				return err
			}
			if err := model.VideoResyncTaskStatusTx(tx, task.VideoID, task.TaskType); err != nil {
				return err
			}
			delTask = task
			return nil
		})
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				result.Skipped++
				continue
			}
			if err == enum.ErrTaskRunningCannotDelete {
				result.Skipped++
				continue
			}
			return nil, enum.ErrDatabase.WithMsg(fmt.Sprintf("删除任务失败: %v", err))
		}

		if deleteReq.DeleteFiles && delTask != nil {
			deleteTaskOutputFiles(ctx, delTask)
		}
		result.Deleted++
	}
	return result, nil
}

// deleteTaskOutputFiles 删除指定任务对应的输出文件（尽力而为，删除失败仅记录日志不阻断）。
// 文件命名规则与输出文件分类（video_logic.classifyOutputFile）保持一致：
//   - 覆盖模式（overwrite + 衍生视频）：输出即处理源文件本身，直接删除该文件；
//   - subtitle：<base>.srt
//   - subtitle_burn：subtitled_video 类文件
//   - deblur：repaired_video 类文件
//   - upscale：upscaled_video 类文件
func deleteTaskOutputFiles(ctx context.Context, task *model.Task) {
	video, err := model.VideoGetByID(ctx, task.VideoID)
	if err != nil || video == nil {
		logger.Logger.Warn("删除任务输出文件失败：视频记录不存在",
			zap.String("task_id", task.ID),
		)
		return
	}

	outputDir := model.VideoOutputDir(ctx, video)
	baseName := model.VideoBaseName(video)

	removeFile := func(path string) {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			logger.Logger.Warn("删除任务输出文件失败",
				zap.String("task_id", task.ID),
				zap.String("path", path),
				zap.Error(err),
			)
		}
	}

	// 覆盖模式：输出即被覆盖的处理源文件，直接删除该文件本身
	if task.Overwrite && task.SourcePath != "" && task.SourcePath != video.Path {
		removeFile(task.SourcePath)
		return
	}

	// 字幕 srt 文件路径是确定的，直接删除
	if task.TaskType == model.TaskTypeSubtitle {
		removeFile(filepath.Join(outputDir, baseName+".srt"))
		return
	}

	// 其余类型为输出目录中的视频文件，按分类匹配删除
	targetType := ""
	switch task.TaskType {
	case model.TaskTypeSubtitleBurn:
		targetType = "subtitled_video"
	case model.TaskTypeDeblur, model.TaskTypeRepair:
		targetType = "repaired_video"
	case model.TaskTypeUpscale:
		targetType = "upscaled_video"
	default:
		return
	}

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !model.IsVideoFile(e.Name()) {
			continue
		}
		if classifyOutputFile(e.Name(), baseName) != targetType {
			continue
		}
		removeFile(filepath.Join(outputDir, e.Name()))
	}
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
		Overwrite:         task.Overwrite,
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
		Overwrite:         item.Overwrite,
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
