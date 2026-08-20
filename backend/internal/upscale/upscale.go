package upscale

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"video-captions/utils/logger"
)

// 清晰度去马赛克执行器默认值：处理器/模型/降噪等级已不在系统配置中，
// 仅作为未按任务指定时的兜底，创建清晰度去马赛克任务时由弹窗逐次选择。
const (
	DefaultProcessor  = "realesrgan"
	DefaultModel      = "realesr-animevideov3"
	DefaultNoiseLevel = -1
)

// Config 清晰度修复 Docker 配置
type Config struct {
	DockerImage string
	// Device 计算设备，支持四种：cpu（CPU）、cuda:0（NVIDIA CUDA）、mps（Apple Silicon MPS）、xpu:0（Intel XPU）
	Device string
	// Processor 清晰度修复处理器：realesrgan / realcugan / libplacebo
	Processor string
	// Model 模型名称（realesrgan/realcugan 时）或着色器路径（libplacebo 时）
	Model string
	// Factor 清晰度修复缩放倍数，仅对 realesrgan / realcugan 生效，默认 2
	Factor int
	// NoiseLevel 噪声等级：Real-ESRGAN 支持 -1(无)/0/1，Real-CUGAN 支持 -1(保守)/0/1/2/3
	NoiseLevel int
}

// ProgressCallback 清晰度去马赛克进度回调，progress 为 0-100，message 为当前进度行原始文本
type ProgressCallback func(progress int, message string)

// Executor 清晰度去马赛克执行器，在本地执行 Docker 清晰度去马赛克命令
type Executor struct {
	mu     sync.RWMutex
	config Config
}

// NewExecutor 创建清晰度去马赛克执行器实例
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
		return errors.New("清晰度去马赛克 Docker 镜像不能为空")
	}

	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("未找到 docker 命令，请确认 Docker 已安装并启动: %w", err)
	}

	if cfg.Processor == "" {
		cfg.Processor = "realesrgan"
	}
	if cfg.Factor <= 0 {
		cfg.Factor = 2
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
	if hostPath, ok := resolveHostPathViaDocker(containerPath); ok {
		return hostPath
	}
	return resolveHostPathViaMountinfo(containerPath)
}

// containerMountEntry 表示 docker inspect 返回的挂载条目
type containerMountEntry struct {
	Source      string `json:"Source"`
	Destination string `json:"Destination"`
}

// cachedMounts 缓存 docker inspect 的挂载结果，避免每次清晰度修复都执行一次。
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
			if idx := strings.Index(line, "docker-"); idx >= 0 {
				s := line[idx+len("docker-"):]
				if end := strings.Index(s, ".scope"); end >= 0 {
					return s[:end]
				}
			}
			if idx := strings.Index(line, "/docker/"); idx >= 0 {
				return strings.TrimSpace(line[idx+len("/docker/"):])
			}
		}
	}

	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		return hostname
	}
	return ""
}

// resolveHostPathViaMountinfo 通过 /proc/self/mountinfo 解析容器内路径到宿主机路径。
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
		if len(fields) < 5 {
			continue
		}

		mountPoint := fields[4]
		hostRoot := fields[3]

		if mountPoint == "/" {
			continue
		}

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

// isContainerNameChar 判断字符是否可用于 Docker 容器名（[a-zA-Z0-9_.-]）
func isContainerNameChar(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z':
		return true
	case r >= 'A' && r <= 'Z':
		return true
	case r >= '0' && r <= '9':
		return true
	case r == '.' || r == '_' || r == '-':
		return true
	}
	return false
}

