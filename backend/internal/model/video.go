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
	Width    int    `gorm:"default:0;comment:视频宽度（像素）" json:"width"`
	Height   int    `gorm:"default:0;comment:视频高度（像素）" json:"height"`
	Duration int64  `gorm:"default:0;comment:视频时长（秒）" json:"duration"`
	Size     int64  `gorm:"default:0;comment:视频文件大小（字节）" json:"size"`
	// 各任务类型的最新状态（空串表示从未创建过该类型任务）。
	// 以最新任务记录为准由 VideoResyncTaskStatusTx 维护，视频列表据此展示状态。
	SubtitleStatus     string `gorm:"type:varchar(32);not null;default:'';comment:字幕生成任务状态" json:"subtitle_status"`
	SubtitleBurnStatus string `gorm:"type:varchar(32);not null;default:'';comment:字幕写入任务状态" json:"subtitle_burn_status"`
	DeblurStatus       string `gorm:"type:varchar(32);not null;default:'';comment:去马赛克任务状态" json:"deblur_status"`
	UpscaleStatus      string `gorm:"type:varchar(32);not null;default:'';comment:清晰度修复任务状态" json:"upscale_status"`
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
func VideoUpsertByPath(ctx context.Context, path string, size int64, duration int64, width, height int) (*Video, error) {
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
		video.Width = width
		video.Height = height
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
		Width:    width,
		Height:   height,
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
	// SortBy 排序字段（白名单：size）；为空时按更新时间倒序
	SortBy string
	// Order 排序方向：asc / desc
	Order string
}

// VideoListAll 查询所有未软删除的视频记录
func VideoListAll(ctx context.Context) ([]*Video, error) {
	var videos []*Video
	if err := DB.WithContext(ctx).Find(&videos).Error; err != nil {
		return nil, fmt.Errorf("查询所有视频失败: %w", err)
	}
	return videos, nil
}

// VideoList 分页查询视频列表，默认按更新时间倒序，支持指定字段排序
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
	if err := db.Order(videoOrderClause(query.SortBy, query.Order)).Offset(offset).Limit(pageSize).Find(&videos).Error; err != nil {
		return nil, 0, fmt.Errorf("查询视频列表失败: %w", err)
	}
	return videos, total, nil
}

