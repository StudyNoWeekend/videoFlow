package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"video-captions/bootstrap"
	"video-captions/internal/logic"
	"video-captions/internal/model"
	"video-captions/internal/router"
	"video-captions/internal/scanner"
	"video-captions/internal/scheduler"
	"video-captions/utils/logger"
)

func main() {
	cfg, err := bootstrap.InitConfig()
	if err != nil {
		log.Fatalf("初始化配置失败: %v", err)
	}

	// 初始化 zap 日志
	if err := logger.InitLogger(cfg.Log.Level, cfg.Log.Path); err != nil {
		log.Fatalf("初始化日志失败: %v", err)
	}
	defer logger.Sync()
	defer logger.Close()

	// 初始化 SQLite 数据库
	_, err = bootstrap.InitDB(&cfg.Database)
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	// 兼容旧命令：reset-password 等 CLI 子命令已移除，密码重置改为通过网页触发
	if len(os.Args) > 1 {
		fmt.Println("错误: 不再支持命令行参数")
		fmt.Println()
		fmt.Println("密码重置功能已迁移到网页端：")
		fmt.Println("  1. 打开登录页，点击“忘记密码”")
		fmt.Println("  2. 输入用户名，点击“获取重置令牌”")
		fmt.Println("  3. 到服务器日志（如 docker compose logs -f）中查看令牌并粘贴回页面")
		os.Exit(1)
	}

	// ---- 以下为正常 HTTP 服务启动流程 ----

	// 初始化 ASR Provider
	if err := bootstrap.InitASR(); err != nil {
		log.Fatalf("初始化 ASR Provider 失败: %v", err)
	}

	// 初始化 ffmpeg 执行环境
	if err := bootstrap.InitFFmpeg(&cfg.FFmpeg); err != nil {
		log.Fatalf("初始化 ffmpeg 执行环境失败: %v", err)
	}

	// 若数据库中已持久化 ffmpeg 配置，则覆盖配置文件默认值
	if err := logic.NewSettingLogic().ApplyFFmpegFromSettings(context.Background()); err != nil {
		log.Fatalf("加载已保存的 ffmpeg 配置失败: %v", err)
	}

	// 初始化去马赛克执行器
	if err := bootstrap.InitRepair(context.Background()); err != nil {
		log.Fatalf("初始化去马赛克执行器失败: %v", err)
	}

	// 若数据库中已持久化去马赛克配置，则覆盖配置文件默认值（出错不阻断启动）
	if err := logic.NewSettingLogic().ApplyRepairFromSettings(context.Background()); err != nil {
		logger.Logger.Warn("加载已保存的去马赛克配置失败，将使用默认配置", zap.Error(err))
	}

	// 初始化清晰度去马赛克执行器
	if err := bootstrap.InitUpscale(context.Background()); err != nil {
		log.Fatalf("初始化清晰度去马赛克执行器失败: %v", err)
	}

	// 若数据库中已持久化清晰度去马赛克配置，则覆盖配置文件默认值（出错不阻断启动）
	if err := logic.NewSettingLogic().ApplyUpscaleFromSettings(context.Background()); err != nil {
		logger.Logger.Warn("加载已保存的清晰度去马赛克配置失败，将使用默认配置", zap.Error(err))
	}

	// 启动任务调度器
	taskScheduler := scheduler.NewTaskScheduler(model.DB)
	// 注册到全局实例，供任务取消等业务逻辑调用
	scheduler.Default = taskScheduler

	// 兜底清理：将上次非正常退出（容器被强杀、断电等）残留的 running 任务标记为失败
	if affected, err := model.TaskMarkRunningAsFailed(context.Background(), "服务异常终止"); err != nil {
		logger.Logger.Error("清理残留运行中任务失败", zap.Error(err))
	} else if affected > 0 {
		logger.Logger.Info("已将残留运行中任务标记为失败（原因：上次服务异常终止）", zap.Int64("count", affected))
	}

	// 回填视频各任务类型状态字段（与最新任务记录对齐，兼容升级前的历史数据）
	if err := model.VideoResyncAllTaskStatus(context.Background()); err != nil {
		logger.Logger.Error("回填视频任务状态失败", zap.Error(err))
	}

	if err := taskScheduler.Start(context.Background()); err != nil {
		log.Fatalf("启动任务调度器失败: %v", err)
	}

	// 启动视频目录自动扫描器（不阻塞 HTTP 服务启动）
	videoScanner := scanner.NewScanner(model.DB, logic.NewVideoLogic())
	if err := videoScanner.Start(context.Background()); err != nil {
		log.Fatalf("启动视频目录扫描器失败: %v", err)
	}

	// 初始化路由（后端端口固定 8080，不暴露给宿主机，由 nginx 反向代理）
	r := router.SetupRouter(&cfg.HTTP)

	addr := fmt.Sprintf(":%d", cfg.HTTP.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  time.Duration(cfg.HTTP.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.HTTP.WriteTimeout) * time.Second,
	}

	// 启动 HTTP 服务
	go func() {
		logger.Logger.Info("HTTP 服务启动", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP 服务启动失败: %v", err)
		}
	}()

	// 等待中断信号，优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Logger.Info("正在关闭服务...")

	// 1. 先停止调度器，让 worker 不再认领新任务，但已 running 的任务继续执行
	taskScheduler.Stop()

	// 2. 停止所有下载任务（复用取消路径：杀进程 → 清缓存 → 改状态）
	logic.GetGlobalDownloadLogic().StopAll()

	// 3. 所有 worker 已完成（或 context 已 cancel），再将 running 任务标记为失败
	//    （先停调度器再标失败，避免 worker 落定终态覆盖"程序重启"标记）
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if affected, err := model.TaskMarkRunningAsFailed(shutdownCtx, "程序重启"); err != nil {
		logger.Logger.Error("标记运行中任务为失败时出错", zap.Error(err))
	} else if affected > 0 {
		logger.Logger.Info("已将运行中任务标记为失败（原因：程序重启）", zap.Int64("count", affected))
	}

	// 3. 标记完任务后再全量回填视频状态字段（确保重启后视频徽标准确）
	if err := model.VideoResyncAllTaskStatus(shutdownCtx); err != nil {
		logger.Logger.Error("回填视频任务状态失败", zap.Error(err))
	}

	// 4. 关闭 HTTP 服务
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Logger.Error("HTTP 服务关闭异常", zap.Error(err))
	}

	// 5. 停止视频目录扫描器
	videoScanner.Stop()

	// 6. 关闭数据库连接
	if model.DB != nil {
		if sqlDB, err := model.DB.DB(); err == nil {
			sqlDB.Close()
		}
	}

	logger.Logger.Info("服务已退出")
}
