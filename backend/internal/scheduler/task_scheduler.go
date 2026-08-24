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
	"video-captions/internal/upscale"
	"video-captions/utils/logger"
)

const (
	// defaultSubtitleConcurrency 默认字幕并发数
	defaultSubtitleConcurrency = 2
	// defaultSubtitleBurnConcurrency 默认字幕写入视频并发数
	defaultSubtitleBurnConcurrency = 1
	// defaultRepairConcurrency 默认去马赛克并发数
	defaultRepairConcurrency = 1
	// defaultUpscaleConcurrency 默认清晰度去马赛克并发数
	defaultUpscaleConcurrency = 1
	// defaultPollInterval 默认调度器轮询间隔（秒）
	defaultPollInterval = 2
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
	// repairConcurrency 当前允许同时运行的去马赛克任务数量
	repairConcurrency atomic.Int32
	// upscaleConcurrency 当前允许同时运行的清晰度去马赛克任务数量
	upscaleConcurrency atomic.Int32
	// pollInterval 调度器轮询间隔（秒），运行时可通过 settings 表动态调整
	pollInterval atomic.Int32

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
		taskCh:         make(chan *model.Task, maxConcurrencyLimit*4),
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
	// 注：不关闭 taskCh。context cancel 后 supervisor 不再投递新任务，
	// worker 通过 <-s.ctx.Done() 退出；已在 channel 中的任务在被 worker
	// 领取后会看到 ctx 已取消，执行中的操作随之终止。
	s.wg.Wait()
	logger.Logger.Info("任务调度器已停止")
}

// supervisorLoop 监管循环：定期加载配置、调整 worker 数量、派发待处理任务
func (s *TaskScheduler) supervisorLoop() {
	defer s.wg.Done()

	// 启动时立即执行一次
	s.loadConcurrency()
	s.adjustWorkers()
	s.dispatchPending()

	timer := time.NewTimer(time.Duration(s.pollInterval.Load()) * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			s.loadConcurrency()
			s.adjustWorkers()
			s.dispatchPending()
			timer.Reset(time.Duration(s.pollInterval.Load()) * time.Second)
		case <-s.ctx.Done():
			// 停止前将并发数调整为 0，通知多余 worker 退出
			s.subtitleConcurrency.Store(0)
			s.subtitleBurnConcurrency.Store(0)
			s.repairConcurrency.Store(0)
			s.upscaleConcurrency.Store(0)
			s.adjustWorkers()
			return
		}
	}
}

