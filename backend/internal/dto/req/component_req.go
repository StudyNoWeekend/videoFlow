package req

// ComponentInstallReq 组件安装请求
type ComponentInstallReq struct {
	ComponentType string `json:"component_type" binding:"required,oneof=lada ffmpeg"`
}

// ComponentUninstallReq 组件卸载请求
type ComponentUninstallReq struct {
	ComponentType string `json:"component_type" binding:"required,oneof=lada ffmpeg"`
}