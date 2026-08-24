package logic

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"video-captions/enum"
	"video-captions/internal/dto/req"
	"video-captions/internal/dto/res"
	"video-captions/internal/ffmpeg"
	"video-captions/internal/model"
	"video-captions/utils/logger"
)

// VideoLogic 视频管理业务逻辑
type VideoLogic struct{}

// NewVideoLogic 创建视频管理 logic 实例
func NewVideoLogic() *VideoLogic {
	return &VideoLogic{}
}

// ScanConfiguredDir 读取配置的视频目录并执行扫描
func (l *VideoLogic) ScanConfiguredDir(ctx context.Context) (*res.VideoScanRes, error) {
	videoDir := model.SettingGet(ctx, model.SettingKeyVideoDir)
	if videoDir == "" {
		return nil, enum.ErrInvalidParam.WithMsg("视频目录未配置")
	}

	info, err := os.Stat(videoDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, enum.ErrInvalidParam.WithMsg("视频目录不存在")
		}
		return nil, enum.ErrInternalServer.WithMsg(fmt.Sprintf("读取视频目录失败: %v", err))
	}
	if !info.IsDir() {
		return nil, enum.ErrInvalidParam.WithMsg("视频目录不是目录")
	}

	return l.ScanDir(ctx, &req.VideoScanReq{Path: videoDir})
}

// ScanDir 递归扫描本地目录，识别视频文件并持久化
// 按文件路径去重，已存在记录时更新元信息
func (l *VideoLogic) ScanDir(ctx context.Context, scanReq *req.VideoScanReq) (*res.VideoScanRes, error) {
	path := scanReq.Path
	if path == "" {
		return nil, enum.ErrInvalidParam
	}

	// 验证目录存在且可读
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, enum.ErrInvalidParam.WithMsg("扫描目录不存在")
		}
		return nil, enum.ErrInternalServer.WithMsg(fmt.Sprintf("读取目录失败: %v", err))
	}
	if !info.IsDir() {
		return nil, enum.ErrInvalidParam.WithMsg("扫描路径不是目录")
	}

	scanned := 0
	err = filepath.WalkDir(path, func(filePath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// 跳过无权限访问的子目录或文件，继续扫描
			return nil
		}
		if d.IsDir() {
			// 跳过视频输出目录：目录名与父目录中某个视频文件同名（不含扩展名）
			if isVideoOutputDir(filePath) {
				return filepath.SkipDir
			}
			return nil
		}
		if !model.IsVideoFile(filePath) {
			return nil
		}

		fileInfo, err := d.Info()
		if err != nil {
			return nil
		}

		duration := getVideoDuration(ctx, filePath)
		width, height := getVideoResolution(ctx, filePath)
		_, upsertErr := model.VideoUpsertByPath(ctx, filePath, fileInfo.Size(), duration, width, height)
		if upsertErr != nil {
			return upsertErr
		}
		scanned++
		return nil
	})
	if err != nil {
		return nil, enum.ErrDatabase.WithMsg(fmt.Sprintf("扫描保存视频失败: %v", err))
	}

	return &res.VideoScanRes{Scanned: scanned}, nil
}

