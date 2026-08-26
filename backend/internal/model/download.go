package model

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Download 状态枚举
const (
	DownloadStatusPending     = "pending"
	DownloadStatusProbing     = "probing"
	DownloadStatusDownloading = "downloading"
	DownloadStatusCompleted   = "completed"
	DownloadStatusFailed      = "failed"
	DownloadStatusCancelled   = "cancelled"
)

// Download 下载任务数据模型
type Download struct {
	BaseModel
	URL            string `gorm:"type:varchar(2048);not null;comment:视频链接" json:"url"`
	Status         string `gorm:"type:varchar(32);not null;default:'pending';index:idx_download_status;comment:下载状态 pending/probing/downloading/completed/failed/cancelled" json:"status"`
	Progress       int    `gorm:"default:0;comment:进度 0-100" json:"progress"`
	ProgressMsg    string `gorm:"type:text;comment:当前进度描述" json:"progress_msg"`
	ErrorMsg       string `gorm:"type:text;comment:错误信息" json:"error_msg"`
	FileName       string `gorm:"type:varchar(512);default:'';comment:下载后的文件名" json:"file_name"`
	FileSize       int64  `gorm:"default:0;comment:文件大小（字节）" json:"file_size"`
	Duration       int64  `gorm:"default:0;comment:视频时长（秒）" json:"duration"`
	Title          string `gorm:"type:varchar(512);default:'';comment:视频标题" json:"title"`
	DownloadSpeed  int64  `gorm:"default:0;comment:当前下载速度（字节/秒）" json:"download_speed"`
	TotalSize      int64  `gorm:"default:0;comment:总大小（字节）" json:"total_size"`
	DownloadedSize int64  `gorm:"default:0;comment:已下载大小（字节）" json:"downloaded_size"`
	Overwrite      bool   `gorm:"default:false;comment:文件冲突时覆盖还是自动重命名" json:"overwrite"`
	DownloadDir    string `gorm:"type:varchar(2048);default:'';comment:下载存放目录" json:"download_dir"`
}

// TableName 指定表名
func (Download) TableName() string {
	return "downloads"
}

// BeforeCreate 创建前自动生成 UUID 主键并设置默认状态
func (d *Download) BeforeCreate(tx *gorm.DB) error {
	if d.ID == "" {
		d.ID = uuid.NewString()
	}
	if d.Status == "" {
		d.Status = DownloadStatusPending
	}
	return nil
}

// DownloadCreate 创建下载任务记录
func DownloadCreate(ctx context.Context, download *Download) error {
	return DB.WithContext(ctx).Create(download).Error
}

// DownloadGetByID 根据 ID 查询下载任务
func DownloadGetByID(ctx context.Context, id string) (*Download, error) {
	var d Download
	err := DB.WithContext(ctx).Where("id = ?", id).First(&d).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &d, nil
}

// DownloadListQuery 下载记录列表查询参数
type DownloadListQuery struct {
	Page     int
	PageSize int
	SortBy   string
	Order    string
}

// DownloadList 分页查询下载任务列表
func DownloadList(ctx context.Context, query *DownloadListQuery) ([]*Download, int64, error) {
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

	var total int64
	db := DB.WithContext(ctx).Model(&Download{})
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计下载总数失败: %w", err)
	}

	offset := (page - 1) * pageSize
	var list []*Download
	if err := db.Order(downloadOrderClause(query.SortBy, query.Order)).Offset(offset).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, fmt.Errorf("查询下载列表失败: %w", err)
	}
	return list, total, nil
}

// downloadOrderClause 根据排序字段与方向生成 ORDER BY 子句。
// 排序字段必须来自白名单，避免 SQL 注入；未匹配时回退到创建时间倒序。
func downloadOrderClause(sortBy, order string) string {
	dir := "DESC"
	if strings.EqualFold(order, "asc") {
		dir = "ASC"
	}
	switch sortBy {
	case "created_at":
		return "created_at " + dir + ", id DESC"
	case "file_size":
		return "file_size " + dir + ", id DESC"
	default:
		return "created_at DESC, id DESC"
	}
}

// DownloadUpdateStatus 更新下载任务状态
func DownloadUpdateStatus(ctx context.Context, id string, status string, progress int, progressMsg string) error {
	updates := map[string]interface{}{
		"status":     status,
		"progress":   progress,
		"updated_at": time.Now().Unix(),
	}
	if progressMsg != "" {
		updates["progress_msg"] = progressMsg
	}
	return DB.WithContext(ctx).Model(&Download{}).Where("id = ?", id).Updates(updates).Error
}

// DownloadUpdateProgress 仅更新下载进度（保持当前状态为 downloading）
func DownloadUpdateProgress(ctx context.Context, id string, progress int, progressMsg string) error {
	updates := map[string]interface{}{
		"progress":   progress,
		"updated_at": time.Now().Unix(),
	}
	if progressMsg != "" {
		updates["progress_msg"] = progressMsg
	}
	return DB.WithContext(ctx).Model(&Download{}).Where("id = ? AND status = ?", id, DownloadStatusDownloading).Updates(updates).Error
}

// DownloadMarkCompleted 标记下载任务完成，记录文件信息
func DownloadMarkCompleted(ctx context.Context, id string, fileName string, fileSize int64, duration int64) error {
	return DB.WithContext(ctx).Model(&Download{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":          DownloadStatusCompleted,
		"progress":        100,
		"progress_msg":    "下载完成",
		"file_name":       fileName,
		"file_size":       fileSize,
		"total_size":      fileSize,
		"downloaded_size": fileSize,
		"updated_at":      time.Now().Unix(),
	}).Error
}

// DownloadMarkFailed 标记下载任务失败
func DownloadMarkFailed(ctx context.Context, id string, errMsg string) error {
	return DB.WithContext(ctx).Model(&Download{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":       DownloadStatusFailed,
		"progress_msg": "下载失败",
		"error_msg":    errMsg,
		"updated_at":   time.Now().Unix(),
	}).Error
}

// DownloadMarkCancelled 标记下载任务已取消
func DownloadMarkCancelled(ctx context.Context, id string) error {
	return DB.WithContext(ctx).Model(&Download{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":       DownloadStatusCancelled,
		"progress":     0,
		"progress_msg": "已取消",
		"updated_at":   time.Now().Unix(),
	}).Error
}

// DownloadDelete 根据 ID 删除下载任务记录（软删除）
func DownloadDelete(ctx context.Context, id string) error {
	result := DB.WithContext(ctx).Delete(&Download{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("删除下载记录失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
