package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"video-captions/bootstrap"
	"video-captions/enum"
	"video-captions/internal/ffmpeg"
	"video-captions/internal/model"
	"video-captions/internal/subtitle"
	"video-captions/utils/logger"
)

const (
	// defaultSubtitleConcurrency 默认字幕并发数
	defaultSubtitleConcurrency = 2
	// defaultSubtitleBurnConcurrency 默认字幕写入视频并发数
	defaultSubtitleBurnConcurrency = 1
	// defaultRepairConcurrency 默认修复并发数
	defaultRepairConcurrency = 1
	// defaultTranslateConcurrency 默认翻译并发数
	defaultTranslateConcurrency = 1
	// maxConcurrencyLimit 允许设置的最大并发数上限
	maxConcurrencyLimit = 50
)

// TaskScheduler 基于 SQLite 的任务调度器
type TaskScheduler struct {
	db *gorm.DB
		// subtitleConcurrency 当前允许同时运行的字幕任务数量
		subtitleConcurrency atomic.Int32
		// subtitleBurnConcurrency 当前允许同时运行的字幕写入视频任务数量
		subtitleBurnConcurrency atomic.Int32
		// repairConcurrency 当前允许同时运行的修复任务数量
		repairConcurrency atomic.Int32
	// translateConcurrency 当前允许同时运行的翻译任务数量
	translateConcurrency atomic.Int32

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	taskCh chan *model.Task

	workerMu     sync.Mutex
	workers      map[int]chan struct{}
	nextWorkerID int

	// cancelMu 保护 runningCancels：运行中任务 ID 与其独立取消函数，
	// 用于按任务取消（CancelByID 触发，中断正在执行的 ffmpeg/docker/ASR/Ollama 调用）
	cancelMu       sync.Mutex
	runningCancels map[string]context.CancelFunc
}

// Default 全局任务调度器实例，供任务取消等业务逻辑调用，在 main 中创建后赋值
var Default *TaskScheduler

// NewTaskScheduler 创建任务调度器实例
func NewTaskScheduler(db *gorm.DB) *TaskScheduler {
	return &TaskScheduler{
		db:             db,
		taskCh:         make(chan *model.Task),
		workers:        make(map[int]chan struct{}),
		runningCancels: make(map[string]context.CancelFunc),
	}
}

// Start 启动调度器，包括监管协程和 worker 池
func (s *TaskScheduler) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.loadConcurrency()

	s.wg.Add(1)
	go s.supervisorLoop()

	logger.Logger.Info("任务调度器已启动",
		zap.Int("subtitle_concurrency", int(s.subtitleConcurrency.Load())),
		zap.Int("subtitle_burn_concurrency", int(s.subtitleBurnConcurrency.Load())),
		zap.Int("repair_concurrency", int(s.repairConcurrency.Load())),
	)
	return nil
}

// Stop 优雅停止调度器，等待所有 worker 处理完当前任务后退出
func (s *TaskScheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	// 关闭任务通道，让阻塞在接收上的 worker 立即退出
	close(s.taskCh)
	s.wg.Wait()
	logger.Logger.Info("任务调度器已停止")
}

// supervisorLoop 监管循环：定期加载配置、调整 worker 数量、派发待处理任务
func (s *TaskScheduler) supervisorLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// 启动时立即执行一次
	s.loadConcurrency()
	s.adjustWorkers()
	s.dispatchPending()

	for {
		select {
		case <-ticker.C:
			s.loadConcurrency()
			s.adjustWorkers()
			s.dispatchPending()
		case <-s.ctx.Done():
			// 停止前将并发数调整为 0，通知多余 worker 退出
			s.subtitleConcurrency.Store(0)
			s.subtitleBurnConcurrency.Store(0)
			s.repairConcurrency.Store(0)
			s.adjustWorkers()
			return
		}
	}
}

