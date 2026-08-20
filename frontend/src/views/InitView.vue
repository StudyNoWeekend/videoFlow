<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { initSystem } from '@/api/auth'
import { validateUsername, validatePassword } from '@/utils/validate'

const router = useRouter()
const { t } = useI18n()
const auth = useAuthStore()

const loading = ref(false)

const form = reactive({
  username: '',
  password: '',
  confirm_password: '',
})

async function handleInit(): Promise<void> {
  if (!form.username || !form.password || !form.confirm_password) {
    ElMessage.warning(t('auth.error.fill_fields'))
    return
  }
  // 校验用户名与密码格式（与后端 auth_validate.go 规则一致）
  const usernameErr = validateUsername(form.username)
  if (usernameErr) {
    ElMessage.warning(t(usernameErr))
    return
  }
  const pwdErr = validatePassword(form.password)
  if (pwdErr) {
    ElMessage.warning(t(pwdErr))
    return
  }
  if (form.password !== form.confirm_password) {
    ElMessage.warning(t('auth.error.password_mismatch'))
    return
  }

  loading.value = true
  try {
    const res = await initSystem(form)
    auth.login(res.token, res.user)
    ElMessage.success(t('auth.success.init'))
    router.replace('/')
  } catch {
    // 错误已在拦截器中处理
  } finally {
    loading.value = false
  }
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
        <p class="auth-subtitle">{{ t('auth.init.subtitle') }}</p>
      </div>

      <div class="auth-card">
        <h2 class="section-title">{{ t('auth.init.step1_title') }}</h2>
        <p class="section-desc">{{ t('auth.init.step1_desc') }}</p>

        <el-form @submit.prevent="handleInit" class="auth-form">
          <el-form-item>
            <el-input
              v-model="form.username"
              :placeholder="t('auth.placeholder.username')"
              size="large"
              clearable
            />
          </el-form-item>
          <el-form-item>
            <el-input
              v-model="form.password"
              type="password"
              :placeholder="t('auth.placeholder.password')"
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
            />
          </el-form-item>
          <el-form-item>
            <el-button
              type="primary"
              size="large"
              :loading="loading"
              class="auth-btn"
              @click="handleInit"
            >
              {{ t('auth.btn.init') }}
            </el-button>
          </el-form-item>
        </el-form>
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
</style>
