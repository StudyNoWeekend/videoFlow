package req

// VideoScanReq 视频目录扫描请求
type VideoScanReq struct {
	Path string `json:"path" binding:"omitempty"`
}

// VideoListReq 视频列表分页查询请求
type VideoListReq struct {
	Page     int `form:"page" binding:"omitempty,gte=1"`
	PageSize int `form:"page_size" binding:"omitempty,gte=1,lte=100"`
}

// VideoUpdateReq 视频信息更新请求
type VideoUpdateReq struct {
	Name string `json:"name" binding:"required,max=255"`
}

// VideoBatchDeleteReq 视频批量删除请求
type VideoBatchDeleteReq struct {
	IDs []string `json:"ids" binding:"required,min=1"`
	// DeleteFiles 是否同时删除视频对应的输出目录（srt、烧录/去马赛克/清晰度修复产物）
	DeleteFiles bool `json:"delete_files"`
}