// loadConcurrency 从 settings 表加载字幕/字幕写入/修复/翻译并发数
func (s *TaskScheduler) loadConcurrency() {
	subtitle := parseConcurrency(
		model.SettingGetOrDefault(s.ctx, model.SettingKeySubtitleConcurrency, model.DefaultSubtitleConcurrency),
		defaultSubtitleConcurrency,
	)
	subtitleBurn := parseConcurrency(
		model.SettingGetOrDefault(s.ctx, model.SettingKeySubtitleBurnConcurrency, model.DefaultSubtitleBurnConcurrency),
		defaultSubtitleBurnConcurrency,
	)
	repair := parseConcurrency(
		model.SettingGetOrDefault(s.ctx, model.SettingKeyRepairConcurrency, model.DefaultRepairConcurrency),
		defaultRepairConcurrency,
	)
	translate := parseConcurrency(
		model.SettingGetOrDefault(s.ctx, model.SettingKeyTranslateConcurrency, model.DefaultTranslateConcurrency),
		defaultTranslateConcurrency,
	)

	oldSubtitle := s.subtitleConcurrency.Load()
	oldSubtitleBurn := s.subtitleBurnConcurrency.Load()
	oldRepair := s.repairConcurrency.Load()
	oldTranslate := s.translateConcurrency.Load()
	if oldSubtitle != int32(subtitle) || oldSubtitleBurn != int32(subtitleBurn) || oldRepair != int32(repair) || oldTranslate != int32(translate) {
		s.subtitleConcurrency.Store(int32(subtitle))
		s.subtitleBurnConcurrency.Store(int32(subtitleBurn))
		s.repairConcurrency.Store(int32(repair))
		s.translateConcurrency.Store(int32(translate))
		logger.Logger.Info("调度器并发数已调整",
			zap.Int("old_subtitle_concurrency", int(oldSubtitle)),
			zap.Int("new_subtitle_concurrency", subtitle),
			zap.Int("old_subtitle_burn_concurrency", int(oldSubtitleBurn)),
			zap.Int("new_subtitle_burn_concurrency", subtitleBurn),
			zap.Int("old_repair_concurrency", int(oldRepair)),
			zap.Int("new_repair_concurrency", repair),
			zap.Int("old_translate_concurrency", int(oldTranslate)),
			zap.Int("new_translate_concurrency", translate),
		)
	}
}

// parseConcurrency 解析并发数字符串，非法值返回默认值
func parseConcurrency(valueStr string, defaultValue int) int {
	if valueStr == "" {
		return defaultValue
	}
	v, err := strconv.Atoi(valueStr)
	if err != nil || v < 0 || v > maxConcurrencyLimit {
		return defaultValue
	}
	return v
}

// adjustWorkers 根据当前最大并发数增加或减少 worker 协程
func (s *TaskScheduler) adjustWorkers() {
	target := int(s.subtitleConcurrency.Load() + s.subtitleBurnConcurrency.Load() + s.repairConcurrency.Load() + s.translateConcurrency.Load())

	s.workerMu.Lock()
	defer s.workerMu.Unlock()

	current := len(s.workers)
	// 增加 worker
	for i := current; i < target; i++ {
		id := s.nextWorkerID
		s.nextWorkerID++
		quit := make(chan struct{})
		s.workers[id] = quit
		s.wg.Add(1)
		go s.workerLoop(id, quit)
	}

	// 减少 worker：关闭多余 worker 的 quit 通道
	for current > target {
		for id, quit := range s.workers {
			delete(s.workers, id)
			close(quit)
			current--
			break
		}
	}
}

// workerLoop worker 主循环，从任务通道领取并执行任务
func (s *TaskScheduler) workerLoop(id int, quit <-chan struct{}) {
	defer s.wg.Done()

	for {
		select {
		case task := <-s.taskCh:
			if task == nil {
				return
			}
			s.runTask(s.ctx, task)
		case <-quit:
			return
		case <-s.ctx.Done():
			return
		}
	}
}

// runTask 为任务派生独立 context 并执行，支持按任务取消（CancelByID 触发）
func (s *TaskScheduler) runTask(ctx context.Context, task *model.Task) {
	taskCtx, taskCancel := context.WithCancel(ctx)
	s.registerCancel(task.ID, taskCancel)
	defer s.unregisterCancel(task.ID)

	// 任务在派发后、真正启动前已被请求取消（DB 状态为 cancelling）时，直接落定取消
	dbCtx := context.WithoutCancel(ctx)
	if s.isCancelling(dbCtx, task.ID) {
		s.markCancelled(taskCtx, task, "用户已取消")
		return
	}

	s.processTask(taskCtx, task)
}

// registerCancel 注册运行中任务的取消函数
func (s *TaskScheduler) registerCancel(taskID string, cancel context.CancelFunc) {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	s.runningCancels[taskID] = cancel
}

// unregisterCancel 移除运行中任务的取消函数
func (s *TaskScheduler) unregisterCancel(taskID string) {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	delete(s.runningCancels, taskID)
}