// loadConcurrency 从 settings 表加载字幕/字幕写入/去马赛克并发数及调度器轮询间隔
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
	upscale := parseConcurrency(
		model.SettingGetOrDefault(s.ctx, model.SettingKeyUpscaleConcurrency, model.DefaultUpscaleConcurrency),
		defaultUpscaleConcurrency,
	)
	pollInterval := parsePollInterval(
		model.SettingGetOrDefault(s.ctx, model.SettingKeySchedulerPollInterval, defaultPollIntervalStr()),
	)

	oldSubtitle := s.subtitleConcurrency.Load()
	oldSubtitleBurn := s.subtitleBurnConcurrency.Load()
	oldRepair := s.repairConcurrency.Load()
	oldUpscale := s.upscaleConcurrency.Load()
	oldPollInterval := s.pollInterval.Load()
	if oldSubtitle != int32(subtitle) || oldSubtitleBurn != int32(subtitleBurn) || oldRepair != int32(repair) || oldUpscale != int32(upscale) || oldPollInterval != int32(pollInterval) {
		s.subtitleConcurrency.Store(int32(subtitle))
		s.subtitleBurnConcurrency.Store(int32(subtitleBurn))
		s.repairConcurrency.Store(int32(repair))
		s.upscaleConcurrency.Store(int32(upscale))
		s.pollInterval.Store(int32(pollInterval))
		logger.Logger.Info("调度器并发数与轮询间隔已调整",
			zap.Int("subtitle_concurrency", subtitle),
			zap.Int("subtitle_burn_concurrency", subtitleBurn),
			zap.Int("repair_concurrency", repair),
			zap.Int("upscale_concurrency", upscale),
			zap.Int("poll_interval", pollInterval),
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

// parsePollInterval 解析调度器轮询间隔（秒），非法值返回默认值
func parsePollInterval(valueStr string) int {
	if valueStr == "" {
		return defaultPollInterval
	}
	v, err := strconv.Atoi(valueStr)
	if err != nil || v < 1 {
		return defaultPollInterval
	}
	return v
}

// defaultPollIntervalStr 调度器轮询间隔默认值：优先取配置文件，未配置时用内置默认值
func defaultPollIntervalStr() string {
	if bootstrap.Config != nil && bootstrap.Config.Scheduler.PollInterval > 0 {
		return strconv.Itoa(bootstrap.Config.Scheduler.PollInterval)
	}
	return model.DefaultSchedulerPollInterval
}

// adjustWorkers 根据当前最大并发数增加或减少 worker 协程
func (s *TaskScheduler) adjustWorkers() {
	target := int(s.subtitleConcurrency.Load() + s.subtitleBurnConcurrency.Load() + s.repairConcurrency.Load() + s.upscaleConcurrency.Load())

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

	var out taskOutput
	s.processTask(taskCtx, task, &out)

	// 任务被取消后清理遗留输出文件，避免在输出目录留下半成品
	var ended model.Task
	if err := s.db.WithContext(dbCtx).Select("status").Where("id = ?", task.ID).First(&ended).Error; err == nil && ended.Status == model.TaskStatusCancelled {
		s.cleanupCancelledOutput(&out)
	}
}

// taskOutput 记录任务执行期间产生的输出文件，用于任务取消后的清理。
// 确定性文件（如 srt、烧录/清晰度修复产物）直接记录路径；
// 输出文件名不确定的任务（去马赛克由外部程序生成）则记录执行前输出目录快照，
// 取消时删除本次执行新增的文件。
type taskOutput struct {
	produced []string
	// snapshotDir / snapshotFiles：执行前输出目录快照（仅去马赛克等文件名不确定的任务使用）
	snapshotDir   string
	snapshotFiles map[string]struct{}
}

// recordProduced 记录任务确定性产出的文件路径
func (o *taskOutput) recordProduced(path string) {
	o.produced = append(o.produced, path)
}

// snapshotOutputDir 记录输出目录执行前的文件快照
func (o *taskOutput) snapshotOutputDir(dir string) {
	o.snapshotDir = dir
	o.snapshotFiles = make(map[string]struct{})
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		o.snapshotFiles[e.Name()] = struct{}{}
	}
}

// findNewOutputVideo 在执行完成后从输出目录中找出本次执行新产生的视频文件
// （执行前快照之外的文件）。优先返回名字含 repaired 标识的文件（去马赛克产物），
// 否则返回第一个新视频文件；未找到时返回空串。
func findNewOutputVideo(dir string, snapshot map[string]struct{}) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var fallback string
	for _, e := range entries {
		if _, existed := snapshot[e.Name()]; existed {
			continue
		}
		if !model.IsVideoFile(e.Name()) {
			continue
		}
		if strings.Contains(strings.ToLower(e.Name()), "repaired") {
			return filepath.Join(dir, e.Name())
		}
		if fallback == "" {
			fallback = filepath.Join(dir, e.Name())
		}
	}
	return fallback
}

