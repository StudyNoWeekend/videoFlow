package i18n

var messages = map[string]map[string]string{
	"zh": {
		"component.docker.name":      "Docker",
		"component.docker.desc":      "容器运行时环境，所有 Docker 组件的依赖",
		"component.ffmpeg.name":      "FFmpeg",
		"component.ffmpeg.desc":      "音视频处理工具，用于音频提取和视频处理",
		"component.whisper_asr.name": "Whisper ASR",
		"component.whisper_asr.desc": "语音识别服务，用于生成视频字幕",
		"component.lada.name":        "Lada",
		"component.lada.desc":        "视频修复工具，用于去除视频马赛克",
		"status.installed":           "已安装",
		"status.missing":             "未安装",
		"status.installing":          "安装中",
		"status.error":               "异常",
		"install.start":              "开始安装",
		"install.pulling":            "正在拉取镜像",
		"install.starting":           "正在启动容器",
		"install.waiting":            "等待服务就绪",
		"install.verifying":          "验证安装",
		"install.completed":          "安装完成",
		"install.failed":             "安装失败",
		"install.cancelled":          "安装已取消",
		"uninstall.start":            "开始卸载",
		"uninstall.stopping":         "正在停止容器",
		"uninstall.removing":         "正在移除容器",
		"uninstall.rmi":              "正在删除镜像",
		"uninstall.completed":        "卸载完成",
		"uninstall.failed":           "卸载失败",
		"error.docker.required":      "Docker 未安装，请先安装 Docker",
	},
	"en": {
		"component.docker.name":      "Docker",
		"component.docker.desc":      "Container runtime, required by all Docker components",
		"component.ffmpeg.name":      "FFmpeg",
		"component.ffmpeg.desc":      "Audio/video processing tool for audio extraction and video processing",
		"component.whisper_asr.name": "Whisper ASR",
		"component.whisper_asr.desc": "Speech recognition service for generating subtitles",
		"component.lada.name":        "Lada",
		"component.lada.desc":        "Video restoration tool for removing mosaics",
		"status.installed":           "Installed",
		"status.missing":             "Not Installed",
		"status.installing":          "Installing",
		"status.error":               "Error",
		"install.start":              "Starting installation",
		"install.pulling":            "Pulling image",
		"install.starting":           "Starting container",
		"install.waiting":            "Waiting for service ready",
		"install.verifying":          "Verifying installation",
		"install.completed":          "Installation completed",
		"install.failed":             "Installation failed",
		"install.cancelled":          "Installation cancelled",
		"uninstall.start":            "Starting uninstall",
		"uninstall.stopping":         "Stopping container",
		"uninstall.removing":         "Removing container",
		"uninstall.rmi":              "Removing image",
		"uninstall.completed":        "Uninstall completed",
		"uninstall.failed":           "Uninstall failed",
		"error.docker.required":      "Docker is not installed. Please install Docker first",
	},
}

// T 根据语言键获取消息
func T(lang, key string) string {
	if msg, ok := messages[lang][key]; ok {
		return msg
	}
	// fallback to zh
	if msg, ok := messages["zh"][key]; ok {
		return msg
	}
	return key
}

// DetectLang 从 Accept-Language 获取优先语言
func DetectLang(acceptLang string) string {
	if len(acceptLang) >= 2 && acceptLang[:2] == "en" {
		return "en"
	}
	return "zh"
}
