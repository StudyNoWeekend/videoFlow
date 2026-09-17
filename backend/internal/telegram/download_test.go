package telegram

import (
	"testing"
	"time"

	"github.com/gotd/td/tgerr"
)

// newTgerrError 构造一个 Telegram RPC 错误用于测试错误识别逻辑
func newTgerrError(message string) error {
	return tgerr.New(420, message)
}

func TestIsFileReferenceExpired(t *testing.T) {
	if !isFileReferenceExpired(newTgerrError("FILE_REFERENCE_EXPIRED")) {
		t.Error("FILE_REFERENCE_EXPIRED 应被识别为文件引用过期")
	}
	if !isFileReferenceExpired(newTgerrError("FILE_REFERENCE_INVALID")) {
		t.Error("FILE_REFERENCE_INVALID 应被识别为文件引用过期")
	}
	if isFileReferenceExpired(newTgerrError("FLOOD_WAIT_5")) {
		t.Error("FLOOD_WAIT 不应被识别为文件引用过期")
	}
}

func TestProgressThrottler(t *testing.T) {
	throttle := newProgressThrottler(time.Second)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// 首次必须回调
	if !throttle.allow(0, base) {
		t.Fatal("首次进度应允许回调")
	}

	// 同一百分比且间隔不足 1 秒，不应回调
	for i := 1; i <= 5; i++ {
		if throttle.allow(0, base.Add(time.Duration(i)*100*time.Millisecond)) {
			t.Errorf("同一百分比且间隔不足时不应回调（第 %d 次）", i)
		}
	}

	// 百分比变化后应立即回调
	if !throttle.allow(50, base.Add(600*time.Millisecond)) {
		t.Error("百分比变化后应允许回调")
	}

	// 百分比不变但超过间隔后应回调
	if !throttle.allow(50, base.Add(1600*time.Millisecond)) {
		t.Error("超过节流间隔后应允许回调")
	}
}

func TestProgressThrottlerReport(t *testing.T) {
	throttle := newProgressThrottler(time.Second)

	var calls int
	var lastDownloaded, lastTotal int64
	onProgress := func(downloaded, total int64) {
		calls++
		lastDownloaded, lastTotal = downloaded, total
	}

	// 回调为空时不应 panic
	throttle.report(1, 100, nil)

	// 百分比未变化且间隔不足 1 秒，只应回调一次
	for i := 1; i <= 5; i++ {
		throttle.report(int64(i)*10, 10000, onProgress)
	}
	if calls != 1 {
		t.Errorf("同一百分比下的回调次数 = %d，期望 1", calls)
	}

	// 进度跨过百分比阈值后应再次回调
	throttle.report(6000, 10000, onProgress)
	if calls != 2 {
		t.Errorf("百分比变化后的回调次数 = %d，期望 2", calls)
	}
	if lastDownloaded != 6000 || lastTotal != 10000 {
		t.Errorf("回调参数 = (%d, %d)，期望 (6000, 10000)", lastDownloaded, lastTotal)
	}
}

func TestProgressReporterCollectsElemError(t *testing.T) {
	reporter := newProgressReporter(100, nil)

	// tdl 的 Download 不返回元素级错误，必须经 OnDone 收集
	if reporter.Err() != nil {
		t.Fatal("初始状态不应有错误")
	}

	want := newTgerrError("FILE_REFERENCE_EXPIRED")
	reporter.OnDone(nil, want)
	if reporter.Err() != want {
		t.Errorf("OnDone 上报的错误未被收集")
	}
}

func TestKindLabel(t *testing.T) {
	tests := []struct {
		kind string
		want string
	}{
		{MediaTypeVideo, "视频"},
		{MediaTypePhoto, "图片"},
		{MediaTypeAudio, "音频"},
		{MediaTypeDocument, "文件"},
		{"unknown", "文件"},
	}

	for _, tt := range tests {
		if got := kindLabel(tt.kind); got != tt.want {
			t.Errorf("kindLabel(%q) = %q, want %q", tt.kind, got, tt.want)
		}
	}
}
