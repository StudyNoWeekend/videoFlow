package telegram

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
	"github.com/iyear/tdl/core/downloader"
	"go.uber.org/zap"
)

// 下载参数
const (
	// maxDownloadAttempts 单个文件的最大下载尝试次数
	maxDownloadAttempts = 3
	// progressInterval 进度回调的最小时间间隔
	progressInterval = time.Second
)

// ProgressFunc 下载进度回调，downloaded 为已下载字节数，total 为总字节数
type ProgressFunc func(downloaded, total int64)

// ResolveMedia 解析消息链接并返回媒体信息，非视频类媒体会被拒绝
func (e *Engine) ResolveMedia(ctx context.Context, rawURL string) (*MediaInfo, error) {
	cur, authorized, err := e.WaitReady(ctx, readyWaitTimeout)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, ErrNotLoggedIn
	}

	msg, err := fetchMessage(ctx, cur, rawURL)
	if err != nil {
		return nil, err
	}

	info, err := classifyMedia(msg)
	if err != nil {
		return nil, err
	}
	if !info.IsVideo() {
		return nil, fmt.Errorf("%w（该消息是%s）", ErrNotVideoMedia, kindLabel(info.Kind))
	}
	return info, nil
}

// DownloadMedia 下载消息中的视频到 targetPath。
// 文件引用可能过期，下载过程中会在需要时重新解析消息。
func (e *Engine) DownloadMedia(ctx context.Context, rawURL, targetPath string, onProgress ProgressFunc) error {
	info, err := e.ResolveMedia(ctx, rawURL)
	if err != nil {
		return err
	}
	if info.Size <= 0 {
		return errors.New("媒体大小异常，无法下载")
	}

	file, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	var lastErr error
	for attempt := 1; attempt <= maxDownloadAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		lastErr = e.attempt(ctx, info, file, onProgress)
		if lastErr == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// file_reference 会过期，重新解析消息换取新的下载位置
		if isFileReferenceExpired(lastErr) {
			refreshed, refreshErr := e.ResolveMedia(ctx, rawURL)
			if refreshErr != nil {
				return fmt.Errorf("刷新文件引用失败: %w", refreshErr)
			}
			info = refreshed
			e.log.Info("Telegram 文件引用已过期，已刷新并重试", zap.Int("attempt", attempt))
			continue
		}

		// 限流已由 tclient 默认中间件等待，这里兜底处理超时类错误
		if tgerr.Is(lastErr, tg.ErrTimeout) {
			e.log.Warn("Telegram 下载超时，准备重试", zap.Int("attempt", attempt))
			continue
		}

		// 其余错误重试没有意义，直接失败
		break
	}

	return fmt.Errorf("Telegram 下载失败: %w", lastErr)
}

// attempt 通过 tdl 的多线程下载引擎执行一次完整下载
func (e *Engine) attempt(ctx context.Context, info *MediaInfo, file *os.File, onProgress ProgressFunc) error {
	cur, err := e.runtime()
	if err != nil {
		return err
	}

	reporter := newProgressReporter(info.Size, onProgress)
	d := downloader.New(downloader.Options{
		Pool:     cur.pool,
		Threads:  e.config().Threads,
		Iter:     newSingleIter(file, info),
		Progress: reporter,
	})
	// 单条消息只有一个元素，limit 固定为 1
	if err := d.Download(ctx, 1); err != nil {
		return err
	}
	// tdl 的 Download 只透传取消类错误，元素级错误经 Progress.OnDone 上报，这里必须读取
	if err := reporter.Err(); err != nil {
		return err
	}

	// 兜底校验：确认落盘大小与媒体大小一致，避免把不完整文件当成成功
	if fi, statErr := file.Stat(); statErr == nil && fi.Size() != info.Size {
		return fmt.Errorf("文件下载不完整：期望 %d 字节，实际 %d 字节", info.Size, fi.Size())
	}
	return nil
}

// isFileReferenceExpired 判断错误是否为文件引用过期
func isFileReferenceExpired(err error) bool {
	return tgerr.Is(err,
		tg.ErrFileReferenceExpired,
		"FILE_REFERENCE_INVALID",
		"FILE_REFERENCE_EMPTY",
	)
}

// singleIter 单条消息的下载迭代器，实现 tdl downloader 的 Iter/Elem/File 接口
type singleIter struct {
	file  *os.File
	media *MediaInfo
	done  bool
}

func newSingleIter(file *os.File, media *MediaInfo) *singleIter {
	return &singleIter{file: file, media: media}
}

// Next 实现 downloader.Iter
func (it *singleIter) Next(context.Context) bool {
	if it.done {
		return false
	}
	it.done = true
	return true
}

// Value 实现 downloader.Iter
func (it *singleIter) Value() downloader.Elem { return it }

// Err 实现 downloader.Iter
func (it *singleIter) Err() error { return nil }

// File 实现 downloader.Elem
func (it *singleIter) File() downloader.File { return it }

// To 实现 downloader.Elem
func (it *singleIter) To() io.WriterAt { return it.file }

// AsTakeout 实现 downloader.Elem；普通账号下载不使用 takeout 会话
func (it *singleIter) AsTakeout() bool { return false }

// Location 实现 downloader.File
func (it *singleIter) Location() tg.InputFileLocationClass { return it.media.Location }

// Size 实现 downloader.File
func (it *singleIter) Size() int64 { return it.media.Size }

// DC 实现 downloader.File
func (it *singleIter) DC() int { return it.media.DC }

// progressReporter 对接 tdl downloader 的进度回调，按「百分比变化或超过间隔」节流，
// 并收集元素级错误（tdl 的 Download 不返回元素级错误）
type progressReporter struct {
	total      int64
	onProgress ProgressFunc
	throttle   *progressThrottler

	mu  sync.Mutex
	err error
}

func newProgressReporter(total int64, onProgress ProgressFunc) *progressReporter {
	return &progressReporter{
		total:      total,
		onProgress: onProgress,
		throttle:   newProgressThrottler(progressInterval),
	}
}

// OnAdd 实现 downloader.Progress
func (r *progressReporter) OnAdd(downloader.Elem) {}

// OnDownload 实现 downloader.Progress
func (r *progressReporter) OnDownload(_ downloader.Elem, state downloader.ProgressState) {
	r.throttle.report(state.Downloaded, state.Total, r.onProgress)
}

// OnDone 实现 downloader.Progress
func (r *progressReporter) OnDone(_ downloader.Elem, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.err = err
}

// Err 返回元素级下载错误
func (r *progressReporter) Err() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.err
}

// progressThrottler 进度回调节流器
type progressThrottler struct {
	interval time.Duration

	mu      sync.Mutex
	lastAt  time.Time
	lastPct int
}

func newProgressThrottler(interval time.Duration) *progressThrottler {
	return &progressThrottler{interval: interval}
}

// allow 判断本次进度是否需要回调
func (t *progressThrottler) allow(pct int, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if pct == t.lastPct && now.Sub(t.lastAt) < t.interval {
		return false
	}
	t.lastPct = pct
	t.lastAt = now
	return true
}

// report 按节流策略回调进度
func (t *progressThrottler) report(downloaded, total int64, onProgress ProgressFunc) {
	if onProgress == nil {
		return
	}

	pct := 0
	if total > 0 {
		pct = int(downloaded * 100 / total)
	}
	if !t.allow(pct, time.Now()) {
		return
	}
	onProgress(downloaded, total)
}
