package model

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Video 视频数据模型
type Video struct {
	BaseModel
	Path     string `gorm:"type:varchar(1024);not null;uniqueIndex:idx_video_path;comment:视频文件绝对路径" json:"path"`
	Name     string `gorm:"type:varchar(255);not null;comment:视频文件名" json:"name"`
	Duration int64  `gorm:"default:0;comment:视频时长（秒）" json:"duration"`
	Size     int64  `gorm:"default:0;comment:视频文件大小（字节）" json:"size"`
}

// TableName 指定表名
func (Video) TableName() string {
	return "videos"
}

// BeforeCreate 创建前自动生成 UUID 主键
func (v *Video) BeforeCreate(tx *gorm.DB) error {
	if v.ID == "" {
		v.ID = uuid.NewString()
	}
	return nil
}

// videoExtensions 支持的常见视频格式后缀集合
var videoExtensions = map[string]struct{}{
	".mp4":  {},
	".mkv":  {},
	".avi":  {},
	".mov":  {},
	".webm": {},
	".flv":  {},
	".m4v":  {},
	".wmv":  {},
}

// IsVideoFile 根据扩展名判断是否为支持的视频文件
func IsVideoFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	_, ok := videoExtensions[ext]
	return ok
}

// GetSupportedExtensions 返回所有支持的视频格式扩展名列表
func GetSupportedExtensions() []string {
	exts := make([]string, 0, len(videoExtensions))
	for ext := range videoExtensions {
		exts = append(exts, ext)
	}
	return exts
}

// VideoCreate 创建视频记录
func VideoCreate(ctx context.Context, video *Video) error {
	return DB.WithContext(ctx).Create(video).Error
}

// VideoGetByPath 根据文件路径查询视频记录
func VideoGetByPath(ctx context.Context, path string) (*Video, error) {
	var video Video
	err := DB.WithContext(ctx).Where("path = ?", path).First(&video).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &video, nil
}

// VideoUpsertByPath 按路径去重：存在则更新元信息，不存在则创建
func VideoUpsertByPath(ctx context.Context, path string, size int64, duration int64) (*Video, error) {
	name := filepath.Base(path)

	video, err := VideoGetByPath(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("按路径查询视频失败: %w", err)
	}

	now := time.Now().Unix()
	if video != nil {
		// 更新已有记录
		video.Name = name
		video.Size = size
		video.Duration = duration
		video.UpdatedAt = now
		if err := DB.WithContext(ctx).Save(video).Error; err != nil {
			return nil, fmt.Errorf("更新视频记录失败: %w", err)
		}
		return video, nil
	}

	// 创建新记录
	video = &Video{
		BaseModel: BaseModel{
			CreatedAt: now,
			UpdatedAt: now,
		},
		Path:     path,
		Name:     name,
		Size:     size,
		Duration: duration,
	}
	if err := VideoCreate(ctx, video); err != nil {
		return nil, fmt.Errorf("创建视频记录失败: %w", err)
	}
	return video, nil
}

// VideoListQuery 视频列表查询参数
type VideoListQuery struct {
	Page     int
	PageSize int
}

// VideoListAll 查询所有未软删除的视频记录
func VideoListAll(ctx context.Context) ([]*Video, error) {
	var videos []*Video
	if err := DB.WithContext(ctx).Find(&videos).Error; err != nil {
		return nil, fmt.Errorf("查询所有视频失败: %w", err)
	}
	return videos, nil
}

// VideoList 分页查询视频列表，按更新时间倒序
func VideoList(ctx context.Context, query *VideoListQuery) ([]*Video, int64, error) {
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
	db := DB.WithContext(ctx).Model(&Video{})
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计视频总数失败: %w", err)
	}

	var videos []*Video
	offset := (page - 1) * pageSize
	if err := db.Order("updated_at DESC, id DESC").Offset(offset).Limit(pageSize).Find(&videos).Error; err != nil {
		return nil, 0, fmt.Errorf("查询视频列表失败: %w", err)
	}
	return videos, total, nil
}

// VideoGetByID 根据 ID 查询视频记录
func VideoGetByID(ctx context.Context, id string) (*Video, error) {
	var video Video
	err := DB.WithContext(ctx).Where("id = ?", id).First(&video).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &video, nil
}

// VideoUpdate 更新视频记录（目前仅支持修改文件名）
func VideoUpdate(ctx context.Context, id string, name string) (*Video, error) {
	video, err := VideoGetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if video == nil {
		return nil, nil
	}

	video.Name = name
	video.UpdatedAt = time.Now().Unix()
	if err := DB.WithContext(ctx).Save(video).Error; err != nil {
		return nil, fmt.Errorf("更新视频记录失败: %w", err)
	}
	return video, nil
}

// VideoDelete 根据 ID 删除视频记录（软删除）
func VideoDelete(ctx context.Context, id string) error {
	result := DB.WithContext(ctx).Delete(&Video{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("删除视频记录失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
