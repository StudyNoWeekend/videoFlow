package req

// ComponentInstallReq 组件安装请求
type ComponentInstallReq struct {
	ComponentType string `json:"component_type" binding:"required,oneof=whisper_asr lada ffmpeg"`
	// Whisper ASR 参数
	ASREngine string `json:"asr_engine" binding:"omitempty,oneof=openai_whisper faster_whisper whisperx"`
	ASRModel  string `json:"asr_model"`
	ASRDevice string `json:"asr_device" binding:"omitempty,oneof=cpu cuda"`
	HFToken   string `json:"hf_token"`
}

// ComponentUninstallReq 组件卸载请求
type ComponentUninstallReq struct {
	ComponentType string `json:"component_type" binding:"required,oneof=whisper_asr lada ffmpeg"`
}