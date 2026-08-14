package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"video-captions/bootstrap"
	"video-captions/enum"
	"video-captions/internal/dto/req"
	"video-captions/internal/dto/res"
	"video-captions/internal/ffmpeg"
	"video-captions/internal/model"
	"video-captions/internal/repair"
	"video-captions/utils/logger"

	"go.uber.org/zap"
)

// SettingLogic 运行时配置业务逻辑
type SettingLogic struct{}

// NewSettingLogic 创建运行时配置 logic 实例
func NewSettingLogic() *SettingLogic {
	return &SettingLogic{}
}

const (
	settingKeyFFmpegProvider   = "ffmpeg.provider"
	settingKeyFFmpegSSHHost    = "ffmpeg.ssh_host"
	settingKeyFFmpegSSHPort    = "ffmpeg.ssh_port"
	settingKeyFFmpegSSHUser    = "ffmpeg.ssh_user"
	settingKeyFFmpegSSHKeyPath = "ffmpeg.ssh_key_path"
	settingKeyFFmpegSSHArgs    = "ffmpeg.ssh_args"
)

// GetSettings 获取统一设置，优先读取 settings 表，不存在则使用配置文件默认值
func (l *SettingLogic) GetSettings(ctx context.Context) (*res.SettingRes, error) {
	videoDir := model.SettingGetOrDefault(ctx, model.SettingKeyVideoDir, l.defaultVideoDir())
	scanInterval := l.settingIntOrDefault(ctx, model.SettingKeyScanInterval, l.defaultScanInterval(), 1, 86400)
	asrURL := model.SettingGetOrDefault(ctx, model.SettingKeyASRURL, l.defaultASRURL())
	asrLanguage := model.SettingGetOrDefault(ctx, model.SettingKeyASRLanguage, l.defaultASRLanguage())
	asrVadFilter := parseBoolString(model.SettingGetOrDefault(ctx, model.SettingKeyASRVadFilter, l.defaultASRBoolStr(model.DefaultASRVadFilter, false)))
	asrTask := model.SettingGetOrDefault(ctx, model.SettingKeyASRTask, l.defaultASRString(func(c *bootstrap.AppConfigASR) string { return c.Task }, model.DefaultASRTask))
	asrEncode := parseBoolString(model.SettingGetOrDefault(ctx, model.SettingKeyASREncode, l.defaultASRBoolStr(model.DefaultASREncode, true)))
	asrInitialPrompt := model.SettingGetOrDefault(ctx, model.SettingKeyASRInitialPrompt, l.defaultASRString(func(c *bootstrap.AppConfigASR) string { return c.InitialPrompt }, model.DefaultASRInitialPrompt))
	asrWordTimestamps := parseBoolString(model.SettingGetOrDefault(ctx, model.SettingKeyASRWordTimestamps, l.defaultASRBoolStr(model.DefaultASRWordTimestamps, false)))
	asrOutput := model.SettingGetOrDefault(ctx, model.SettingKeyASROutput, l.defaultASRString(func(c *bootstrap.AppConfigASR) string { return c.Output }, model.DefaultASROutput))
	repairDockerImage := model.SettingGetOrDefault(ctx, model.SettingKeyRepairDockerImage, model.DefaultRepairDockerImage)
	repairDevice := model.SettingGetOrDefault(ctx, model.SettingKeyRepairDevice, model.DefaultRepairDevice)
	subtitleConcurrency := l.settingIntOrDefault(ctx, model.SettingKeySubtitleConcurrency, l.defaultSubtitleConcurrency(), 1, 50)
	subtitleBurnConcurrency := l.settingIntOrDefault(ctx, model.SettingKeySubtitleBurnConcurrency, l.defaultSubtitleBurnConcurrency(), 1, 50)
	repairConcurrency := l.settingIntOrDefault(ctx, model.SettingKeyRepairConcurrency, l.defaultRepairConcurrency(), 1, 50)
	translateConcurrency := l.settingIntOrDefault(ctx, model.SettingKeyTranslateConcurrency, l.defaultTranslateConcurrency(), 1, 50)

	return &res.SettingRes{
		VideoDir:                videoDir,
		ScanInterval:            scanInterval,
		ASRURL:                  asrURL,
		ASRLanguage:             asrLanguage,
		ASRVadFilter:            asrVadFilter,
		ASRTask:                 asrTask,
		ASREncode:               asrEncode,
		ASRInitialPrompt:        asrInitialPrompt,
		ASRWordTimestamps:       asrWordTimestamps,
		ASROutput:               asrOutput,
		RepairDockerImage:       repairDockerImage,
		RepairDevice:            repairDevice,
		SubtitleConcurrency:     subtitleConcurrency,
		SubtitleBurnConcurrency: subtitleBurnConcurrency,
		RepairConcurrency:       repairConcurrency,
		TranslateConcurrency:    translateConcurrency,
	}, nil
}

