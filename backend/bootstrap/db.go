package bootstrap

import (
	"context"
	"fmt"
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

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
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
	); err != nil {
		return err
	}
	return model.MigrateTaskType(ctx, db)
}
