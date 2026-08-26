package model

import (
	"context"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func initOutputDirDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "outputdir.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&Setting{}); err != nil {
		t.Fatalf("migrate setting: %v", err)
	}
	DB = db
}

func TestVideoOutputDir(t *testing.T) {
	ctx := context.Background()
	initOutputDirDB(t)

	input := "/data/input"
	output := "/data/output"
	video := &Video{Path: filepath.Join(input, "movie.mp4"), Name: "movie.mp4"}

	// 未配置 output_dir（行不存在）：默认输出到 /output，不再落入输入树
	if got := VideoOutputDir(ctx, video); got != filepath.Join(DefaultOutputDir, "movie") {
		t.Fatalf("no output_dir: got %s", got)
	}

	// 行存在但值为空串：同样落 /output
	if err := SettingSet(ctx, SettingKeyOutputDir, ""); err != nil {
		t.Fatal(err)
	}
	if got := VideoOutputDir(ctx, video); got != filepath.Join(DefaultOutputDir, "movie") {
		t.Fatalf("empty output_dir: got %s", got)
	}

	// 配置 output_dir 且视频在输入目录根下：output/<base>
	if err := SettingSet(ctx, SettingKeyOutputDir, output); err != nil {
		t.Fatal(err)
	}
	if err := SettingSet(ctx, SettingKeyVideoDir, input); err != nil {
		t.Fatal(err)
	}
	if got := VideoOutputDir(ctx, video); got != filepath.Join(output, "movie") {
		t.Fatalf("root video: got %s", got)
	}

	// 视频在输入目录子文件夹：镜像相对结构 output/<相对子目录>/<base>
	subVideo := &Video{Path: filepath.Join(input, "Series A", "ep01.mp4"), Name: "ep01.mp4"}
	if got := VideoOutputDir(ctx, subVideo); got != filepath.Join(output, "Series A", "ep01") {
		t.Fatalf("subfolder video: got %s", got)
	}

	// 配置了 output_dir 但视频不在 audio/输入目录下（手动扫描其它路径）：output/<base>
	outsideVideo := &Video{Path: "/elsewhere/clip.mkv", Name: "clip.mkv"}
	if got := VideoOutputDir(ctx, outsideVideo); got != filepath.Join(output, "clip") {
		t.Fatalf("outside video: got %s", got)
	}

	// 输入目录本身是文件（边界）：按不在输入目录下处理
	edge := &Video{Path: filepath.Join(input, "a.mp4"), Name: "a.mp4"}
	if got := VideoOutputDir(ctx, edge); got != filepath.Join(output, "a") {
		t.Fatalf("edge video: got %s", got)
	}
}
