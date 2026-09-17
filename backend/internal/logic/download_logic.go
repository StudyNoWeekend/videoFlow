package logic

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"video-captions/enum"
	"video-captions/internal/component"
	"video-captions/internal/dto/req"
	"video-captions/internal/dto/res"
	"video-captions/internal/model"
	"video-captions/internal/telegram"
	"video-captions/utils/logger"
)

// telegramReadyTimeout 创建 Telegram 下载任务时等待连接就绪的最长时间
const telegramReadyTimeout = 5 * time.Second

// maxConcurrentTelegramDownloads 同时进行的 Telegram 下载上限。
// Telegram 对用户账号的并发请求较敏感，并发过高容易触发风控，这里做保守限制。
const maxConcurrentTelegramDownloads = 2

// DownloadLogic 下载任务管理业务逻辑
type DownloadLogic struct {
	// cancelMu 保护 runningCancels：进行中下载任务的取消函数
	cancelMu       sync.Mutex
	runningCancels map[string]context.CancelFunc

	// pathMu 保护 runningPaths：进行中下载任务的目标文件路径
	pathMu       sync.Mutex
	runningPaths map[string]string

	// tgSlots 限制同时进行的 Telegram 下载数量
	tgSlots chan struct{}

	// wg 跟踪运行中的下载 goroutine，保证优雅关闭时等待其落定终态
	wg sync.WaitGroup
}

// NewDownloadLogic 创建下载任务 logic 实例
func NewDownloadLogic() *DownloadLogic {
	return &DownloadLogic{
		runningCancels: make(map[string]context.CancelFunc),
		runningPaths:   make(map[string]string),
		tgSlots:        make(chan struct{}, maxConcurrentTelegramDownloads),
	}
}

var (
	globalDL     *DownloadLogic
	globalDLOnce sync.Once
)

// GetGlobalDownloadLogic 返回全局唯一的 DownloadLogic 实例
func GetGlobalDownloadLogic() *DownloadLogic {
	globalDLOnce.Do(func() { globalDL = NewDownloadLogic() })
	return globalDL
}

// CreateDownload 创建下载任务，校验 URL 与依赖组件后启动后台下载
func (l *DownloadLogic) CreateDownload(ctx context.Context, createReq *req.DownloadCreateReq) (*res.DownloadRes, error) {
	if createReq.URL == "" {
		return nil, enum.ErrInvalidParam.WithMsg("视频链接不能为空")
	}
	// 仅允许 http/https 链接，防止以 -- 开头的输入被 yt-dlp 解析为命令行选项（参数注入）
	if !isValidDownloadURL(createReq.URL) {
		return nil, enum.ErrInvalidParam.WithMsg("视频链接必须以 http:// 或 https:// 开头")
	}

	// 识别链接所属平台，Telegram 链接改走 MTProto 下载
	platform := DetectPlatform(createReq.URL)
	if err := checkPlatformDependencies(ctx, platform); err != nil {
		return nil, err
	}

	download := &model.Download{
		URL:         createReq.URL,
		Platform:    platform,
		Status:      model.DownloadStatusPending,
		Overwrite:   createReq.Overwrite,
		DownloadDir: createReq.DownloadDir,
	}
	if err := model.DownloadCreate(ctx, download); err != nil {
		logger.WithTraceID(ctx).Error("创建下载任务失败", zap.Error(err))
		return nil, enum.ErrDatabase
	}

	// 启动后台 goroutine 执行下载
	dlCtx, dlCancel := context.WithCancel(context.Background())
	l.registerCancel(download.ID, dlCancel)
	l.wg.Add(1)
	go l.runDownload(dlCtx, download)

	return downloadToRes(download), nil
}

// isValidDownloadURL 校验下载链接必须为 http/https 协议，防止参数注入
func isValidDownloadURL(u string) bool {
	return strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://")
}

// DetectPlatform 根据链接识别下载平台：Telegram 链接走 MTProto，其余交给 yt-dlp
func DetectPlatform(rawURL string) string {
	if telegram.IsTelegramURL(rawURL) {
		return model.DownloadPlatformTelegram
	}
	return model.DownloadPlatformYtDlp
}

// checkPlatformDependencies 校验指定平台所需的依赖是否就绪
func checkPlatformDependencies(ctx context.Context, platform string) error {
	if platform != model.DownloadPlatformTelegram {
		if missing := component.CheckTaskDependencies(ctx, model.TaskTypeDownload); len(missing) > 0 {
			return componentMissingErr(missing)
		}
		return nil
	}

	engine := telegram.Global()
	if engine == nil || !engine.Configured() {
		return enum.ErrTelegramNotConfigured
	}

	// 连接可能仍在建立中，短暂等待后再判定登录状态
	_, authorized, err := engine.ReadyState(ctx, telegramReadyTimeout)
	if err != nil {
		return enum.ErrTelegramNotReady
	}
	if !authorized {
		return enum.ErrTelegramNotLoggedIn
	}
	return nil
}