// videoOrderClause 根据排序字段与方向生成 ORDER BY 子句。
// 排序字段必须来自白名单，避免 SQL 注入；未匹配时回退到更新时间倒序。
func videoOrderClause(sortBy, order string) string {
	dir := "DESC"
	if strings.EqualFold(order, "asc") {
		dir = "ASC"
	}
	switch sortBy {
	case "size":
		return "size " + dir + ", id DESC"
	default:
		return "updated_at DESC, id DESC"
	}
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

// VideoDelete 根据 ID 删除视频记录（软删除）。
// videos.path 上有唯一索引，软删除行仍占据索引位，会阻止同一文件重新扫描入库；
// 因此删除成功后同步改写 path 释放占位（幂等：仅 RowsAffected>0 的那次删除会执行）。
func VideoDelete(ctx context.Context, id string) error {
	result := DB.WithContext(ctx).Delete(&Video{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("删除视频记录失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	if err := DB.WithContext(ctx).Unscoped().Model(&Video{}).
		Where("id = ?", id).
		Update("path", gorm.Expr("path || ':deleted:' || id")).Error; err != nil {
		return fmt.Errorf("释放视频路径唯一索引占位失败: %w", err)
	}
	return nil
}

// VideoBaseName 返回视频文件名去除扩展名后的名称，例如 "movie.mp4" -> "movie"
func VideoBaseName(v *Video) string {
	return strings.TrimSuffix(v.Name, filepath.Ext(v.Name))
}

// VideoTaskStatusColumn 返回任务类型在 videos 表中对应的状态字段列名，
// 未知类型返回空串（调用方应忽略）。
func VideoTaskStatusColumn(taskType string) string {
	switch taskType {
	case TaskTypeSubtitle:
		return "subtitle_status"
	case TaskTypeSubtitleBurn:
		return "subtitle_burn_status"
	case TaskTypeDeblur, TaskTypeRepair:
		return "deblur_status"
	case TaskTypeUpscale:
		return "upscale_status"
	default:
		return ""
	}
}

// VideoSetTaskStatusTx 在事务中直接设置视频指定任务类型的状态字段。
// 调用方已知目标状态时用它，避免子查询。
func VideoSetTaskStatusTx(tx *gorm.DB, videoID, taskType, status string) error {
	column := VideoTaskStatusColumn(taskType)
	if column == "" || videoID == "" {
		return nil
	}
	return tx.Model(&Video{}).Where("id = ?", videoID).Update(column, status).Error
}

// VideoResyncTaskStatusTx 在事务中把视频指定任务类型的状态字段与最新任务记录对齐：
// 取该类型最新一条任务的状态写入，无任务则清空字段。
func VideoResyncTaskStatusTx(tx *gorm.DB, videoID, taskType string) error {
	column := VideoTaskStatusColumn(taskType)
	if column == "" || videoID == "" {
		return nil
	}
	var status string
	if err := tx.Model(&Task{}).
		Where("video_id = ? AND task_type = ?", videoID, taskType).
		Order("created_at DESC, id DESC").
		Limit(1).
		Pluck("status", &status).Error; err != nil {
		return err
	}
	return tx.Model(&Video{}).Where("id = ?", videoID).Update(column, status).Error
}

// resyncTaskTypes 视频状态全量回填覆盖的任务类型
var resyncTaskTypes = []string{TaskTypeSubtitle, TaskTypeSubtitleBurn, TaskTypeDeblur, TaskTypeUpscale}

// VideoResyncTaskStatusWithArtifactTx 在事务中把视频指定任务类型的状态字段与最新任务记录对齐；
// 查不到任务记录时探测输出目录兜底：产物仍存在则视为已完成（completed），
// 否则清空字段。用于取消排队任务等记录缺失但产物可能仍在的场景。
// 注意：产物文件将在事务提交后才被删除的路径（如 DeleteTask deleteFiles=true）
// 不能用它，事务内文件尚未删除，探测会误判。
func VideoResyncTaskStatusWithArtifactTx(tx *gorm.DB, videoID, taskType string) error {
	column := VideoTaskStatusColumn(taskType)
	if column == "" || videoID == "" {
		return nil
	}
	var status string
	if err := tx.Model(&Task{}).
		Where("video_id = ? AND task_type = ?", videoID, taskType).
		Order("created_at DESC, id DESC").
		Limit(1).
		Pluck("status", &status).Error; err != nil {
		return err
	}
	if status == "" {
		var v Video
		if err := tx.Select("id", "path", "name").Where("id = ?", videoID).First(&v).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil
			}
			return err
		}
		if VideoArtifactStatuses(&v)[taskType] {
			status = TaskStatusCompleted
		}
	}
	return tx.Model(&Video{}).Where("id = ?", videoID).Update(column, status).Error
}

// taskStatusFieldValue 读取 Video 结构体上指定任务类型对应的状态字段值
func taskStatusFieldValue(v *Video, taskType string) string {
	switch taskType {
	case TaskTypeSubtitle:
		return v.SubtitleStatus
	case TaskTypeSubtitleBurn:
		return v.SubtitleBurnStatus
	case TaskTypeDeblur, TaskTypeRepair:
		return v.DeblurStatus
	case TaskTypeUpscale:
		return v.UpscaleStatus
	}
	return ""
}

// VideoResyncAllTaskStatus 全量把 videos 表各任务状态字段与最新任务记录对齐：
// 第一阶段按最新任务记录写入，无任务记录则清空字段；
// 第二阶段对仍为空的字段探测输出目录兜底，产物仍存在则视为已完成（completed）。
// 用于启动时回填历史数据，以及调度器停摆（running 批量置失败）后的一致性修复。
func VideoResyncAllTaskStatus(ctx context.Context) error {
	// 第一阶段：与最新任务记录对齐（纯 SQL）
	for _, taskType := range resyncTaskTypes {
		column := VideoTaskStatusColumn(taskType)
		stmt := fmt.Sprintf(
			"UPDATE videos SET %s = COALESCE((SELECT t.status FROM tasks t WHERE t.video_id = videos.id AND t.task_type = ? AND t.deleted_at IS NULL ORDER BY t.created_at DESC, t.id DESC LIMIT 1), '')",
			column,
		)
		if err := DB.WithContext(ctx).Exec(stmt, taskType).Error; err != nil {
			return fmt.Errorf("回填视频 %s 状态失败: %w", taskType, err)
		}
	}

	// 第二阶段：记录缺失的类型探测产物文件兜底，只补空字段、绝不降级已有状态
	var videos []*Video
	if err := DB.WithContext(ctx).
		Select("id", "path", "name", "subtitle_status", "subtitle_burn_status", "deblur_status", "upscale_status").
		Where("subtitle_status = ? OR subtitle_burn_status = ? OR deblur_status = ? OR upscale_status = ?", "", "", "", "").
		Find(&videos).Error; err != nil {
		return fmt.Errorf("查询待回填视频失败: %w", err)
	}
	for _, v := range videos {
		artifactStatuses := VideoArtifactStatuses(v)
		updates := map[string]interface{}{}
		for _, taskType := range resyncTaskTypes {
			if taskStatusFieldValue(v, taskType) == "" && artifactStatuses[taskType] {
				updates[VideoTaskStatusColumn(taskType)] = TaskStatusCompleted
			}
		}
		if len(updates) == 0 {
			continue
		}
		if err := DB.WithContext(ctx).Model(&Video{}).Where("id = ?", v.ID).Updates(updates).Error; err != nil {
			return fmt.Errorf("回填视频 %s 产物状态失败: %w", v.ID, err)
		}
	}
	return nil
}

// VideoOutputDir 计算视频的任务输出目录：
//   - 配置了 output_dir 且视频位于 video_dir 下：output_dir/<相对子目录>/<base>，
//     镜像输入目录结构，避免不同子目录下的同名视频互相覆盖；
//   - 配置了 output_dir 但视频不在 video_dir 下（如手动扫描其它路径）：output_dir/<base>；
//   - 未配置 output_dir 时使用默认输出目录 DefaultOutputDir（/output，
//     Docker 部署映射宿主机本地输出目录的标准挂载点），同样遵循上述结构；
//     不再回退到输入树，避免任务产物混入待扫描的视频源目录。
func VideoOutputDir(ctx context.Context, v *Video) string {
	base := VideoBaseName(v)
	outputDir := SettingGet(ctx, SettingKeyOutputDir)
	if outputDir == "" {
		outputDir = DefaultOutputDir
	}

	videoDir := SettingGet(ctx, SettingKeyVideoDir)
	if videoDir != "" {
		// 仅当视频在输入目录内部时镜像相对子目录结构
		if rel, err := filepath.Rel(videoDir, filepath.Dir(v.Path)); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			return filepath.Join(outputDir, rel, base)
		}
	}
	return filepath.Join(outputDir, base)
}
