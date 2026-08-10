package component

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"video-captions/internal/utils"
)

// Detector 组件检测器
type Detector struct {
	mu       sync.RWMutex
	cached   []ComponentInfo
	cachedAt time.Time
}

// NewDetector 创建新的检测器
func NewDetector() *Detector {
	return &Detector{}
}

// DetectAll 检测所有组件
func (d *Detector) DetectAll(ctx context.Context) []ComponentInfo {
	results := make([]ComponentInfo, 0, 4)

	results = append(results, d.detectDocker(ctx))
	results = append(results, d.detectFFmpeg(ctx))
	results = append(results, d.detectWhisperASR(ctx))
	results = append(results, d.detectLada(ctx))

	d.mu.Lock()
	d.cached = results
	d.cachedAt = time.Now()
	d.mu.Unlock()

	return results
}

// GetCached 返回缓存的检测结果
func (d *Detector) GetCached() []ComponentInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.cached
}

// GetComponentStatus 检测单个组件状态
func (d *Detector) GetComponentStatus(ctx context.Context, componentType ComponentType) ComponentInfo {
	switch componentType {
	case ComponentDocker:
		return d.detectDocker(ctx)
	case ComponentFFmpeg:
		return d.detectFFmpeg(ctx)
	case ComponentWhisperASR:
		return d.detectWhisperASR(ctx)
	case ComponentLada:
		return d.detectLada(ctx)
	default:
		return ComponentInfo{
			Type:     componentType,
			Status:   StatusError,
			ErrorMsg: fmt.Sprintf("unknown component type: %s", componentType),
		}
	}
}

func (d *Detector) detectDocker(ctx context.Context) ComponentInfo {
	info := ComponentInfo{
		Type:        ComponentDocker,
		Name:        "Docker",
		Description: "Container runtime, required by all Docker components",
		NeedsDocker: false,
	}

	// Check docker --version
	version, err := runCommand(ctx, "docker", "--version")
	if err != nil {
		info.Status = StatusMissing
		// 容器内缺少 docker CLI 时给出清晰指引
		inContainer := utils.IsRunningInContainer()
		if inContainer {
			info.ErrorMsg = "当前运行在 Docker 容器内，但未找到 docker CLI。" +
				"请使用最新镜像（已内置 docker-cli），" +
				"或在 docker run 时挂载宿主机 docker socket：-v /var/run/docker.sock:/var/run/docker.sock"
		}
		return info
	}
	info.Version = parseDockerVersion(version)

	// Check docker info
	_, err = runCommand(ctx, "docker", "info")
	if err != nil {
		info.Status = StatusError
		inContainer := utils.IsRunningInContainer()
		if inContainer {
			info.ErrorMsg = "Docker daemon 不可达。请确保 docker run 时挂载了宿主机 Docker 套接字：" +
				"-v /var/run/docker.sock:/var/run/docker.sock"
		} else {
			info.ErrorMsg = "Docker daemon is not running"
		}
		return info
	}

	info.Status = StatusInstalled
	return info
}

func (d *Detector) detectFFmpeg(ctx context.Context) ComponentInfo {
	info := ComponentInfo{
		Type:        ComponentFFmpeg,
		Name:        "FFmpeg",
		Description: "Audio/video processing tool for audio extraction and video processing",
		NeedsDocker: false,
	}

	version, err := runCommand(ctx, "ffmpeg", "-version")
	if err != nil {
		info.Status = StatusMissing
		return info
	}

	info.Version = parseFFmpegVersion(version)
	info.Status = StatusInstalled
	return info
}

func (d *Detector) detectWhisperASR(ctx context.Context) ComponentInfo {
	info := ComponentInfo{
		Type:        ComponentWhisperASR,
		Name:        "Whisper ASR",
		Description: "Speech recognition service for generating subtitles",
		NeedsDocker: true,
	}

	// Check if the whisper-asr container exists and is running
	out, err := runCommand(ctx, "docker", "ps", "--filter", "name=whisper-asr-webservice", "--format", "{{.ID}}")
	if err != nil || strings.TrimSpace(out) == "" {
		info.Status = StatusMissing
		return info
	}

	// Check container status
	statusOut, err := runCommand(ctx, "docker", "ps", "--filter", "name=whisper-asr-webservice", "--format", "{{.Status}}")
	if err != nil || !strings.HasPrefix(strings.ToLower(strings.TrimSpace(statusOut)), "up") {
		info.Status = StatusError
		info.ErrorMsg = "Container is not running"
		return info
	}

	info.Status = StatusInstalled
	return info
}

func (d *Detector) detectLada(ctx context.Context) ComponentInfo {
	info := ComponentInfo{
		Type:        ComponentLada,
		Name:        "Lada",
		Description: "Video restoration tool for removing mosaics",
		NeedsDocker: true,
	}

	// Lada uses disposable containers (docker run --rm), so just check if the image exists
	_, err := runCommand(ctx, "docker", "image", "inspect", "ladaapp/lada:latest")
	if err != nil {
		info.Status = StatusMissing
		// 检查是否是因为 docker daemon 不可达而非镜像不存在
		if _, checkErr := runCommand(ctx, "docker", "info"); checkErr != nil {
			info.ErrorMsg = "Docker daemon 不可达，无法检测 Lada 镜像。" +
				"请确认 docker run 时已挂载 /var/run/docker.sock"
		}
		return info
	}

	info.Status = StatusInstalled
	return info
}

// runCommand 执行命令并返回输出
func runCommand(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// runCommandWithCallback 执行命令并通过回调逐行输出
func runCommandWithCallback(ctx context.Context, name string, args []string, callback func(line string)) error {
	cmd := exec.CommandContext(ctx, name, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	// Read stdout
	done := make(chan struct{})
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				callback(string(buf[:n]))
			}
			if err != nil {
				break
			}
		}
		close(done)
	}()

	// Read stderr
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				callback(string(buf[:n]))
			}
			if err != nil {
				break
			}
		}
	}()

	<-done
	return cmd.Wait()
}

// tryJSONParse 尝试解析 JSON 输出
func tryJSONParse(out string, v any) error {
	return json.Unmarshal([]byte(out), v)
}

func parseDockerVersion(out string) string {
	lines := strings.SplitN(out, "\n", 2)
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return ""
}

func parseFFmpegVersion(out string) string {
	lines := strings.SplitN(out, "\n", 2)
	if len(lines) > 0 {
		// ffmpeg version N-12345-gabcdef or ffmpeg version 4.4
		line := strings.TrimSpace(lines[0])
		parts := strings.SplitN(line, " ", 3)
		if len(parts) >= 2 {
			return parts[2]
		}
		return line
	}
	return ""
}
