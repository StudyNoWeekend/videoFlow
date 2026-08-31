package model

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"video-captions/utils/logger"
)

// Setting 运行时配置表，用于持久化前端可实时修改的配置项
type Setting struct {
	Key       string `gorm:"type:varchar(128);primaryKey;comment:配置键" json:"key"`
	Value     string `gorm:"type:text;comment:配置值" json:"value"`
	CreatedAt int64  `gorm:"autoCreateTime;comment:创建时间戳" json:"created_at"`
	UpdatedAt int64  `gorm:"autoUpdateTime;comment:更新时间戳" json:"updated_at"`
}

// 统一设置项的键名
const (
	SettingKeyVideoDir                = "video_dir"
	SettingKeyOutputDir               = "output_dir"
	SettingKeyScanInterval            = "scan_interval"
	SettingKeyASRURL                  = "asr_url"
	SettingKeyASRLanguage             = "asr_language"
	SettingKeyASRVadFilter            = "asr_vad_filter"
	SettingKeyASRTask                 = "asr_task"
	SettingKeyASREncode               = "asr_encode"
	SettingKeyASRInitialPrompt        = "asr_initial_prompt"
	SettingKeyASRWordTimestamps       = "asr_word_timestamps"
	SettingKeyASROutput               = "asr_output"
	SettingKeyRepairDockerImage       = "repair_docker_image"
	SettingKeyRepairDevice            = "repair_device"
	SettingKeySubtitleConcurrency     = "subtitle_concurrency"
	SettingKeySubtitleBurnConcurrency = "subtitle_burn_concurrency"
	SettingKeyRepairConcurrency       = "repair_concurrency"
	SettingKeyTranslateConcurrency    = "translate_concurrency"
	SettingKeySchedulerPollInterval   = "scheduler_poll_interval"
	SettingKeyUpscaleDockerImage      = "upscale_docker_image"
	SettingKeyUpscaleDevice           = "upscale_device"
	SettingKeyUpscaleConcurrency      = "upscale_concurrency"
	SettingKeyDownloadConcurrency     = "download_concurrency"
)

// 统一设置项默认值（字符串形式持久化）
const (
	DefaultVideoDir     = ""
	DefaultOutputDir    = "/output"
	DefaultScanInterval = "60"
	// DefaultASRURL ASR 服务默认地址；出于安全考虑不内置任何公网地址，
	// 部署时通过 settings 页面、config.yaml 的 asr.url 或环境变量配置
	DefaultASRURL            = ""
	DefaultASRLanguage       = "zh"
	DefaultASRVadFilter      = "false"
	DefaultASRTask           = "transcribe"
	DefaultASREncode         = "true"
	DefaultASRInitialPrompt  = ""
	DefaultASRWordTimestamps = "false"
	DefaultASROutput         = "json"
	DefaultRepairDockerImage = "ladaapp/lada:latest"
	// DefaultRepairDevice 去马赛克默认计算设备
	// 支持四种设备：cpu（CPU）、cuda:0（NVIDIA CUDA）、mps（Apple Silicon MPS）、xpu:0（Intel XPU）
	DefaultRepairDevice            = "cpu"
	DefaultSubtitleConcurrency     = "2"
	DefaultSubtitleBurnConcurrency = "1"
	DefaultRepairConcurrency       = "1"
	DefaultTranslateConcurrency    = "1"
	DefaultSchedulerPollInterval   = "2"
	DefaultUpscaleDockerImage      = "ghcr.io/k4yt3x/video2x:latest"
	DefaultUpscaleDevice           = "cpu"
	DefaultUpscaleConcurrency      = "1"
)

// TableName 指定表名
func (Setting) TableName() string {
	return "settings"
}

// SettingGet 根据 key 查询配置值，不存在返回空字符串。
// 记录不存在视为未配置；其他查询错误记日志后同样返回空串，避免被误当成"未配置"静默降级。
func SettingGet(ctx context.Context, key string) string {
	var s Setting
	err := DB.WithContext(ctx).Where("key = ?", key).First(&s).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) && logger.Logger != nil {
			logger.Logger.Error("读取设置失败", zap.String("key", key), zap.Error(err))
		}
		return ""
	}
	return s.Value
}

// SettingSet 保存或更新配置项。
// 使用 Upsert 仅更新 value/updated_at，避免 Save 全字段更新把已存在记录的 created_at 清零。
func SettingSet(ctx context.Context, key string, value string) error {
	now := time.Now().Unix()
	return DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "key"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"value":      value,
			"updated_at": now,
		}),
	}).Create(&Setting{
		Key:       key,
		Value:     value,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error
}

// SettingGetOrDefault 根据 key 查询配置值，不存在返回默认值
func SettingGetOrDefault(ctx context.Context, key, defaultValue string) string {
	value := SettingGet(ctx, key)
	if value == "" {
		return defaultValue
	}
	return value
}