// UpdateSettings 保存统一设置，持久化到 settings 表并立即生效
func (l *SettingLogic) UpdateSettings(ctx context.Context, updateReq *req.SettingReq) error {
	if updateReq.VideoDir != "" {
		info, err := os.Stat(updateReq.VideoDir)
		if err != nil {
			return enum.ErrInvalidParam.WithMsg(fmt.Sprintf("视频目录不存在或无法访问: %v", err))
		}
		if !info.IsDir() {
			return enum.ErrInvalidParam.WithMsg("视频目录必须是目录")
		}
	}

	if updateReq.ScanInterval <= 0 {
		return enum.ErrInvalidParam.WithMsg("扫描间隔必须大于 0")
	}
	if updateReq.SubtitleConcurrency <= 0 || updateReq.SubtitleBurnConcurrency <= 0 || updateReq.RepairConcurrency <= 0 || updateReq.TranslateConcurrency <= 0 {
		return enum.ErrInvalidParam.WithMsg("并发数必须大于 0")
	}
	// 校验修复设备，支持四种：CPU（cpu）、NVIDIA CUDA（cuda:0）、Apple Silicon MPS（mps）、Intel XPU（xpu:0）
	validRepairDevices := map[string]bool{"cpu": true, "cuda:0": true, "mps": true, "xpu:0": true}
	if !validRepairDevices[updateReq.RepairDevice] {
		return enum.ErrInvalidParam.WithMsg("修复设备必须是 cpu、cuda:0、mps 或 xpu:0")
	}

	settings := map[string]string{
		model.SettingKeyVideoDir:                updateReq.VideoDir,
		model.SettingKeyScanInterval:            strconv.Itoa(updateReq.ScanInterval),
		model.SettingKeyASRURL:                  updateReq.ASRURL,
		model.SettingKeyASRLanguage:             updateReq.ASRLanguage,
		model.SettingKeyASRVadFilter:            strconv.FormatBool(updateReq.ASRVadFilter),
		model.SettingKeyASRTask:                 updateReq.ASRTask,
		model.SettingKeyASREncode:               strconv.FormatBool(updateReq.ASREncode),
		model.SettingKeyASRInitialPrompt:        updateReq.ASRInitialPrompt,
		model.SettingKeyASRWordTimestamps:       strconv.FormatBool(updateReq.ASRWordTimestamps),
		model.SettingKeyASROutput:               updateReq.ASROutput,
		model.SettingKeyRepairDockerImage:       updateReq.RepairDockerImage,
		model.SettingKeyRepairDevice:            updateReq.RepairDevice,
		model.SettingKeySubtitleConcurrency:     strconv.Itoa(updateReq.SubtitleConcurrency),
		model.SettingKeySubtitleBurnConcurrency: strconv.Itoa(updateReq.SubtitleBurnConcurrency),
		model.SettingKeyRepairConcurrency:       strconv.Itoa(updateReq.RepairConcurrency),
		model.SettingKeyTranslateConcurrency:    strconv.Itoa(updateReq.TranslateConcurrency),
	}

	for key, value := range settings {
		if err := model.SettingSet(ctx, key, value); err != nil {
			return enum.ErrDatabase.WithMsg(fmt.Sprintf("保存设置 %s 失败: %v", key, err))
		}
	}

	// 设置已持久化，立即应用到运行时组件；失败仅告警不阻断保存（重启后仍会生效）
	if err := l.ApplyASRFromSettings(ctx); err != nil {
		logger.Logger.Warn("ASR 配置重新加载失败", zap.Error(err))
	}
	if err := l.ApplyRepairFromSettings(ctx); err != nil {
		logger.Logger.Warn("视频修复配置重新加载失败", zap.Error(err))
	}

	return nil
}

