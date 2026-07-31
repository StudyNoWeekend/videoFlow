package model

import (
	"context"

	"gorm.io/gorm"
)

// DB 全局数据库实例，由 bootstrap 初始化后注入
var DB *gorm.DB

// BaseModel 基础模型，包含通用字段
type BaseModel struct {
	ID        string         `gorm:"type:char(36);primaryKey;comment:唯一标识" json:"id"`
	CreatedAt int64          `gorm:"autoCreateTime;comment:创建时间戳" json:"created_at"`
	UpdatedAt int64          `gorm:"autoUpdateTime;comment:更新时间戳" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index;comment:软删除时间" json:"-"`
}

// Paginate 通用分页封装
func Paginate(page, pageSize int) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if page < 1 {
			page = 1
		}
		if pageSize < 1 {
			pageSize = 20
		}
		if pageSize > 100 {
			pageSize = 100
		}
		offset := (page - 1) * pageSize
		return db.Offset(offset).Limit(pageSize)
	}
}

// CheckDBHealth 检查数据库连接健康状态
func CheckDBHealth(ctx context.Context) error {
	if DB == nil {
		return nil
	}
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// MigrateTaskType 将 tasks 表中 task_type 为空的记录更新为 subtitle
func MigrateTaskType(ctx context.Context, db *gorm.DB) error {
	return db.WithContext(ctx).Model(&Task{}).
		Where("task_type IS NULL OR task_type = ?", "").
		Update("task_type", TaskTypeSubtitle).Error
}