// sanitizeContainerName 将视频文件名转换为合法的 Docker 容器名（不含 "_upscale" 后缀）。
// 仅保留 [a-zA-Z0-9_.-]，中文等非法字符替换为 "_"；trim 首尾的 "._-"；
// 空名或首位非字母数字时补充 "upscale_" 前缀；容器名以字母或数字开头。
// 若原文件名含非法字符（被改写），追加 videoPath 的 sha256 短哈希避免不同目录同名文件冲突。
func sanitizeContainerName(baseName, videoPath string) string {
	name := strings.TrimSuffix(baseName, filepath.Ext(baseName))

	var b strings.Builder
	for _, r := range name {
		if isContainerNameChar(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	sanitized := strings.Trim(b.String(), "._-")

	if sanitized == "" {
		sanitized = "upscale"
	}

	if sanitized == name {
		return sanitized
	}

	sum := sha256.Sum256([]byte(videoPath))
	suffix := "_" + hex.EncodeToString(sum[:4])

	const maxBaseLen = 64
	if len(sanitized) > maxBaseLen {
		sanitized = sanitized[:maxBaseLen]
	}
	if first := sanitized[0]; !((first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') || (first >= '0' && first <= '9')) {
		sanitized = "upscale_" + sanitized
	}
	return sanitized + suffix
}

// stopAndRemoveContainer 停止并移除指定名称的 Docker 容器（docker rm -f）。
// 用于任务取消后的清理：取消任务会杀死 docker run 客户端进程，但 Docker daemon 中
// 正在运行的容器不会随之停止，--rm 仅在容器自行退出后生效，因此需显式清理被孤立的容器。
func stopAndRemoveContainer(containerName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", containerName)
	out, err := cmd.CombinedOutput()
	if err == nil {
		logger.Logger.Info("已清理被孤立的清晰度去马赛克容器",
			zap.String("container", containerName),
			zap.String("output", strings.TrimSpace(string(out))),
		)
		return
	}

	if strings.Contains(strings.ToLower(string(out)), "no such container") {
		return
	}
	logger.Logger.Warn("清理被孤立的清晰度去马赛克容器失败",
		zap.String("container", containerName),
		zap.String("output", strings.TrimSpace(string(out))),
		zap.Error(err),
	)
}

// gpuSpec 将宿主机 GPU 透传给容器（--gpus 的值）。
// compute 供 CUDA 推理，video 供 nvenc 硬件编码。
const gpuSpec = `all,"capabilities=compute,video"`

// buildRunArgs 根据处理器类型构建 docker run 参数。
// realesrgan / realcugan 使用 -s <factor> 整数缩放；
// libplacebo 使用 -w <targetWidth> -h <targetHeight> 精确尺寸。
//
// 设备（cfg.Device）实际映射：
//   - cuda：通过 --gpus 透传宿主机 NVIDIA GPU，video2x 默认使用 Vulkan 设备 0（即该 GPU）
//   - cpu / 其余值：不传任何设备参数，video2x 回退到容器内默认 Vulkan 设备
//     （amd64 镜像在 Apple Silicon 上由 QEMU 模拟，只有 llvmpipe 软件渲染器可用）
//   - mps / xpu：video2x 不支持 MPS（Metal），Linux 容器内也无从访问 Apple/Intel GPU，
//     配置值不会传入命令，等同于 CPU 运行，设置页已不再提供这两个选项
func buildRunArgs(cfg Config, containerName, hostParentDir, inputFile, outputFile string) []string {
	args := []string{"run", "--rm"}
	if strings.HasPrefix(cfg.Device, "cuda") {
		args = append(args, "--gpus", gpuSpec)
	}

	processor := cfg.Processor
	model := cfg.Model

	args = append(args,
		"--name", containerName,
		"--mount", fmt.Sprintf("type=bind,src=%s,dst=/mnt", hostParentDir),
		cfg.DockerImage,
		"-i", "/mnt/"+inputFile,
		"-o", "/mnt/"+outputFile,
		"-p", processor,
	)

	switch processor {
	case "libplacebo":
		if model != "" {
			args = append(args, "--libplacebo-shader", model)
		}
		if cfg.Factor > 0 {
			// For libplacebo, factor can be used as an alternative fallback;
			// primary usage is -w / -h for exact dimensions.
			// video2x supports -w / -h for libplacebo output resolution.
		}
	default:
		// realesrgan / realcugan
		if cfg.Factor > 0 {
			args = append(args, "-s", strconv.Itoa(cfg.Factor))
		}
		if model != "" {
			args = append(args, fmt.Sprintf("--%s-model", processor), model)
		}
		// 噪声等级：Real-ESRGAN 支持 -1/0/1，Real-CUGAN 支持 -1/0/1/2/3
		if cfg.NoiseLevel >= 0 {
			args = append(args, "-n", strconv.Itoa(cfg.NoiseLevel))
		}
	}

	return args
}

// gpuFailureHint 根据清晰度去马赛克命令的输出识别 GPU 失败原因，返回附加到错误信息的中文提示。
func gpuFailureHint(output string) string {
	lower := strings.ToLower(output)
	switch {
	case strings.Contains(lower, "could not select device driver"):
		return "\n提示: 宿主机 Docker 无法使用 NVIDIA GPU，请确认已安装 NVIDIA 驱动和 NVIDIA Container Toolkit，或在设置中将清晰度去马赛克设备切回 cpu"
	case strings.Contains(lower, "cuda is not available"):
		return "\n提示: 容器内 CUDA 不可用，请更新 NVIDIA 驱动并确认 GPU 为 Turing 及以上架构（RTX 20xx 及之后），或在设置中将清晰度去马赛克设备切回 cpu"
	}
	return ""
}

// modelFailureHint 识别“镜像内模型缺失 / 放大倍数不支持”类错误，
// 返回附加到错误信息的中文提示。
func modelFailureHint(output, model string, factor int) string {
	if strings.Contains(output, "model param file not found") {
		return fmt.Sprintf("\n提示: 清晰度去马赛克镜像中缺少模型 %s 的 ×%d 版本（该模型可能仅支持 ×4，或镜像未内置此模型）。请选择该模型支持的放大倍数，或改用 realesr-animevideov3；如需指定模型，可在设置中更换包含完整模型的清晰度去马赛克 Docker 镜像", model, factor)
	}
	return ""
}

var (
	// progressRegex 匹配清晰度修复进度行中的百分比，支持小数，例如 "5%"、"(0.21%)"。
	// 使用 [\d.]+ 避免 "(0.21%)" 被误解析成 21（\d+ 只能匹配到 % 前的整数）。
	progressRegex = regexp.MustCompile(`([\d.]+)%`)
	// progressCleanupRegex 用于剔除进度条里的百分比与 ASCII 进度条，
	// 保留 "52/1000 [00:05<01:35, 6.27frame/s]" 等可读信息。
	progressCleanupRegex = regexp.MustCompile(`^.*?[\d.]+%\|[^|]*\|\s*`)
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
	f, err := strconv.ParseFloat(matches[1], 64)
	if err != nil || f < 0 || f > 100 {
		return -1, line
	}
	p := int(math.Round(f))

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
	var outputMu sync.Mutex
	var wg sync.WaitGroup

	// writeOutput 将一行输出追加到共享缓冲区。
	// stdout/stderr 由两个 goroutine 并发读取，strings.Builder 非并发安全，
	// 不加锁会导致输出丢失/错乱（错误信息 output 为空即是该竞态的表现）。
	writeOutput := func(line string) {
		outputMu.Lock()
		outputBuf.WriteString(line)
		outputBuf.WriteByte('\n')
		outputMu.Unlock()
	}

	streamReader := func(r io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		scanner.Split(splitProgressLine)
		for scanner.Scan() {
			line := scanner.Text()
			writeOutput(line)
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

// outputFileName 根据输入文件路径和目标高度生成输出文件名。
// 格式：{baseName}_upscaled_{targetHeight}p{ext}
// 例如：myvideo_upscaled_720p.mp4
func outputFileName(videoPath string, targetHeight int) string {
	ext := filepath.Ext(videoPath)
	baseName := strings.TrimSuffix(filepath.Base(videoPath), ext)
	return fmt.Sprintf("%s_upscaled_%dp%s", baseName, targetHeight, ext)
}

// probeResolution 通过本地 ffprobe 获取视频分辨率（宽×高）。
// ffprobe 不可用或解析失败时返回错误，调用方应回退到配置默认的放大倍数。
func probeResolution(ctx context.Context, videoPath string) (int, int, error) {
	args := []string{
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "csv=p=0",
		videoPath,
	}
	out, err := exec.CommandContext(ctx, "ffprobe", args...).CombinedOutput()
	if err != nil {
		return 0, 0, fmt.Errorf("ffprobe 解析分辨率失败: %w, output: %s", err, strings.TrimSpace(string(out)))
	}
	parts := strings.Split(strings.TrimSpace(string(out)), ",")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("ffprobe 输出格式异常: %s", strings.TrimSpace(string(out)))
	}
	width, errW := strconv.Atoi(strings.TrimSpace(parts[0]))
	height, errH := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errW != nil || errH != nil || width <= 0 || height <= 0 {
		return 0, 0, fmt.Errorf("ffprobe 输出无法解析: %s", strings.TrimSpace(string(out)))
	}
	return width, height, nil
}

// Execute 在本地执行清晰度去马赛克命令，并通过 onProgress 实时回调进度。
// 返回输出文件路径（同目录下的 _upscaled_{targetHeight}p 文件）以及可能的错误。
// processorOverride/modelOverride 非空时覆盖执行器默认处理器/模型；
// noiseLevelOverride 为创建任务时选择的降噪等级（-1=无/保守，0-3 递增），直接覆盖默认值。
func (e *Executor) Execute(ctx context.Context, videoPath string, targetWidth, targetHeight int, onProgress ProgressCallback, processorOverride, modelOverride string, noiseLevelOverride int) (output string, err error) {
	e.mu.RLock()
	cfg := e.config
	e.mu.RUnlock()

	// 如果传入了按任务覆盖的处理器/模型/降噪等级，则使用覆盖值
	if processorOverride != "" {
		cfg.Processor = processorOverride
	}
	if modelOverride != "" {
		cfg.Model = modelOverride
	}
	cfg.NoiseLevel = noiseLevelOverride

	// 历史遗留的 mps/xpu 等设备值不会被传入命令（等同 CPU 运行），
	// 记录告警避免误以为已启用对应加速。
	if cfg.Device != "" && cfg.Device != "cpu" && !strings.HasPrefix(cfg.Device, "cuda") {
		logger.Logger.Warn("清晰度去马赛克设备配置当前实现中不可用，已按 CPU 运行",
			zap.String("configured_device", cfg.Device),
		)
	}

	parentDir := filepath.Dir(videoPath)
	baseName := filepath.Base(videoPath)

	// 自动将容器内路径翻译为宿主机路径（Docker 部署场景）
	hostParentDir := resolveHostPath(parentDir)

	// 生成输出文件名
	outFile := outputFileName(videoPath, targetHeight)

	// 用文件（净化后的）名称作为容器名称，方便识别。
	containerName := sanitizeContainerName(baseName, videoPath) + "_upscale"

	// 根据源视频分辨率推导整数放大倍数，使 -s 与创建任务时选择的“×2/×3/×4”一致。
	// 此前固定使用配置默认倍数（2），会导致实际倍数与用户选择不符，
	// 并可能因模型仅有 ×4 版本而报“模型文件缺失”。探测失败时回退默认倍数。
	if srcW, srcH, perr := probeResolution(ctx, videoPath); perr == nil && srcW > 0 && srcH > 0 {
		if fw, fh := targetWidth/srcW, targetHeight/srcH; fw == fh && fw >= 2 && fw <= 4 {
			cfg.Factor = fw
		}
	}

	args := buildRunArgs(cfg, containerName, hostParentDir, baseName, outFile)

	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := streamCommand(cmd, onProgress)
	_ = out // full command output retained for error inspection if needed

	// 任务被取消时，docker run 客户端进程会被杀死，但 Docker daemon 中的容器
	// 仍会继续运行，--rm 仅在容器自行退出后生效，此处显式清理。
	if ctx.Err() != nil {
		stopAndRemoveContainer(containerName)
	}

	if err != nil {
		hint := ""
		if strings.HasPrefix(cfg.Device, "cuda") {
			hint += gpuFailureHint(string(out))
		}
		hint += modelFailureHint(string(out), cfg.Model, cfg.Factor)
		// 将 docker 原始输出一并拼入错误信息，便于定位失败原因（此前被丢弃导致 output 为空）
		return "", fmt.Errorf("清晰度去马赛克命令执行失败: %w%s\n清晰度去马赛克命令输出: %s", err, hint, strings.TrimSpace(string(out)))
	}

	return filepath.Join(parentDir, outFile), nil
}
