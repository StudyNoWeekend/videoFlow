package repair

import (
	"regexp"
	"strings"
	"testing"
)

// validContainerNameRegex 匹配 Docker 容器名规则：[a-zA-Z0-9][a-zA-Z0-9_.-]*
var validContainerNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

// TestSanitizeContainerNameValid 断言生成的容器名符合 Docker 命名规则
func TestSanitizeContainerNameValid(t *testing.T) {
	cases := []string{
		"中文.mp4",
		"我的视频 (2).mkv",
		"video.mp4",
		"video 2026 v2.mp4",
		"!!!.mp4",
		"中文",
		".mp4",
		"-foo-.mp4",
		"スペース　テスト.mp4",
		"a.b_c-d.mp4",
	}
	for _, name := range cases {
		got := sanitizeContainerName(name, "/data/lib/"+name)
		if !validContainerNameRegex.MatchString(got) {
			t.Errorf("sanitizeContainerName(%q) = %q，不符合 Docker 容器名规则", name, got)
		}
	}
}

// TestSanitizeContainerNameASCII 全 ASCII 合法文件名应保持原名
func TestSanitizeContainerNameASCII(t *testing.T) {
	got := sanitizeContainerName("my_video.mp4", "/data/my_video.mp4")
	if got != "my_video" {
		t.Errorf("全 ASCII 文件名应保持原名，得到 %q", got)
	}
}

// TestSanitizeContainerNameChinese 中文文件名应被转义，且不同路径的同类文件不重名
func TestSanitizeContainerNameChinese(t *testing.T) {
	a := sanitizeContainerName("测试视频.mp4", "/data/a/测试视频.mp4")
	b := sanitizeContainerName("测试视频.mp4", "/data/b/测试视频.mp4")
	c := sanitizeContainerName("其他视频.mp4", "/data/a/其他视频.mp4")

	if a == b {
		t.Errorf("不同目录下的同名中文文件应生成不同容器名：a=%q b=%q", a, b)
	}
	if a == c {
		t.Errorf("不同中文文件名应生成不同容器名：a=%q c=%q", a, c)
	}
	if !strings.HasPrefix(a, "repair") && a[0] == '_' {
		t.Errorf("容器名不能以下划线开头：%q", a)
	}
}

// TestSanitizeContainerNameEmpty 空名/隐藏文件名应回退到有效占位名
func TestSanitizeContainerNameEmpty(t *testing.T) {
	got := sanitizeContainerName(".mp4", "/data/.mp4")
	if !validContainerNameRegex.MatchString(got) || strings.HasSuffix(got, "_lada") {
		t.Errorf(".mp4 应生成合法占位名，得到 %q", got)
	}
}

// TestSanitizeContainerNameDeterministic 同一路径应生成稳定名称
func TestSanitizeContainerNameDeterministic(t *testing.T) {
	path := "/data/lib/中文视频.mp4"
	a := sanitizeContainerName("中文视频.mp4", path)
	b := sanitizeContainerName("中文视频.mp4", path)
	if a != b {
		t.Errorf("同一路径应生成稳定名称：a=%q b=%q", a, b)
	}
}

// findArg 返回 args 中 flag 后面的值；不存在时第二个返回值为 false
func findArg(args []string, flag string) (string, bool) {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag {
			return args[i+1], true
		}
	}
	return "", false
}

// findFlagIndex 返回 flag 在 args 中的下标；不存在时返回 -1
func findFlagIndex(args []string, flag string) int {
	for i, a := range args {
		if a == flag {
			return i
		}
	}
	return -1
}

