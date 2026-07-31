package res

// HealthRes 健康检查响应
type HealthRes struct {
	Status string `json:"status"`
}

// ReadyRes 就绪检查响应
type ReadyRes struct {
	Status string `json:"status"`
}
