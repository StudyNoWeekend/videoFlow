import { ref, computed } from 'vue'
import { defineStore } from 'pinia'
import { checkAuthStatus, type UserInfo } from '@/api/auth'

const TOKEN_KEY = 'videoflow-token'
const USER_KEY = 'videoflow-user'

function parseTokenPayload(token: string): UserInfo | null {
  try {
    const parts = token.split('.')
    if (parts.length !== 3) return null
    const payload = JSON.parse(atob(parts[1]))
    return {
      id: payload.user_id || '',
      username: payload.username || '',
    }
  } catch {
    return null
  }
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string | null>(localStorage.getItem(TOKEN_KEY))
  const user = ref<UserInfo | null>(loadUser())
  const isInitialized = ref(false)
  const statusLoaded = ref(false)

  function loadUser(): UserInfo | null {
    try {
      const raw = localStorage.getItem(USER_KEY)
      return raw ? JSON.parse(raw) : null
    } catch {
      return null
    }
  }

  const isAuthenticated = computed(() => token.value !== null && user.value !== null)

  /**
   * 查询后端初始化状态
   */
  async function loadStatus(): Promise<void> {
    const res = await checkAuthStatus()
    isInitialized.value = res.initialized
    statusLoaded.value = true
  }

  /**
   * 登录：保存 token 和用户信息
   */
  function login(newToken: string, newUser: UserInfo): void {
    token.value = newToken
    user.value = newUser
    localStorage.setItem(TOKEN_KEY, newToken)
    localStorage.setItem(USER_KEY, JSON.stringify(newUser))
  }

  /**
   * 退出登录
   */
  function logout(): void {
    token.value = null
    user.value = null
    statusLoaded.value = false
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(USER_KEY)
  }

  /**
   * 检查 token 是否有效
   */
  function isTokenValid(): boolean {
    if (!token.value) return false
    try {
      const parts = token.value.split('.')
      if (parts.length !== 3) return false
      const payload = JSON.parse(atob(parts[1]))
      if (payload.exp) {
        return Date.now() < payload.exp * 1000
      }
      return true
    } catch {
      return false
    }
  }

  return {
    token,
    user,
    isInitialized,
    statusLoaded,
    isAuthenticated,
    loadStatus,
    login,
    logout,
    isTokenValid,
  }
})