// ListDownloads 分页查询下载任务列表
func (l *DownloadLogic) ListDownloads(ctx context.Context, listReq *req.DownloadListReq) (*res.DownloadListRes, error) {
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

	items, total, err := model.DownloadList(ctx, &model.DownloadListQuery{
		Page:     page,
		PageSize: pageSize,
		SortBy:   listReq.SortBy,
		Order:    listReq.Order,
	})
	if err != nil {
		logger.WithTraceID(ctx).Error("查询下载列表失败", zap.Error(err))
		return nil, enum.ErrDatabase
	}

	list := make([]*res.DownloadRes, 0, len(items))
	for _, item := range items {
		list = append(list, downloadToRes(item))
	}

	return &res.DownloadListRes{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// CancelDownload 取消进行中的下载任务
func (l *DownloadLogic) CancelDownload(ctx context.Context, id string) (*res.DownloadRes, error) {
	if id == "" {
		return nil, enum.ErrInvalidParam
	}

	dl, err := model.DownloadGetByID(ctx, id)
	if err != nil {
		logger.WithTraceID(ctx).Error("查询下载任务失败", zap.String("download_id", id), zap.Error(err))
		return nil, enum.ErrDatabase
	}
	if dl == nil {
		return nil, enum.ErrDownloadNotFound
	}

	// 只有 pending/probing/downloading 可以取消
	if dl.Status != model.DownloadStatusPending &&
		dl.Status != model.DownloadStatusProbing &&
		dl.Status != model.DownloadStatusDownloading {
		return nil, enum.ErrTaskNotCancelable
	}

	// 触发取消（杀死 yt-dlp 进程）
	if cancel := l.getCancel(id); cancel != nil {
		cancel()
	}

	// 清理 yt-dlp 遗留的 .part / .ytdl 缓存文件
	if path := l.unregisterPath(id); path != "" {
		l.cleanupDownloadCache(path)
	}

	// 带状态守卫地落定取消终态，避免覆盖已完成的下载（终态竞态）
	affected, err := model.DownloadMarkCancelled(ctx, id)
	if err != nil {
		logger.WithTraceID(ctx).Error("取消下载任务失败", zap.String("download_id", id), zap.Error(err))
		return nil, enum.ErrDatabase
	}
	if affected == 0 {
		// 并发窗口内任务已进入终态（如刚好下载完成）
		return nil, enum.ErrTaskNotCancelable
	}

	dl, err = model.DownloadGetByID(ctx, id)
	if err != nil || dl == nil {
		return nil, enum.ErrDatabase
	}
	return downloadToRes(dl), nil
}

// DeleteDownload 删除下载记录
func (l *DownloadLogic) DeleteDownload(ctx context.Context, id string, deleteFile bool) error {
	if id == "" {
		return enum.ErrInvalidParam
	}

	dl, err := model.DownloadGetByID(ctx, id)
	if err != nil {
		logger.WithTraceID(ctx).Error("查询下载任务失败", zap.String("download_id", id), zap.Error(err))
		return enum.ErrDatabase
	}
	if dl == nil {
		return enum.ErrDownloadNotFound
	}

	// 运行中的不允许删除
	if dl.Status == model.DownloadStatusDownloading || dl.Status == model.DownloadStatusProbing {
		return enum.ErrTaskRunningCannotDelete
	}

	// 如果勾选了删除本地文件
	if deleteFile && dl.FileName != "" && dl.FileSize > 0 {
		// 下载目录优先取任务自身记录，其次回退到本地视频目录
		dir := dl.DownloadDir
		if dir == "" {
			dir = getInputDir(ctx)
		}
		if dir != "" {
			filePath := filepath.Join(dir, dl.FileName)
			if _, statErr := os.Stat(filePath); statErr == nil {
				if removeErr := os.Remove(filePath); removeErr != nil {
					logger.WithTraceID(ctx).Warn("删除下载文件失败",
						zap.String("download_id", id),
						zap.String("file_path", filePath),
						zap.Error(removeErr),
					)
				} else {
					logger.WithTraceID(ctx).Info("已删除下载文件",
						zap.String("download_id", id),
						zap.String("file_path", filePath),
					)
				}
			}
		}
	}

	if err := model.DownloadDelete(ctx, id); err != nil {
		if err == gorm.ErrRecordNotFound {
			return enum.ErrDownloadNotFound
		}
		logger.WithTraceID(ctx).Error("删除下载记录失败", zap.String("download_id", id), zap.Error(err))
		return enum.ErrDatabase
	}
	return nil
}

// probeResult 探测阶段得到的媒体元信息
type probeResult struct {
	// Title 视频标题（不含扩展名）
	Title string
	// Duration 时长（秒）
	Duration int64
	// Ext 预期扩展名，含前导点
	Ext string
}

// runDownload 后台执行下载任务（在独立 goroutine 中运行）
func (l *DownloadLogic) runDownload(ctx context.Context, dl *model.Download) {
	defer l.wg.Done()
	defer l.unregisterCancel(dl.ID)
	defer l.unregisterPath(dl.ID)

	logger.WithTraceID(ctx).Info("开始执行下载任务",
		zap.String("download_id", dl.ID),
		zap.String("platform", dl.Platform),
		zap.String("url", dl.URL),
	)

	dbCtx := context.WithoutCancel(ctx)

	// 1. 探测阶段：获取视频元信息
	l.updateProgress(dbCtx, dl.ID, model.DownloadStatusProbing, 0, "正在解析视频信息...")

	probe, err := l.probeMedia(ctx, dl)
	if err != nil {
		// 检查是否为取消操作
		if l.isCancelled(ctx) {
			_, _ = model.DownloadMarkCancelled(dbCtx, dl.ID)
			return
		}
		logger.WithTraceID(ctx).Error("解析视频信息失败", zap.String("download_id", dl.ID), zap.Error(err))
		model.DownloadMarkFailed(dbCtx, dl.ID, fmt.Sprintf("解析视频信息失败: %v", err))
		return
	}

	// 更新标题和时长
	model.DB.WithContext(dbCtx).Model(&model.Download{}).Where("id = ?", dl.ID).Updates(map[string]interface{}{
		"title":        probe.Title,
		"duration":     probe.Duration,
		"status":       model.DownloadStatusDownloading,
		"progress":     0,
		"progress_msg": "正在下载...",
	})

	logger.WithTraceID(ctx).Info("视频信息解析完成",
		zap.String("download_id", dl.ID),
		zap.String("title", probe.Title),
		zap.Int64("duration", probe.Duration),
	)

	// 2. 下载阶段：获取下载目录（用户指定dir优先，否则用本地视频目录）
	inputDir := dl.DownloadDir
	if inputDir == "" {
		inputDir = getInputDir(dbCtx)
	}
	if inputDir == "" {
		model.DownloadMarkFailed(dbCtx, dl.ID, "下载目录未配置，请在设置中配置视频目录或指定下载路径")
		return
	}
	if err := os.MkdirAll(inputDir, 0755); err != nil {
		model.DownloadMarkFailed(dbCtx, dl.ID, fmt.Sprintf("创建输入目录失败: %v", err))
		return
	}

	var (
		downloadedPath string
		fileSize       int64
	)
	if dl.Platform == model.DownloadPlatformTelegram {
		downloadedPath, fileSize, err = l.downloadFromTelegram(ctx, dl, probe, inputDir)
	} else {
		downloadedPath, fileSize, err = l.downloadFromYtDlp(ctx, dl, probe, inputDir)
	}
	if err != nil {
		// 检查是否为取消操作
		if l.isCancelled(ctx) {
			_, _ = model.DownloadMarkCancelled(dbCtx, dl.ID)
			return
		}
		logger.WithTraceID(ctx).Error("下载失败", zap.String("download_id", dl.ID), zap.Error(err))
		model.DownloadMarkFailed(dbCtx, dl.ID, fmt.Sprintf("下载失败: %v", err))
		return
	}

	// 3. 完成阶段：将文件录入视频库
	if fileSize <= 0 {
		if fi, statErr := os.Stat(downloadedPath); statErr == nil {
			fileSize = fi.Size()
		}
	}
	fileName := filepath.Base(downloadedPath)

	// 调用 VideoUpsertByPath 写入 videos 表
	if _, upsertErr := model.VideoUpsertByPath(dbCtx, downloadedPath, fileSize, probe.Duration, 0, 0); upsertErr != nil {
		logger.WithTraceID(ctx).Warn("下载完成但录入视频库失败",
			zap.String("download_id", dl.ID),
			zap.String("path", downloadedPath),
			zap.Error(upsertErr),
		)
		// 不阻断，仍标记下载完成
	}

	model.DownloadMarkCompleted(dbCtx, dl.ID, fileName, fileSize, probe.Duration)
	logger.WithTraceID(ctx).Info("下载任务执行完成",
		zap.String("download_id", dl.ID),
		zap.String("file", downloadedPath),
	)
}

// probeMedia 按平台探测视频元信息
func (l *DownloadLogic) probeMedia(ctx context.Context, dl *model.Download) (*probeResult, error) {
	if dl.Platform == model.DownloadPlatformTelegram {
		return probeTelegram(ctx, dl.URL)
	}

	title, duration, err := l.probeVideoInfo(ctx, dl.URL)
	if err != nil {
		return nil, err
	}
	return &probeResult{Title: title, Duration: duration, Ext: ".mp4"}, nil
}

// probeTelegram 通过 Telegram 接口解析消息中的媒体
func probeTelegram(ctx context.Context, rawURL string) (*probeResult, error) {
	engine := telegram.Global()
	if engine == nil {
		return nil, errors.New("Telegram 引擎未初始化")
	}

	info, err := engine.ResolveMedia(ctx, rawURL)
	if err != nil {
		return nil, err
	}

	ext := filepath.Ext(info.Name)
	title := strings.TrimSuffix(info.Name, ext)
	if title == "" {
		title = fmt.Sprintf("telegram_%d", info.MessageID)
	}
	if ext == "" {
		ext = ".mp4"
	}

	return &probeResult{Title: title, Duration: info.Duration, Ext: ext}, nil
}

// downloadFromYtDlp 调用 yt-dlp 下载，最终扩展名由 yt-dlp 决定
func (l *DownloadLogic) downloadFromYtDlp(ctx context.Context, dl *model.Download, probe *probeResult, inputDir string) (string, int64, error) {
	// 处理输出文件冲突：overwrite=false 时自动编号，true 时直接覆盖
	outputTemplate := filepath.Join(inputDir, "%(title)s.%(ext)s")
	if !dl.Overwrite {
		// 预估目标路径（yt-dlp 默认用 mp4），检查是否冲突
		titleSafe := sanitizeFilename(probe.Title)
		expectedPath := filepath.Join(inputDir, titleSafe+".mp4")
		uniquePath := resolveUniquePath(expectedPath)
		if uniquePath != expectedPath {
			// 有冲突，使用带编号的模板
			baseName := strings.TrimSuffix(filepath.Base(uniquePath), ".mp4")
			outputTemplate = filepath.Join(inputDir, baseName+".%(ext)s")
			logger.WithTraceID(ctx).Info("文件冲突，自动编号",
				zap.String("download_id", dl.ID),
				zap.String("original", expectedPath),
				zap.String("unique", uniquePath),
			)
		}
	}

	// 用预测路径预注册到 runningPaths（executeDownload 会从 Destination 行更新为实际路径）
	titleSafe := sanitizeFilename(probe.Title)
	predictedPath := strings.ReplaceAll(outputTemplate, "%(title)s", titleSafe)
	predictedPath = strings.ReplaceAll(predictedPath, "%(ext)s", "mp4")
	l.registerPath(dl.ID, predictedPath)

	return l.executeDownload(ctx, dl.ID, dl.URL, outputTemplate, dl.Overwrite)
}

// downloadFromTelegram 通过内置 MTProto 客户端下载，目标路径在下载前即可确定
func (l *DownloadLogic) downloadFromTelegram(ctx context.Context, dl *model.Download, probe *probeResult, inputDir string) (string, int64, error) {
	engine := telegram.Global()
	if engine == nil {
		return "", 0, errors.New("Telegram 引擎未初始化")
	}

	// 排队获取并发槽位：同时进行的 Telegram 下载过多容易触发账号风控
	select {
	case l.tgSlots <- struct{}{}:
		defer func() { <-l.tgSlots }()
	case <-ctx.Done():
		return "", 0, ctx.Err()
	}

	targetPath := filepath.Join(inputDir, sanitizeFilename(probe.Title)+probe.Ext)
	if !dl.Overwrite {
		targetPath = resolveUniquePath(targetPath)
	}
	l.registerPath(dl.ID, targetPath)

	tracker := &speedTracker{}
	onProgress := func(downloaded, total int64) {
		if total <= 0 {
			return
		}

		pct := float64(downloaded) / float64(total) * 100
		progress := int(math.Round(pct))
		// 完成前不展示 100%，与 yt-dlp 分支保持一致
		if progress > 99 {
			progress = 99
		}

		dbCtx := context.WithoutCancel(ctx)
		model.DB.WithContext(dbCtx).Model(&model.Download{}).Where("id = ?", dl.ID).Updates(map[string]interface{}{
			"status":          model.DownloadStatusDownloading,
			"progress":        progress,
			"progress_msg":    fmt.Sprintf("正在下载...  %.1f%%", pct),
			"download_speed":  tracker.update(downloaded),
			"total_size":      total,
			"downloaded_size": downloaded,
			"updated_at":      time.Now().Unix(),
		})
	}

	if err := engine.DownloadMedia(ctx, dl.URL, targetPath, onProgress); err != nil {
		// 删除不完整的文件，避免被视频扫描器当作正常文件入库
		if removeErr := os.Remove(targetPath); removeErr != nil && !os.IsNotExist(removeErr) {
			logger.WithTraceID(ctx).Warn("清理未完成的下载文件失败",
				zap.String("download_id", dl.ID),
				zap.String("path", targetPath),
				zap.Error(removeErr),
			)
		}
		return "", 0, err
	}

	fi, statErr := os.Stat(targetPath)
	if statErr != nil {
		return "", 0, fmt.Errorf("下载完成后读取文件失败: %w", statErr)
	}
	return targetPath, fi.Size(), nil
}

// speedTracker 由进度回调推算下载速度
type speedTracker struct {
	mu       sync.Mutex
	lastAt   time.Time
	lastSize int64
	speed    int64
}

func (s *speedTracker) update(downloaded int64) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if !s.lastAt.IsZero() {
		if elapsed := now.Sub(s.lastAt).Seconds(); elapsed > 0 {
			s.speed = int64(float64(downloaded-s.lastSize) / elapsed)
		}
	}
	s.lastAt = now
	s.lastSize = downloaded
	return s.speed
}

// isCancelled 检查 context 是否被取消
func (l *DownloadLogic) isCancelled(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

// ytDlpProxyArgs 返回 yt-dlp 的代理参数；未启用或未配置代理时返回 nil。
// 返回值必须插在 "--" 终止符之前，否则会被 yt-dlp 当成待下载的 URL。
func ytDlpProxyArgs(ctx context.Context) []string {
	return buildProxyArgs(proxyEnabledForYtdlp(ctx), getProxyURL(ctx))
}

// buildProxyArgs 由「是否启用」与「代理地址」构造 yt-dlp 代理参数
func buildProxyArgs(enabled bool, proxyURL string) []string {
	if !enabled || proxyURL == "" {
		return nil
	}
	return []string{"--proxy", proxyURL}
}

// probeVideoInfo 探测视频标题和时长
func (l *DownloadLogic) probeVideoInfo(ctx context.Context, url string) (title string, duration int64, err error) {
	// 元信息探测整体限时，避免远端挂起时无限占用工作槽
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	proxyArgs := ytDlpProxyArgs(ctx)

	// 获取标题
	titleArgs := append(proxyArgs, "--print", "title", "--", url)
	titleCmd := exec.CommandContext(probeCtx, "yt-dlp", titleArgs...)
	titleCmd.Env = os.Environ()
	titleCmd.Env = append(titleCmd.Env, "PYTHONUNBUFFERED=1")
	titleOut, titleErr := titleCmd.Output()
	if titleErr != nil {
		return "", 0, fmt.Errorf("获取视频标题失败: %w\noutput: %s", titleErr, string(titleOut))
	}
	title = strings.TrimSpace(string(titleOut))
	if title == "" {
		title = "unknown"
	}

	// 获取时长（秒）
	durationArgs := append(proxyArgs, "--print", "duration_string", "--", url)
	durationCmd := exec.CommandContext(probeCtx, "yt-dlp", durationArgs...)
	durationCmd.Env = os.Environ()
	durationCmd.Env = append(durationCmd.Env, "PYTHONUNBUFFERED=1")
	durOut, durErr := durationCmd.Output()
	if durErr == nil {
		durationStr := strings.TrimSpace(string(durOut))
		duration = parseDurationStr(durationStr)
	}

	return title, duration, nil
}

// 解析 yt-dlp 的 duration_string（格式 HH:MM:SS 或 MM:SS）
func parseDurationStr(s string) int64 {
	parts := strings.Split(s, ":")
	if len(parts) == 0 {
		return 0
	}
	var total int64
	// 从右往左解析
	for i := len(parts) - 1; i >= 0; i-- {
		v, err := strconv.ParseInt(parts[i], 10, 64)
		if err != nil {
			return 0
		}
		if i == len(parts)-1 {
			total += v // 秒
		} else if i == len(parts)-2 {
			total += v * 60 // 分
		} else {
			total += v * 3600 // 时
		}
	}
	return total
}

// ytDlpProgressRegex 匹配 yt-dlp 的进度行，如：
// [download]  45.2% of 450.36MiB at 4.41MiB/s ETA 01:42
// [download]   0.0% of    1.25GiB at Unknown ETA
// [download] 100.0% of 450.36MiB
var ytDlpProgressRegex = regexp.MustCompile(`\[download\]\s+(\d+\.?\d*)%\s+of\s+(\d+\.?\d*)\s*(\w+)\s+at\s+(\d+\.?\d*)\s*(\w+)/s`)

// sizeUnitMap 大小单位到字节的映射
var sizeUnitMap = map[string]int64{
	"B":   1,
	"KiB": 1024,
	"MiB": 1024 * 1024,
	"GiB": 1024 * 1024 * 1024,
	"TiB": 1024 * 1024 * 1024 * 1024,
	"KB":  1000,
	"MB":  1000 * 1000,
	"GB":  1000 * 1000 * 1000,
}

// parseByteSize 解析 yt-dlp 的大小字符串为字节数，如 "450.36MiB" → 472280719
func parseByteSize(valueStr, unitStr string) int64 {
	v, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return 0
	}
	unit := sizeUnitMap[unitStr]
	if unit == 0 {
		return 0
	}
	return int64(v * float64(unit))
}

// executeDownload 执行 yt-dlp 下载并解析进度
func (l *DownloadLogic) executeDownload(ctx context.Context, downloadID, url, outputTemplate string, overwrite bool) (downloadedPath string, fileSize int64, err error) {
	args := []string{
		"-o", outputTemplate,
		"--newline",
	}
	args = append(args, ytDlpProxyArgs(ctx)...)
	if overwrite {
		args = append(args, "--force-overwrites")
	}
	// "--" 终止选项解析，防止 URL 内容被 yt-dlp 当作选项（纵深防御，入参已校验 scheme）。
	// 必须放在所有选项之后，否则 "--" 后面的内容会被当成额外的 URL。
	args = append(args, "--", url)

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	// 设置 PYTHONUNBUFFERED=1，确保 yt-dlp（Python 进程）的输出即时刷新
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "PYTHONUNBUFFERED=1")

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", 0, fmt.Errorf("创建 stderr pipe 失败: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", 0, fmt.Errorf("创建 stdout pipe 失败: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", 0, fmt.Errorf("启动 yt-dlp 失败: %w", err)
	}

	// 并发读取 stderr 收集错误信息（正常运行时 stderr 内容很少，仅错误报告）
	var stderrBuf strings.Builder
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			stderrBuf.WriteString(scanner.Text())
			stderrBuf.WriteString("\n")
		}
	}()

	// 从 stdout 读取所有输出行（包含进度和文件路径）
	// yt-dlp 新版（2026+）将进度输出到 stdout
	// 注意：B站等平台将视频流和音频流分开下载，每段独立输出 0-100% 进度。
	// 需要跨流累积计算整体进度，避免切换流时进度暴跌。
	lastUpdate := time.Now()
	lastProgress := -1
	var dlPath string
	// 跨流累积统计
	var completedSize int64       // 已完成流的字节数
	var currentStreamTotal int64  // 当前流的总大小
	var isFirstStream bool = true // 是否第一条流（跳过累积重置）

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*64), 1024*64)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// 检测新的流开始（Destination 行标志着流切换）
		if strings.Contains(line, "[download] Destination: ") {
			if !isFirstStream {
				// 上一条流已完成，将它的 total 计入 completedSize
				completedSize += currentStreamTotal
			}
			isFirstStream = false
			currentStreamTotal = 0
		}

		// 检测文件路径相关的输出行
		newPath := parseDownloadPath(line, dlPath)
		if newPath != dlPath {
			dlPath = newPath
			// 将实时路径同步到 runningPaths，以便取消时能精确清理缓存
			l.registerPath(downloadID, dlPath)
		}

		// 匹配进度、总大小、速度
		if matches := ytDlpProgressRegex.FindStringSubmatch(line); len(matches) >= 6 {
			pctStr := matches[1]
			sizeVal := matches[2]
			sizeUnit := matches[3]
			speedVal := matches[4]
			speedUnit := matches[5]

			pct, parseErr := strconv.ParseFloat(pctStr, 64)
			if parseErr == nil {
				streamTotal := parseByteSize(sizeVal, sizeUnit)
				speed := parseByteSize(speedVal, speedUnit)
				streamDownloaded := int64(float64(streamTotal) * pct / 100.0)

				// 更新当前流的总大小（用每次遇到的最大值，因为第一行可能是 0.0% 时 total 已确定）
				if streamTotal > currentStreamTotal {
					currentStreamTotal = streamTotal
				}

				// 整体大小 = 已完成流总和 + 当前流
				overallTotal := completedSize + currentStreamTotal
				overallDownloaded := completedSize + streamDownloaded

				// 计算整体进度百分比
				var overallPct float64
				if overallTotal > 0 {
					overallPct = float64(overallDownloaded) / float64(overallTotal) * 100.0
				}
				progress := int(math.Round(overallPct))
				if progress > 99 {
					progress = 99
				}

				// 只在百分比变化或至少 1 秒时才更新 DB 写入
				if progress != lastProgress || time.Since(lastUpdate) >= time.Second {
					lastProgress = progress

					// 一次更新：进度、描述、速度、大小
					dbCtx := context.WithoutCancel(ctx)
					model.DB.WithContext(dbCtx).Model(&model.Download{}).Where("id = ?", downloadID).Updates(map[string]interface{}{
						"status":          model.DownloadStatusDownloading,
						"progress":        progress,
						"progress_msg":    fmt.Sprintf("正在下载...  %.1f%%", overallPct),
						"download_speed":  speed,
						"total_size":      overallTotal,
						"downloaded_size": overallDownloaded,
						"updated_at":      time.Now().Unix(),
					})

					lastUpdate = time.Now()
				}
			}
		}
	}

	err = cmd.Wait()
	if err != nil {
		// 检查是否是被取消
		select {
		case <-ctx.Done():
			return "", 0, ctx.Err()
		default:
		}
		// 等待 stderr 采集 goroutine 结束后再读取，避免 data race
		<-stderrDone
		// 拼入 stderr 输出以获得详细错误信息
		stderrStr := strings.TrimSpace(stderrBuf.String())
		if stderrStr != "" {
			return "", 0, fmt.Errorf("yt-dlp 执行失败: %w\n%s", err, stderrStr)
		}
		return "", 0, fmt.Errorf("yt-dlp 执行失败: %w", err)
	}

	// 成功路径同样等待 stderr goroutine 结束（收集诊断信息用）
	<-stderrDone

	// 检查 yt-dlp 是否产生了输出文件（exit 0 但无文件 = extractor 未匹配）
	if dlPath == "" {
		stderrStr := strings.TrimSpace(stderrBuf.String())
		if stderrStr != "" {
			return "", 0, fmt.Errorf("yt-dlp 未生成文件: %s", stderrStr)
		}
		return "", 0, fmt.Errorf("yt-dlp 未生成文件，链接可能不受支持")
	}

	downloadedPath = dlPath

	// 获取文件大小
	if downloadedPath != "" {
		if fi, statErr := os.Stat(downloadedPath); statErr == nil {
			fileSize = fi.Size()
		}
	}

	return downloadedPath, fileSize, nil
}

