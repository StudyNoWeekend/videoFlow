package logic

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gorm.io/gorm"

	"video-captions/enum"
	"video-captions/internal/dto/req"
	"video-captions/internal/dto/res"
	"video-captions/internal/model"
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
			return nil
		}
		if !model.IsVideoFile(filePath) {
			return nil
		}

		fileInfo, err := d.Info()
		if err != nil {
			return nil
		}

		// TODO: duration 在 Task 5 中通过 ffmpeg 提取，当前暂时置为 0
		_, upsertErr := model.VideoUpsertByPath(ctx, filePath, fileInfo.Size(), 0)
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
	})
	if err != nil {
		return nil, enum.ErrDatabase.WithMsg(fmt.Sprintf("查询视频列表失败: %v", err))
	}

	list := make([]*res.VideoRes, 0, len(videos))
	for _, v := range videos {
		videoRes := videoModelToRes(v)
		videoRes.SubtitleTask = taskSnapshotByVideoIDAndType(ctx, v.ID, model.TaskTypeSubtitle)
		videoRes.RepairTask = taskSnapshotByVideoIDAndType(ctx, v.ID, model.TaskTypeRepair)
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

// videoModelToRes 将视频模型转换为响应结构
func videoModelToRes(v *model.Video) *res.VideoRes {
	return &res.VideoRes{
		ID:        v.ID,
		Path:      v.Path,
		Name:      v.Name,
		Duration:  v.Duration,
		Size:      v.Size,
		CreatedAt: v.CreatedAt,
		UpdatedAt: v.UpdatedAt,
	}
}

// taskSnapshotByVideoIDAndType 查询并转换指定视频的最新指定类型任务快照
func taskSnapshotByVideoIDAndType(ctx context.Context, videoID, taskType string) *res.TaskSnapshotRes {
	task, err := model.TaskGetLatestByVideoIDAndType(ctx, videoID, taskType)
	if err != nil || task == nil {
		return nil
	}
	return &res.TaskSnapshotRes{
		ID:        task.ID,
		Status:    task.Status,
		Progress:  task.Progress,
		ErrorMsg:  task.ErrorMsg,
		UpdatedAt: task.UpdatedAt,
	}
}
