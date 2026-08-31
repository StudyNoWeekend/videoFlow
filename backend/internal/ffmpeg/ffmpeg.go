package ffmpeg

import (
	"bufio"
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

// burnSizeRegex 用于从 ffmpeg 进度输出中匹配已写入大小，兼容不同版本的单位：
// ffmpeg 7.x 输出 KiB（二进制），ffmpeg 6.x 及更早输出 kB（十进制）。
// 例如 "frame=  123 fps= 12 q=28.0 size=   122240KiB time=..." 或 "size=   122240kB"
var burnSizeRegex = regexp.MustCompile(`size=\s*(\d+)\s*([kK][iI]?B)`)

// parseBurnSizeBytes 从 ffmpeg 进度行解析当前已写入大小（字节）。
// 单位含 i（如 KiB/IEC）按 1024 换算，否则（kB，旧版 ffmpeg）按 1000 换算。
func parseBurnSizeBytes(line string) (int64, bool) {
	matches := burnSizeRegex.FindStringSubmatch(line)
	if len(matches) < 3 {
		return 0, false
	}
	n, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0, false
	}
	unit := strings.ToLower(matches[2])
	if strings.Contains(unit, "i") {
		return n * 1024, true
	}
	return n * 1000, true
}

// BurnProgressCallback 字幕烧录进度回调，currentBytes 为当前已写入字节数，totalBytes 为输入视频总字节数
type BurnProgressCallback func(currentBytes, totalBytes int64)

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
		// accept-new：首次连接自动记录主机密钥，之后严格校验，防止中间人攻击
		"-o", "StrictHostKeyChecking=accept-new",
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

// GetResolution 获取视频文件的分辨率（宽 x 高）。
// 本地模式下先检查文件是否存在；优先使用 ffprobe，失败时回退到 ffmpeg -i 解析。
func GetResolution(ctx context.Context, videoPath string) (width, height int, err error) {
	if err := ensureFFmpeg(ctx); err != nil {
		return 0, 0, err
	}

	if !isRemote() {
		if _, err := os.Stat(videoPath); err != nil {
			if os.IsNotExist(err) {
				return 0, 0, fmt.Errorf("%w: %s", enum.ErrVideoNotFound, videoPath)
			}
			return 0, 0, fmt.Errorf("%w: %v", enum.ErrVideoNotFound, err)
		}
	}

	// 优先使用 ffprobe 获取分辨率
	w, h, err := getResolutionByFFprobe(ctx, videoPath)
	if err == nil {
		return w, h, nil
	}

	// ffprobe 失败时回退到 ffmpeg -i 解析 stderr
	w, h, err = getResolutionByFFmpeg(ctx, videoPath)
	if err == nil {
		return w, h, nil
	}

	return 0, 0, err
}

// getResolutionByFFprobe 使用 ffprobe 获取视频分辨率
func getResolutionByFFprobe(ctx context.Context, videoPath string) (int, int, error) {
	args := []string{
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "csv=p=0",
		videoPath,
	}

	output, err := run(ctx, "ffprobe", args...)
	if err != nil {
		return 0, 0, fmt.Errorf("ffprobe execute failed: %w, output: %s", err, string(output))
	}

	line := strings.TrimSpace(string(output))
	parts := strings.Split(line, ",")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("unexpected ffprobe output: %s", line)
	}

	width, errW := strconv.Atoi(strings.TrimSpace(parts[0]))
	height, errH := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errW != nil || errH != nil {
		return 0, 0, fmt.Errorf("parse ffprobe resolution failed: width=%v height=%v", errW, errH)
	}

	return width, height, nil
}

// resolutionParseRegex 用于从 ffmpeg -i 输出中匹配视频流分辨率
var resolutionParseRegex = regexp.MustCompile(`(\d{3,5})x(\d{3,5})`)

