package logic

import (
	"context"
	"errors"
	"testing"

	"video-captions/enum"
	"video-captions/internal/model"
)

func TestDetectPlatform(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "Telegram 公开频道",
			url:  "https://t.me/telegram/193",
			want: model.DownloadPlatformTelegram,
		},
		{
			name: "Telegram 私有频道",
			url:  "https://t.me/c/1697797156/151",
			want: model.DownloadPlatformTelegram,
		},
		{
			name: "Telegram 评论区链接",
			url:  "https://t.me/opencfdchannel/4434?comment=360409",
			want: model.DownloadPlatformTelegram,
		},
		{
			name: "YouTube",
			url:  "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			want: model.DownloadPlatformYtDlp,
		},
		{
			name: "Bilibili",
			url:  "https://www.bilibili.com/video/BV1xx411c7mD",
			want: model.DownloadPlatformYtDlp,
		},
		{
			name: "相似域名不应误判",
			url:  "https://fake-t.me/telegram/193",
			want: model.DownloadPlatformYtDlp,
		},
		{
			name: "普通站点",
			url:  "https://example.com/video/1",
			want: model.DownloadPlatformYtDlp,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectPlatform(tt.url); got != tt.want {
				t.Errorf("DetectPlatform(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

// TestCheckPlatformDependenciesTelegram 未注入 Telegram 引擎时应给出「未配置」提示，
// 而不是走 yt-dlp 的组件校验
func TestCheckPlatformDependenciesTelegram(t *testing.T) {
	err := checkPlatformDependencies(context.Background(), model.DownloadPlatformTelegram)
	if err == nil {
		t.Fatal("未配置 Telegram 引擎时应当返回错误")
	}

	var bizErr *enum.BizError
	if !errors.As(err, &bizErr) {
		t.Fatalf("错误类型 = %T，期望 *enum.BizError", err)
	}
	if bizErr.Code != enum.ErrTelegramNotConfigured.Code {
		t.Errorf("错误码 = %d，期望 %d", bizErr.Code, enum.ErrTelegramNotConfigured.Code)
	}
}
