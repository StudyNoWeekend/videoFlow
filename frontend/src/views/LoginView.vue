<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { loginByPassword } from '@/api/auth'

const router = useRouter()
const { t } = useI18n()
const auth = useAuthStore()

const loading = ref(false)
const form = ref({ username: '', password: '' })

onMounted(async () => {
  await auth.loadStatus()
  if (!auth.isInitialized) {
    router.replace('/init')
    return
  }
})

async function handleLogin(): Promise<void> {
  if (!form.value.username || !form.value.password) {
    ElMessage.warning(t('auth.error.fill_fields'))
    return
  }
  loading.value = true
  try {
    const res = await loginByPassword(form.value)
    auth.login(res.token, res.user)
    ElMessage.success(t('auth.success.login'))
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
        <p class="auth-subtitle">{{ t('auth.login.subtitle') }}</p>
      </div>

      <div class="auth-card">
        <el-form @submit.prevent="handleLogin" class="auth-form">
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
              @keyup.enter="handleLogin"
            />
          </el-form-item>
          <el-form-item>
            <el-button
              type="primary"
              size="large"
              :loading="loading"
              class="auth-btn"
              @click="handleLogin"
            >
              {{ t('auth.btn.login') }}
            </el-button>
          </el-form-item>
        </el-form>

        <div class="auth-footer">
          <span class="auth-footer__text">{{ t('auth.forgot.hint') }}</span>
          <el-button link type="primary" @click="router.push('/reset-password')">
            {{ t('auth.forgot.link') }}
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

.auth-form {
  width: 100%;
}

.auth-btn {
  width: 100%;
}

.auth-footer {
  text-align: center;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--vf-border);
}

.auth-footer__text {
  font-size: 13px;
  color: var(--vf-text-muted);
  margin-right: 4px;
}
</style>