// getResolutionByFFmpeg 使用 ffmpeg -i 解析 stderr 中的 Stream #0:0 分辨率
func getResolutionByFFmpeg(ctx context.Context, videoPath string) (int, int, error) {
	output, err := run(ctx, "ffmpeg", "-i", videoPath)
	outputStr := string(output)

	matches := resolutionParseRegex.FindStringSubmatch(outputStr)
	if len(matches) >= 3 {
		w, errW := strconv.Atoi(matches[1])
		h, errH := strconv.Atoi(matches[2])
		if errW == nil && errH == nil && w > 0 && h > 0 {
			return w, h, nil
		}
	}

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return 0, 0, fmt.Errorf("resolution not found in ffmpeg output")
		}
		return 0, 0, fmt.Errorf("%w: %v", enum.ErrFFmpegExecute, err)
	}

	return 0, 0, fmt.Errorf("resolution not found in ffmpeg output")
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

// escapeSubtitlesPath 转义 ffmpeg subtitles 滤镜中的字幕文件路径。
// ffmpeg 滤镜语法中用单引号包裹路径可屏蔽空格、冒号等特殊字符，
// 仅需转义路径中的单引号（\'）。
func escapeSubtitlesPath(path string) string {
	escaped := strings.ReplaceAll(path, "'", "\\'")
	return "'" + escaped + "'"
}

// BurnSubtitles 将 SRT 字幕烧录到视频中（硬字幕），生成带有内嵌字幕的新视频文件。
// videoPath: 输入视频路径；srtPath: SRT 字幕文件路径；outputPath: 输出视频路径。
// 本地模式下所有路径均为本地路径；SSH 模式下 videoPath 和 outputPath 为远程路径，
// srtPath 为本地路径（函数内部会自动上传到远程临时文件）。
func BurnSubtitles(ctx context.Context, videoPath, srtPath, outputPath string, onProgress BurnProgressCallback) error {
	if err := ensureFFmpeg(ctx); err != nil {
		return err
	}

	if isRemote() {
		return burnSubtitlesRemote(ctx, videoPath, srtPath, outputPath)
	}
	return burnSubtitlesLocal(ctx, videoPath, srtPath, outputPath, onProgress)
}

// burnSubtitlesLocal 在本机调用 ffmpeg 将字幕烧录到视频
func burnSubtitlesLocal(ctx context.Context, videoPath, srtPath, outputPath string, onProgress BurnProgressCallback) error {
	// 检查视频文件是否存在
	videoInfo, err := os.Stat(videoPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", enum.ErrVideoNotFound, videoPath)
		}
		return fmt.Errorf("%w: %v", enum.ErrVideoNotFound, err)
	}
	totalBytes := videoInfo.Size()

	// 检查字幕文件是否存在
	if _, err := os.Stat(srtPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: 字幕文件不存在: %s", enum.ErrSubtitleBurn, srtPath)
		}
		return fmt.Errorf("%w: %v", enum.ErrSubtitleBurn, err)
	}

	// 确保输出目录存在
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("%w: %v", enum.ErrCreateOutputDir, err)
	}

	filterArg := "subtitles=" + escapeSubtitlesPath(srtPath)
	args := []string{
		"-i", videoPath,
		"-vf", filterArg,
		"-c:v", "libx264",
		"-crf", "23",
		"-c:a", "copy",
		"-y",
		outputPath,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("%w: 创建 stderr pipe 失败: %v", enum.ErrSubtitleBurn, err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w: 启动 ffmpeg 失败: %v", enum.ErrSubtitleBurn, err)
	}

	// 逐行读取 stderr，解析 size 字段。
	// 注意：不能使用 bufio.Scanner 按行切分——ffmpeg 会输出无换行符控制的超长内容
	// （如字体加载、版本信息等），一旦单行超过 Scanner 的 token 上限会提前退出，
	// 导致 stderr 管道无人消费、ffmpeg 阻塞在写 stderr 上而死锁。
	// 这里采用 bufio.Reader 逐字节读取，按 \r 或 \n 自行切分，无长度上限。
	var lastReportedBytes int64 = -1
	reader := bufio.NewReaderSize(stderr, 64*1024)
	var lineBuf strings.Builder
	for {
		b, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				// 处理末尾无换行符的残留内容
				if lineBuf.Len() > 0 {
					if currentBytes, ok := parseBurnSizeBytes(lineBuf.String()); ok && currentBytes != lastReportedBytes {
						lastReportedBytes = currentBytes
						if onProgress != nil {
							onProgress(currentBytes, totalBytes)
						}
					}
				}
				break
			}
			return fmt.Errorf("%w: 读取 ffmpeg 输出失败: %v", enum.ErrSubtitleBurn, err)
		}
		if b == '\r' || b == '\n' {
			line := lineBuf.String()
			lineBuf.Reset()
			if currentBytes, ok := parseBurnSizeBytes(line); ok {
				if currentBytes != lastReportedBytes {
					lastReportedBytes = currentBytes
					if onProgress != nil {
						onProgress(currentBytes, totalBytes)
					}
				}
			}
			continue
		}
		lineBuf.WriteByte(b)
	}

	// 等待命令完成
	waitErr := cmd.Wait()

	if waitErr != nil {
		return fmt.Errorf("%w: %v", enum.ErrSubtitleBurn, waitErr)
	}
	return nil
}

