package logic

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"video-captions/internal/dto/req"
	"video-captions/internal/model"
	"video-captions/utils/logger"
)

func initReproDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "repro.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Video{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	model.DB = db
	return db
}

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func dumpList(t *testing.T, ctx context.Context, l *VideoLogic) {
	t.Helper()
	res, err := l.List(ctx, &req.VideoListReq{Page: 1, PageSize: 100})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, v := range res.List {
		fmt.Printf("  VIDEO name=%q path=%q\n", v.Name, v.Path)
		for _, f := range v.OutputFiles {
			fmt.Printf("      output file name=%q type=%q size=%d\n", f.Name, f.FileType, f.Size)
		}
	}
}

// Scenario A: 不同子文件夹中同名视频，各自带有同名输出目录
func TestReproSubfolderSameName(t *testing.T) {
	if err := logger.InitLogger("error"); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	initReproDB(t)

	root := filepath.Join(t.TempDir(), "lib")
	touch(t, filepath.Join(root, "Series A", "ep01.mp4"))
	touch(t, filepath.Join(root, "Series A", "ep01", "ep01_subtitled.mp4"))
	touch(t, filepath.Join(root, "Series B", "ep01.mp4"))
	touch(t, filepath.Join(root, "Series B", "ep01", "ep01_subtitled.mp4"))

	l := NewVideoLogic()
	if _, err := l.ScanDir(ctx, &req.VideoScanReq{Path: root}); err != nil {
		t.Fatalf("scan: %v", err)
	}
	fmt.Println("== Scenario A: same-name videos in different subfolders ==")
	dumpList(t, ctx, l)
}

// Scenario B: 子文件夹名与父目录中某个视频文件同名 -> isVideoOutputDir 误判
func TestReproSubfolderMisfire(t *testing.T) {
	if err := logger.InitLogger("error"); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	initReproDB(t)

	root := filepath.Join(t.TempDir(), "lib2")
	touch(t, filepath.Join(root, "movie.mp4"))
	touch(t, filepath.Join(root, "movie", "part1.mp4"))
	touch(t, filepath.Join(root, "movie", "part2.mp4"))

	l := NewVideoLogic()
	if _, err := l.ScanDir(ctx, &req.VideoScanReq{Path: root}); err != nil {
		t.Fatalf("scan: %v", err)
	}
	fmt.Println("== Scenario B: real subfolder whose name matches a parent video ==")
	dumpList(t, ctx, l)
}

// Scenario C: 视频在子文件夹，输出目录里有与其它视频混淆的文件
func TestReproSubfolderOutput(t *testing.T) {
	if err := logger.InitLogger("error"); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	initReproDB(t)

	root := filepath.Join(t.TempDir(), "lib3")
	touch(t, filepath.Join(root, "sub", "clip.mp4"))
	touch(t, filepath.Join(root, "sub", "clip", "clip.srt"))
	touch(t, filepath.Join(root, "sub", "clip", "clip_translated.srt"))
	touch(t, filepath.Join(root, "top.mp4"))
	touch(t, filepath.Join(root, "top", "top.srt"))

	l := NewVideoLogic()
	if _, err := l.ScanDir(ctx, &req.VideoScanReq{Path: root}); err != nil {
		t.Fatalf("scan: %v", err)
	}
	fmt.Println("== Scenario C: subfolder video with output dir ==")
	dumpList(t, ctx, l)
}