// getCancel 获取运行中任务的取消函数，任务未启动或已结束返回 nil
func (s *TaskScheduler) getCancel(taskID string) context.CancelFunc {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	return s.runningCancels[taskID]
}

// CancelByID 取消指定任务。
// - 等待中的任务：直接落定为 cancelled（原子更新，与调度器认领并发安全）；
// - 运行中的任务：置为 cancelling 并触发其独立 context 取消，正在执行的
//   ffmpeg/docker/ASR/Ollama 调用随即被中断，worker 最终将其落定为 cancelled；
// - 已是 cancelling：幂等返回成功。
func (s *TaskScheduler) CancelByID(ctx context.Context, taskID string) error {
	if taskID == "" {
		return enum.ErrInvalidParam
	}
	dbCtx := context.WithoutCancel(ctx)

	// 等待中的任务：原子更新 pending → cancelled
	res := s.db.WithContext(dbCtx).Model(&model.Task{}).
		Where("id = ? AND status = ?", taskID, model.TaskStatusPending).
		Updates(map[string]interface{}{
			"status":       model.TaskStatusCancelled,
			"error_msg":    "用户已取消",
			"progress_msg": "用户已取消",
			"updated_at":   time.Now().Unix(),
		})
	if res.Error != nil {
		return enum.ErrDatabase.WithMsg(fmt.Sprintf("取消任务失败: %v", res.Error))
	}
	if res.RowsAffected > 0 {
		logger.Logger.Info("等待中的任务已取消", zap.String("task_id", taskID))
		return nil
	}

	// 运行中/正在取消的任务：置为 cancelling 并触发取消（条件更新避免覆盖刚写入的终态）
	res = s.db.WithContext(dbCtx).Model(&model.Task{}).
		Where("id = ? AND status IN ?", taskID, []string{model.TaskStatusRunning, model.TaskStatusCancelling}).
		Updates(map[string]interface{}{
			"status":       model.TaskStatusCancelling,
			"progress_msg": "正在取消",
			"updated_at":   time.Now().Unix(),
		})
	if res.Error != nil {
		return enum.ErrDatabase.WithMsg(fmt.Sprintf("取消任务失败: %v", res.Error))
	}
	if res.RowsAffected > 0 {
		if cancel := s.getCancel(taskID); cancel != nil {
			cancel()
		}
		logger.Logger.Info("运行中的任务取消请求已发送", zap.String("task_id", taskID))
		return nil
	}

	// 条件更新未命中，说明任务已是不存在或 already 为终态（completed/failed/cancelled）
	task, err := model.TaskGetByID(dbCtx, taskID)
	if err != nil {
		return enum.ErrDatabase.WithMsg(fmt.Sprintf("查询任务失败: %v", err))
	}
	if task == nil {
		return enum.ErrTaskNotFound
	}
	return enum.ErrTaskNotCancelable
}

// isCancelling 判断任务是否处于正在取消的状态
func (s *TaskScheduler) isCancelling(ctx context.Context, taskID string) bool {
	var task model.Task
	err := s.db.WithContext(ctx).Select("status").Where("id = ?", taskID).First(&task).Error
	return err == nil && task.Status == model.TaskStatusCancelling
}

// dispatchPending 从数据库中认领 pending 任务并派发给 worker
func (s *TaskScheduler) dispatchPending() {
	s.dispatchPendingByType(model.TaskTypeSubtitle, s.subtitleConcurrency.Load())
	s.dispatchPendingByType(model.TaskTypeSubtitleBurn, s.subtitleBurnConcurrency.Load())
	s.dispatchPendingByType(model.TaskTypeDeblur, s.repairConcurrency.Load())
	s.dispatchPendingByType(model.TaskTypeTranslate, s.translateConcurrency.Load())
}

// dispatchPendingByType 按任务类型认领并派发 pending 任务
func (s *TaskScheduler) dispatchPendingByType(taskType string, limit int32) {
	if limit <= 0 {
		return
	}

	// 统计当前运行中的该类型任务数，计算空闲槽位
	runningCount, err := model.TaskCountByStatusTx(s.db.WithContext(s.ctx), model.TaskStatusRunning, taskType)
	if err != nil {
		logger.Logger.Error("统计运行中任务失败", zap.String("task_type", taskType), zap.Error(err))
		return
	}

	freeSlots := int(limit) - int(runningCount)
	if freeSlots <= 0 {
		return
	}

	for i := 0; i < freeSlots; i++ {
		var task *model.Task
		err := s.db.WithContext(s.ctx).Transaction(func(tx *gorm.DB) error {
			var e error
			task, e = model.TaskClaimPendingTx(tx, taskType)
			return e
		})
		if err != nil {
			logger.Logger.Error("认领 pending 任务失败", zap.String("task_type", taskType), zap.Error(err))
			return
		}
		if task == nil {
			return
		}

		select {
		case s.taskCh <- task:
		case <-s.ctx.Done():
			// 调度器即将停止，将已认领的任务回退为 pending，避免任务残留为 running
			if err := model.TaskResetFailedTx(s.db.WithContext(s.ctx), task.ID); err != nil {
				logger.Logger.Error("任务回退失败", zap.String("task_id", task.ID), zap.Error(err))
				return
			}
			return
		}
	}
}

