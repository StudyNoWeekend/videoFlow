package bootstrap

import (
	"fmt"

	"video-captions/internal/ffmpeg"
)

// InitFFmpeg 根据配置初始化 ffmpeg 执行环境
func InitFFmpeg(cfg *AppConfigFFmpeg) error {
	if err := ffmpeg.Init(ffmpeg.Config{
		Provider:   cfg.Provider,
		SSHHost:    cfg.SSHHost,
		SSHPort:    cfg.SSHPort,
		SSHUser:    cfg.SSHUser,
		SSHKeyPath: cfg.SSHKeyPath,
		SSHArgs:    cfg.SSHArgs,
	}); err != nil {
		return fmt.Errorf("初始化 ffmpeg 执行环境失败: %w", err)
	}
	return nil
}