// ApplyFFmpegFromSettings 服务启动时从 settings 表加载 ffmpeg 配置并生效
func (l *SettingLogic) ApplyFFmpegFromSettings(ctx context.Context) error {
	provider := model.SettingGet(ctx, settingKeyFFmpegProvider)
	if provider == "" {
		return nil
	}
	cfg := l.loadFFmpegConfig(ctx)
	return ffmpeg.Reload(cfg)
}

// ApplyASRFromSettings 从 settings 表重新加载 ASR 配置并立即生效（保存后/启动时调用）
func (l *SettingLogic) ApplyASRFromSettings(ctx context.Context) error {
	// InitASR 内部已从 settings 表读取全部 ASR 配置项
	return bootstrap.InitASR()
}

// ApplyRepairFromSettings 从 settings 表加载视频修复配置并立即生效（保存后/启动时调用）
func (l *SettingLogic) ApplyRepairFromSettings(ctx context.Context) error {
	cfg := l.loadRepairConfig(ctx)
	if bootstrap.RepairExecutor == nil {
		exec := repair.NewExecutor(cfg)
		if err := exec.Init(cfg); err != nil {
			return err
		}
		bootstrap.RepairExecutor = exec
		return nil
	}
	return bootstrap.RepairExecutor.Reload(cfg)
}

// loadRepairConfig 从 settings 表优先读取视频修复配置，未设置时回退到配置文件/默认值
func (l *SettingLogic) loadRepairConfig(ctx context.Context) repair.Config {
	image := model.SettingGetOrDefault(ctx, model.SettingKeyRepairDockerImage, l.defaultRepairDockerImage())
	device := model.SettingGetOrDefault(ctx, model.SettingKeyRepairDevice, l.defaultRepairDevice())
	return repair.Config{DockerImage: image, Device: device}
}

// loadFFmpegConfig 从 settings 表或配置文件加载 ffmpeg 配置
func (l *SettingLogic) loadFFmpegConfig(ctx context.Context) ffmpeg.Config {
	provider := settingOrDefault(ctx, settingKeyFFmpegProvider, l.defaultFFmpegProvider())
	sshHost := settingOrDefault(ctx, settingKeyFFmpegSSHHost, l.defaultFFmpegString(func(c *bootstrap.AppConfigFFmpeg) string { return c.SSHHost }))
	sshPortStr := settingOrDefault(ctx, settingKeyFFmpegSSHPort, l.defaultFFmpegString(func(c *bootstrap.AppConfigFFmpeg) string { return strconv.Itoa(c.SSHPort) }))
	sshUser := settingOrDefault(ctx, settingKeyFFmpegSSHUser, l.defaultFFmpegString(func(c *bootstrap.AppConfigFFmpeg) string { return c.SSHUser }))
	sshKeyPath := settingOrDefault(ctx, settingKeyFFmpegSSHKeyPath, l.defaultFFmpegString(func(c *bootstrap.AppConfigFFmpeg) string { return c.SSHKeyPath }))

	sshPort := 0
	if v, err := strconv.Atoi(sshPortStr); err == nil {
		sshPort = v
	}

	argsJSON := settingOrDefault(ctx, settingKeyFFmpegSSHArgs, "[]")
	var sshArgs []string
	_ = json.Unmarshal([]byte(argsJSON), &sshArgs)

	return ffmpeg.Config{
		Provider:   provider,
		SSHHost:    sshHost,
		SSHPort:    sshPort,
		SSHUser:    sshUser,
		SSHKeyPath: sshKeyPath,
		SSHArgs:    sshArgs,
	}
}

func (l *SettingLogic) defaultFFmpegProvider() string {
	if bootstrap.Config != nil {
		return bootstrap.Config.FFmpeg.Provider
	}
	return "local"
}

