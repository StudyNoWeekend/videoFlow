package ffmpeg

import (
	"bytes"
	"context"
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
	"time"

	"video-captions/enum"
)

// 用于从 ffmpeg -i 输出中匹配时长的正则表达式
var durationRegex = regexp.MustCompile(`Duration: (\d{2}):(\d{2}):(\d{2}\.\d+)`)

// Provider 定义 ffmpeg 执行环境类型
type Provider string

const (
	// ProviderLocal 在本机 PATH 中调用 ffmpeg
	ProviderLocal Provider = "local"
	// ProviderSSH 通过 SSH 在远程主机上调用 ffmpeg
	ProviderSSH Provider = "ssh"
)

// Config ffmpeg 主机环境配置
type Config struct {
	Provider   string
	SSHHost    string
	SSHPort    int
	SSHUser    string
	SSHKeyPath string
	SSHArgs    []string
}

// hostEnv 保存当前 ffmpeg 执行环境，读写受锁保护以支持运行时热切换
type hostEnv struct {
	provider    Provider
	sshBaseArgs []string
	remoteHost  string
}

var (
	mu  sync.RWMutex
	env hostEnv
)

// currentEnv 安全读取当前执行环境
func currentEnv() hostEnv {
	mu.RLock()
	defer mu.RUnlock()
	return env
}

// Init 根据配置初始化 ffmpeg 执行环境
func Init(cfg Config) error {
	return applyConfig(cfg)
}

// Reload 运行时重新加载 ffmpeg 执行环境配置
func Reload(cfg Config) error {
	return applyConfig(cfg)
}

// applyConfig 校验并切换执行环境
func applyConfig(cfg Config) error {
	provider := Provider(cfg.Provider)
	if provider == "" {
		provider = ProviderLocal
	}

	switch provider {
	case ProviderLocal:
		if _, err := exec.LookPath("ffmpeg"); err != nil {
			return fmt.Errorf("%w: %v", enum.ErrFFmpegNotFound, err)
		}
		mu.Lock()
		env = hostEnv{provider: ProviderLocal}
		mu.Unlock()
		return nil
	case ProviderSSH:
		if cfg.SSHHost == "" {
			return errors.New("ffmpeg ssh 主机地址不能为空")
		}
		remoteHost := cfg.SSHHost
		if cfg.SSHUser != "" {
			remoteHost = cfg.SSHUser + "@" + cfg.SSHHost
		}
		sshBaseArgs := buildSSHBaseArgs(cfg)

		// 启动或重载时校验远程 ffmpeg 是否可用
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if _, err := runSSH(ctx, remoteHost, sshBaseArgs, "ffmpeg", "-version"); err != nil {
			return fmt.Errorf("远程 ffmpeg 不可用: %w", err)
		}

		mu.Lock()
		env = hostEnv{
			provider:    ProviderSSH,
			sshBaseArgs: sshBaseArgs,
			remoteHost:  remoteHost,
		}
		mu.Unlock()
		return nil
	default:
		return fmt.Errorf("不支持的 ffmpeg provider: %s", cfg.Provider)
	}
}

// buildSSHBaseArgs 构造 ssh 连接的基础参数
func buildSSHBaseArgs(cfg Config) []string {
	args := []string{
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
	}
	if len(cfg.SSHArgs) > 0 {
		args = append(args, cfg.SSHArgs...)
	}
	if cfg.SSHPort > 0 {
		args = append(args, "-p", strconv.Itoa(cfg.SSHPort))
	}
	if cfg.SSHKeyPath != "" {
		args = append(args, "-i", cfg.SSHKeyPath)
	}
	return args
}

// isRemote 判断当前是否通过 SSH 执行
func isRemote() bool {
	return currentEnv().provider == ProviderSSH
}

// ensureFFmpeg 检查 ffmpeg 是否可用
func ensureFFmpeg(ctx context.Context) error {
	if isRemote() {
		return nil
	}
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("%w: %v", enum.ErrFFmpegNotFound, err)
	}
	return nil
}

// runLocal 在本机执行命令并返回合并输出
func runLocal(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

// runSSH 通过 SSH 在远程主机执行命令并返回合并输出
func runSSH(ctx context.Context, remoteHost string, sshBaseArgs []string, name string, args ...string) ([]byte, error) {
	sshArgs := make([]string, len(sshBaseArgs))
	copy(sshArgs, sshBaseArgs)
	sshArgs = append(sshArgs, remoteHost, name)
	sshArgs = append(sshArgs, args...)
	cmd := exec.CommandContext(ctx, "ssh", sshArgs...)
	return cmd.CombinedOutput()
}

// run 根据当前执行环境调用本地或远程命令
func run(ctx context.Context, name string, args ...string) ([]byte, error) {
	e := currentEnv()
	if e.provider == ProviderSSH {
		return runSSH(ctx, e.remoteHost, e.sshBaseArgs, name, args...)
	}
	return runLocal(ctx, name, args...)
}

// ExtractAudio 从视频文件中提取音频并保存为 MP3 格式。
// ASR 接口通常不支持 MP4 直接上传，因此先通过 ffmpeg 提取音频后再送入 ASR。
// 输出文件命名为 {video_base_name}_{timestamp}.mp3，存放在 outputDir
func ExtractAudio(ctx context.Context, videoPath, outputDir string) (string, error) {
	if err := ensureFFmpeg(ctx); err != nil {
		return "", err
	}

	// 本地模式时检查视频文件是否存在
	if !isRemote() {
		if _, err := os.Stat(videoPath); err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("%w: %s", enum.ErrVideoNotFound, videoPath)
			}
			return "", fmt.Errorf("%w: %v", enum.ErrVideoNotFound, err)
		}
	}

	// 确保输出目录存在
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("%w: %v", enum.ErrCreateOutputDir, err)
	}

	// 构造输出文件名：{video_base_name}_{timestamp}.mp3
	baseName := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	timestamp := time.Now().Unix()
	outputFileName := fmt.Sprintf("%s_%d.mp3", baseName, timestamp)
	audioPath := filepath.Join(outputDir, outputFileName)

	if isRemote() {
		return extractAudioRemote(ctx, videoPath, audioPath)
	}
	return extractAudioLocal(ctx, videoPath, audioPath)
}

