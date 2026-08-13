package res

// TaskSnapshotRes 任务状态快照
type TaskSnapshotRes struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Progress  int    `json:"progress"`
	ErrorMsg  string `json:"error_msg,omitempty"`
	UpdatedAt int64  `json:"updated_at"`
}

// OutputFileRes 视频输出目录中的文件信息
type OutputFileRes struct {
	Name      string `json:"name"`
	Path      string `json:"path,omitempty"`
	Size      int64  `json:"size"`
	IsVideo   bool   `json:"is_video"`
	FileType  string `json:"file_type"` // subtitle / translated / subtitled_video / repaired_video / unknown
	UpdatedAt int64  `json:"updated_at"`
}

// VideoRes 视频信息响应
type VideoRes struct {
	ID              string           `json:"id"`
	Path            string           `json:"path"`
	Name            string           `json:"name"`
	Duration        int64            `json:"duration"`
	Size            int64            `json:"size"`
	CreatedAt       int64            `json:"created_at"`
	UpdatedAt       int64            `json:"updated_at"`
	SubtitleTask     *TaskSnapshotRes `json:"subtitle_task,omitempty"`
	SubtitleBurnTask *TaskSnapshotRes `json:"subtitle_burn_task,omitempty"`
	DeblurTask       *TaskSnapshotRes `json:"deblur_task,omitempty"`
	OutputDir       string           `json:"output_dir,omitempty"`    // 输出目录路径
	OutputFiles     []*OutputFileRes `json:"output_files,omitempty"` // 输出文件列表
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
