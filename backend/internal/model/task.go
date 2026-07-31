package model

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// 任务状态枚举
const (
	TaskStatusPending   = "pending"
	TaskStatusRunning   = "running"
	TaskStatusCompleted = "completed"
	TaskStatusFailed    = "failed"
)

// 任务类型枚举
const (
	TaskTypeSubtitle  = "subtitle"
	TaskTypeRepair    = "repair"
	TaskTypeTranslate = "translate"
)

// Task 字幕/修复任务数据模型
type Task struct {
	BaseModel
	VideoID     string `gorm:"type:char(36);not null;index:idx_task_video_id;comment:关联视频ID" json:"video_id"`
	TaskType    string `gorm:"type:varchar(32);not null;default:'subtitle';index:idx_task_task_type;comment:任务类型 subtitle/repair" json:"task_type"`
	Status      string `gorm:"type:varchar(32);not null;default:'pending';index:idx_task_status;comment:任务状态 pending/running/completed/failed" json:"status"`
	Progress    int    `gorm:"default:0;comment:进度 0-100" json:"progress"`
	ProgressMsg string `gorm:"type:text;comment:当前进度描述，如剩余时间、处理速度" json:"progress_msg"`
	ResultJSON  string `gorm:"type:text;comment:ASR 结果 JSON" json:"-"`
	ErrorMsg    string `gorm:"type:text;comment:错误信息" json:"error_msg"`
	RetryCount  int    `gorm:"default:0;comment:重试次数" json:"retry_count"`
}

// TableName 指定表名
func (Task) TableName() string {
	return "tasks"
}

// BeforeCreate 创建前自动生成 UUID 主键并设置默认状态
func (t *Task) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	if t.Status == "" {
		t.Status = TaskStatusPending
	}
	return nil
}

// TaskWithVideo 任务列表关联视频信息查询结构
type TaskWithVideo struct {
	Task
	VideoPath     string `json:"-"`
	VideoName     string `json:"-"`
	VideoDuration int64  `json:"-"`
	VideoSize     int64  `json:"-"`
}

// TaskListQuery 任务列表查询参数
type TaskListQuery struct {
	Page     int
	PageSize int
	TaskType string
}

// TaskCreate 创建任务记录
func TaskCreate(ctx context.Context, task *Task) error {
	return DB.WithContext(ctx).Create(task).Error
}

// TaskCreateTx 在事务中创建任务记录
func TaskCreateTx(tx *gorm.DB, task *Task) error {
	return tx.Create(task).Error
}

// TaskGetByID 根据 ID 查询任务
func TaskGetByID(ctx context.Context, id string) (*Task, error) {
	var task Task
	err := DB.WithContext(ctx).Where("id = ?", id).First(&task).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &task, nil
}

// TaskGetByIDTx 在事务中根据 ID 查询任务
func TaskGetByIDTx(tx *gorm.DB, id string) (*Task, error) {
	var task Task
	err := tx.Where("id = ?", id).First(&task).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &task, nil
}

// TaskGetLatestByVideoIDAndType 查询指定视频最近一条指定类型的任务（按 created_at 倒序）
func TaskGetLatestByVideoIDAndType(ctx context.Context, videoID, taskType string) (*Task, error) {
	var task Task
	err := DB.WithContext(ctx).
		Where("video_id = ? AND task_type = ?", videoID, taskType).
		Order("created_at DESC, id DESC").
		First(&task).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &task, nil
}