// processTask 根据任务类型分发到对应的执行逻辑
func (s *TaskScheduler) processTask(ctx context.Context, task *model.Task) {
	logger.Logger.Info("开始执行任务",
		zap.String("task_id", task.ID),
		zap.String("task_type", task.TaskType),
		zap.String("video_id", task.VideoID),
	)

	// panic 保护，避免单个任务导致 worker 崩溃
	defer func() {
		if r := recover(); r != nil {
			logger.Logger.Error("任务执行发生 panic",
				zap.String("task_id", task.ID),
				zap.String("task_type", task.TaskType),
				zap.Any("recover", r),
			)
			s.markFailed(ctx, task, fmt.Errorf("任务执行异常: %v", r))
		}
	}()

		switch task.TaskType {
		case model.TaskTypeSubtitle:
			s.processSubtitleTask(ctx, task)
		case model.TaskTypeSubtitleBurn:
			s.processSubtitleBurnTask(ctx, task)
		case model.TaskTypeDeblur, model.TaskTypeRepair:
			s.processRepairTask(ctx, task)
		case model.TaskTypeTranslate:
			s.processTranslateTask(ctx, task)
	default:
		s.markFailed(ctx, task, fmt.Errorf("未知的任务类型: %s", task.TaskType))
	}
}

	
	// processSubtitleTask 执行字幕生成任务：提取音频 -> ASR -> 保存 SRT 文件
	func (s *TaskScheduler) processSubtitleTask(ctx context.Context, task *model.Task) {
	if bootstrap.ASRProvider == nil {
		s.markFailed(ctx, task, fmt.Errorf("ASR Provider 未初始化"))
		return
	}

	// 更新进度：任务启动
	s.updateProgress(ctx, task.ID, 10, "任务已启动，正在准备")

	// 1. 获取视频信息
	video, err := model.VideoGetByID(ctx, task.VideoID)
	if err != nil {
		s.markFailed(ctx, task, fmt.Errorf("获取视频信息失败: %w", err))
		return
	}
	if video == nil {
		s.markFailed(ctx, task, fmt.Errorf("视频记录不存在"))
		return
	}
	s.updateProgress(ctx, task.ID, 20, "正在获取视频信息")

	// 若视频时长为 0，尝试通过 ffmpeg 获取并更新
	if video.Duration == 0 {
		if duration, err := ffmpeg.GetDuration(ctx, video.Path); err == nil && duration > 0 {
			video.Duration = int64(duration + 0.5)
			if e := s.db.WithContext(ctx).Save(video).Error; e != nil {
				logger.Logger.Error("更新视频时长失败",
					zap.String("video_id", video.ID),
					zap.Error(e),
				)
			}
		}
	}
	s.updateProgress(ctx, task.ID, 30, "正在获取视频时长")

	// 2. 提取音频到临时目录
	tempDir, err := os.MkdirTemp("", "videoFlow-audio-*")
	if err != nil {
		s.markFailed(ctx, task, fmt.Errorf("创建音频临时目录失败: %w", err))
		return
	}
	defer os.RemoveAll(tempDir)

	audioPath, err := ffmpeg.ExtractAudio(ctx, video.Path, tempDir)
	if err != nil {
		s.markFailed(ctx, task, fmt.Errorf("提取音频失败: %w", err))
		return
	}
	s.updateProgress(ctx, task.ID, 60, "音频提取完成，正在识别语音内容")

	// 3. 调用 ASR 转录（同步接口，执行时间随文件大小而定，不设超时，无限等待）
	segments, err := bootstrap.ASRProvider.Transcribe(ctx, audioPath)
	if err != nil {
		s.markFailed(ctx, task, fmt.Errorf("ASR 转录失败: %w", err))
		return
	}
	s.updateProgress(ctx, task.ID, 90, "语音识别完成，正在生成字幕文件")

	// 4. 保存字幕结果到数据库
	resultJSON, err := json.Marshal(segments)
	if err != nil {
		s.markFailed(ctx, task, fmt.Errorf("序列化字幕结果失败: %w", err))
		return
	}

	// 5. 将字幕写入 SRT 文件，保存到视频同名输出目录
	subSegs := make([]subtitle.Segment, len(segments))
	for i, seg := range segments {
		subSegs[i] = subtitle.Segment{Start: seg.Start, End: seg.End, Text: seg.Text}
	}
	srtContent := subtitle.ToSRT(subSegs)
	videoDir := filepath.Dir(video.Path)
	videoBase := filepath.Base(video.Path)
	videoExt := filepath.Ext(videoBase)
	baseName := strings.TrimSuffix(videoBase, videoExt)
	outputDir := filepath.Join(videoDir, baseName)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		s.markFailed(ctx, task, fmt.Errorf("创建输出目录失败: %w", err))
		return
	}
	srtFilePath := filepath.Join(outputDir, baseName+".srt")

	if err := os.WriteFile(srtFilePath, []byte(srtContent), 0644); err != nil {
		s.markFailed(ctx, task, fmt.Errorf("写入 SRT 文件失败: %w", err))
		return
	}
	logger.Logger.Info("字幕文件已保存",
		zap.String("task_id", task.ID),
		zap.String("srt_file", srtFilePath),
	)

	s.markCompleted(ctx, task.ID, string(resultJSON), "字幕生成完成")
	logger.Logger.Info("字幕任务执行完成",
		zap.String("task_id", task.ID),
		zap.String("audio_path", filepath.Base(audioPath)),
		zap.String("srt_file", srtFilePath),
	)
}

