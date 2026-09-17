package logic

import (
	"context"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"video-captions/bootstrap"
	"video-captions/internal/model"
)

// initProxyDB 用临时 SQLite 初始化 settings 表，避免污染真实数据库
func initProxyDB(t *testing.T) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "proxy.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	if err := db.AutoMigrate(&model.Setting{}); err != nil {
		t.Fatalf("迁移 settings 表失败: %v", err)
	}

	// 测试期间不依赖配置文件，仅验证 settings 表与默认值逻辑
	bootstrap.Config = nil
	model.DB = db
}

func TestBuildProxyArgs(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		proxy   string
		want    []string
	}{
		{"启用且有代理", true, "socks5://127.0.0.1:1080", []string{"--proxy", "socks5://127.0.0.1:1080"}},
		{"启用但未配置代理", true, "", nil},
		{"开关关闭", false, "socks5://127.0.0.1:1080", nil},
		{"开关关闭且无代理", false, "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildProxyArgs(tt.enabled, tt.proxy)
			if len(got) != len(tt.want) {
				t.Fatalf("buildProxyArgs(%v, %q) = %v, want %v", tt.enabled, tt.proxy, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("第 %d 项 = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestValidateProxyURL(t *testing.T) {
	valid := []string{
		"",
		"socks5://127.0.0.1:1080",
		"socks5h://127.0.0.1:1080",
		"http://192.168.2.160:7890",
		"https://proxy.example.com:8443",
		"SOCKS5://127.0.0.1:1080",
		"http://user:pass@127.0.0.1:7890",
	}
	for _, raw := range valid {
		if err := validateProxyURL(raw); err != nil {
			t.Errorf("validateProxyURL(%q) 期望通过，实际报错: %v", raw, err)
		}
	}

	invalid := []string{
		"127.0.0.1:1080",
		"ftp://127.0.0.1:1080",
		"socks4://127.0.0.1:1080",
		"http://",
		"socks5://",
	}
	for _, raw := range invalid {
		if err := validateProxyURL(raw); err == nil {
			t.Errorf("validateProxyURL(%q) 期望报错，实际通过", raw)
		}
	}
}

// TestGetProxyURLPriority 校验读取优先级：新键 → 旧键（兼容）→ 默认值
func TestGetProxyURLPriority(t *testing.T) {
	ctx := context.Background()

	t.Run("优先读新键", func(t *testing.T) {
		initProxyDB(t)
		if err := model.SettingSet(ctx, model.SettingKeyProxyURL, "socks5://new:1080"); err != nil {
			t.Fatal(err)
		}
		if err := model.SettingSet(ctx, model.SettingKeyTelegramProxy, "http://legacy:7890"); err != nil {
			t.Fatal(err)
		}
		if got := getProxyURL(ctx); got != "socks5://new:1080" {
			t.Errorf("getProxyURL() = %q, 期望新键的值", got)
		}
	})

	t.Run("新键为空时回退到旧键", func(t *testing.T) {
		initProxyDB(t)
		if err := model.SettingSet(ctx, model.SettingKeyTelegramProxy, "http://legacy:7890"); err != nil {
			t.Fatal(err)
		}
		if got := getProxyURL(ctx); got != "http://legacy:7890" {
			t.Errorf("getProxyURL() = %q, 期望回退到旧键的值", got)
		}
	})

	t.Run("都未配置时为空", func(t *testing.T) {
		initProxyDB(t)
		if got := getProxyURL(ctx); got != "" {
			t.Errorf("getProxyURL() = %q, 期望为空", got)
		}
	})
}

func TestProxyEnabledForYtdlp(t *testing.T) {
	ctx := context.Background()

	t.Run("默认开启", func(t *testing.T) {
		initProxyDB(t)
		if !proxyEnabledForYtdlp(ctx) {
			t.Error("未配置时应默认开启")
		}
	})

	t.Run("显式关闭", func(t *testing.T) {
		initProxyDB(t)
		if err := model.SettingSet(ctx, model.SettingKeyProxyForYtdlp, "false"); err != nil {
			t.Fatal(err)
		}
		if proxyEnabledForYtdlp(ctx) {
			t.Error("显式关闭后应返回 false")
		}
	})
}
