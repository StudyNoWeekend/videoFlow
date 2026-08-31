package req

// DownloadCreateReq 创建下载任务请求
type DownloadCreateReq struct {
	URL         string `json:"url" binding:"required"`
	Overwrite   bool   `json:"overwrite"`              // false=文件冲突时自动重命名, true=覆盖
	DownloadDir string `json:"download_dir,omitempty"` // 下载存放目录，为空时使用本地视频目录
}

// DownloadListReq 下载任务列表分页查询请求
type DownloadListReq struct {
	Page     int    `form:"page" binding:"omitempty,gte=1"`
	PageSize int    `form:"page_size" binding:"omitempty,gte=1,lte=100"`
	SortBy   string `form:"sort_by" binding:"omitempty"`
	Order    string `form:"order" binding:"omitempty,oneof=asc desc"`
}
