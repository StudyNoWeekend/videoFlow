package scanner

import (
	"context"
	"os"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"video-captions/internal/logic"
	"video-captions/internal/model"
	"video-captions/utils/logger"
)

// defaultScanInterval 默认扫描间隔
const defaultScanInterval = 60 * time.Second

// Scanner 视频目录自动扫描器
type Scanner struct {
	db         *gorm.DB
	videoLogic *logic.VideoLogic
	interval   time.Duration
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// NewScanner 创建视频目录扫描器实例
func NewScanner(db *gorm.DB, videoLogic *logic.VideoLogic) *Scanner {
	return &Scanner{
		db:         db,
		videoLogic: videoLogic,
		interval:   defaultScanInterval,
	}
}

// Start 启动扫描器：配置目录存在时立即扫描一次，随后按 scan_interval 周期性扫描
func (s *Scanner) Start(ctx context.Context) error {
	scanCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	s.loadInterval(scanCtx)

	// 启动时若已配置 video_dir 则立即执行一次扫描
	if videoDir := model.SettingGet(scanCtx, model.SettingKeyVideoDir); videoDir != "" {
		s.runScan(scanCtx)
	}

	s.wg.Add(1)
	go s.loop(scanCtx)

	logger.Logger.Info("视频目录扫描器已启动", zap.Duration("interval", s.interval))
	return nil
}

// Stop 优雅停止扫描器，等待当前扫描完成后退出
func (s *Scanner) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	logger.Logger.Info("视频目录扫描器已停止")
}

// loop 周期性扫描循环
func (s *Scanner) loop(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.runScan(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// runScan 执行一次完整扫描：扫描目录并清理已不存在的本地视频记录
func (s *Scanner) runScan(ctx context.Context) {
	s.loadInterval(ctx)

	if _, err := s.videoLogic.ScanConfiguredDir(ctx); err != nil {
		logger.Logger.Warn("自动扫描视频目录失败", zap.Error(err))
	}

	if err := s.cleanupMissingVideos(ctx); err != nil {
		logger.Logger.Warn("清理已删除视频记录失败", zap.Error(err))
	}
}

// cleanupMissingVideos 查询所有视频记录，软删除本地文件已不存在的记录
func (s *Scanner) cleanupMissingVideos(ctx context.Context) error {
	videos, err := model.VideoListAll(ctx)
	if err != nil {
		return err
	}

	for _, v := range videos {
		if _, statErr := os.Stat(v.Path); os.IsNotExist(statErr) {
			if delErr := s.db.WithContext(ctx).Delete(v).Error; delErr != nil {
				logger.Logger.Warn("软删除视频记录失败",
					zap.String("path", v.Path),
					zap.Error(delErr),
				)
			} else {
				logger.Logger.Info("视频文件已不存在，软删除记录",
					zap.String("path", v.Path),
				)
			}
		}
	}
	return nil
}

// loadInterval 从 settings 表读取扫描间隔，若未配置或非法则使用默认值
func (s *Scanner) loadInterval(ctx context.Context) {
	valueStr := model.SettingGet(ctx, model.SettingKeyScanInterval)
	if valueStr == "" {
		s.interval = defaultScanInterval
		return
	}

	v, err := strconv.Atoi(valueStr)
	if err != nil || v <= 0 {
		s.interval = defaultScanInterval
		return
	}

	s.interval = time.Duration(v) * time.Second
}
