package model

import (
	"context"
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
	if err := db.AutoMigrate(&Video{}, &Task{}); err != nil {
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