// TaskClaimPendingTx 在事务中按任务类型认领一个最早的 pending 任务，状态置为 running
func TaskClaimPendingTx(tx *gorm.DB, taskType string) (*Task, error) {
	var task Task
	query := tx.Where("status = ?", TaskStatusPending)
	if taskType != "" {
		query = query.Where("task_type = ?", taskType)
	}
	if err := query.Order("created_at ASC, id ASC").First(&task).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	now := time.Now().Unix()
	task.Status = TaskStatusRunning
	task.Progress = 0
	task.ErrorMsg = ""
	task.UpdatedAt = now
	if err := tx.Save(&task).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

// TaskList 分页查询任务列表，并关联视频信息
func TaskList(ctx context.Context, query *TaskListQuery) ([]*TaskWithVideo, int64, error) {
	page := query.Page
	pageSize := query.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	db := DB.WithContext(ctx).Model(&Task{}).
		Select("tasks.*, videos.path as video_path, videos.name as video_name, videos.duration as video_duration, videos.size as video_size").
		Joins("left join videos on videos.id = tasks.video_id")

	if query.TaskType != "" {
		db = db.Where("tasks.task_type = ?", query.TaskType)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计任务总数失败: %w", err)
	}

	offset := (page - 1) * pageSize
	var list []*TaskWithVideo
	if err := db.Order("tasks.created_at DESC, tasks.id DESC").Offset(offset).Limit(pageSize).Scan(&list).Error; err != nil {
		return nil, 0, fmt.Errorf("查询任务列表失败: %w", err)
	}
	return list, total, nil
}

// TaskUpdateStatusTx 在事务中更新任务状态、进度与进度描述
func TaskUpdateStatusTx(tx *gorm.DB, id string, status string, progress int, progressMsg string) error {
	updates := map[string]interface{}{
		"status":     status,
		"progress":   progress,
		"updated_at": time.Now().Unix(),
	}
	if progressMsg != "" {
		updates["progress_msg"] = progressMsg
	}
	return tx.Model(&Task{}).Where("id = ?", id).Updates(updates).Error
}

// TaskUpdateResultTx 在事务中将任务标记为完成并保存结果
func TaskUpdateResultTx(tx *gorm.DB, id string, resultJSON string, progressMsg string) error {
	updates := map[string]interface{}{
		"status":      TaskStatusCompleted,
		"progress":    100,
		"result_json": resultJSON,
		"error_msg":   "",
		"updated_at":  time.Now().Unix(),
	}
	if progressMsg != "" {
		updates["progress_msg"] = progressMsg
	}
	return tx.Model(&Task{}).Where("id = ?", id).Updates(updates).Error
}

// TaskUpdateFailedTx 在事务中将任务标记为失败
func TaskUpdateFailedTx(tx *gorm.DB, id string, errMsg string, retryCount int) error {
	return tx.Model(&Task{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":      TaskStatusFailed,
		"error_msg":   errMsg,
		"retry_count": retryCount,
		"updated_at":  time.Now().Unix(),
	}).Error
}

// TaskResetFailedTx 在事务中将失败任务重置为 pending（手动重试或调度器停止时回退）
func TaskResetFailedTx(tx *gorm.DB, id string) error {
	return tx.Model(&Task{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     TaskStatusPending,
		"progress":   0,
		"error_msg":  "",
		"updated_at": time.Now().Unix(),
	}).Error
}

// TaskCountByStatusTx 在事务中按任务类型统计指定状态的任务数量
func TaskCountByStatusTx(tx *gorm.DB, status, taskType string) (int64, error) {
	var count int64
	query := tx.Model(&Task{}).Where("status = ?", status)
	if taskType != "" {
		query = query.Where("task_type = ?", taskType)
	}
	err := query.Count(&count).Error
	return count, err
}

// TaskDelete 根据 ID 删除任务记录（软删除）
func TaskDelete(ctx context.Context, id string) error {
	result := DB.WithContext(ctx).Delete(&Task{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// TaskMarkRunningAsFailed 将所有 running 状态的任务标记为失败，用于程序重启时的优雅关闭
func TaskMarkRunningAsFailed(ctx context.Context, reason string) (int64, error) {
	result := DB.WithContext(ctx).Model(&Task{}).
		Where("status = ?", TaskStatusRunning).
		Updates(map[string]interface{}{
			"status":     TaskStatusFailed,
			"error_msg":  reason,
			"updated_at": time.Now().Unix(),
		})
	return result.RowsAffected, result.Error
}