// parseDownloadPath 从 yt-dlp 输出行中解析下载文件路径。
// 后续行会覆盖前面的路径（如合并后生成最终路径）。
func parseDownloadPath(line, currentPath string) string {
	// [download] Destination: /path/to/file
	if idx := strings.Index(line, "[download] Destination: "); idx >= 0 {
		path := strings.TrimSpace(line[idx+len("[download] Destination: "):])
		return strings.Trim(path, "\"")
	}
	// [download] /path has already been downloaded
	if idx := strings.Index(line, "[download] "); idx >= 0 {
		if strings.Contains(line, "has already been downloaded") {
			path := line[idx+len("[download] "):]
			path = strings.TrimSuffix(path, " has already been downloaded")
			return strings.TrimSpace(path)
		}
	}
	// [Merger] Merging formats into "/path/to/file"
	if idx := strings.Index(line, "[Merger] Merging formats into "); idx >= 0 {
		path := strings.TrimSpace(line[idx+len("[Merger] Merging formats into "):])
		return strings.Trim(path, "\"")
	}
	return currentPath
}

// getInputDir 获取输入目录
func getInputDir(ctx context.Context) string {
	inputDir := model.SettingGet(ctx, model.SettingKeyVideoDir)
	return inputDir
}

// updateProgress 更新下载进度
func (l *DownloadLogic) updateProgress(ctx context.Context, id string, status string, progress int, progressMsg string) {
	if err := model.DownloadUpdateStatus(ctx, id, status, progress, progressMsg); err != nil {
		logger.WithTraceID(ctx).Error("更新下载进度失败",
			zap.String("download_id", id),
			zap.Int("progress", progress),
			zap.Error(err),
		)
	}
}