// burnSubtitlesRemote 通过 SSH 在远程主机上将字幕烧录到视频。
// 由于字幕文件在本地，需要先上传到远程临时文件，再执行 ffmpeg，最后清理临时文件。
func burnSubtitlesRemote(ctx context.Context, videoPath, srtPath, outputPath string) error {
	e := currentEnv()

	// 读取本地字幕文件内容
	srtContent, err := os.ReadFile(srtPath)
	if err != nil {
		return fmt.Errorf("%w: 读取字幕文件失败: %v", enum.ErrSubtitleBurn, err)
	}

	// 上传字幕文件到远程临时路径
	remoteSrtPath := fmt.Sprintf("/tmp/videoflow_subtitle_%d.srt", time.Now().Unix())
	if err := uploadFileRemote(ctx, e, remoteSrtPath, srtContent); err != nil {
		return fmt.Errorf("%w: %v", enum.ErrSubtitleBurn, err)
	}
	defer cleanupRemoteFile(ctx, e, remoteSrtPath)

	// 在远程执行 ffmpeg 烧录字幕
	filterArg := "subtitles=" + escapeSubtitlesPath(remoteSrtPath)
	args := []string{
		"-i", videoPath,
		"-vf", filterArg,
		"-c:v", "libx264",
		"-crf", "23",
		"-c:a", "copy",
		"-y",
		outputPath,
	}

	output, err := runSSH(ctx, e.remoteHost, e.sshBaseArgs, "ffmpeg", args...)
	if err != nil {
		return fmt.Errorf("%w: %v, output: %s", enum.ErrSubtitleBurn, err, string(output))
	}
	return nil
}

// uploadFileRemote 通过 SSH 将内容写入远程文件
func uploadFileRemote(ctx context.Context, e hostEnv, remotePath string, content []byte) error {
	remoteCmd := fmt.Sprintf("cat > '%s'", remotePath)
	sshArgs := make([]string, 0, len(e.sshBaseArgs)+4)
	sshArgs = append(sshArgs, e.sshBaseArgs...)
	sshArgs = append(sshArgs, e.remoteHost, "sh", "-c", remoteCmd)

	cmd := exec.CommandContext(ctx, "ssh", sshArgs...)
	cmd.Stdin = bytes.NewReader(content)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("上传文件到远程失败: %v, output: %s", err, string(output))
	}
	return nil
}

// cleanupRemoteFile 删除远程临时文件，忽略错误
func cleanupRemoteFile(ctx context.Context, e hostEnv, remotePath string) {
	_, _ = runSSH(ctx, e.remoteHost, e.sshBaseArgs, "rm", "-f", remotePath)
}
