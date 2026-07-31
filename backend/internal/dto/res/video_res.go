package res

// TaskSnapshotRes 任务状态快照
type TaskSnapshotRes struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Progress  int    `json:"progress"`
	ErrorMsg  string `json:"error_msg,omitempty"`
	UpdatedAt int64  `json:"updated_at"`
}

// VideoRes 视频信息响应
type VideoRes struct {
	ID           string           `json:"id"`
	Path         string           `json:"path"`
	Name         string           `json:"name"`
	Duration     int64            `json:"duration"`
	Size         int64            `json:"size"`
	CreatedAt    int64            `json:"created_at"`
	UpdatedAt    int64            `json:"updated_at"`
	SubtitleTask *TaskSnapshotRes `json:"subtitle_task,omitempty"`
	RepairTask   *TaskSnapshotRes `json:"repair_task,omitempty"`
}

// VideoListRes 视频列表分页响应
type VideoListRes struct {
	List     []*VideoRes `json:"list"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

// VideoScanRes 视频扫描结果响应
type VideoScanRes struct {
	Scanned int `json:"scanned"`
}