// cleanupCancelledOutput 清理任务取消后遗留的输出文件，删除失败仅记录日志不阻断
func (s *TaskScheduler) cleanupCancelledOutput(out *taskOutput) {
	for _, path := range out.produced {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			logger.Logger.Warn("清理任务取消遗留文件失败",
				zap.String("path", path),
				zap.Error(err),
			)
		} else if err == nil {
			logger.Logger.Info("已清理任务取消遗留文件", zap.String("path", path))
		}
	}

	if out.snapshotDir == "" {
		return
	}
	entries, err := os.ReadDir(out.snapshotDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if _, existed := out.snapshotFiles[e.Name()]; existed {
			continue
		}
		path := filepath.Join(out.snapshotDir, e.Name())
		if err := os.RemoveAll(path); err != nil {
			logger.Logger.Warn("清理任务取消遗留文件失败",
				zap.String("path", path),
				zap.Error(err),
			)
		} else {
			logger.Logger.Info("已清理任务取消遗留文件", zap.String("path", path))
		}
	}
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
//   - 等待中的任务：直接落定为 cancelled（原子更新，与调度器认领并发安全）；
//   - 运行中的任务：置为 cancelling 并触发其独立 context 取消，正在执行的
//     ffmpeg/docker/ASR/Ollama 调用随即被中断，worker 最终将其落定为 cancelled；
//   - 已是 cancelling：幂等返回成功。
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
		// 等待中的任务被直接落定为已取消，同步视频对应任务类型的状态字段
		// 使用 Resync 而非直接 Set cancelled：同类型可能有多个 pending 排队中，
		// 取消的不是最旧一条时，视频字段应回退到更早的任务状态（或清空），
		// 避免非最新 pending 取消后错误显示为 cancelled。
		var t model.Task
		if err := s.db.WithContext(dbCtx).Select("video_id", "task_type").Where("id = ?", taskID).First(&t).Error; err == nil {
			if err := model.VideoResyncTaskStatusTx(s.db.WithContext(dbCtx), t.VideoID, t.TaskType); err != nil {
				logger.Logger.Error("同步视频任务状态失败", zap.String("task_id", taskID), zap.Error(err))
			}
		}
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
	s.dispatchPendingByType(model.TaskTypeUpscale, s.upscaleConcurrency.Load())
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
			if e != nil || task == nil {
				return e
			}
			// 任务进入 running，同步视频对应任务类型状态字段
			return model.VideoSetTaskStatusTx(tx, task.VideoID, task.TaskType, model.TaskStatusRunning)
		})
		if err != nil {
			logger.Logger.Error("认领 pending 任务失败", zap.String("task_type", taskType), zap.Error(err))
			return
		}
		if task == nil {
			return
		}

		// 用 goroutine 异步投递，不阻塞 supervisor 循环；
		// channel 有足够缓冲区，无可用 worker 时任务堆积在 channel 中，
		// 调度器停止时通过 context cancel 放弃已认领待投递的任务。
		taskCopy := task
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			select {
			case s.taskCh <- taskCopy:
			case <-s.ctx.Done():
				// 调度器已停止，将已认领的任务回退为 pending
				if err := model.TaskResetFailedTx(s.db.WithContext(s.ctx), taskCopy.ID); err != nil {
					logger.Logger.Error("任务回退失败", zap.String("task_id", taskCopy.ID), zap.Error(err))
					return
				}
				if err := model.VideoSetTaskStatusTx(s.db.WithContext(s.ctx), taskCopy.VideoID, taskCopy.TaskType, model.TaskStatusPending); err != nil {
					logger.Logger.Error("回退视频任务状态失败", zap.String("task_id", taskCopy.ID), zap.Error(err))
				}
			}
		}()
	}
}

// processTask 根据任务类型分发到对应的执行逻辑
func (s *TaskScheduler) processTask(ctx context.Context, task *model.Task, out *taskOutput) {
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
		s.processSubtitleTask(ctx, task, out)
	case model.TaskTypeSubtitleBurn:
		s.processSubtitleBurnTask(ctx, task, out)
	case model.TaskTypeDeblur, model.TaskTypeRepair:
		s.processRepairTask(ctx, task, out)
	case model.TaskTypeUpscale:
		s.processUpscaleTask(ctx, task, out)
	default:
		s.markFailed(ctx, task, fmt.Errorf("未知的任务类型: %s", task.TaskType))
	}
}

// processSubtitleTask 执行字幕生成任务：提取音频 -> ASR -> 保存 SRT 文件
func (s *TaskScheduler) processSubtitleTask(ctx context.Context, task *model.Task, out *taskOutput) {
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
			newDuration := int64(duration + 0.5)
			if e := s.db.WithContext(ctx).Model(&model.Video{}).Where("id = ?", video.ID).Update("duration", newDuration).Error; e != nil {
				logger.Logger.Error("更新视频时长失败",
					zap.String("video_id", video.ID),
					zap.Error(e),
				)
			}
		}
	}
	s.updateProgress(ctx, task.ID, 30, "正在获取视频时长")

	// 2. 生成字幕文件（提取音频 -> ASR -> 写 SRT）
	outputDir := model.VideoOutputDir(ctx, video)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		s.markFailed(ctx, task, fmt.Errorf("创建输出目录失败: %w", err))
		return
	}
	srtFilePath := filepath.Join(outputDir, model.VideoBaseName(video)+".srt")
	out.recordProduced(srtFilePath)

	resultJSON, err := s.generateSubtitleFile(ctx, video, srtFilePath, func(progress int, msg string) {
		s.updateProgress(ctx, task.ID, progress, msg)
	})
	if err != nil {
		s.markFailed(ctx, task, err)
		return
	}
	s.updateProgress(ctx, task.ID, 95, "字幕文件生成完成")
	logger.Logger.Info("字幕文件已保存",
		zap.String("task_id", task.ID),
		zap.String("srt_file", srtFilePath),
	)

	s.markCompleted(ctx, task.ID, resultJSON, "字幕生成完成")
	logger.Logger.Info("字幕任务执行完成",
		zap.String("task_id", task.ID),
		zap.String("srt_file", srtFilePath),
	)
}

