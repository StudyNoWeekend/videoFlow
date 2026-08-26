package req

// DownloadCreateReq 创建下载任务请求
type DownloadCreateReq struct {
	URL         string `json:"url" binding:"required"`
	Overwrite   bool   `json:"overwrite"`              // false=文件冲突时自动重命名, true=覆盖
	DownloadDir string `json:"download_dir,omitempty"` // 下载存放目录，为空时使用本地视频目录
}
