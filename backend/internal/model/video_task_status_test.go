package model

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func initVideoStatusDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "video_status.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&Video{}, &Task{}, &Setting{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	DB = db
}

func mkVideo(t *testing.T, path string) *Video {
	t.Helper()
	v := &Video{Path: path, Name: filepath.Base(path)}
	if err := VideoCreate(context.Background(), v); err != nil {
		t.Fatalf("create video: %v", err)
	}
	return v
}

func mkTask(t *testing.T, videoID, taskType, status string) *Task {
	t.Helper()
	return mkTaskAt(t, videoID, taskType, status, 0)
}

// mkTaskAt 创建任务并显式指定 created_at（created_at 为秒级精度，测试需保证先后可区分）
func mkTaskAt(t *testing.T, videoID, taskType, status string, createdAt int64) *Task {
	t.Helper()
	task := &Task{VideoID: videoID, TaskType: taskType, Status: status, BaseModel: BaseModel{CreatedAt: createdAt}}
	if err := TaskCreate(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	return task
}

// videoStatusOf 读取视频对应任务类型的状态字段
func videoStatusOf(v *Video, taskType string) string {
	switch taskType {
	case TaskTypeSubtitle:
		return v.SubtitleStatus
	case TaskTypeSubtitleBurn:
		return v.SubtitleBurnStatus
	case TaskTypeDeblur, TaskTypeRepair:
		return v.DeblurStatus
	case TaskTypeUpscale:
		return v.UpscaleStatus
	}
	return ""
}

func TestVideoTaskStatusColumn(t *testing.T) {
	cases := map[string]string{
		TaskTypeSubtitle:     "subtitle_status",
		TaskTypeSubtitleBurn: "subtitle_burn_status",
		TaskTypeDeblur:       "deblur_status",
		TaskTypeRepair:       "deblur_status",
		TaskTypeUpscale:      "upscale_status",
		"unknown":            "",
	}
	for taskType, want := range cases {
		if got := VideoTaskStatusColumn(taskType); got != want {
			t.Fatalf("VideoTaskStatusColumn(%s) = %s, want %s", taskType, got, want)
		}
	}
}

func TestVideoSetTaskStatusTx(t *testing.T) {
	initVideoStatusDB(t)
	v := mkVideo(t, "/v/a.mp4")

	DB.Transaction(func(tx *gorm.DB) error {
		if err := VideoSetTaskStatusTx(tx, v.ID, TaskTypeSubtitle, TaskStatusCompleted); err != nil {
			t.Fatalf("set subtitle status: %v", err)
		}
		return nil
	})

	got := videoByID(t, v.ID)
	if got.SubtitleStatus != TaskStatusCompleted {
		t.Fatalf("subtitle_status = %s, want %s", got.SubtitleStatus, TaskStatusCompleted)
	}
	if got.SubtitleBurnStatus != "" {
		t.Fatalf("subtitle_burn_status should stay empty, got %s", got.SubtitleBurnStatus)
	}
}

func TestVideoResyncTaskStatusTx(t *testing.T) {
	initVideoStatusDB(t)
	v := mkVideo(t, "/v/b.mp4")

	// 无任务记录时，重同步应清空状态字段
	DB.Transaction(func(tx *gorm.DB) error {
		return VideoSetTaskStatusTx(tx, v.ID, TaskTypeSubtitle, TaskStatusCompleted)
	})
	DB.Transaction(func(tx *gorm.DB) error {
		return VideoResyncTaskStatusTx(tx, v.ID, TaskTypeSubtitle)
	})
	if got := videoByID(t, v.ID).SubtitleStatus; got != "" {
		t.Fatalf("no task should clear status, got %s", got)
	}

	// 最新任务状态为 completed，重同步应写入 completed
	mkTaskAt(t, v.ID, TaskTypeSubtitle, TaskStatusCompleted, 1000)
	DB.Transaction(func(tx *gorm.DB) error {
		return VideoResyncTaskStatusTx(tx, v.ID, TaskTypeSubtitle)
	})
	if got := videoByID(t, v.ID).SubtitleStatus; got != TaskStatusCompleted {
		t.Fatalf("resync with completed task = %s, want %s", got, TaskStatusCompleted)
	}

	// 再创建一条更新时间更晚的 pending 任务，重同步应取最新任务（pending）
	mkTaskAt(t, v.ID, TaskTypeSubtitle, TaskStatusPending, 2000)
	DB.Transaction(func(tx *gorm.DB) error {
		return VideoResyncTaskStatusTx(tx, v.ID, TaskTypeSubtitle)
	})
	if got := videoByID(t, v.ID).SubtitleStatus; got != TaskStatusPending {
		t.Fatalf("resync with newer pending task = %s, want %s", got, TaskStatusPending)
	}
}

func TestVideoResyncAllTaskStatus(t *testing.T) {
	initVideoStatusDB(t)
	ctx := context.Background()

	// 视频 a：仅有字幕任务已完成；视频 b：仅烧录任务失败
	a := mkVideo(t, "/v/c.mp4")
	b := mkVideo(t, "/v/d.mp4")
	mkTask(t, a.ID, TaskTypeSubtitle, TaskStatusCompleted)
	mkTask(t, b.ID, TaskTypeSubtitleBurn, TaskStatusFailed)

	// 先写入错误状态，再全量重同步
	DB.Transaction(func(tx *gorm.DB) error {
		if err := VideoSetTaskStatusTx(tx, a.ID, TaskTypeSubtitle, TaskStatusPending); err != nil {
			return err
		}
		return VideoSetTaskStatusTx(tx, b.ID, TaskTypeSubtitleBurn, TaskStatusRunning)
	})
	if err := VideoResyncAllTaskStatus(ctx); err != nil {
		t.Fatalf("resync all: %v", err)
	}

	if got := videoByID(t, a.ID).SubtitleStatus; got != TaskStatusCompleted {
		t.Fatalf("a subtitle_status = %s, want %s", got, TaskStatusCompleted)
	}
	if got := videoByID(t, a.ID).SubtitleBurnStatus; got != "" {
		t.Fatalf("a subtitle_burn_status = %s, want empty", got)
	}
	if got := videoByID(t, b.ID).SubtitleBurnStatus; got != TaskStatusFailed {
		t.Fatalf("b subtitle_burn_status = %s, want %s", got, TaskStatusFailed)
	}
}

func videoByID(t *testing.T, id string) *Video {
	t.Helper()
	var v Video
	if err := DB.First(&v, "id = ?", id).Error; err != nil {
		t.Fatalf("query video: %v", err)
	}
	return &v
}

// initArtifactStatusDB 初始化产物探测测试环境：迁移 Video/Task/Setting 并把
// output_dir 指向独立临时目录，返回该目录
func initArtifactStatusDB(t *testing.T) string {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "video_artifact.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&Video{}, &Task{}, &Setting{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	DB = db
	outputDir := t.TempDir()
	if err := SettingSet(context.Background(), SettingKeyOutputDir, outputDir); err != nil {
		t.Fatal(err)
	}
	return outputDir
}

func touchFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestVideoArtifactStatuses(t *testing.T) {
	outputDir := initArtifactStatusDB(t)
	inputDir := t.TempDir()
	v := mkVideo(t, filepath.Join(inputDir, "movie.mp4"))
	outDir := filepath.Join(outputDir, "movie")

	// 无任何产物：全部为 false
	for taskType, exists := range VideoArtifactStatuses(v) {
		if exists {
			t.Fatalf("%s should be false without artifacts", taskType)
		}
	}

	// 与本底同名的文件不算产物（fixed_ 开头若不跳过会误判为去马赛克产物）
	touchFile(t, filepath.Join(outDir, "movie.mp4"))
	if statuses := VideoArtifactStatuses(v); statuses[TaskTypeDeblur] || statuses[TaskTypeUpscale] || statuses[TaskTypeSubtitleBurn] {
		t.Fatal("same-name file must not be counted as artifact")
	}

	touchFile(t, filepath.Join(outDir, "movie.srt"))
	touchFile(t, filepath.Join(outDir, "movie_subtitled_123.mp4"))
	touchFile(t, filepath.Join(outDir, "movie_upscaled_720p.mp4"))
	touchFile(t, filepath.Join(outDir, "movie_fixed_1.mp4"))

	statuses := VideoArtifactStatuses(v)
	if !statuses[TaskTypeSubtitle] {
		t.Fatal("subtitle artifact not detected")
	}
	if !statuses[TaskTypeSubtitleBurn] {
		t.Fatal("subtitle burn artifact (timestamped variant) not detected")
	}
	if !statuses[TaskTypeUpscale] {
		t.Fatal("upscale artifact not detected")
	}
	if !statuses[TaskTypeDeblur] {
		t.Fatal("deblur artifact (fixed_ prefix) not detected")
	}
}

func resyncWithArtifact(t *testing.T, videoID, taskType string) {
	t.Helper()
	if err := DB.Transaction(func(tx *gorm.DB) error {
		return VideoResyncTaskStatusWithArtifactTx(tx, videoID, taskType)
	}); err != nil {
		t.Fatalf("resync with artifact: %v", err)
	}
}

func TestVideoResyncTaskStatusWithArtifactTx(t *testing.T) {
	outputDir := initArtifactStatusDB(t)
	inputDir := t.TempDir()

	// 无任务记录、产物存在 → 兜底为 completed
	va := mkVideo(t, filepath.Join(inputDir, "a.mp4"))
	touchFile(t, filepath.Join(outputDir, "a", "a.srt"))
	resyncWithArtifact(t, va.ID, TaskTypeSubtitle)
	if got := videoByID(t, va.ID).SubtitleStatus; got != TaskStatusCompleted {
		t.Fatalf("artifact fallback = %s, want completed", got)
	}

	// 无任务记录、无产物 → 清空
	vb := mkVideo(t, filepath.Join(inputDir, "b.mp4"))
	DB.Transaction(func(tx *gorm.DB) error {
		return VideoSetTaskStatusTx(tx, vb.ID, TaskTypeSubtitle, TaskStatusRunning)
	})
	resyncWithArtifact(t, vb.ID, TaskTypeSubtitle)
	if got := videoByID(t, vb.ID).SubtitleStatus; got != "" {
		t.Fatalf("no record no artifact = %s, want empty", got)
	}

	// 任务记录存在（pending）+ 产物存在 → 记录优先
	mkTaskAt(t, va.ID, TaskTypeSubtitle, TaskStatusPending, 1000)
	resyncWithArtifact(t, va.ID, TaskTypeSubtitle)
	if got := videoByID(t, va.ID).SubtitleStatus; got != TaskStatusPending {
		t.Fatalf("record wins = %s, want pending", got)
	}
}

func TestVideoResyncAllTaskStatusArtifactFallback(t *testing.T) {
	outputDir := initArtifactStatusDB(t)
	inputDir := t.TempDir()

	// a：产物存在、无记录 → 兜底 completed；b：有 failed 记录 + 产物 → 保持 failed；c：无产物 → 空
	a := mkVideo(t, filepath.Join(inputDir, "a.mp4"))
	b := mkVideo(t, filepath.Join(inputDir, "b.mp4"))
	c := mkVideo(t, filepath.Join(inputDir, "c.mp4"))
	touchFile(t, filepath.Join(outputDir, "a", "a.srt"))
	touchFile(t, filepath.Join(outputDir, "b", "b.srt"))
	mkTask(t, b.ID, TaskTypeSubtitle, TaskStatusFailed)

	if err := VideoResyncAllTaskStatus(context.Background()); err != nil {
		t.Fatalf("resync all: %v", err)
	}

	if got := videoByID(t, a.ID).SubtitleStatus; got != TaskStatusCompleted {
		t.Fatalf("a subtitle_status = %s, want completed (artifact fallback)", got)
	}
	if got := videoByID(t, b.ID).SubtitleStatus; got != TaskStatusFailed {
		t.Fatalf("b subtitle_status = %s, want failed (record wins)", got)
	}
	if got := videoByID(t, c.ID).SubtitleStatus; got != "" {
		t.Fatalf("c subtitle_status = %s, want empty", got)
	}

	// 删除产物后再回填 → 清空
	if err := os.Remove(filepath.Join(outputDir, "a", "a.srt")); err != nil {
		t.Fatal(err)
	}
	if err := VideoResyncAllTaskStatus(context.Background()); err != nil {
		t.Fatalf("resync all after artifact removed: %v", err)
	}
	if got := videoByID(t, a.ID).SubtitleStatus; got != "" {
		t.Fatalf("a subtitle_status after artifact removed = %s, want empty", got)
	}
}