// processSubtitleBurnTask 执行字幕写入视频任务：查找 SRT 文件 -> ffmpeg 烧录
func (s *TaskScheduler) processSubtitleBurnTask(ctx context.Context, task *model.Task) {
	// 更新进度：任务启动
	s.updateProgress(ctx, task.ID, 10, "任务已启动，正在准备")

	// 1. 获取视频信息
	video, err := model.VideoGetByID(ctx, task.VideoID)
	if err != nil {
		s.markFailed(ctx, task, fmt.Errorf("获取视频信息失败: %w", err))
		return
	}
	if video == nil {
		s.markFailed(ctx, task, fmt.Errorf("视频记录不存在"))
		return
	}
	s.updateProgress(ctx, task.ID, 20, "正在获取视频信息")

	// 2. 查找最近已完成字幕任务的 SRT 文件
	subtitleTask, err := model.TaskGetLatestByVideoIDAndType(ctx, task.VideoID, model.TaskTypeSubtitle)
	if err != nil {
		s.markFailed(ctx, task, fmt.Errorf("获取字幕任务失败: %w", err))
		return
	}
	if subtitleTask == nil {
		s.markFailed(ctx, task, fmt.Errorf("未找到对应的字幕任务"))
		return
	}
	if subtitleTask.Status != model.TaskStatusCompleted {
		s.markFailed(ctx, task, fmt.Errorf("字幕任务尚未完成，当前状态: %s", subtitleTask.Status))
		return
	}
	s.updateProgress(ctx, task.ID, 30, "字幕任务已完成，正在查找 SRT 文件")

	// 3. 构造 SRT 文件路径
	videoDir := filepath.Dir(video.Path)
	videoBase := filepath.Base(video.Path)
	videoExt := filepath.Ext(videoBase)
	baseName := strings.TrimSuffix(videoBase, videoExt)
	outputDir := filepath.Join(videoDir, baseName)
	srtFilePath := filepath.Join(outputDir, baseName+".srt")

	// 检查 SRT 文件是否存在
	if _, err := os.Stat(srtFilePath); err != nil {
		s.markFailed(ctx, task, fmt.Errorf("SRT 字幕文件不存在: %s", srtFilePath))
		return
	}
	s.updateProgress(ctx, task.ID, 50, "SRT 文件已找到，正在将字幕写入视频")

	// 4. 将字幕烧录到视频中（硬字幕），带实时进度回传
	subtitledPath := filepath.Join(outputDir, baseName+"_subtitled"+videoExt)
	lastBurnUpdate := time.Now()
	if err := ffmpeg.BurnSubtitles(ctx, video.Path, srtFilePath, subtitledPath, func(currentBytes, totalBytes int64) {
		// 节流：至少间隔 2 秒更新一次，避免频繁写库
		if time.Since(lastBurnUpdate) < 2*time.Second {
			return
		}
		lastBurnUpdate = time.Now()

		var msg string
		if totalBytes > 0 {
			msg = fmt.Sprintf("正在将字幕写入视频（%s/%s）", humanizeBytes(currentBytes), humanizeBytes(totalBytes))
		} else {
			msg = "正在将字幕写入视频"
		}
		s.updateProgress(ctx, task.ID, 50, msg)
	}); err != nil {
		s.markFailed(ctx, task, fmt.Errorf("字幕写入视频失败: %w", err))
		return
	}
	logger.Logger.Info("字幕已写入视频",
		zap.String("task_id", task.ID),
		zap.String("subtitled_video", subtitledPath),
	)

	s.markCompleted(ctx, task.ID, "", "字幕写入视频完成")
	logger.Logger.Info("字幕写入视频任务执行完成",
		zap.String("task_id", task.ID),
		zap.String("srt_file", srtFilePath),
		zap.String("subtitled_video", subtitledPath),
	)
}

