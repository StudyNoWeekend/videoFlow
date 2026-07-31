package bootstrap

import (
	"context"
	"fmt"
	"strconv"

	"video-captions/internal/asr"
	"video-captions/internal/model"
)

// ASRProvider 全局 ASR Provider 实例
var ASRProvider asr.ASRProvider

// InitASR 从 settings 读取 ASR 配置并初始化 ASR Provider
func InitASR() error {
	ctx := context.Background()

	url := model.SettingGetOrDefault(ctx, model.SettingKeyASRURL, defaultASRStr(model.DefaultASRURL, ""))
	if url == "" {
		return fmt.Errorf("ASR URL 未配置")
	}

	language := model.SettingGetOrDefault(ctx, model.SettingKeyASRLanguage, model.DefaultASRLanguage)
	if language == "" {
		language = model.DefaultASRLanguage
	}

	vadFilter := parseBoolSetting(ctx, model.SettingKeyASRVadFilter, model.DefaultASRVadFilter)
	task := model.SettingGetOrDefault(ctx, model.SettingKeyASRTask, model.DefaultASRTask)
	encode := parseBoolSetting(ctx, model.SettingKeyASREncode, model.DefaultASREncode)
	initialPrompt := model.SettingGetOrDefault(ctx, model.SettingKeyASRInitialPrompt, model.DefaultASRInitialPrompt)
	wordTimestamps := parseBoolSetting(ctx, model.SettingKeyASRWordTimestamps, model.DefaultASRWordTimestamps)
	output := model.SettingGetOrDefault(ctx, model.SettingKeyASROutput, model.DefaultASROutput)

	ASRProvider = asr.NewASRClientWithOpts(url, language, vadFilter, asr.ASRClientOptions{
		Task:           task,
		Encode:         encode,
		InitialPrompt:  initialPrompt,
		WordTimestamps: wordTimestamps,
		Output:         output,
	})
	return nil
}

// defaultASRStr 返回配置或默认值
func defaultASRStr(def, fallback string) string {
	if Config != nil && Config.ASR.URL != "" {
		return Config.ASR.URL
	}
	if def != "" {
		return def
	}
	return fallback
}

// parseBoolSetting 从 settings 表读取布尔配置，解析失败时使用默认值
func parseBoolSetting(ctx context.Context, key, defaultVal string) bool {
	val := model.SettingGetOrDefault(ctx, key, defaultVal)
	v, err := strconv.ParseBool(val)
	if err != nil {
		v, _ = strconv.ParseBool(defaultVal)
	}
	return v
}
