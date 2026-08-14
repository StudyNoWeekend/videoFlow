package req

// SettingReq 统一设置保存请求
type SettingReq struct {
	VideoDir          string `json:"video_dir"`
	ScanInterval      int    `json:"scan_interval" binding:"gte=1"`
	ASRURL            string `json:"asr_url" binding:"required,url"`
	ASRLanguage       string `json:"asr_language" binding:"required"`
	ASRVadFilter      bool   `json:"asr_vad_filter"`
	ASRTask           string `json:"asr_task" binding:"omitempty,oneof=transcribe translate"`
	ASREncode         bool   `json:"asr_encode"`
	ASRInitialPrompt  string `json:"asr_initial_prompt"`
	ASRWordTimestamps bool   `json:"asr_word_timestamps"`
	ASROutput         string `json:"asr_output" binding:"omitempty,oneof=txt vtt srt tsv json"`
	RepairDockerImage string `json:"repair_docker_image" binding:"required"`
	// RepairDevice 视频修复计算设备，支持 cpu / cuda:0 / mps / xpu:0 四种
	RepairDevice        string `json:"repair_device" binding:"required,oneof=cpu cuda:0 mps xpu:0"`
	SubtitleConcurrency int    `json:"subtitle_concurrency" binding:"required,gte=1,lte=50"`
	// SubtitleBurnConcurrency 字幕烧录（写入视频）并发数
	SubtitleBurnConcurrency int `json:"subtitle_burn_concurrency" binding:"required,gte=1,lte=50"`
	RepairConcurrency       int `json:"repair_concurrency" binding:"required,gte=1,lte=50"`
	// TranslateConcurrency 翻译并发数
	TranslateConcurrency int `json:"translate_concurrency" binding:"required,gte=1,lte=50"`
}
