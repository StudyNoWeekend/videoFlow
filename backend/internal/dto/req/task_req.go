package req

// TaskCreateReq 创建任务请求
type TaskCreateReq struct {
	VideoID  string `json:"video_id" binding:"required"`
	TaskType string `json:"task_type" binding:"required,oneof=subtitle repair"`
}

// TaskListReq 任务列表分页查询请求
type TaskListReq struct {
	Page     int    `form:"page" binding:"omitempty,gte=1"`
	PageSize int    `form:"page_size" binding:"omitempty,gte=1,lte=100"`
	TaskType string `form:"task_type" binding:"omitempty,oneof=subtitle repair"`
}
