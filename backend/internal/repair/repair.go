package repair

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
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

// Execute 在本地执行视频修复命令，并通过 onProgress 实时回调进度。
// 返回命令完整输出（stdout + stderr）以及可能的错误。
func (e *Executor) Execute(ctx context.Context, videoPath string, onProgress ProgressCallback) (output string, err error) {
	e.mu.RLock()
	cfg := e.config
	e.mu.RUnlock()

	parentDir := filepath.Dir(videoPath)
	baseName := filepath.Base(videoPath)

	args := []string{
		"run", "--rm",
		"--mount", fmt.Sprintf("type=bind,src=%s,dst=/mnt", parentDir),
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