// processRepairTask 执行视频修复任务：本地执行 Docker 修复命令 -> 保存输出
func (s *TaskScheduler) processRepairTask(ctx context.Context, task *model.Task) {
	if bootstrap.RepairExecutor == nil {
		s.markFailed(ctx, task, fmt.Errorf("视频去马赛克执行器未初始化"))
		return
	}

	// 更新进度：任务启动
	s.updateProgress(ctx, task.ID, 0, "任务已启动，正在准备")

	// 1. 获取视频信息
	video, err := model.VideoGetByID(ctx, task.VideoID)
	if err != nil {
		s.markFailed(ctx, task, fmt.Errorf("获取视频信息失败: %w", err))
		return
	}
	if video == nil {
		s.markFailed(ctx, task, fmt.Errorf("视频记录不存在"))
		return
	}
		s.updateProgress(ctx, task.ID, 50, "正在获取视频信息，准备去马赛克")

	// 2. 确定实际处理源文件：优先使用用户选择的衍生视频（如字幕合成视频），否则为原视频
	sourcePath := video.Path
	if task.SourcePath != "" {
		sourcePath = task.SourcePath
		// 源文件在创建任务时已校验为存在且位于原视频同名输出目录内
		if _, err := os.Stat(sourcePath); err != nil {
			s.markFailed(ctx, task, fmt.Errorf("处理源文件不存在: %s", sourcePath))
			return
		}
	}

	// 3. 准备输出目录：创建同名子目录；仅在源文件不在输出目录内时硬链接过去
	//    （衍生视频本就在输出目录内，直接使用，避免 os.Link 对同一文件自身报错）
	videoDir := filepath.Dir(video.Path)
	videoBase := filepath.Base(video.Path)
	videoExt := filepath.Ext(videoBase)
	baseName := strings.TrimSuffix(videoBase, videoExt)
	outputDir := filepath.Join(videoDir, baseName)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		s.markFailed(ctx, task, fmt.Errorf("创建输出目录失败: %w", err))
		return
	}
	linkedPath := sourcePath
	if filepath.Dir(sourcePath) != outputDir {
		linkedPath = filepath.Join(outputDir, videoBase)
		if err := os.Link(sourcePath, linkedPath); err != nil {
			s.markFailed(ctx, task, fmt.Errorf("创建视频硬链接失败: %w", err))
			return
		}
		// 无论修复成功或失败，最后都清理硬链接
		defer os.Remove(linkedPath)
	}

	// 4. 调用修复执行器，传入输出目录中的源文件路径，
	//    repair 会自动以 linkedPath 的父目录（即输出目录）为 Docker mount 根目录
	lastProgress := -1
	lastProgressMsg := ""
	lastUpdateAt := time.Time{}
	output, err := bootstrap.RepairExecutor.Execute(ctx, linkedPath, func(progress int, message string) {
		now := time.Now()
		progressChanged := progress != lastProgress
		msgChanged := message != "" && message != lastProgressMsg

		// 百分比变化时立即更新；消息变化时至少 5 秒更新一次，避免频繁写库
		if !progressChanged && (!msgChanged || now.Sub(lastUpdateAt) < 5*time.Second) {
			return
		}

		lastProgress = progress
		if msgChanged {
			lastProgressMsg = message
		}
		lastUpdateAt = now
		s.updateProgress(ctx, task.ID, progress, lastProgressMsg)
		logger.Logger.Info("视频去马赛克进度更新",
			zap.String("task_id", task.ID),
			zap.Int("progress", progress),
			zap.String("message", lastProgressMsg),
		)
	})
	if err != nil {
		s.markFailed(ctx, task, fmt.Errorf("视频去马赛克失败: %w\noutput: %s", err, output))
		return
	}

	s.markCompleted(ctx, task.ID, output, "视频去马赛克完成")
	logger.Logger.Info("去马赛克任务执行完成",
		zap.String("task_id", task.ID),
		zap.String("video_path", video.Path),
	)
}