// extractAudioLocal 在本机调用 ffmpeg 提取音频为 MP3
func extractAudioLocal(ctx context.Context, videoPath, audioPath string) (string, error) {
	args := []string{
		"-i", videoPath,
		"-vn",
		"-c:a", "libmp3lame",
		"-q:a", "2",
		"-y",
		audioPath,
	}

	output, err := run(ctx, "ffmpeg", args...)
	if err != nil {
		return "", fmt.Errorf("%w: %v, output: %s", enum.ErrAudioExtract, err, string(output))
	}
	return audioPath, nil
}

// extractAudioRemote 通过 SSH 调用远程 ffmpeg，将 MP3 音频流回写到本地文件
func extractAudioRemote(ctx context.Context, videoPath, audioPath string) (string, error) {
	e := currentEnv()

	args := []string{
		"-i", videoPath,
		"-vn",
		"-c:a", "libmp3lame",
		"-q:a", "2",
		"-f", "mp3",
		"-",
	}

	sshArgs := make([]string, len(e.sshBaseArgs))
	copy(sshArgs, e.sshBaseArgs)
	sshArgs = append(sshArgs, e.remoteHost, "ffmpeg")
	sshArgs = append(sshArgs, args...)

	cmd := exec.CommandContext(ctx, "ssh", sshArgs...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	outFile, err := os.Create(audioPath)
	if err != nil {
		return "", fmt.Errorf("%w: %v", enum.ErrCreateOutputDir, err)
	}
	defer outFile.Close()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("%w: %v", enum.ErrAudioExtract, err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("%w: %v", enum.ErrAudioExtract, err)
	}

	if _, err := io.Copy(outFile, stdout); err != nil {
		_ = cmd.Wait()
		return "", fmt.Errorf("%w: %v", enum.ErrAudioExtract, err)
	}

	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("%w: %v, stderr: %s", enum.ErrAudioExtract, err, stderr.String())
	}
	return audioPath, nil
}

// GetDuration 获取视频文件时长（秒）
func GetDuration(ctx context.Context, videoPath string) (float64, error) {
	if err := ensureFFmpeg(ctx); err != nil {
		return 0, err
	}

	// 本地模式时检查视频文件是否存在
	if !isRemote() {
		if _, err := os.Stat(videoPath); err != nil {
			if os.IsNotExist(err) {
				return 0, fmt.Errorf("%w: %s", enum.ErrVideoNotFound, videoPath)
			}
			return 0, fmt.Errorf("%w: %v", enum.ErrVideoNotFound, err)
		}
	}

	// 优先使用 ffprobe 获取时长
	duration, err := getDurationByFFprobe(ctx, videoPath)
	if err == nil {
		return duration, nil
	}

	// ffprobe 失败时回退到 ffmpeg -i 解析 stderr
	duration, err = getDurationByFFmpeg(ctx, videoPath)
	if err == nil {
		return duration, nil
	}

	return 0, err
}

// getDurationByFFprobe 使用 ffprobe 获取视频时长
func getDurationByFFprobe(ctx context.Context, videoPath string) (float64, error) {
	args := []string{
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprintwrappers=1:nokey=1",
		videoPath,
	}

	output, err := run(ctx, "ffprobe", args...)
	if err != nil {
		return 0, fmt.Errorf("ffprobe execute failed: %w, output: %s", err, string(output))
	}

	durationStr := strings.TrimSpace(string(output))
	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, fmt.Errorf("parse ffprobe duration failed: %w", err)
	}

	return duration, nil
}

// getDurationByFFmpeg 使用 ffmpeg -i 解析 stderr 中的 Duration 字段获取视频时长
func getDurationByFFmpeg(ctx context.Context, videoPath string) (float64, error) {
	output, err := run(ctx, "ffmpeg", "-i", videoPath)
	outputStr := string(output)

	// 尝试从输出中解析时长
	duration, parseErr := parseDuration(outputStr)
	if parseErr == nil {
		return duration, nil
	}

	// 若解析失败且命令执行也失败，则返回执行错误
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return 0, fmt.Errorf("%w: %v, output: %s", enum.ErrDurationParse, parseErr, outputStr)
		}
		return 0, fmt.Errorf("%w: %v, output: %s", enum.ErrFFmpegExecute, err, outputStr)
	}

	return 0, fmt.Errorf("%w: %v, output: %s", enum.ErrDurationParse, parseErr, outputStr)
}

// parseDuration 从 ffmpeg 输出文本中解析 Duration 字段并转换为秒
func parseDuration(output string) (float64, error) {
	matches := durationRegex.FindStringSubmatch(output)
	if len(matches) != 4 {
		return 0, errors.New("duration not found in ffmpeg output")
	}

	hours, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("parse hours failed: %w", err)
	}
	minutes, err := strconv.Atoi(matches[2])
	if err != nil {
		return 0, fmt.Errorf("parse minutes failed: %w", err)
	}
	seconds, err := strconv.ParseFloat(matches[3], 64)
	if err != nil {
		return 0, fmt.Errorf("parse seconds failed: %w", err)
	}

	return float64(hours)*3600 + float64(minutes)*60 + seconds, nil
}
