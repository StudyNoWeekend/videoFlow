package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger 全局 zap 日志实例
var Logger *zap.Logger

// fileWriter 保存文件写入器引用，供进程退出时关闭
var fileWriter *weeklyWriter

// InitLogger 初始化 zap 日志。
// level: debug / info / warn / error
// path: 日志输出目录，为空时仅输出到 stdout；非空时同时输出到该目录下按周轮转的文件
func InitLogger(level, path string) error {
	cfg := zap.NewProductionConfig()
	switch level {
	case "debug":
		cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "warn":
		cfg.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		cfg.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	// 始终输出到标准输出
	ws := zapcore.AddSync(os.Stdout)

	// 当配置了日志目录时，同时输出到文件
	if path != "" {
		fw := newWeeklyWriter(path, "app")
		fileWriter = fw
		ws = zapcore.NewMultiWriteSyncer(ws, fw)
	}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(cfg.EncoderConfig),
		ws,
		cfg.Level,
	)

	l := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	Logger = l
	return nil
}

// Sync 刷出日志缓冲
func Sync() {
	if Logger != nil {
		_ = Logger.Sync()
	}
}

// Close 关闭日志文件写入器并停止后台清理 goroutine
func Close() {
	if fileWriter != nil {
		_ = fileWriter.Close()
	}
}

// weeklyWriter 按 ISO 周轮转的日志文件写入器。
// 文件命名格式：{dir}/{prefix}-{year}W{week}.log
// 后台 goroutine 每小时清理超过 1 周的旧日志文件。
type weeklyWriter struct {
	dir         string
	prefix      string
	mu          sync.Mutex
	file        *os.File
	currentWeek string // 如 "2026-W34"
	maxAgeWeeks int    // 保留的周数
	stopCh      chan struct{}
	stopped     bool
}

func newWeeklyWriter(dir, prefix string) *weeklyWriter {
	w := &weeklyWriter{
		dir:         dir,
		prefix:      prefix,
		maxAgeWeeks: 1,
		stopCh:      make(chan struct{}),
	}
	// 启动后台清理 goroutine
	go w.cleanupLoop()
	return w
}

// Write 实现 zapcore.WriteSyncer 接口
func (w *weeklyWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	week := currentISOWeek()
	if week != w.currentWeek || w.file == nil {
		// 周号变更或首次写入，切换文件
		if w.file != nil {
			w.file.Close()
		}
		fpath := filepath.Join(w.dir, fmt.Sprintf("%s-%s.log", w.prefix, week))
		// 确保目录存在
		if err := os.MkdirAll(w.dir, 0755); err != nil {
			return 0, fmt.Errorf("创建日志目录失败: %w", err)
		}
		f, err := os.OpenFile(fpath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return 0, fmt.Errorf("打开日志文件失败: %w", err)
		}
		w.file = f
		w.currentWeek = week
	}

	return w.file.Write(p)
}

// Sync 实现 zapcore.WriteSyncer 接口
func (w *weeklyWriter) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		return w.file.Sync()
	}
	return nil
}

// Close 关闭 writer 和后台清理 goroutine
func (w *weeklyWriter) Close() error {
	w.mu.Lock()
	if !w.stopped {
		w.stopped = true
		close(w.stopCh)
	}
	var err error
	if w.file != nil {
		err = w.file.Close()
		w.file = nil
	}
	w.mu.Unlock()
	return err
}

// cleanupLoop 每小时检查并清理超过 maxAgeWeeks 的旧日志文件
func (w *weeklyWriter) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.cleanup()
		case <-w.stopCh:
			return
		}
	}
}

func (w *weeklyWriter) cleanup() {
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return
	}

	// 收集匹配的日志文件及其中提取的 ISO 周标识
	type logFile struct {
		name string
		week string // "2026-W34"
	}
	var files []logFile
	prefix := w.prefix + "-"
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".log") {
			continue
		}
		// 提取周标识：app-2026-W34.log
		weekPart := strings.TrimSuffix(strings.TrimPrefix(name, prefix), ".log")
		if len(weekPart) == 0 {
			continue
		}
		files = append(files, logFile{name: name, week: weekPart})
	}

	if len(files) == 0 {
		return
	}

	// 收集要删除的旧文件：保留当前周及前 maxAgeWeeks 周的日志
	keep := make(map[string]bool)
	// 从当前周向前保留 maxAgeWeeks + 1 周
	for i := 0; i <= w.maxAgeWeeks; i++ {
		t := time.Now().AddDate(0, 0, -7*i)
		year, week := t.ISOWeek()
		keep[fmt.Sprintf("%d-W%02d", year, week)] = true
	}

	for _, f := range files {
		if !keep[f.week] {
			fpath := filepath.Join(w.dir, f.name)
			os.Remove(fpath)
		}
	}
}

// currentISOWeek 返回当前 ISO 周字符串，如 "2026-W34"
func currentISOWeek() string {
	year, week := time.Now().ISOWeek()
	return fmt.Sprintf("%d-W%02d", year, week)
}
