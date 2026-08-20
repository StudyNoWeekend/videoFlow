/**
 * 账户格式校验（与后端 backend/internal/logic/auth_validate.go 规则保持一致，改动需同步）：
 *  - 用户名：3-64 位，仅允许字母和数字（ASCII）
 *  - 密码：6-64 位，必须同时包含字母和数字（其它字符不限制）
 */

const USERNAME_RE = /^[a-zA-Z0-9]{3,64}$/
const LETTER_RE = /[a-zA-Z]/
const DIGIT_RE = /[0-9]/

/**
 * 校验用户名格式
 * @returns 错误提示的 i18n key，合规时返回 null
 */
export function validateUsername(username: string): string | null {
  if (!USERNAME_RE.test(username)) {
    return 'auth.error.username_invalid'
  }
  return null
}

/**
 * 校验密码格式
 * @returns 错误提示的 i18n key，合规时返回 null
 */
export function validatePassword(password: string): string | null {
  if (!password || password.length < 6 || password.length > 64) {
    return 'auth.error.password_too_short'
  }
  if (!LETTER_RE.test(password) || !DIGIT_RE.test(password)) {
    return 'auth.error.password_weak'
  }
  return null
}