func (l *SettingLogic) defaultFFmpegString(getter func(*bootstrap.AppConfigFFmpeg) string) string {
	if bootstrap.Config != nil {
		return getter(&bootstrap.Config.FFmpeg)
	}
	return ""
}

func (l *SettingLogic) defaultVideoDir() string {
	if bootstrap.Config != nil {
		return bootstrap.Config.Video.Dir
	}
	return model.DefaultVideoDir
}

func (l *SettingLogic) defaultScanInterval() int {
	if bootstrap.Config != nil && bootstrap.Config.Scan.Interval > 0 {
		return bootstrap.Config.Scan.Interval
	}
	if v, err := strconv.Atoi(model.DefaultScanInterval); err == nil {
		return v
	}
	return 60
}

func (l *SettingLogic) defaultASRURL() string {
	if bootstrap.Config != nil && bootstrap.Config.ASR.URL != "" {
		return bootstrap.Config.ASR.URL
	}
	return model.DefaultASRURL
}

func (l *SettingLogic) defaultASRLanguage() string {
	if bootstrap.Config != nil && bootstrap.Config.ASR.Language != "" {
		return bootstrap.Config.ASR.Language
	}
	return model.DefaultASRLanguage
}

// defaultASRString 从配置文件读取 ASR 字符串字段，未配置时使用默认值
func (l *SettingLogic) defaultASRString(getter func(*bootstrap.AppConfigASR) string, def string) string {
	if bootstrap.Config != nil {
		if v := getter(&bootstrap.Config.ASR); v != "" {
			return v
		}
	}
	return def
}

// defaultASRBoolStr 从配置文件读取 ASR 布尔字段，未配置时使用默认值字符串
func (l *SettingLogic) defaultASRBoolStr(def string, fallback bool) string {
	if bootstrap.Config != nil {
		// 配置文件中布尔零值时 fallback 到默认值
		return strconv.FormatBool(fallback)
	}
	return def
}

func (l *SettingLogic) defaultRepairDockerImage() string {
	if bootstrap.Config != nil && bootstrap.Config.Repair.DockerImage != "" {
		return bootstrap.Config.Repair.DockerImage
	}
	return model.DefaultRepairDockerImage
}

func (l *SettingLogic) defaultRepairDevice() string {
	if bootstrap.Config != nil && bootstrap.Config.Repair.Device != "" {
		return bootstrap.Config.Repair.Device
	}
	return model.DefaultRepairDevice
}

// defaultSubtitleConcurrency 字幕默认并发数。
// 注意：与调度器 loadConcurrency 的兜底保持一致（settings 表未设置时使用的值），
// 避免界面显示值与调度器实际并发数不一致。
func (l *SettingLogic) defaultSubtitleConcurrency() int {
	if v, err := strconv.Atoi(model.DefaultSubtitleConcurrency); err == nil {
		return v
	}
	return 2
}

func (l *SettingLogic) defaultSubtitleBurnConcurrency() int {
	if v, err := strconv.Atoi(model.DefaultSubtitleBurnConcurrency); err == nil {
		return v
	}
	return 1
}

func (l *SettingLogic) defaultRepairConcurrency() int {
	if v, err := strconv.Atoi(model.DefaultRepairConcurrency); err == nil {
		return v
	}
	return 1
}

func (l *SettingLogic) defaultTranslateConcurrency() int {
	if v, err := strconv.Atoi(model.DefaultTranslateConcurrency); err == nil {
		return v
	}
	return 1
}

func (l *SettingLogic) settingIntOrDefault(ctx context.Context, key string, defaultValue, min, max int) int {
	valueStr := model.SettingGet(ctx, key)
	if valueStr == "" {
		return defaultValue
	}
	v, err := strconv.Atoi(valueStr)
	if err != nil || v < min || v > max {
		return defaultValue
	}
	return v
}

func parseBoolString(s string) bool {
	v, _ := strconv.ParseBool(s)
	return v
}

func settingOrDefault(ctx context.Context, key, defaultValue string) string {
	value := model.SettingGet(ctx, key)
	if value == "" {
		return defaultValue
	}
	return value
}
