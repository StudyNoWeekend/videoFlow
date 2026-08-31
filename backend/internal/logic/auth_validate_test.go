package logic

import (
	"errors"
	"strings"
	"testing"

	"video-captions/enum"
)

// 用户名：3-64 位字母+数字
func TestValidateUsername(t *testing.T) {
	cases := []struct {
		username string
		wantErr  bool
	}{
		{"abc", false},                   // 纯字母，合规
		{"abc123", false},                // 字母+数字，合规
		{"123", false},                   // 纯数字（规则允许字母或数字），合规
		{strings.Repeat("a", 64), false}, // 上限 64 位
		{"ab", true},                     // 少于 3 位
		{strings.Repeat("a", 65), true},  // 超过 64 位
		{"abc_", true},                   // 含下划线
		{"abc-1", true},                  // 含连字符
		{"abc 1", true},                  // 含空格
		{"张三", true},                     // 非 ASCII 字母
		{"", true},                       // 空
	}
	for _, c := range cases {
		err := validateUsername(c.username)
		if (err != nil) != c.wantErr {
			t.Errorf("validateUsername(%q) = %v, wantErr=%v", c.username, err, c.wantErr)
		}
		if err != nil && !errors.Is(err, enum.ErrInvalidUsername) {
			t.Errorf("validateUsername(%q) 应返回 ErrInvalidUsername, 实际 %v", c.username, err)
		}
	}
}

// 密码：6-64 位且同时包含字母和数字
func TestValidatePassword(t *testing.T) {
	cases := []struct {
		password string
		wantErr  bool
	}{
		{"abc123", false},                // 字母+数字，合规
		{"a1b2c3", false},                // 字母+数字，合规
		{"Aa1234", false},                // 大小写字母+数字，合规
		{"a1!@#$", false},                // 其它字符不限制，含字母+数字即可
		{"abc def1a", false},             // 含空格，但字母+数字齐全，合规
		{strings.Repeat("a1", 33), true}, // 66 位，超过 64 位上限
		{"abcdef", true},                 // 纯字母，缺数字
		{"123456", true},                 // 纯数字，缺字母
		{"abc12", true},                  // 少于 6 位
		{"abcde1", false},                // 正好 6 位且含字母数字
		{"", true},                       // 空
	}
	for _, c := range cases {
		err := validatePassword(c.password)
		if (err != nil) != c.wantErr {
			t.Errorf("validatePassword(%q) = %v, wantErr=%v", c.password, err, c.wantErr)
		}
		if err != nil && !errors.Is(err, enum.ErrWeakPassword) {
			t.Errorf("validatePassword(%q) 应返回 ErrWeakPassword, 实际 %v", c.password, err)
		}
	}
}