// List 分页查询视频列表
func (l *VideoLogic) List(ctx context.Context, listReq *req.VideoListReq) (*res.VideoListRes, error) {
	videos, total, err := model.VideoList(ctx, &model.VideoListQuery{
		Page:     listReq.Page,
		PageSize: listReq.PageSize,
		SortBy:   listReq.SortBy,
		Order:    listReq.Order,
	})
	if err != nil {
		return nil, enum.ErrDatabase.WithMsg(fmt.Sprintf("查询视频列表失败: %v", err))
	}

	list := make([]*res.VideoRes, 0, len(videos))
	for _, v := range videos {
		outputDir := outputDirPath(ctx, v)
		videoRes := videoModelToRes(v)
		videoRes.SubtitleTask = taskSnapshotByVideoIDAndType(v, model.TaskTypeSubtitle)
		videoRes.SubtitleBurnTask = taskSnapshotByVideoIDAndType(v, model.TaskTypeSubtitleBurn)
		videoRes.DeblurTask = taskSnapshotByVideoIDAndType(v, model.TaskTypeDeblur)
		videoRes.UpscaleTask = taskSnapshotByVideoIDAndType(v, model.TaskTypeUpscale)
		videoRes.OutputDir = outputDir
		videoRes.OutputFiles = listOutputFiles(ctx, outputDir, v)
		list = append(list, videoRes)
	}

	page := listReq.Page
	pageSize := listReq.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	return &res.VideoListRes{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// Update 更新视频信息
func (l *VideoLogic) Update(ctx context.Context, id string, updateReq *req.VideoUpdateReq) (*res.VideoRes, error) {
	if id == "" {
		return nil, enum.ErrInvalidParam
	}

	video, err := model.VideoUpdate(ctx, id, updateReq.Name)
	if err != nil {
		return nil, enum.ErrDatabase.WithMsg(fmt.Sprintf("更新视频失败: %v", err))
	}
	if video == nil {
		return nil, enum.ErrNotFound
	}

	return videoModelToRes(video), nil
}

// Delete 删除视频记录
func (l *VideoLogic) Delete(ctx context.Context, id string) error {
	if id == "" {
		return enum.ErrInvalidParam
	}

	if err := model.VideoDelete(ctx, id); err != nil {
		if err == gorm.ErrRecordNotFound {
			return enum.ErrNotFound
		}
		return enum.ErrDatabase.WithMsg(fmt.Sprintf("删除视频失败: %v", err))
	}
	return nil
}

// BatchDelete 批量删除视频记录，可选同时删除对应输出目录。
// 不存在的记录跳过，部分失败不中断整体流程。
func (l *VideoLogic) BatchDelete(ctx context.Context, deleteReq *req.VideoBatchDeleteReq) (*res.BatchDeleteRes, error) {
	seen := make(map[string]struct{}, len(deleteReq.IDs))
	result := &res.BatchDeleteRes{}
	for _, id := range deleteReq.IDs {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}

		video, err := model.VideoGetByID(ctx, id)
		if err != nil {
			return nil, enum.ErrDatabase.WithMsg(fmt.Sprintf("查询视频失败: %v", err))
		}
		if video == nil {
			result.Skipped++
			continue
		}

		if deleteReq.DeleteFiles {
			outputDir := model.VideoOutputDir(ctx, video)
			if err := os.RemoveAll(outputDir); err != nil {
				logger.Logger.Warn("删除视频输出目录失败",
					zap.String("path", outputDir),
					zap.Error(err),
				)
			}
		}

		if err := model.VideoDelete(ctx, id); err != nil {
			if err == gorm.ErrRecordNotFound {
				result.Skipped++
				continue
			}
			return nil, enum.ErrDatabase.WithMsg(fmt.Sprintf("删除视频失败: %v", err))
		}
		result.Deleted++
	}
	return result, nil
}

// videoModelToRes 将视频模型转换为响应结构
func videoModelToRes(v *model.Video) *res.VideoRes {
	return &res.VideoRes{
		ID:        v.ID,
		Path:      v.Path,
		Name:      v.Name,
		Width:     v.Width,
		Height:    v.Height,
		Duration:  v.Duration,
		Size:      v.Size,
		CreatedAt: v.CreatedAt,
		UpdatedAt: v.UpdatedAt,
	}
}

// getVideoDuration 通过 ffmpeg 获取视频时长，失败时返回 0 并仅记录日志
func getVideoDuration(ctx context.Context, path string) int64 {
	duration, err := ffmpeg.GetDuration(ctx, path)
	if err != nil {
		logger.Logger.Warn("获取视频时长失败",
			zap.String("path", path),
			zap.Error(err),
		)
		return 0
	}
	return int64(duration)
}

// getVideoResolution 通过 ffmpeg 获取视频分辨率，失败时返回 0,0 并仅记录日志
func getVideoResolution(ctx context.Context, path string) (int, int) {
	w, h, err := ffmpeg.GetResolution(ctx, path)
	if err != nil {
		logger.Logger.Warn("获取视频分辨率失败",
			zap.String("path", path),
			zap.Error(err),
		)
		return 0, 0
	}
	return w, h
}

// taskSnapshotByVideoIDAndType 返回视频指定任务类型的状态快照。
// 状态以 videos 表对应字段为准（由任务生命周期同步维护），不再依赖输出文件判断。
func taskSnapshotByVideoIDAndType(video *model.Video, taskType string) *res.TaskSnapshotRes {
	status := videoTaskStatus(video, taskType)
	if status == "" {
		return nil
	}
	return &res.TaskSnapshotRes{Status: status}
}

// videoTaskStatus 读取视频表存储的指定任务类型状态字段
func videoTaskStatus(video *model.Video, taskType string) string {
	switch taskType {
	case model.TaskTypeSubtitle:
		return video.SubtitleStatus
	case model.TaskTypeSubtitleBurn:
		return video.SubtitleBurnStatus
	case model.TaskTypeDeblur, model.TaskTypeRepair:
		return video.DeblurStatus
	case model.TaskTypeUpscale:
		return video.UpscaleStatus
	default:
		return ""
	}
}

// isVideoOutputDir 判断目录是否为视频同名输出目录（即父目录中存在 <目录名>.mp4 等视频文件）
func isVideoOutputDir(dirPath string) bool {
	parentDir := filepath.Dir(dirPath)
	dirName := filepath.Base(dirPath)
	for _, ext := range model.GetSupportedExtensions() {
		if _, err := os.Stat(filepath.Join(parentDir, dirName+ext)); err == nil {
			return true
		}
	}
	return false
}

// outputDirPath 返回视频的输出目录（集中计算逻辑见 model.VideoOutputDir）
func outputDirPath(ctx context.Context, v *model.Video) string {
	return model.VideoOutputDir(ctx, v)
}

// listOutputFiles 读取视频输出目录中的文件列表，并分类标记文件类型
func listOutputFiles(ctx context.Context, outputDir string, v *model.Video) []*res.OutputFileRes {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return nil
	}

	videoExt := filepath.Ext(v.Path)
	baseName := strings.TrimSuffix(v.Name, videoExt)

	files := make([]*res.OutputFileRes, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		// 跳过原始视频的硬链接副本（去马赛克任务创建的），避免重复
		if e.Name() == v.Name {
			continue
		}
		files = append(files, &res.OutputFileRes{
			Name:      e.Name(),
			Path:      filepath.Join(outputDir, e.Name()),
			Size:      info.Size(),
			IsVideo:   model.IsVideoFile(e.Name()),
			FileType:  classifyOutputFile(e.Name(), baseName),
			UpdatedAt: info.ModTime().Unix(),
		})
	}
	return files
}