// generateSubtitleFile 提取音频并通过 ASR 转录，将 SRT 字幕写入 srtFilePath。
// onProgress 用于上报阶段进度；返回 ASR 结果 JSON，供字幕任务存入任务记录。
// 被字幕生成任务与字幕烧录任务（未检测到字幕文件时自动生成）共用。
func (s *TaskScheduler) generateSubtitleFile(ctx context.Context, video *model.Video, srtFilePath string, onProgress func(progress int, msg string)) (string, error) {
	if bootstrap.ASRProvider == nil {
		return "", fmt.Errorf("ASR Provider 未初始化")
	}

	// 提取音频到临时目录
	tempDir, err := os.MkdirTemp("", "videoFlow-audio-*")
	if err != nil {
		return "", fmt.Errorf("创建音频临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	onProgress(40, "正在提取音频")
	audioPath, err := ffmpeg.ExtractAudio(ctx, video.Path, tempDir)
	if err != nil {
		return "", fmt.Errorf("提取音频失败: %w", err)
	}
	onProgress(60, "音频提取完成，正在识别语音内容")

	// 调用 ASR 转录（同步接口，执行时间随文件大小而定，不设超时，无限等待）
	segments, err := bootstrap.ASRProvider.Transcribe(ctx, audioPath)
	if err != nil {
		return "", fmt.Errorf("ASR 转录失败: %w", err)
	}
	onProgress(90, "语音识别完成，正在生成字幕文件")

	// 序列化 ASR 结果
	resultJSON, err := json.Marshal(segments)
	if err != nil {
		return "", fmt.Errorf("序列化字幕结果失败: %w", err)
	}

	// 将字幕写入 SRT 文件
	subSegs := make([]subtitle.Segment, len(segments))
	for i, seg := range segments {
		subSegs[i] = subtitle.Segment{Start: seg.Start, End: seg.End, Text: seg.Text}
	}
	srtContent := subtitle.ToSRT(subSegs)
	if err := os.WriteFile(srtFilePath, []byte(srtContent), 0644); err != nil {
		return "", fmt.Errorf("写入 SRT 文件失败: %w", err)
	}
	return string(resultJSON), nil
}

// processSubtitleBurnTask 执行字幕写入视频任务：未生成过字幕时先自动生成，再 ffmpeg 烧录
func (s *TaskScheduler) processSubtitleBurnTask(ctx context.Context, task *model.Task, out *taskOutput) {
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

	// 2. 按「数据库记录 -> 输出目录文件 -> 自动生成」的顺序查找字幕文件：
	//    - 数据库有已完成字幕任务且 srt 文件仍在：直接使用；
	//    - 记录缺失或文件已不存在（任务被删除/旧版本产物）：再查输出目录中是否有 srt；
	//    - 仍找不到：自动生成字幕（提取音频 -> ASR -> 写 SRT），随后烧录。
	outputDir := model.VideoOutputDir(ctx, video)
	baseName := model.VideoBaseName(video)
	srtFilePath := filepath.Join(outputDir, baseName+".srt")

	// 第一步：优先按数据库记录判断（最新字幕任务已完成即视为存在，再校验文件是否仍在）
	srtFound := false
	subTask, err := model.TaskGetLatestByVideoIDAndType(ctx, video.ID, model.TaskTypeSubtitle)
	if err == nil && subTask != nil && subTask.Status == model.TaskStatusCompleted {
		if _, statErr := os.Stat(srtFilePath); statErr == nil {
			srtFound = true
			s.updateProgress(ctx, task.ID, 50, "检测到已完成的字幕任务，正在将字幕写入视频")
		}
	}

	// 第二步：数据库无完成记录（如任务被删除）或文件已不存在，再查输出目录
	if !srtFound {
		if _, statErr := os.Stat(srtFilePath); statErr == nil {
			srtFound = true
			s.updateProgress(ctx, task.ID, 50, "在输出目录中检测到字幕文件，正在将字幕写入视频")
		}
	}

	burnStart := 50
	if !srtFound {
		// 第三步：仍没有字幕文件，自动生成字幕。
		// 创建真实的任务记录到 tasks 表，避免启动回填时状态丢失。
		if bootstrap.ASRProvider == nil {
			s.markFailed(ctx, task, fmt.Errorf("未检测到字幕文件，且语音识别组件未就绪，无法自动生成字幕"))
			return
		}
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			s.markFailed(ctx, task, fmt.Errorf("创建输出目录失败: %w", err))
			return
		}
		out.recordProduced(srtFilePath)
		s.updateProgress(ctx, task.ID, 30, "未检测到字幕文件，正在自动生成字幕")

		// 创建字幕任务记录，避免重启回填时覆盖掉烧录中自动生成的字幕状态。
		// 设置初始为 running（避免调度器拦截），完成后更新为 completed/failed。
		subTask := &model.Task{
			VideoID:  video.ID,
			TaskType: model.TaskTypeSubtitle,
			Status:   model.TaskStatusRunning,
		}
		if err := model.TaskCreateTx(s.db.WithContext(ctx), subTask); err != nil {
			logger.Logger.Warn("创建字幕任务记录失败", zap.Error(err))
		}
		if err := model.VideoSetTaskStatusTx(s.db.WithContext(ctx), video.ID, model.TaskTypeSubtitle, model.TaskStatusRunning); err != nil {
			logger.Logger.Warn("同步字幕生成状态失败",
				zap.String("video_id", video.ID),
				zap.Error(err),
			)
		}

		resultJSON, genErr := s.generateSubtitleFile(ctx, video, srtFilePath, func(progress int, msg string) {
			s.updateProgress(ctx, task.ID, progress, msg)
		})
		if genErr != nil {
			// 自动生成字幕失败：将字幕任务标记为 failed
			if subTask != nil && subTask.ID != "" {
				if e := model.TaskUpdateFailedTx(s.db.WithContext(ctx), subTask.ID, genErr.Error(), 0); e != nil {
					logger.Logger.Warn("标记字幕任务失败", zap.Error(e))
				}
			}
			// 回退字幕状态到最新记录
			if e := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				return model.VideoResyncTaskStatusTx(tx, video.ID, model.TaskTypeSubtitle)
			}); e != nil {
				logger.Logger.Warn("回退字幕生成状态失败",
					zap.String("video_id", video.ID),
					zap.Error(e),
				)
			}
			s.markFailed(ctx, task, fmt.Errorf("自动生成字幕失败: %w", genErr))
			return
		}
		// 字幕生成成功：标记字幕任务为 completed
		if subTask != nil && subTask.ID != "" {
			if e := model.TaskUpdateResultTx(s.db.WithContext(ctx), subTask.ID, resultJSON, ""); e != nil {
				logger.Logger.Warn("标记字幕任务完成失败", zap.Error(e))
			}
		}
		if err := model.VideoSetTaskStatusTx(s.db.WithContext(ctx), video.ID, model.TaskTypeSubtitle, model.TaskStatusCompleted); err != nil {
			logger.Logger.Warn("同步字幕生成状态失败",
				zap.String("video_id", video.ID),
				zap.Error(err),
			)
		}
		s.updateProgress(ctx, task.ID, 95, "字幕文件生成完成，正在准备写入视频")
		burnStart = 95
	}

	// 3. 确定实际处理源文件：优先使用用户选择的衍生视频（如去马赛克视频），否则为原视频
	sourcePath := video.Path
	if task.SourcePath != "" {
		sourcePath = task.SourcePath
		// 源文件在创建任务时已校验为存在且位于原视频同名输出目录内
		if _, err := os.Stat(sourcePath); err != nil {
			s.markFailed(ctx, task, fmt.Errorf("处理源文件不存在: %s", sourcePath))
			return
		}
	}

	// 4. 将字幕烧录到视频中（硬字幕），带实时进度回传。
	//    覆盖模式（选择了衍生视频）：先烧录到临时文件，成功后替换处理源文件本身，
	//    不再生成 <base>_subtitled 新文件；原视频始终生成新文件。
	videoExt := filepath.Ext(video.Path)
	overwrite := task.Overwrite && sourcePath != video.Path
	subtitledPath := filepath.Join(outputDir, baseName+"_subtitled"+videoExt)
	burnPath := subtitledPath
	if overwrite {
		// 临时文件与源文件同目录，保证替换原子且在同一文件系统
		burnPath = filepath.Join(outputDir, fmt.Sprintf("%s_burn_tmp_%s%s", baseName, task.ID[:8], videoExt))
	} else if _, err := os.Stat(subtitledPath); err == nil {
		// 同名产物已存在，加时间戳后缀避免静默覆盖
		nonce := time.Now().UnixMilli()
		subtitledPath = filepath.Join(outputDir, fmt.Sprintf("%s_subtitled_%d%s", baseName, nonce, videoExt))
		burnPath = subtitledPath
	}
	out.recordProduced(burnPath)
	lastBurnUpdate := time.Now()
	if err := ffmpeg.BurnSubtitles(ctx, sourcePath, srtFilePath, burnPath, func(currentBytes, totalBytes int64) {
		// 节流：至少间隔 2 秒更新一次，避免频繁写库
		if time.Since(lastBurnUpdate) < 2*time.Second {
			return
		}
		lastBurnUpdate = time.Now()

		// 烧录进度映射到 [burnStart, 100)，避免长时间停留在固定百分比
		progress := burnStart
		if totalBytes > 0 {
			progress = burnStart + int(float64(100-burnStart)*float64(currentBytes)/float64(totalBytes))
			if progress > 99 {
				progress = 99
			}
		}
		var msg string
		if totalBytes > 0 {
			msg = fmt.Sprintf("正在将字幕写入视频（%s/%s）", formatMB(currentBytes), formatMB(totalBytes))
		} else {
			msg = "正在将字幕写入视频"
		}
		s.updateProgress(ctx, task.ID, progress, msg)
	}); err != nil {
		// 覆盖模式下清理残留的临时文件，避免输出目录留下半成品
		if overwrite {
			os.Remove(burnPath)
		}
		s.markFailed(ctx, task, fmt.Errorf("字幕写入视频失败: %w", err))
		return
	}
	if overwrite {
		// 用烧录结果替换处理源文件（同目录 rename，原子操作）
		if err := os.Rename(burnPath, sourcePath); err != nil {
			os.Remove(burnPath)
			s.markFailed(ctx, task, fmt.Errorf("替换处理源文件失败: %w", err))
			return
		}
		burnPath = sourcePath
	}
	logger.Logger.Info("字幕已写入视频",
		zap.String("task_id", task.ID),
		zap.String("subtitled_video", burnPath),
	)

	s.markCompleted(ctx, task.ID, "", "字幕写入视频完成")
	logger.Logger.Info("字幕写入视频任务执行完成",
		zap.String("task_id", task.ID),
		zap.String("srt_file", srtFilePath),
		zap.String("subtitled_video", burnPath),
	)
}

