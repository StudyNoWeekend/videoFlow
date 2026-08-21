package res

// SettingRes 统一设置响应
type SettingRes struct {
	VideoDir                string `json:"video_dir"`
	OutputDir               string `json:"output_dir"`
	ScanInterval            int    `json:"scan_interval"`
	ASRURL                  string `json:"asr_url"`
	ASRLanguage             string `json:"asr_language"`
	ASRVadFilter            bool   `json:"asr_vad_filter"`
	ASRTask                 string `json:"asr_task"`
	ASREncode               bool   `json:"asr_encode"`
	ASRInitialPrompt        string `json:"asr_initial_prompt"`
	ASRWordTimestamps       bool   `json:"asr_word_timestamps"`
	ASROutput               string `json:"asr_output"`
	RepairDockerImage       string `json:"repair_docker_image"`
	RepairDevice            string `json:"repair_device"`
	SubtitleConcurrency     int    `json:"subtitle_concurrency"`
	SubtitleBurnConcurrency int    `json:"subtitle_burn_concurrency"`
	RepairConcurrency       int    `json:"repair_concurrency"`
	SchedulerPollInterval   int    `json:"scheduler_poll_interval"`
	UpscaleDockerImage      string `json:"upscale_docker_image"`
	UpscaleDevice           string `json:"upscale_device"`
	UpscaleConcurrency      int    `json:"upscale_concurrency"`
}
