package res

// DownloadRes 下载任务信息响应
type DownloadRes struct {
	ID             string `json:"id"`
	URL            string `json:"url"`
	Status         string `json:"status"`
	Progress       int    `json:"progress"`
	ProgressMsg    string `json:"progress_msg,omitempty"`
	ErrorMsg       string `json:"error_msg,omitempty"`
	FileName       string `json:"file_name,omitempty"`
	FileSize       int64  `json:"file_size,omitempty"`
	Duration       int64  `json:"duration,omitempty"`
	Title          string `json:"title,omitempty"`
	DownloadSpeed  int64  `json:"download_speed,omitempty"`
	TotalSize      int64  `json:"total_size,omitempty"`
	DownloadedSize int64  `json:"downloaded_size,omitempty"`
	Overwrite      bool   `json:"overwrite"`
	DownloadDir    string `json:"download_dir,omitempty"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

// DownloadListRes 下载列表分页响应
type DownloadListRes struct {
	List     []*DownloadRes `json:"list"`
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}