// StopAll 取消所有运行中的下载任务，清理缓存文件并标记数据库。
// 复用与 CancelDownload 相同的取消路径：杀进程 → 清缓存 → 改状态。
func (l *DownloadLogic) StopAll() {
	logger.Logger.Info("正在取消所有下载任务...")

	// 1. 备份 IDs，避免遍历时持有锁
	l.cancelMu.Lock()
	ids := make([]string, 0, len(l.runningCancels))
	for id := range l.runningCancels {
		ids = append(ids, id)
	}
	l.cancelMu.Unlock()

	if len(ids) == 0 {
		logger.Logger.Info("没有正在进行的下载任务")
	} else {
		logger.Logger.Info("正在取消下载任务", zap.Int("count", len(ids)))
	}

	// 2. 逐一取消（复用取消路径）
	for _, id := range ids {
		if cancel := l.getCancel(id); cancel != nil {
			cancel()
		}
		// 清理缓存
		if path := l.unregisterPath(id); path != "" {
			l.cleanupDownloadCache(path)
		}
	}

	// 3. 等待下载 goroutine 落定终态，避免 DB 连接关闭后仍有写入
	done := make(chan struct{})
	go func() {
		l.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		logger.Logger.Info("所有下载 goroutine 已退出")
	case <-time.After(10 * time.Second):
		logger.Logger.Warn("等待下载 goroutine 退出超时，继续关闭流程")
	}

	// 4. 批量标记 DB 中所有运行中/待处理的下载为 cancelled
	// （goroutine 的 DownloadMarkCancelled 可能来不及执行，这里兜底）
	err := model.DB.WithContext(context.Background()).Model(&model.Download{}).
		Where("status IN ?", []string{
			model.DownloadStatusPending,
			model.DownloadStatusProbing,
			model.DownloadStatusDownloading,
		}).
		Updates(map[string]interface{}{
			"status":       model.DownloadStatusCancelled,
			"progress":     0,
			"progress_msg": "程序关闭，下载已取消",
			"error_msg":    "程序关闭，下载已取消",
			"updated_at":   time.Now().Unix(),
		}).Error
	if err != nil {
		logger.Logger.Warn("批量标记下载任务为取消状态失败", zap.Error(err))
	} else {
		logger.Logger.Info("已标记所有未完成下载为取消状态")
	}
}