// assertCommonArgs 断言与设备无关的公共参数完整且顺序正确
// （--name/--mount 位于镜像名之前，--input/--device 是 lada 程序参数，位于镜像名之后）
func assertCommonArgs(t *testing.T, args []string) {
	t.Helper()
	imageIdx := findFlagIndex(args, "ladaapp/lada:latest")
	if imageIdx == -1 {
		t.Fatalf("参数中缺少镜像名: %v", args)
	}
	for _, flag := range []string{"run", "--rm", "--name", "--mount"} {
		if findFlagIndex(args, flag) == -1 || findFlagIndex(args, flag) > imageIdx {
			t.Errorf("Docker 标志 %s 缺失或位于镜像名之后: %v", flag, args)
		}
	}
	if got, ok := findArg(args, "--name"); !ok || got != "video_lada" {
		t.Errorf("--name 应为 video_lada，得到 %q", got)
	}
	if got, ok := findArg(args, "--mount"); !ok || got != "type=bind,src=/host/videos,dst=/mnt" {
		t.Errorf("--mount 不符合预期，得到 %q", got)
	}
	for _, flag := range []string{"--input", "--device"} {
		if findFlagIndex(args, flag) < imageIdx {
			t.Errorf("lada 参数 %s 应位于镜像名之后: %v", flag, args)
		}
	}
	if got, ok := findArg(args, "--input"); !ok || got != "/mnt/测试.mp4" {
		t.Errorf("--input 应为 /mnt/测试.mp4，得到 %q", got)
	}
}

// TestBuildRunArgsCPU cpu 设备不应包含 --gpus
func TestBuildRunArgsCPU(t *testing.T) {
	args := buildRunArgs(Config{DockerImage: "ladaapp/lada:latest", Device: "cpu"},
		"video_lada", "/host/videos", "测试.mp4")
	assertCommonArgs(t, args)
	if _, ok := findArg(args, "--gpus"); ok {
		t.Errorf("cpu 设备不应包含 --gpus: %v", args)
	}
	if got, ok := findArg(args, "--device"); !ok || got != "cpu" {
		t.Errorf("--device 应为 cpu，得到 %q", got)
	}
}

// TestBuildRunArgsCUDA cuda 设备应包含 --gpus 且位于镜像名之前
func TestBuildRunArgsCUDA(t *testing.T) {
	args := buildRunArgs(Config{DockerImage: "ladaapp/lada:latest", Device: "cuda:0"},
		"video_lada", "/host/videos", "测试.mp4")
	assertCommonArgs(t, args)
	got, ok := findArg(args, "--gpus")
	if !ok {
		t.Fatalf("cuda 设备应包含 --gpus: %v", args)
	}
	if got != gpuSpec {
		t.Errorf("--gpus 应为 %q，得到 %q", gpuSpec, got)
	}
	if findFlagIndex(args, "--gpus") > findFlagIndex(args, "ladaapp/lada:latest") {
		t.Errorf("--gpus 应位于镜像名之前: %v", args)
	}
	if dev, ok := findArg(args, "--device"); !ok || dev != "cuda:0" {
		t.Errorf("--device 应为 cuda:0，得到 %q", dev)
	}
}

// TestBuildRunArgsNonCUDA mps / xpu:0 设备不应包含 --gpus
func TestBuildRunArgsNonCUDA(t *testing.T) {
	for _, device := range []string{"mps", "xpu:0"} {
		args := buildRunArgs(Config{DockerImage: "ladaapp/lada:latest", Device: device},
			"video_lada", "/host/videos", "测试.mp4")
		assertCommonArgs(t, args)
		if _, ok := findArg(args, "--gpus"); ok {
			t.Errorf("%s 设备不应包含 --gpus: %v", device, args)
		}
	}
}

// TestGpuFailureHint 根据输出内容返回对应的 GPU 失败提示
func TestGpuFailureHint(t *testing.T) {
	if hint := gpuFailureHint("docker: Error response from daemon: could not select device driver \"\" with capabilities: [[compute]]"); !strings.Contains(hint, "NVIDIA Container Toolkit") {
		t.Errorf("缺少 NVIDIA Container Toolkit 的提示，得到 %q", hint)
	}
	if hint := gpuFailureHint("GPU cuda:0 selected but CUDA is not available"); !strings.Contains(hint, "Turing") {
		t.Errorf("缺少驱动/GPU 架构的提示，得到 %q", hint)
	}
	if hint := gpuFailureHint("some other error"); hint != "" {
		t.Errorf("无关错误不应返回提示，得到 %q", hint)
	}
}
