package req

// TaskCreateReq 创建任务请求
type TaskCreateReq struct {
	VideoID  string `json:"video_id" binding:"required"`
	TaskType string `json:"task_type" binding:"required,oneof=subtitle subtitle_burn deblur upscale"`
	// SourcePath 可选：实际处理源文件路径，为空时默认使用关联视频（如仅选择原视频）
	SourcePath string `json:"source_path"`
	// TargetWidth 清晰度修复目标宽度（像素），仅 upscale 任务时需要
	TargetWidth int `json:"target_width"`
	// TargetHeight 清晰度修复目标高度（像素），仅 upscale 任务时需要
	TargetHeight int `json:"target_height"`
	// UpscaleProcessor 清晰度修复处理器类型，创建清晰度去马赛克任务时前端弹窗选择
	UpscaleProcessor string `json:"upscale_processor"`
	// UpscaleModel 清晰度修复模型/着色器名，创建清晰度去马赛克任务时前端弹窗选择
	UpscaleModel string `json:"upscale_model"`
	// UpscaleNoiseLevel 降噪等级，仅 realesrgan/realcugan 生效（-1=无/保守，0-3 递增），创建清晰度去马赛克任务时前端弹窗选择
	UpscaleNoiseLevel int `json:"upscale_noise_level" binding:"omitempty,gte=-1,lte=3"`
}

// TaskListReq 任务列表分页查询请求
type TaskListReq struct {
	Page     int    `form:"page" binding:"omitempty,gte=1"`
	PageSize int    `form:"page_size" binding:"omitempty,gte=1,lte=100"`
	TaskType string `form:"task_type" binding:"omitempty,oneof=subtitle subtitle_burn deblur upscale"`
}
