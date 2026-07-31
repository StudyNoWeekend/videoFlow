package bootstrap

import (
	"context"

	"video-captions/internal/repair"
	"video-captions/utils/logger"

	"go.uber.org/zap"
)

// RepairExecutor 全局视频修复执行器实例，供 scheduler 使用
var RepairExecutor *repair.Executor

// InitRepair 从 config 读取修复配置并初始化执行器。
// Docker 不可用时仅打印警告，不阻塞服务启动。
func InitRepair(ctx context.Context) error {
	cfg := loadRepairConfig()
	repairExec := repair.NewExecutor(cfg)
	if err := repairExec.Init(cfg); err != nil {
		logger.Logger.Warn("视频修复执行器初始化失败，视频修复功能暂不可用",
			zap.Error(err),
		)
		// 不返回错误，允许服务继续启动
		return nil
	}

	RepairExecutor = repairExec
	return nil
}

func loadRepairConfig() repair.Config {
	cfg := repair.Config{}
	if Config != nil {
		cfg.DockerImage = Config.Repair.DockerImage
		cfg.Device = Config.Repair.Device
	}
	if cfg.DockerImage == "" {
		cfg.DockerImage = "ladaapp/lada:latest"
	}
	if cfg.Device == "" {
		cfg.Device = "cpu" // 默认 CPU 设备，可选值：cpu / cuda:0 / mps / xpu:0
	}
	return cfg
}