// updateProgress 在事务中更新任务进度
func (s *TaskScheduler) updateProgress(ctx context.Context, taskID string, progress int, progressMsg string) {
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return model.TaskUpdateStatusTx(tx, taskID, model.TaskStatusRunning, progress, progressMsg)
	}); err != nil {
		logger.Logger.Error("更新任务进度失败",
			zap.String("task_id", taskID),
			zap.Int("progress", progress),
			zap.Error(err),
		)
	}
}

// markCompleted 在事务中将任务标记为完成
func (s *TaskScheduler) markCompleted(ctx context.Context, taskID string, resultJSON string, progressMsg string) {
	// 用户已请求取消时，落定为 cancelled 而非 completed，避免取消与完成的竞态
	if s.isCancelling(context.WithoutCancel(ctx), taskID) {
		logger.Logger.Info("任务完成前已被请求取消", zap.String("task_id", taskID))
		s.markCancelled(ctx, &model.Task{BaseModel: model.BaseModel{ID: taskID}}, "")
		return
	}
	if err := s.db.WithContext(context.WithoutCancel(ctx)).Transaction(func(tx *gorm.DB) error {
		return model.TaskUpdateResultTx(tx, taskID, resultJSON, progressMsg)
	}); err != nil {
		logger.Logger.Error("标记任务完成失败",
			zap.String("task_id", taskID),
			zap.Error(err),
		)
	}
}

// markFailed 在事务中将任务标记为失败，并将重试次数 +1
func (s *TaskScheduler) markFailed(ctx context.Context, task *model.Task, err error) {
	// 用户已请求取消时，落定为 cancelled 而非 failed
	if s.isCancelling(context.WithoutCancel(ctx), task.ID) {
		logger.Logger.Info("已执行任务被请求取消，落定为已取消",
			zap.String("task_id", task.ID),
			zap.String("task_type", task.TaskType),
			zap.Error(err),
		)
		s.markCancelled(ctx, task, "用户已取消")
		return
	}

	logger.Logger.Error("任务执行失败",
		zap.String("task_id", task.ID),
		zap.String("task_type", task.TaskType),
		zap.Error(err),
	)
	if e := s.db.WithContext(context.WithoutCancel(ctx)).Transaction(func(tx *gorm.DB) error {
		return model.TaskUpdateFailedTx(tx, task.ID, err.Error(), task.RetryCount+1)
	}); e != nil {
		logger.Logger.Error("标记任务失败状态失败",
			zap.String("task_id", task.ID),
			zap.Error(e),
		)
	}
}

// markCancelled 在事务中将任务标记为已取消
func (s *TaskScheduler) markCancelled(ctx context.Context, task *model.Task, msg string) {
	if msg == "" {
		msg = "用户已取消"
	}
	logger.Logger.Info("任务已取消",
		zap.String("task_id", task.ID),
		zap.String("task_type", task.TaskType),
	)
	if e := s.db.WithContext(context.WithoutCancel(ctx)).Transaction(func(tx *gorm.DB) error {
		return model.TaskUpdateCancelledTx(tx, task.ID, msg)
	}); e != nil {
		logger.Logger.Error("标记任务取消状态失败",
			zap.String("task_id", task.ID),
			zap.Error(e),
		)
	}
}

