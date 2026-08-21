package bootstrap

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config 全局配置
var Config *AppConfig

// AppVersion 应用版本号
const AppVersion = "v0.3.2"

// AppConfig 应用配置总入口
type AppConfig struct {
	App         AppConfigApp         `mapstructure:"app"`
	HTTP        AppConfigHTTP        `mapstructure:"http"`
	Log         AppConfigLog         `mapstructure:"log"`
	Database    AppConfigDatabase    `mapstructure:"database"`
	ASR         AppConfigASR         `mapstructure:"asr"`
	Worker      AppConfigWorker      `mapstructure:"worker"`
	FFmpeg      AppConfigFFmpeg      `mapstructure:"ffmpeg"`
	Video       AppConfigVideo       `mapstructure:"video"`
	Output      AppConfigOutput      `mapstructure:"output"`
	Scan        AppConfigScan        `mapstructure:"scan"`
	Repair      AppConfigRepair      `mapstructure:"repair"`
	Upscale     AppConfigUpscale     `mapstructure:"upscale"`
	Scheduler   AppConfigScheduler   `mapstructure:"scheduler"`
	Concurrency AppConfigConcurrency `mapstructure:"concurrency"`
}

// AppConfigApp 应用基础配置
type AppConfigApp struct {
	Name string `mapstructure:"name"`
	Env  string `mapstructure:"env"`
	Port int    `mapstructure:"port"`
}

// AppConfigHTTP HTTP 服务配置
type AppConfigHTTP struct {
	Port         int `mapstructure:"port"`
	ReadTimeout  int `mapstructure:"read_timeout"`
	WriteTimeout int `mapstructure:"write_timeout"`
}

// AppConfigLog 日志配置
type AppConfigLog struct {
	Level string `mapstructure:"level"`
	Path  string `mapstructure:"path"`
}

// AppConfigDatabase 数据库配置
type AppConfigDatabase struct {
	Driver          string `mapstructure:"driver"`
	DSN             string `mapstructure:"dsn"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"`
}

// AppConfigASR ASR 引擎配置（对应 Whisper ASR Webservice /asr 接口参数）
type AppConfigASR struct {
	Provider       string `mapstructure:"provider"`
	URL            string `mapstructure:"url"`
	APIKey         string `mapstructure:"api_key"`
	Model          string `mapstructure:"model"`
	MaxConcurrency int    `mapstructure:"max_concurrency"`
	Language       string `mapstructure:"language"`
	VadFilter      bool   `mapstructure:"vad_filter"`
	// 以下为 /asr 接口的 query 参数
	Task           string `mapstructure:"task"`            // transcribe 或 translate，默认 transcribe
	Encode         bool   `mapstructure:"encode"`          // 是否先通过 ffmpeg 编码音频，默认 true
	InitialPrompt  string `mapstructure:"initial_prompt"`  // 初始提示词
	WordTimestamps bool   `mapstructure:"word_timestamps"` // 是否生成词级时间戳，默认 false
	Output         string `mapstructure:"output"`          // 输出格式 txt/vtt/srt/tsv/json，默认 json
}

// AppConfigWorker worker 并发配置
type AppConfigWorker struct {
	MaxConcurrency int `mapstructure:"max_concurrency"`
}

// AppConfigFFmpeg ffmpeg 执行环境配置
type AppConfigFFmpeg struct {
	Provider   string   `mapstructure:"provider"`
	SSHHost    string   `mapstructure:"ssh_host"`
	SSHPort    int      `mapstructure:"ssh_port"`
	SSHUser    string   `mapstructure:"ssh_user"`
	SSHKeyPath string   `mapstructure:"ssh_key_path"`
	SSHArgs    []string `mapstructure:"ssh_args"`
}

// AppConfigVideo 本地视频目录配置
type AppConfigVideo struct {
	Dir string `mapstructure:"dir"`
}

// AppConfigOutput 任务输出目录配置
type AppConfigOutput struct {
	Dir string `mapstructure:"dir"`
}

// AppConfigScan 扫描配置
type AppConfigScan struct {
	Interval int `mapstructure:"interval"`
}

// AppConfigRepair 去马赛克 Docker 配置
type AppConfigRepair struct {
	DockerImage string `mapstructure:"docker_image"`
	// Device 计算设备，支持四种：cpu / cuda:0（NVIDIA CUDA）/ mps（Apple Silicon）/ xpu:0（Intel XPU）
	Device string `mapstructure:"device"`
}

// AppConfigUpscale 清晰度修复 Docker 配置
type AppConfigUpscale struct {
	DockerImage string `mapstructure:"docker_image"`
	// Device 计算设备，支持四种：cpu / cuda:0（NVIDIA CUDA）/ mps（Apple Silicon）/ xpu:0（Intel XPU）
	Device    string `mapstructure:"device"`
	Processor string `mapstructure:"processor"`
	Model     string `mapstructure:"model"`
}

// AppConfigScheduler 调度器配置
type AppConfigScheduler struct {
	PollInterval int `mapstructure:"poll_interval"`
}

// AppConfigConcurrency 并发数配置
type AppConfigConcurrency struct {
	Subtitle int `mapstructure:"subtitle"`
	Repair   int `mapstructure:"repair"`
}

// InitConfig 初始化配置，支持环境变量覆盖
func InitConfig() (*AppConfig, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("./config")
	v.AddConfigPath("../config")
	v.AddConfigPath("./backend/config")
	v.AddConfigPath("../backend/config")

	// 支持环境变量覆盖，格式：APP_NAME_HTTP_PORT
	v.SetEnvPrefix("app")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	cfg := &AppConfig{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 环境变量覆盖敏感配置
	if apiKey := os.Getenv("APP_ASR_API_KEY"); apiKey != "" {
		cfg.ASR.APIKey = apiKey
	}

	// 后端端口固定为 8080，不允许通过环境变量或配置文件修改
	// Docker 容器内通过 nginx 反向代理，仅暴露前端端口
	cfg.HTTP.Port = 8080

	Config = cfg
	return cfg, nil
}
