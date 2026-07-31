package model

import (
	"context"
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
	SettingKeyVideoDir             = "video_dir"
	SettingKeyScanInterval         = "scan_interval"
	SettingKeyASRURL               = "asr_url"
	SettingKeyASRLanguage          = "asr_language"
	SettingKeyASRVadFilter         = "asr_vad_filter"
	SettingKeyASRTask              = "asr_task"
	SettingKeyASREncode            = "asr_encode"
	SettingKeyASRInitialPrompt     = "asr_initial_prompt"
	SettingKeyASRWordTimestamps    = "asr_word_timestamps"
	SettingKeyASROutput            = "asr_output"
	SettingKeyRepairDockerImage    = "repair_docker_image"
	SettingKeyRepairDevice         = "repair_device"
	SettingKeySubtitleConcurrency  = "subtitle_concurrency"
	SettingKeyRepairConcurrency    = "repair_concurrency"
	SettingKeyTranslateConcurrency = "translate_concurrency"
)

// 统一设置项默认值（字符串形式持久化）
const (
	DefaultVideoDir          = ""
	DefaultScanInterval      = "60"
	DefaultASRURL            = "http://1.12.70.219:9999/asr"
	DefaultASRLanguage       = "zh"
	DefaultASRVadFilter      = "false"
	DefaultASRTask           = "transcribe"
	DefaultASREncode         = "true"
	DefaultASRInitialPrompt  = ""
	DefaultASRWordTimestamps = "false"
	DefaultASROutput         = "json"
	DefaultRepairDockerImage = "ladaapp/lada:latest"
	// DefaultRepairDevice 视频修复默认计算设备
	// 支持四种设备：cpu（CPU）、cuda:0（NVIDIA CUDA）、mps（Apple Silicon MPS）、xpu:0（Intel XPU）
	DefaultRepairDevice         = "cpu"
	DefaultSubtitleConcurrency  = "2"
	DefaultRepairConcurrency    = "1"
	DefaultTranslateConcurrency = "1"
)

// TableName 指定表名
func (Setting) TableName() string {
	return "settings"
}

// SettingGet 根据 key 查询配置值，不存在返回空字符串
func SettingGet(ctx context.Context, key string) string {
	var s Setting
	err := DB.WithContext(ctx).Where("key = ?", key).First(&s).Error
	if err != nil {
		return ""
	}
	return s.Value
}

// SettingSet 保存或更新配置项
func SettingSet(ctx context.Context, key string, value string) error {
	return DB.WithContext(ctx).Save(&Setting{
		Key:   key,
		Value: value,
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
