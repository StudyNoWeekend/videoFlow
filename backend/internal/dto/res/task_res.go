package res

// TaskRes 任务信息响应
type TaskRes struct {
	ID                string      `json:"id"`
	VideoID           string      `json:"video_id"`
	TaskType          string      `json:"task_type"`
	Status            string      `json:"status"`
	SourcePath        string      `json:"source_path,omitempty"` // 实际处理源文件路径，空表示使用关联视频
	Overwrite         bool        `json:"overwrite,omitempty"`   // 是否覆盖处理源文件（仅衍生视频有效）
	TargetWidth       int         `json:"target_width,omitempty"`
	TargetHeight      int         `json:"target_height,omitempty"`
	UpscaleProcessor  string      `json:"upscale_processor,omitempty"`
	UpscaleModel      string      `json:"upscale_model,omitempty"`
	UpscaleNoiseLevel int         `json:"upscale_noise_level,omitempty"`
	Progress          int         `json:"progress"`
	ProgressMsg       string      `json:"progress_msg,omitempty"`
	Result            interface{} `json:"result,omitempty"`
	ErrorMsg          string      `json:"error_msg,omitempty"`
	RetryCount        int         `json:"retry_count"`
	CreatedAt         int64       `json:"created_at"`
	UpdatedAt         int64       `json:"updated_at"`
	Video             *VideoRes   `json:"video,omitempty"`
}

// TaskListRes 任务列表分页响应
type TaskListRes struct {
	List     []*TaskRes `json:"list"`
	Total    int64      `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
}