// registerCancel 注册下载任务的取消函数
func (l *DownloadLogic) registerCancel(id string, cancel context.CancelFunc) {
	l.cancelMu.Lock()
	defer l.cancelMu.Unlock()
	l.runningCancels[id] = cancel
}

// unregisterCancel 移除下载任务的取消函数
func (l *DownloadLogic) unregisterCancel(id string) {
	l.cancelMu.Lock()
	defer l.cancelMu.Unlock()
	delete(l.runningCancels, id)
}

// getCancel 获取下载任务的取消函数
func (l *DownloadLogic) getCancel(id string) context.CancelFunc {
	l.cancelMu.Lock()
	defer l.cancelMu.Unlock()
	return l.runningCancels[id]
}

// cleanupDownloadCache 清理 yt-dlp 下载时留下的 .part 和 .ytdl 缓存文件
func (l *DownloadLogic) cleanupDownloadCache(downloadPath string) {
	for _, suffix := range []string{".part", ".ytdl"} {
		cachePath := downloadPath + suffix
		if err := os.Remove(cachePath); err != nil && !os.IsNotExist(err) {
			logger.Logger.Warn("清理下载缓存文件失败",
				zap.String("path", cachePath),
				zap.Error(err),
			)
		} else if err == nil {
			logger.Logger.Info("已清理下载缓存文件", zap.String("path", cachePath))
		}
	}
}

