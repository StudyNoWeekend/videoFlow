package component

// ComponentType 组件类型
type ComponentType string

const (
	ComponentDocker     ComponentType = "docker"
	ComponentFFmpeg     ComponentType = "ffmpeg"
	ComponentWhisperASR ComponentType = "whisper_asr"
	ComponentLada       ComponentType = "lada"
	ComponentVideo2X    ComponentType = "video2x"
)

// ComponentStatus 组件状态
type ComponentStatus string

const (
	StatusInstalled  ComponentStatus = "installed"
	StatusMissing    ComponentStatus = "missing"
	StatusInstalling ComponentStatus = "installing"
	StatusError      ComponentStatus = "error"
)

// ComponentInfo 组件信息
type ComponentInfo struct {
	Type        ComponentType   `json:"type"`
	Name        string          `json:"name"`
	Status      ComponentStatus `json:"status"`
	Version     string          `json:"version"`
	ErrorMsg    string          `json:"error_msg,omitempty"`
	Description string          `json:"description"`
	NeedsDocker bool            `json:"needs_docker"`
}

// ComponentList 组件列表响应
type ComponentList struct {
	Components []ComponentInfo `json:"components"`
}
