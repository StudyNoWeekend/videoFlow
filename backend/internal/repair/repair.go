package repair

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// Config 视频修复 Docker 配置
type Config struct {
	DockerImage string
	// Device 计算设备，支持四种：cpu（CPU）、cuda:0（NVIDIA CUDA）、mps（Apple Silicon MPS）、xpu:0（Intel XPU）
	Device string
}

// ProgressCallback 修复进度回调，progress 为 0-100，message 为当前进度行原始文本
type ProgressCallback func(progress int, message string)

// Executor 视频修复执行器，在本地执行 Docker 修复命令
type Executor struct {
	mu     sync.RWMutex
	config Config
}

// NewExecutor 创建视频修复执行器实例
func NewExecutor(cfg Config) *Executor {
	return &Executor{
		config: cfg,
	}
}

// Init 根据配置初始化执行器，会校验 docker 是否可用。
func (e *Executor) Init(cfg Config) error {
	return e.Reload(cfg)
}

// Reload 运行时重新加载配置，并校验 docker 是否可用。
func (e *Executor) Reload(cfg Config) error {
	if cfg.DockerImage == "" {
		return errors.New("修复 Docker 镜像不能为空")
	}

	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("未找到 docker 命令，请确认 Docker 已安装并启动: %w", err)
	}

	e.mu.Lock()
	e.config = cfg
	e.mu.Unlock()
	return nil
}

// resolveHostPath 将容器内路径转换为宿主机路径。
// 在 Docker 容器内通过宿主机 Docker daemon 执行 docker run 时，bind mount 的 src 路径
// 必须是宿主机的文件系统路径，而非容器内路径。
//
// 优先通过 docker inspect 获取准确的宿主机路径（不受 btrfs subvol / overlay2 等影响），
// 失败时 fallback 到 /proc/self/mountinfo 解析。
// 非 Docker 环境或路径无法映射时返回原路径。
func resolveHostPath(containerPath string) string {
	// 优先通过 docker inspect 获取挂载的 Source（宿主机完整路径）
	if hostPath, ok := resolveHostPathViaDocker(containerPath); ok {
		return hostPath
	}
	// Fallback: 通过 /proc/self/mountinfo 解析（非 btrfs subvol 场景下可用）
	return resolveHostPathViaMountinfo(containerPath)
}

// containerMountEntry 表示 docker inspect 返回的挂载条目
type containerMountEntry struct {
	Source      string `json:"Source"`
	Destination string `json:"Destination"`
}

// cachedMounts 缓存 docker inspect 的挂载结果，避免每次修复都执行一次。
var (
	cachedMounts     []containerMountEntry
	cachedMountsOnce sync.Once
)

// loadContainerMounts 通过 docker inspect 获取当前容器的所有 bind mount 信息。
// 结果在首次调用后缓存。返回 nil 表示无法获取（非 Docker 环境或 docker 不可用）。
func loadContainerMounts() []containerMountEntry {
	cachedMountsOnce.Do(func() {
		containerID := getContainerID()
		if containerID == "" {
			return
		}
		cmd := exec.Command("docker", "inspect",
			"--format", "{{json .Mounts}}", containerID)
		output, err := cmd.Output()
		if err != nil {
			return
		}
		var mounts []containerMountEntry
		if err := json.Unmarshal(output, &mounts); err != nil {
			return
		}
		cachedMounts = mounts
	})
	return cachedMounts
}

// resolveHostPathViaDocker 通过 docker inspect 获取挂载信息，
// 找到 Destination 最长匹配 containerPath 的挂载，返回其 Source（宿主机路径）。
func resolveHostPathViaDocker(containerPath string) (string, bool) {
	mounts := loadContainerMounts()
	if mounts == nil {
		return "", false
	}

	bestMatchLen := 0
	bestSource := ""
	for _, m := range mounts {
		if m.Destination == "" || m.Source == "" {
			continue
		}
		if strings.HasPrefix(containerPath, m.Destination) && len(m.Destination) > bestMatchLen {
			bestMatchLen = len(m.Destination)
			bestSource = m.Source
		}
	}

	if bestSource != "" {
		suffix := containerPath[bestMatchLen:]
		return filepath.Join(bestSource, suffix), true
	}
	return "", false
}

// getContainerID 获取当前容器的 ID。
// 优先从 /proc/self/cgroup 解析，失败时 fallback 到 hostname（Docker 默认为容器短 ID）。
func getContainerID() string {
	data, err := os.ReadFile("/proc/self/cgroup")
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			// cgroup v2: 0::/.../docker-<id>.scope
			if idx := strings.Index(line, "docker-"); idx >= 0 {
				s := line[idx+len("docker-"):]
				if end := strings.Index(s, ".scope"); end >= 0 {
					return s[:end]
				}
			}
			// cgroup v1/v2: ...:/docker/<id>
			if idx := strings.Index(line, "/docker/"); idx >= 0 {
				return strings.TrimSpace(line[idx+len("/docker/"):])
			}
		}
	}

	// Fallback: hostname（Docker 默认为容器短 ID，12 位）
	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		return hostname
	}
	return ""
}