// DirFileInfo 输出目录中视频文件的详细信息，包含分辨率
type DirFileInfo struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	FileType  string `json:"file_type"`
	UpdatedAt int64  `json:"updated_at"`
}

// DirFiles 获取视频输出目录下所有视频文件（含原视频）的详细信息，附带分辨率
func (l *VideoLogic) DirFiles(ctx context.Context, videoID string) ([]*DirFileInfo, error) {
	video, err := model.VideoGetByID(ctx, videoID)
	if err != nil {
		return nil, enum.ErrDatabase.WithMsg(fmt.Sprintf("获取视频失败: %v", err))
	}
	if video == nil {
		return nil, enum.ErrNotFound
	}

	outputDir := outputDirPath(ctx, video)
	files := make([]*DirFileInfo, 0)

	// 把原始视频加入候选列表
	files = append(files, &DirFileInfo{
		Name:      video.Name,
		Path:      video.Path,
		Size:      video.Size,
		Width:     video.Width,
		Height:    video.Height,
		FileType:  "original",
		UpdatedAt: video.UpdatedAt,
	})

	// 读取输出目录中的其他视频文件
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		// 输出目录不存在也没关系，至少返回原视频
		logger.Logger.Warn("读取输出目录失败", zap.String("dir", outputDir), zap.Error(err))
		return files, nil
	}

	videoExt := filepath.Ext(video.Path)
	baseName := strings.TrimSuffix(video.Name, videoExt)

	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		// 跳过原始视频本身（已在上面添加）
		if e.Name() == video.Name {
			continue
		}
		// 只关注视频文件
		if !model.IsVideoFile(e.Name()) {
			continue
		}

		filePath := filepath.Join(outputDir, e.Name())
		w, h := getVideoResolution(ctx, filePath)
		files = append(files, &DirFileInfo{
			Name:      e.Name(),
			Path:      filePath,
			Size:      info.Size(),
			Width:     w,
			Height:    h,
			FileType:  classifyOutputFile(e.Name(), baseName),
			UpdatedAt: info.ModTime().Unix(),
		})
	}
	return files, nil
}

// classifyOutputFile 根据文件名和视频原名判断输出文件的类型
func classifyOutputFile(fileName, videoBaseName string) string {
	name := strings.ToLower(fileName)
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)

	// 字幕文件：<video>.srt
	if ext == ".srt" && base == strings.ToLower(videoBaseName) {
		return "subtitle"
	}

	// 清晰度修复输出视频：含 _upscaled_ 特征，优先级高于 repaired 避免链式文件误分类
	if model.IsVideoFile(fileName) && (strings.Contains(base, "_upscaled")) {
		return "upscaled_video"
	}

	// 字幕合成视频：<video>_subtitled.<ext> 或 <video>_subtitled_<nonce>.<ext>
	if model.IsVideoFile(fileName) && strings.Contains(base, "_subtitled") {
		return "subtitled_video"
	}

	// 去马赛克输出视频：含 repaired / denoised / restored / enhanced / deblurred / deblur 等特征
	if model.IsVideoFile(fileName) {
		if strings.Contains(base, "repaired") ||
			strings.Contains(base, "denoised") ||
			strings.Contains(base, "restored") ||
			strings.Contains(base, "enhanced") ||
			strings.Contains(base, "deblurred") ||
			strings.Contains(base, "deblur") ||
			strings.HasPrefix(base, "repaired_") ||
			strings.HasPrefix(base, "fixed_") {
			return "repaired_video"
		}
	}

	return "unknown"
}
