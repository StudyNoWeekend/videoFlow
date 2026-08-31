package bootstrap

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"video-captions/internal/model"
)

// InitDB 初始化 SQLite 数据库并执行自动迁移
func InitDB(cfg *AppConfigDatabase) (*gorm.DB, error) {
	// 确保数据库文件所在目录存在
	dbPath := cfg.DSN
	if !filepath.IsAbs(dbPath) {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("获取工作目录失败: %w", err)
		}
		dbPath = filepath.Join(wd, dbPath)
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("创建数据库目录失败: %w", err)
	}

	logLevel := logger.Warn
	if Config != nil && Config.Log.Level == "debug" {
		logLevel = logger.Info
	}

	// 自定义 GORM 日志器：
	// - IgnoreRecordNotFoundError 为 true，避免空结果查询（如调度器轮询 pending 任务）刷屏
	// - 非 debug 级别时仅记录慢 SQL 与真实错误，SQL 明细仅在 debug 级别打印
	gormLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logLevel,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)

	// 启用 WAL 模式与 busy_timeout：调度器多 worker 并发写 + 扫描器 + API 读并发时，
	// 默认 rollback journal 模式容易出现 "database is locked"，WAL 允许读写并行
	dsn := dbPath + "?_journal_mode=WAL&_busy_timeout=5000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("打开 SQLite 数据库失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取底层 SQL DB 失败: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)

	// 设置全局 DB 实例
	model.DB = db

	// 自动迁移
	if err := autoMigrate(context.Background(), db); err != nil {
		return nil, fmt.Errorf("数据库自动迁移失败: %w", err)
	}

	return db, nil
}

// autoMigrate 注册所有数据模型并执行迁移
func autoMigrate(ctx context.Context, db *gorm.DB) error {
	if err := db.WithContext(ctx).AutoMigrate(
		model.Video{},
		model.Task{},
		model.Setting{},
		model.User{},
		model.ResetToken{},
		model.Download{},
	); err != nil {
		return err
	}
	return model.MigrateTaskType(ctx, db)
}