// processTranslateTask 执行字幕翻译任务：获取字幕 -> 翻译 -> 保存结果
func (s *TaskScheduler) processTranslateTask(ctx context.Context, task *model.Task) {
	// 1. 检查 TranslationExecutor 是否已初始化
	if bootstrap.TranslationExecutor == nil {
		s.markFailed(ctx, task, fmt.Errorf("翻译执行器未初始化"))
		return
	}

	// 更新进度：任务启动
	s.updateProgress(ctx, task.ID, 0, "")

	// 2. 获取视频信息
	video, err := model.VideoGetByID(ctx, task.VideoID)
	if err != nil {
		s.markFailed(ctx, task, fmt.Errorf("获取视频信息失败: %w", err))
		return
	}
	if video == nil {
		s.markFailed(ctx, task, fmt.Errorf("视频记录不存在"))
		return
	}
	s.updateProgress(ctx, task.ID, 10, "")

	// 3. 获取最近的字幕任务
	subtitleTask, err := model.TaskGetLatestByVideoIDAndType(ctx, task.VideoID, model.TaskTypeSubtitle)
	if err != nil {
		s.markFailed(ctx, task, fmt.Errorf("获取字幕任务失败: %w", err))
		return
	}
	if subtitleTask == nil {
		s.markFailed(ctx, task, fmt.Errorf("未找到对应的字幕任务"))
		return
	}
	if subtitleTask.Status != model.TaskStatusCompleted {
		s.markFailed(ctx, task, fmt.Errorf("字幕任务尚未完成，当前状态: %s", subtitleTask.Status))
		return
	}
	s.updateProgress(ctx, task.ID, 20, "")

	// 4. 从字幕任务的 ResultJSON 中提取字幕片段
	segments, err := subtitle.ParseSegments(subtitleTask.ResultJSON)
	if err != nil {
		s.markFailed(ctx, task, fmt.Errorf("解析字幕结果失败: %w", err))
		return
	}
	if len(segments) == 0 {
		s.markFailed(ctx, task, fmt.Errorf("字幕内容为空"))
		return
	}
	s.updateProgress(ctx, task.ID, 30, "")

	// 5. 提取所有字幕文本，只翻译文本内容，不发送时间轴和序号
	texts := make([]string, 0, len(segments))
	for _, seg := range segments {
		texts = append(texts, seg.Text)
	}
	s.updateProgress(ctx, task.ID, 40, fmt.Sprintf("共 %d 条字幕，开始翻译", len(texts)))

	// 6. 调用翻译执行器一次性翻译所有文本
	translatedTexts, err := bootstrap.TranslationExecutor.TranslateTexts(ctx, texts)
	if err != nil {
		s.markFailed(ctx, task, fmt.Errorf("翻译失败: %w", err))
		return
	}
	s.updateProgress(ctx, task.ID, 70, "翻译完成，正在生成 SRT")

	// 7. 将翻译结果回填到字幕片段中
	for i := range segments {
		if i < len(translatedTexts) {
			segments[i].Text = translatedTexts[i]
		}
	}
	s.updateProgress(ctx, task.ID, 80, "")

	// 8. 用翻译后的文本生成 SRT 文件
	translatedSRT := subtitle.ToSRT(segments)

	// 9. 保存翻译结果到文件
	// 生成输出文件路径：视频同名输出目录，文件名加 _translated.srt
	videoDir := filepath.Dir(video.Path)
	videoName := filepath.Base(video.Path)
	videoExt := filepath.Ext(videoName)
	baseName := strings.TrimSuffix(videoName, videoExt)
	outputDir := filepath.Join(videoDir, baseName)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		s.markFailed(ctx, task, fmt.Errorf("创建输出目录失败: %w", err))
		return
	}
	translatedFileName := baseName + "_translated.srt"
	translatedFilePath := filepath.Join(outputDir, translatedFileName)

	// 写入文件
	if err := os.WriteFile(translatedFilePath, []byte(translatedSRT), 0644); err != nil {
		s.markFailed(ctx, task, fmt.Errorf("保存翻译文件失败: %w", err))
		return
	}
	s.updateProgress(ctx, task.ID, 90, "翻译文件已保存")

	// 8. 标记任务完成，将翻译文件路径保存到结果中
	resultJSON := fmt.Sprintf(`{"translated_file": "%s"}`, translatedFilePath)
	s.markCompleted(ctx, task.ID, resultJSON, "字幕翻译完成")
	logger.Logger.Info("翻译任务执行完成",
		zap.String("task_id", task.ID),
		zap.String("video_id", task.VideoID),
		zap.String("translated_file", translatedFilePath),
	)
}

// humanizeBytes 将字节数格式化为可读的大小字符串，例如 122M、1.5G
func humanizeBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	switch exp {
	case 0:
		return fmt.Sprintf("%dK", bytes/div)
	case 1:
		return fmt.Sprintf("%dM", bytes/div)
	case 2:
		return fmt.Sprintf("%dG", bytes/div)
	default:
		return fmt.Sprintf("%dT", bytes/div)
	}
}
