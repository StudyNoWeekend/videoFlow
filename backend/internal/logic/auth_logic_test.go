package logic

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"video-captions/enum"
	"video-captions/internal/model"
	"video-captions/utils/logger"
)

func initAuthDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "auth.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.ResetToken{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	model.DB = db
}

func createUser(t *testing.T, username string) {
	t.Helper()
	u := model.User{Username: username, Email: username + "@test.local", Password: "hashed"}
	u.ID = uuid.New().String()
	if err := model.DB.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
}

// 用户不存在时返回 ErrUserNotFound，且不消耗冷却时间
func TestGenerateResetTokenForAPIUnknownUser(t *testing.T) {
	if err := logger.InitLogger("error", ""); err != nil {
		t.Fatal(err)
	}
	initAuthDB(t)
	ctx := context.Background()
	l := NewAuthLogic()

	err := l.GenerateResetTokenForAPI(ctx, "not-exist-user")
	if !errors.Is(err, enum.ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
	if len(resetTokenCoolDownMap.last) != 0 {
		t.Fatalf("failed attempt should not consume cooldown, last=%v", resetTokenCoolDownMap.last)
	}
}

// 首次生成成功；60s 冷却内再次触发返回 ErrResetTokenTooFrequent；
// 令牌只写入日志，不通过返回值暴露
func TestGenerateResetTokenForAPICooldown(t *testing.T) {
	if err := logger.InitLogger("error", ""); err != nil {
		t.Fatal(err)
	}
	initAuthDB(t)
	ctx := context.Background()
	username := "cooldown-user"
	createUser(t, username)

	l := NewAuthLogic()
	if err := l.GenerateResetTokenForAPI(ctx, username); err != nil {
		t.Fatalf("first generate should succeed: %v", err)
	}

	// 冷却期内再次请求应失败
	err := l.GenerateResetTokenForAPI(ctx, username)
	var bizErr *enum.BizError
	if !errors.As(err, &bizErr) || bizErr.HttpCode != 429 {
		t.Fatalf("expected 429 cooldown error, got %v", err)
	}

	// 数据库中应只生成一条有效令牌记录
	var count int64
	if err := model.DB.Model(&model.ResetToken{}).Count(&count).Error; err != nil {
		t.Fatalf("count tokens: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 token record, got %d", count)
	}

	// 冷却记录按用户名隔离：其他用户名不受影响
	other := "cooldown-user-2"
	createUser(t, other)
	if err := l.GenerateResetTokenForAPI(ctx, other); err != nil {
		t.Fatalf("other user should not be throttled: %v", err)
	}
}