// processRepairTask 执行去马赛克任务：本地执行 Docker 去马赛克命令 -> 保存输出
func (s *TaskScheduler) processRepairTask(ctx context.Context, task *model.Task, out *taskOutput) {
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
	outputDir := model.VideoOutputDir(ctx, video)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		s.markFailed(ctx, task, fmt.Errorf("创建输出目录失败: %w", err))
		return
	}
	linkedPath := sourcePath
	if filepath.Dir(sourcePath) != outputDir {
		linkedPath = filepath.Join(outputDir, filepath.Base(video.Path))
		if err := os.Link(sourcePath, linkedPath); err != nil {
			s.markFailed(ctx, task, fmt.Errorf("创建视频硬链接失败: %w", err))
			return
		}
		// 无论去马赛克成功或失败，最后都清理硬链接
		defer os.Remove(linkedPath)
	}
	// 去马赛克输出文件名由外部程序生成，记录执行前快照供取消时清理新增文件
	out.snapshotOutputDir(outputDir)

	// 4. 调用去马赛克执行器，传入输出目录中的源文件路径，
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

	// 覆盖模式（选择了衍生视频）：用本次去马赛克产物替换处理源文件本身，
	// 不再保留单独的 repaired 新文件
	if task.Overwrite && sourcePath != video.Path {
		produced := findNewOutputVideo(outputDir, out.snapshotFiles)
		if produced == "" {
			s.markFailed(ctx, task, fmt.Errorf("去马赛克完成但未找到输出文件，无法覆盖处理源文件"))
			return
		}
		if err := os.Rename(produced, sourcePath); err != nil {
			s.markFailed(ctx, task, fmt.Errorf("替换处理源文件失败: %w", err))
			return
		}
		logger.Logger.Info("已用去马赛克结果覆盖处理源文件",
			zap.String("task_id", task.ID),
			zap.String("source_path", sourcePath),
		)
	}

	s.markCompleted(ctx, task.ID, output, "视频去马赛克完成")
	logger.Logger.Info("去马赛克任务执行完成",
		zap.String("task_id", task.ID),
		zap.String("video_path", video.Path),
	)
}

