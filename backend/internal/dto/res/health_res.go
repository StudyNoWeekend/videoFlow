package res

import "video-captions/internal/component"

// HealthRes 健康检查响应
type HealthRes struct {
	Status     string                    `json:"status"`
	Components []component.ComponentInfo `json:"components,omitempty"`
}

// ReadyRes 就绪检查响应
type ReadyRes struct {
	Status string `json:"status"`
}
