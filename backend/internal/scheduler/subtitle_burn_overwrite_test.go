package scheduler

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"video-captions/internal/model"
	"video-captions/utils/logger"
)

// initBurnTestDB 初始化烧录任务测试环境：迁移表并把输入/输出目录指向临时目录
func initBurnTestDB(t *testing.T) (inputDir, outputDir string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "burn_test.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Video{}, &model.Task{}, &model.Setting{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	model.DB = db

	inputDir = t.TempDir()
	outputDir = t.TempDir()
	ctx := context.Background()
	if err := model.SettingSet(ctx, model.SettingKeyVideoDir, inputDir); err != nil {
		t.Fatal(err)
	}
	if err := model.SettingSet(ctx, model.SettingKeyOutputDir, outputDir); err != nil {
		t.Fatal(err)
	}
	return inputDir, outputDir
}

// makeTinyVideo 用 ffmpeg 生成 1 秒的真实测试视频
func makeTinyVideo(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc=duration=1:size=128x72:rate=10",
		"-y", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("生成测试视频失败: %v, output: %s", err, string(out))
	}
}

func mkBurnVideo(t *testing.T, path string) *model.Video {
	t.Helper()
	makeTinyVideo(t, path)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	v := &model.Video{
		BaseModel: model.BaseModel{
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
		Path: path,
		Name: filepath.Base(path),
		Size: info.Size(),
	}
	if err := model.VideoCreate(context.Background(), v); err != nil {
		t.Fatalf("创建视频记录失败: %v", err)
	}
	return v
}

func mkBurnTask(t *testing.T, videoID string, sourcePath string, overwrite bool) *model.Task {
	t.Helper()
	task := &model.Task{
		VideoID:    videoID,
		TaskType:   model.TaskTypeSubtitleBurn,
		Status:     model.TaskStatusRunning,
		SourcePath: sourcePath,
		Overwrite:  overwrite,
	}
	if err := model.DB.Create(task).Error; err != nil {
		t.Fatalf("创建任务记录失败: %v", err)
	}
	return task
}

// prepareSrt 在输出目录写入与视频同名的 srt，避免测试触发 ASR 自动生成
func prepareSrt(t *testing.T, outputDir, baseName string) {
	t.Helper()
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}
	srt := "1\n00:00:00,000 --> 00:00:01,000\ntest subtitle\n\n"
	if err := os.WriteFile(filepath.Join(outputDir, baseName+".srt"), []byte(srt), 0644); err != nil {
		t.Fatal(err)
	}
}

func taskStatusByID(t *testing.T, id string) string {
	t.Helper()
	var task model.Task
	if err := model.DB.Where("id = ?", id).First(&task).Error; err != nil {
		t.Fatalf("查询任务失败: %v", err)
	}
	return task.Status
}

// fileHash 计算文件内容的 sha256，用于比较覆盖前后产物是否变化
func fileHash(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

// TestSubtitleBurnOverwriteDerivedArtifact 复现问题：覆盖模式烧录衍生视频后，产物去向
func TestSubtitleBurnOverwriteDerivedArtifact(t *testing.T) {
	logger.Logger = zap.NewNop()
	inputDir, outputDir := initBurnTestDB(t)

	v := mkBurnVideo(t, filepath.Join(inputDir, "movie.mp4"))
	base := model.VideoBaseName(v)
	outSubDir := filepath.Join(outputDir, base)
	prepareSrt(t, outSubDir, base)

	// 模拟其他任务类型（如去马赛克）的衍生产物
	artifactPath := filepath.Join(outSubDir, base+"_repaired.mp4")
	makeTinyVideo(t, artifactPath)
	hashBefore := fileHash(t, artifactPath)

	// 用户选择衍生视频 + 勾选覆盖
	task := mkBurnTask(t, v.ID, artifactPath, true)
	s := NewTaskScheduler(model.DB)
	s.processSubtitleBurnTask(context.Background(), task, &taskOutput{})

	if got := taskStatusByID(t, task.ID); got != model.TaskStatusCompleted {
		t.Fatalf("task status = %s, want completed", got)
	}

	entries, _ := os.ReadDir(outSubDir)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	t.Logf("覆盖模式执行后输出目录内容: %v", names)

	// 预期：产物被替换（内容变化），不再生成 _subtitled 新文件
	hashAfter := fileHash(t, artifactPath)
	if hashAfter == hashBefore {
		t.Errorf("覆盖模式执行后衍生产物内容未变化，覆盖未生效")
	}
	for _, name := range names {
		if name != base+"_repaired.mp4" && name != base+".srt" {
			t.Errorf("覆盖模式下出现了预期外的新文件: %s", name)
		}
	}
}

// TestSubtitleBurnDerivedArtifactNoOverwrite 不勾选覆盖：应生成 _subtitled 新文件
func TestSubtitleBurnDerivedArtifactNoOverwrite(t *testing.T) {
	logger.Logger = zap.NewNop()
	inputDir, outputDir := initBurnTestDB(t)

	v := mkBurnVideo(t, filepath.Join(inputDir, "movie2.mp4"))
	base := model.VideoBaseName(v)
	outSubDir := filepath.Join(outputDir, base)
	prepareSrt(t, outSubDir, base)

	artifactPath := filepath.Join(outSubDir, base+"_repaired.mp4")
	makeTinyVideo(t, artifactPath)

	task := mkBurnTask(t, v.ID, artifactPath, false)
	s := NewTaskScheduler(model.DB)
	s.processSubtitleBurnTask(context.Background(), task, &taskOutput{})

	if got := taskStatusByID(t, task.ID); got != model.TaskStatusCompleted {
		t.Fatalf("task status = %s, want completed", got)
	}

	subtitledPath := filepath.Join(outSubDir, base+"_subtitled.mp4")
	if _, err := os.Stat(subtitledPath); err != nil {
		entries, _ := os.ReadDir(outSubDir)
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("未找到字幕合成产物 %s，目录内容: %v", subtitledPath, names)
	}
}
