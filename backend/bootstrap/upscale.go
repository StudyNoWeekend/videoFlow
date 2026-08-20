package bootstrap

import (
	"context"

	"video-captions/internal/upscale"
	"video-captions/utils/logger"

	"go.uber.org/zap"
)

// UpscaleExecutor 全局清晰度去马赛克执行器实例，供 scheduler 使用
var UpscaleExecutor *upscale.Executor

// InitUpscale 从 config 读取清晰度去马赛克配置并初始化执行器。
// Docker 不可用时仅打印警告，不阻塞服务启动。
func InitUpscale(ctx context.Context) error {
	cfg := loadUpscaleConfig()
	exec := upscale.NewExecutor(cfg)
	if err := exec.Init(cfg); err != nil {
		logger.Logger.Warn("清晰度去马赛克执行器初始化失败，清晰度去马赛克功能暂不可用",
			zap.Error(err),
		)
		// 不返回错误，允许服务继续启动
		return nil
	}

	UpscaleExecutor = exec
	return nil
}

func loadUpscaleConfig() upscale.Config {
	cfg := upscale.Config{}
	if Config != nil {
		cfg.DockerImage = Config.Upscale.DockerImage
		cfg.Device = Config.Upscale.Device
		cfg.Processor = Config.Upscale.Processor
		cfg.Model = Config.Upscale.Model
	}
	if cfg.DockerImage == "" {
		cfg.DockerImage = "ghcr.io/k4yt3x/video2x:latest"
	}
	if cfg.Device == "" {
		cfg.Device = "cpu"
	}
	if cfg.Processor == "" {
		cfg.Processor = upscale.DefaultProcessor
	}
	if cfg.Model == "" {
		cfg.Model = upscale.DefaultModel
	}
	// 处理器/模型/降噪等级由创建清晰度去马赛克任务时逐次选择，此处仅提供默认兜底
	cfg.NoiseLevel = upscale.DefaultNoiseLevel
	return cfg
}
