package res

// ComponentInfoRes 组件信息响应
type ComponentInfoRes struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Version     string `json:"version"`
	ErrorMsg    string `json:"error_msg,omitempty"`
	Description string `json:"description"`
	NeedsDocker bool   `json:"needs_docker"`
}

// ComponentListRes 组件列表响应
type ComponentListRes struct {
	Components []ComponentInfoRes `json:"components"`
}

// ComponentInstallRes 安装响应（返回 session_id）
type ComponentInstallRes struct {
	SessionID string `json:"session_id"`
}