// processUpscaleTask 执行清晰度去马赛克任务：本地执行 Docker 清晰度去马赛克命令 -> 保存输出
func (s *TaskScheduler) processUpscaleTask(ctx context.Context, task *model.Task, out *taskOutput) {
	if bootstrap.UpscaleExecutor == nil {
		s.markFailed(ctx, task, fmt.Errorf("清晰度去马赛克执行器未初始化"))
		return
	}

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
	s.updateProgress(ctx, task.ID, 10, "正在获取视频信息")

	// 2. 确定实际处理源文件
	sourcePath := video.Path
	if task.SourcePath != "" {
		sourcePath = task.SourcePath
		if _, err := os.Stat(sourcePath); err != nil {
			s.markFailed(ctx, task, fmt.Errorf("处理源文件不存在: %s", sourcePath))
			return
		}
	}

	// 3. 确定目标分辨率
	targetWidth := task.TargetWidth
	targetHeight := task.TargetHeight
	if targetWidth <= 0 || targetHeight <= 0 {
		s.markFailed(ctx, task, fmt.Errorf("目标分辨率无效: %dx%d", targetWidth, targetHeight))
		return
	}

	// 4. 准备输出目录与执行源文件：确保实际执行源位于输出目录内，
	//    docker 以输出目录作为 bind mount 根，清晰度修复产物因此写入输出目录
	//    （原视频不在输出目录内时先硬链接过去，避免污染输入目录）
	outputDir := model.VideoOutputDir(ctx, video)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		s.markFailed(ctx, task, fmt.Errorf("创建输出目录失败: %w", err))
		return
	}
	execPath := sourcePath
	if filepath.Dir(sourcePath) != outputDir {
		execPath = filepath.Join(outputDir, filepath.Base(sourcePath))
		if err := os.Link(sourcePath, execPath); err != nil {
			s.markFailed(ctx, task, fmt.Errorf("创建视频硬链接失败: %w", err))
			return
		}
		// 无论清晰度修复成功或失败，最后都清理硬链接
		defer os.Remove(execPath)
	}
	out.recordProduced(filepath.Join(outputDir, upscale.OutputFileName(execPath, targetHeight)))

	// 5. 调用清晰度去马赛克执行器
	lastProgress := -1
	lastProgressMsg := ""
	lastUpdateAt := time.Time{}
	output, err := bootstrap.UpscaleExecutor.Execute(ctx, execPath, targetWidth, targetHeight, func(progress int, message string) {
		now := time.Now()
		progressChanged := progress != lastProgress
		msgChanged := message != "" && message != lastProgressMsg

		if !progressChanged && (!msgChanged || now.Sub(lastUpdateAt) < 5*time.Second) {
			return
		}

		lastProgress = progress
		if msgChanged {
			lastProgressMsg = message
		}
		lastUpdateAt = now
		s.updateProgress(ctx, task.ID, progress, lastProgressMsg)
		logger.Logger.Info("清晰度修复进度更新",
			zap.String("task_id", task.ID),
			zap.Int("progress", progress),
			zap.String("message", lastProgressMsg),
		)
	}, task.UpscaleProcessor, task.UpscaleModel, task.UpscaleNoiseLevel)
	if err != nil {
		// 失败时 output 恒为空，真实输出已由 Execute 拼入错误信息，避免多余的 “output: ” 后缀
		s.markFailed(ctx, task, fmt.Errorf("清晰度修复失败: %w", err))
		return
	}

	// 覆盖模式（选择了衍生视频）：用清晰度修复产物替换处理源文件本身，
	// 不再保留单独的 upscaled 新文件（产物与源文件同目录，rename 原子）
	if task.Overwrite && sourcePath != video.Path {
		if output == "" {
			s.markFailed(ctx, task, fmt.Errorf("清晰度修复完成但未找到输出文件，无法覆盖处理源文件"))
			return
		}
		if err := os.Rename(output, sourcePath); err != nil {
			s.markFailed(ctx, task, fmt.Errorf("替换处理源文件失败: %w", err))
			return
		}
		logger.Logger.Info("已用清晰度修复结果覆盖处理源文件",
			zap.String("task_id", task.ID),
			zap.String("source_path", sourcePath),
		)
	}

	s.markCompleted(ctx, task.ID, output, "清晰度修复完成")
	logger.Logger.Info("清晰度去马赛克任务执行完成",
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

// markCompleted 在事务中将任务标记为完成，并同步视频对应任务类型的状态字段
func (s *TaskScheduler) markCompleted(ctx context.Context, taskID string, resultJSON string, progressMsg string) {
	// 用户已请求取消时，落定为 cancelled 而非 completed，避免取消与完成的竞态
	if s.isCancelling(context.WithoutCancel(ctx), taskID) {
		logger.Logger.Info("任务完成前已被请求取消", zap.String("task_id", taskID))
		s.markCancelled(ctx, &model.Task{BaseModel: model.BaseModel{ID: taskID}}, "")
		return
	}
	if err := s.db.WithContext(context.WithoutCancel(ctx)).Transaction(func(tx *gorm.DB) error {
		var task model.Task
		if err := tx.Select("video_id", "task_type").Where("id = ?", taskID).First(&task).Error; err != nil {
			return err
		}
		if err := model.VideoSetTaskStatusTx(tx, task.VideoID, task.TaskType, model.TaskStatusCompleted); err != nil {
			return err
		}
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
		if err := model.VideoSetTaskStatusTx(tx, task.VideoID, task.TaskType, model.TaskStatusFailed); err != nil {
			return err
		}
		return model.TaskUpdateFailedTx(tx, task.ID, err.Error(), task.RetryCount+1)
	}); e != nil {
		logger.Logger.Error("标记任务失败状态失败",
			zap.String("task_id", task.ID),
			zap.Error(e),
		)
	}
}

// markCancelled 在事务中将任务标记为已取消，并同步视频对应任务类型的状态字段
func (s *TaskScheduler) markCancelled(ctx context.Context, task *model.Task, msg string) {
	if msg == "" {
		msg = "用户已取消"
	}
	logger.Logger.Info("任务已取消",
		zap.String("task_id", task.ID),
		zap.String("task_type", task.TaskType),
	)
	if e := s.db.WithContext(context.WithoutCancel(ctx)).Transaction(func(tx *gorm.DB) error {
		// 竞态路径传入的 task 可能只带 ID，需补查 video_id / task_type 才能同步视频字段
		t := *task
		if t.VideoID == "" || t.TaskType == "" {
			var row model.Task
			if err := tx.Select("video_id", "task_type").Where("id = ?", task.ID).First(&row).Error; err != nil {
				return err
			}
			t.VideoID, t.TaskType = row.VideoID, row.TaskType
		}
		if err := model.VideoSetTaskStatusTx(tx, t.VideoID, t.TaskType, model.TaskStatusCancelled); err != nil {
			return err
		}
		return model.TaskUpdateCancelledTx(tx, task.ID, msg)
	}); e != nil {
		logger.Logger.Error("标记任务取消状态失败",
			zap.String("task_id", task.ID),
			zap.Error(e),
		)
	}
}

// formatMB 将字节数固定格式化为 MB 单位，例如 122M、2048M
func formatMB(bytes int64) string {
	return fmt.Sprintf("%dM", bytes/(1024*1024))
}
