<script setup lang="ts">
import { ref, watch, onUnmounted, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import {
  getTelegramStatus,
  startTelegramQRLogin,
  getTelegramQRCode,
  submitTelegram2FA,
  type TelegramStatus,
} from '@/api/telegram'

const visible = defineModel<boolean>({ required: true })

const emit = defineEmits<{
  // 登录成功，父组件可据此继续未完成的下载
  (e: 'success'): void
}>()

const { t } = useI18n()

// 轮询间隔：登录态变化较快，与 tdl-filegram 保持一致的 1.5s
const POLL_INTERVAL = 1500

const status = ref<TelegramStatus>({
  configured: false,
  ready: false,
  authenticated: false,
  login_status: '',
})
const qrImage = ref<string>('')
const password = ref<string>('')
const submitting = ref<boolean>(false)

let pollTimer: number | null = null
// 记录已渲染的二维码内容，避免轮询时重复请求图片
let renderedQRURL = ''

const needTwoFA = computed<boolean>(() => status.value.login_status === 'need_2fa')

// 刷新二维码图片：仅在二维码内容变化时请求，避免无谓的图片传输
async function refreshQRImage(): Promise<void> {
  const qrURL = status.value.qr_url || ''
  if (!qrURL || qrURL === renderedQRURL) {
    return
  }
  try {
    const qr = await getTelegramQRCode()
    qrImage.value = qr.qr_image
    renderedQRURL = qr.qr_url
  } catch {
    // 二维码尚未就绪或已过期，等待下次轮询
  }
}

async function loadStatus(): Promise<void> {
  try {
    status.value = await getTelegramStatus()
  } catch {
    return
  }

  if (status.value.authenticated) {
    stopPolling()
    emit('success')
    visible.value = false
    return
  }
  await refreshQRImage()
}

function startPolling(): void {
  if (pollTimer !== null) {
    return
  }
  pollTimer = window.setInterval(loadStatus, POLL_INTERVAL)
}

function stopPolling(): void {
  if (pollTimer !== null) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

// 打开弹窗时查询状态；未登录则发起扫码登录
async function init(): Promise<void> {
  qrImage.value = ''
  renderedQRURL = ''
  password.value = ''

  await loadStatus()
  if (!visible.value || status.value.authenticated) {
    return
  }
  if (!status.value.configured) {
    return
  }

  try {
    status.value = await startTelegramQRLogin()
  } catch {
    // 错误提示已由请求拦截器统一处理
  }
  startPolling()
}

async function handleSubmit2FA(): Promise<void> {
  if (!password.value) {
    ElMessage.warning(t('telegram.login.password_required'))
    return
  }

  submitting.value = true
  try {
    await submitTelegram2FA(password.value)
    password.value = ''
    await loadStatus()
  } catch {
    // 错误提示已由请求拦截器统一处理
  } finally {
    submitting.value = false
  }
}

watch(visible, (opened) => {
  if (opened) {
    init()
    return
  }
  stopPolling()
})

onUnmounted(stopPolling)
</script>

<template>
  <el-dialog
    v-model="visible"
    :title="t('telegram.login.title')"
    width="380px"
    align-center
    class="tg-login-dialog"
    @closed="stopPolling"
  >
    <!-- 未配置应用凭据：引导到设置页填写 -->
    <div v-if="!status.configured" class="tg-login__notice">
      <el-alert type="warning" :closable="false" show-icon :title="t('telegram.login.not_configured')" />
      <p class="tg-login__hint">{{ t('telegram.login.not_configured_hint') }}</p>
    </div>

    <template v-else>
      <p class="tg-login__hint">{{ t('telegram.login.hint') }}</p>

      <div class="tg-login__qr">
        <img v-if="qrImage" :src="qrImage" :alt="t('telegram.login.title')" />
        <div v-else class="tg-login__qr-placeholder">
          <el-icon class="is-loading" size="24"><Loading /></el-icon>
        </div>
      </div>

      <p class="tg-login__status">
        <span class="vf-led vf-led--amber vf-led--pulse"></span>
        {{ status.ready ? t('telegram.login.waiting_scan') : t('telegram.login.connecting') }}
      </p>

      <!-- 账号开启了两步验证，需要补一次密码 -->
      <div v-if="needTwoFA" class="tg-login__2fa">
        <el-input
          v-model="password"
          type="password"
          show-password
          :placeholder="t('telegram.login.password_placeholder')"
          @keydown.enter="handleSubmit2FA"
        />
        <el-button type="primary" :loading="submitting" @click="handleSubmit2FA">
          {{ t('telegram.login.submit_2fa') }}
        </el-button>
      </div>

      <p v-if="status.error" class="tg-login__error">{{ status.error }}</p>
    </template>

    <template #footer>
      <el-button @click="visible = false">{{ t('common.close') }}</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.tg-login__hint {
  margin: 0 0 16px;
  font-family: var(--vf-font-ui);
  font-size: 13px;
  line-height: 1.6;
  color: var(--vf-text-secondary);
  text-align: center;
}

.tg-login__notice {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.tg-login__qr {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 220px;
  height: 220px;
  margin: 0 auto 12px;
  padding: 8px;
  background: #fff;
  border: 1px solid var(--vf-border);
  border-radius: var(--vf-radius);
}

.tg-login__qr img {
  width: 100%;
  height: 100%;
  image-rendering: pixelated;
}

.tg-login__qr-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--vf-text-muted);
}

.tg-login__status {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  margin: 0 0 12px;
  font-family: var(--vf-font-ui);
  font-size: 12px;
  color: var(--vf-text-muted);
}

.tg-login__2fa {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-top: 8px;
}

.tg-login__error {
  margin: 12px 0 0;
  font-family: var(--vf-font-ui);
  font-size: 12px;
  line-height: 1.6;
  color: var(--vf-danger, #f56c6c);
  text-align: center;
  word-break: break-all;
}
</style>