// registerPath 注册下载任务的目标文件路径
func (l *DownloadLogic) registerPath(id, path string) {
	l.pathMu.Lock()
	defer l.pathMu.Unlock()
	l.runningPaths[id] = path
}

// getPath 获取下载任务的目标文件路径
func (l *DownloadLogic) getPath(id string) string {
	l.pathMu.Lock()
	defer l.pathMu.Unlock()
	return l.runningPaths[id]
}

// unregisterPath 移除下载任务的目标文件路径
func (l *DownloadLogic) unregisterPath(id string) string {
	l.pathMu.Lock()
	defer l.pathMu.Unlock()
	path := l.runningPaths[id]
	delete(l.runningPaths, id)
	return path
}

// downloadToRes 将 Download 模型转换为响应结构
func downloadToRes(dl *model.Download) *res.DownloadRes {
	return &res.DownloadRes{
		ID:             dl.ID,
		URL:            dl.URL,
		Platform:       dl.Platform,
		Status:         dl.Status,
		Progress:       dl.Progress,
		ProgressMsg:    dl.ProgressMsg,
		ErrorMsg:       dl.ErrorMsg,
		FileName:       dl.FileName,
		FileSize:       dl.FileSize,
		Duration:       dl.Duration,
		Title:          dl.Title,
		DownloadSpeed:  dl.DownloadSpeed,
		TotalSize:      dl.TotalSize,
		DownloadedSize: dl.DownloadedSize,
		Overwrite:      dl.Overwrite,
		DownloadDir:    dl.DownloadDir,
		CreatedAt:      dl.CreatedAt,
		UpdatedAt:      dl.UpdatedAt,
	}
}

// sanitizeFilename 清理文件名中的非法字符，替换为下划线
func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_",
		"?", "_", "\"", "_", "<", "_", ">", "_", "|", "_",
	)
	return replacer.Replace(name)
}

// resolveUniquePath 检测文件冲突：如果目标路径已存在，自动添加 _1, _2 编号。
func resolveUniquePath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	dir := filepath.Dir(path)
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(filepath.Base(path), ext)

	for i := 1; i < 1000; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s_%d%s", base, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
	return filepath.Join(dir, fmt.Sprintf("%s_%d%s", base, time.Now().Unix(), ext))
}