// resolveHostPathViaMountinfo 通过 /proc/self/mountinfo 解析容器内路径到宿主机路径。
// 注意：在 btrfs subvol / overlay2 等场景下，mountinfo 的 root 字段记录的是
// 相对于文件系统根的路径，而非宿主机绝对路径，此函数在这些场景下可能不准确。
func resolveHostPathViaMountinfo(containerPath string) string {
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return containerPath
	}

	bestMatchLen := 0
	bestHostRoot := ""

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)

		// mountinfo 格式：
		// mount_id parent_id major:minor root mount_point options [optional_fields] - fstype source super_options
		// fields[3]=root（源路径）, fields[4]=mount_point（挂载点），位置固定
		// 注意：optional_fields 是可选的（如无 shared:/master: 标记则不出现），
		// 因此不能用 "-" 的位置来定位前两个字段，否则会误跳过无 optional_fields 的行。
		if len(fields) < 5 {
			continue
		}

		mountPoint := fields[4]
		hostRoot := fields[3]

		// 跳过根挂载点 "/"，避免误匹配
		if mountPoint == "/" {
			continue
		}

		// 找最长匹配的 mount_point 前缀（处理嵌套挂载）
		if strings.HasPrefix(containerPath, mountPoint) && len(mountPoint) > bestMatchLen {
			bestMatchLen = len(mountPoint)
			bestHostRoot = hostRoot
		}
	}

	if bestHostRoot != "" {
		suffix := containerPath[bestMatchLen:]
		return filepath.Join(bestHostRoot, suffix)
	}
	return containerPath
}

// Execute 在本地执行视频修复命令，并通过 onProgress 实时回调进度。
// 返回命令完整输出（stdout + stderr）以及可能的错误。
func (e *Executor) Execute(ctx context.Context, videoPath string, onProgress ProgressCallback) (output string, err error) {
	e.mu.RLock()
	cfg := e.config
	e.mu.RUnlock()

	parentDir := filepath.Dir(videoPath)
	baseName := filepath.Base(videoPath)

	// 自动将容器内路径翻译为宿主机路径（Docker 部署场景）
	// 通过宿主机的 Docker daemon 启动 Lada 容器时，bind mount 的 src 必须是宿主机路径
	hostParentDir := resolveHostPath(parentDir)

// 用文件名称作为容器名称，方便识别
		containerName := strings.TrimSuffix(baseName, filepath.Ext(baseName)) + "_lada"

		args := []string{
			"run", "--rm",
			"--name", containerName,
			"--mount", fmt.Sprintf("type=bind,src=%s,dst=/mnt", hostParentDir),
			cfg.DockerImage,
			"--input", "/mnt/" + baseName,
			"--device", cfg.Device,
		}

	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := streamCommand(cmd, onProgress)
	output = string(out)
	if err != nil {
		return output, fmt.Errorf("修复命令执行失败: %w", err)
	}
	return output, nil
}

var (
	// progressRegex 匹配 docker 进度行中的百分比，例如 "Processing video:   5%|..."
	progressRegex = regexp.MustCompile(`(\d+)%`)
	// progressCleanupRegex 用于剔除 docker 进度条里的百分比与 ASCII 进度条，
	// 保留 "Processed: ... | Remaining: ... | Speed: ..." 等可读信息。
	progressCleanupRegex = regexp.MustCompile(`^.*?\d+%\|[^|]*\|\s*`)
)

// parseProgressLine 从一行输出中解析进度百分比与进度消息。
// 返回 progress（0-100）和清理后的进度消息；若未解析到百分比则返回 -1。
func parseProgressLine(line string) (int, string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return -1, ""
	}
	matches := progressRegex.FindStringSubmatch(line)
	if len(matches) < 2 {
		return -1, line
	}
	p, err := strconv.Atoi(matches[1])
	if err != nil || p < 0 || p > 100 {
		return -1, line
	}

	// 清理进度条前缀，让 progress_msg 更接近 "Processed/Remaining/Speed"
	msg := line
	if cleaned := strings.TrimSpace(progressCleanupRegex.ReplaceAllString(line, "")); cleaned != "" {
		msg = cleaned
	}
	return p, msg
}

// streamCommand 启动命令并流式读取 stdout/stderr，
// 对每一行解析进度并通过 onProgress 回调。
func streamCommand(cmd *exec.Cmd, onProgress ProgressCallback) ([]byte, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("创建 stdout pipe 失败: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("创建 stderr pipe 失败: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动命令失败: %w", err)
	}

	var outputBuf strings.Builder
	var wg sync.WaitGroup

	streamReader := func(r io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		scanner.Split(splitProgressLine)
		for scanner.Scan() {
			line := scanner.Text()
			outputBuf.WriteString(line)
			outputBuf.WriteByte('\n')
			if onProgress == nil {
				continue
			}
			progress, msg := parseProgressLine(line)
			if progress >= 0 {
				onProgress(progress, msg)
			}
		}
	}

	wg.Add(2)
	go streamReader(stdout)
	go streamReader(stderr)

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		return []byte(outputBuf.String()), err
	}
	return []byte(outputBuf.String()), nil
}

// splitProgressLine 自定义 Scanner 切分函数，按 \r、\n 或 \r\n 切分，
// 用于处理 docker 进度条使用 \r 回写的场景。
func splitProgressLine(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			return i + 1, dropCR(data[:i]), nil
		}
		if data[i] == '\r' {
			// 将 \r 作为切分点，保留后续内容
			return i + 1, dropCR(data[:i]), nil
		}
	}
	if atEOF {
		return len(data), dropCR(data), nil
	}
	return 0, nil, nil
}

// dropCR 去除 token 末尾的 \r。
func dropCR(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == '\r' {
		return data[:len(data)-1]
	}
	return data
}
