<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { resetPassword, generateResetToken } from '@/api/auth'
import { validateUsername, validatePassword } from '@/utils/validate'

const router = useRouter()
const route = useRoute()
const { t } = useI18n()

const loading = ref(false)
const requesting = ref(false)
const countdown = ref(0)
let countdownTimer: number | undefined

const form = ref({
  username: '',
  token: '',
  new_password: '',
  confirm_password: '',
})

onMounted(() => {
  // 如果 URL 参数中有 token，自动填入
  const queryToken = route.query.token as string
  if (queryToken) {
    form.value.token = queryToken
  }
})

onUnmounted(() => {
  if (countdownTimer) {
    window.clearInterval(countdownTimer)
  }
})

// 触发生成重置令牌（令牌输出到服务器日志，60s 冷却）
async function handleGetToken(): Promise<void> {
  if (!form.value.username) {
    ElMessage.warning(t('auth.error.fill_fields'))
    return
  }
  // 校验用户名格式（与后端 auth_validate.go 规则一致）
  const usernameErr = validateUsername(form.value.username)
  if (usernameErr) {
    ElMessage.warning(t(usernameErr))
    return
  }
  requesting.value = true
  try {
    await generateResetToken(form.value.username)
    ElMessage.success(t('auth.success.token_generated'))
    startCountdown()
  } catch {
    // 错误已在拦截器中处理
  } finally {
    requesting.value = false
  }
}

function startCountdown(): void {
  countdown.value = 60
  countdownTimer = window.setInterval(() => {
    countdown.value -= 1
    if (countdown.value <= 0 && countdownTimer) {
      window.clearInterval(countdownTimer)
      countdownTimer = undefined
    }
  }, 1000)
}

async function handleReset(): Promise<void> {
  if (!form.value.token || !form.value.new_password || !form.value.confirm_password) {
    ElMessage.warning(t('auth.error.fill_fields'))
    return
  }
  if (form.value.new_password !== form.value.confirm_password) {
    ElMessage.warning(t('auth.error.password_mismatch'))
    return
  }
  // 校验新密码格式（与后端 auth_validate.go 规则一致）
  const pwdErr = validatePassword(form.value.new_password)
  if (pwdErr) {
    ElMessage.warning(t(pwdErr))
    return
  }
  loading.value = true
  try {
    await resetPassword(form.value)
    ElMessage.success(t('auth.success.reset_password'))
    router.push('/login')
  } catch {
    // 错误已在拦截器中处理
  } finally {
    loading.value = false
  }
}

function goBack(): void {
  router.push('/login')
}
</script>

<template>
  <div class="auth-page">
    <div class="auth-container">
      <div class="auth-header">
        <div class="auth-logo">
          <span class="auth-logo__mark">VF</span>
        </div>
        <h1 class="auth-title">VideoFlow</h1>
        <p class="auth-subtitle">{{ t('auth.forgot.subtitle') }}</p>
      </div>

      <div class="auth-card">
        <h2 class="section-title">{{ t('auth.forgot.step1_title') }}</h2>
        <p class="section-desc">{{ t('auth.forgot.step1_desc') }}</p>

        <el-form @submit.prevent="handleGetToken" class="auth-form">
          <el-form-item>
            <el-input
              v-model="form.username"
              :placeholder="t('auth.placeholder.username')"
              size="large"
              clearable
              @keyup.enter="handleGetToken"
            />
          </el-form-item>
          <el-form-item>
            <el-button
              type="primary"
              size="large"
              :loading="requesting"
              :disabled="countdown > 0"
              class="auth-btn"
              @click="handleGetToken"
            >
              {{
                countdown > 0
                  ? t('auth.btn.get_token_countdown', { countdown })
                  : t('auth.btn.get_token')
              }}
            </el-button>
          </el-form-item>
        </el-form>

        <div class="auth-tip">
          <p class="auth-tip__text">{{ t('auth.forgot.step1_tip') }}</p>
        </div>

        <div class="auth-divider" />

        <h2 class="section-title">{{ t('auth.forgot.step2_title') }}</h2>
        <p class="section-desc">{{ t('auth.forgot.step2_desc') }}</p>

        <el-form @submit.prevent="handleReset" class="auth-form">
          <el-form-item>
            <el-input
              v-model="form.token"
              :placeholder="t('auth.placeholder.token')"
              size="large"
              clearable
            />
          </el-form-item>
          <el-form-item>
            <el-input
              v-model="form.new_password"
              type="password"
              :placeholder="t('auth.placeholder.new_password')"
              size="large"
              show-password
            />
          </el-form-item>
          <el-form-item>
            <el-input
              v-model="form.confirm_password"
              type="password"
              :placeholder="t('auth.placeholder.confirm_password')"
              size="large"
              show-password
              @keyup.enter="handleReset"
            />
          </el-form-item>
          <el-form-item>
            <el-button
              type="primary"
              size="large"
              :loading="loading"
              class="auth-btn"
              @click="handleReset"
            >
              {{ t('auth.btn.reset_password') }}
            </el-button>
          </el-form-item>
        </el-form>

        <div class="auth-footer">
          <el-button link type="primary" @click="goBack">
            {{ t('auth.forgot.back_to_login') }}
          </el-button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.auth-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--vf-bg);
}

.auth-container {
  width: 100%;
  max-width: 420px;
  padding: 24px;
}

.auth-header {
  text-align: center;
  margin-bottom: 32px;
}

.auth-logo {
  display: inline-flex;
  margin-bottom: 16px;
}

.auth-logo__mark {
  width: 56px;
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--vf-accent-soft);
  border: 1px solid var(--vf-accent-border);
  color: var(--vf-accent);
  font-family: var(--vf-font-display);
  font-weight: 700;
  font-size: 22px;
  letter-spacing: 0.06em;
  border-radius: var(--vf-radius);
  box-shadow: var(--vf-glow-amber);
}

.auth-title {
  font-family: var(--vf-font-display);
  font-weight: 600;
  font-size: 28px;
  color: var(--vf-text-primary);
  margin: 12px 0 8px;
}

.auth-subtitle {
  color: var(--vf-text-muted);
  font-size: 14px;
  margin: 0;
}

.auth-card {
  background: var(--vf-bg-elevated);
  border: 1px solid var(--vf-border);
  border-radius: var(--vf-radius);
  padding: 28px;
}

.section-title {
  font-family: var(--vf-font-display);
  font-weight: 600;
  font-size: 16px;
  color: var(--vf-text-primary);
  margin: 0 0 4px;
}

.section-desc {
  color: var(--vf-text-muted);
  font-size: 13px;
  margin: 0 0 20px;
}

.auth-form {
  width: 100%;
}

.auth-btn {
  width: 100%;
}

.auth-tip {
  margin-top: 16px;
  padding: 12px;
  background: var(--vf-bg-panel);
  border: 1px solid var(--vf-border);
  border-radius: var(--vf-radius-sm);
  text-align: left;
}

.auth-tip__text {
  font-size: 12px;
  color: var(--vf-text-muted);
  margin: 0;
  line-height: 1.6;
}

.auth-divider {
  height: 1px;
  background: var(--vf-border);
  margin: 24px 0;
}

.auth-footer {
  text-align: center;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--vf-border);
}
</style>
