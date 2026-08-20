package logic

import (
	"regexp"

	"video-captions/enum"
)

// 账户格式校验规则（与前端 frontend/src/utils/validate.ts 保持一致，改动需同步）：
//   - 用户名：3-64 位，仅允许字母和数字（ASCII）
//   - 密码：6-64 位，必须同时包含字母和数字
var (
	usernameRE = regexp.MustCompile(`^[a-zA-Z0-9]{3,64}$`)
	letterRE   = regexp.MustCompile(`[a-zA-Z]`)
	digitRE    = regexp.MustCompile(`[0-9]`)
)

// validateUsername 校验用户名格式：3-64 位字母/数字组合
func validateUsername(username string) error {
	if !usernameRE.MatchString(username) {
		return enum.ErrInvalidUsername
	}
	return nil
}

// validatePassword 校验密码格式：6-64 位，且必须同时包含字母和数字
func validatePassword(password string) error {
	if len(password) < 6 || len(password) > 64 {
		return enum.ErrWeakPassword
	}
	if !letterRE.MatchString(password) || !digitRE.MatchString(password) {
		return enum.ErrWeakPassword
	}
	return nil
}
