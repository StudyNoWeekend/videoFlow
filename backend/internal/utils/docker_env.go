package utils

import (
	"os"
	"strings"
)

// IsRunningInContainer 检测当前进程是否运行在 Docker 容器内。
// 优先检查 CONTAINER_RUNTIME 环境变量（由 Dockerfile 定义），
// 回退到检测 /.dockerenv 文件是否存在（Docker 自动生成）。
func IsRunningInContainer() bool {
	// 优先使用环境变量（在 Dockerfile 中定义 CONTAINER_RUNTIME=docker）
	if os.Getenv("CONTAINER_RUNTIME") == "docker" {
		return true
	}

	// 回退：检查 /.dockerenv 文件（Docker 引擎自动生成）
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	// 回退：检查 /proc/1/cgroup 是否包含 "docker" 字符串
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		return strings.Contains(string(data), "docker")
	}

	return false
}
