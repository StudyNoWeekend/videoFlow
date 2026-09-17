package req

// SettingReq 统一设置保存请求
type SettingReq struct {
	// 目录类字段 max=1024；数值字段统一 omitempty+范围校验，缺省值由 logic 层回填
	VideoDir     string `json:"video_dir" binding:"omitempty,max=1024"`
	OutputDir    string `json:"output_dir" binding:"omitempty,max=1024"`
	ScanInterval int    `json:"scan_interval" binding:"omitempty,gte=1,lte=86400"`
	// ASRURL 为空表示未配置 ASR（可选），非空时必须是合法 URL
	ASRURL            string `json:"asr_url" binding:"omitempty,url,max=1024"`
	ASRLanguage       string `json:"asr_language" binding:"omitempty,max=32"`
	ASRVadFilter      bool   `json:"asr_vad_filter"`
	ASRTask           string `json:"asr_task" binding:"omitempty,oneof=transcribe translate"`
	ASREncode         bool   `json:"asr_encode"`
	ASRInitialPrompt  string `json:"asr_initial_prompt" binding:"omitempty,max=2048"`
	ASRWordTimestamps bool   `json:"asr_word_timestamps"`
	ASROutput         string `json:"asr_output" binding:"omitempty,oneof=txt vtt srt tsv json"`
	RepairDockerImage string `json:"repair_docker_image" binding:"required,max=256"`
	// RepairDevice 去马赛克计算设备，支持 cpu / cuda:0 / mps / xpu:0 四种
	RepairDevice        string `json:"repair_device" binding:"required,oneof=cpu cuda:0 mps xpu:0"`
	SubtitleConcurrency int    `json:"subtitle_concurrency" binding:"omitempty,gte=1,lte=50"`
	// SubtitleBurnConcurrency 字幕烧录（写入视频）并发数
	SubtitleBurnConcurrency int    `json:"subtitle_burn_concurrency" binding:"omitempty,gte=1,lte=50"`
	RepairConcurrency       int    `json:"repair_concurrency" binding:"omitempty,gte=1,lte=50"`
	SchedulerPollInterval   int    `json:"scheduler_poll_interval" binding:"omitempty,gte=1,lte=3600"`
	UpscaleDockerImage      string `json:"upscale_docker_image" binding:"omitempty,max=256"`
	UpscaleDevice           string `json:"upscale_device" binding:"omitempty,oneof=cpu cuda:0 mps xpu:0"`
	UpscaleConcurrency      int    `json:"upscale_concurrency" binding:"omitempty,gte=1,lte=50"`
	// TelegramAppID/TelegramAppHash 来自 my.telegram.org，留空表示停用 Telegram 下载
	TelegramAppID string `json:"telegram_app_id" binding:"omitempty,max=32"`
	// TelegramAppHash 为敏感凭据，保存后会原样回显给已登录用户
	TelegramAppHash string `json:"telegram_app_hash" binding:"omitempty,max=128"`
	TelegramThreads int    `json:"telegram_threads" binding:"omitempty,gte=1,lte=16"`
	TelegramDataDir string `json:"telegram_data_dir" binding:"omitempty,max=1024"`
	// ProxyURL 全局出站代理，Telegram 与 yt-dlp 共用；scheme 白名单在 logic 层校验
	ProxyURL string `json:"proxy_url" binding:"omitempty,max=1024"`
	// ProxyForYtdlp 是否让 yt-dlp 使用全局代理
	ProxyForYtdlp bool `json:"proxy_for_ytdlp"`
}
