<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useSettingsStore } from '@/stores/settings'
import type { Setting } from '@/api/setting'
import { getTelegramStatus, logoutTelegram, type TelegramStatus } from '@/api/telegram'
import ComponentManager from './ComponentManager.vue'
import TelegramLoginDialog from '@/components/TelegramLoginDialog.vue'

const { t } = useI18n()
const settingsStore = useSettingsStore()
const activeTab = ref('system')

const form = ref<Setting>({
  video_dir: '',
  output_dir: '',
  scan_interval: 60,
  asr_url: '',
  asr_language: 'zh',
  asr_vad_filter: false,
  asr_task: 'transcribe',
  asr_encode: true,
  asr_initial_prompt: '',
  asr_word_timestamps: false,
  asr_output: 'json',
  repair_docker_image: '',
  repair_device: 'cpu',
  upscale_docker_image: 'ghcr.io/k4yt3x/video2x:latest',
  upscale_device: 'cpu',
  upscale_concurrency: 1,
  subtitle_concurrency: 2,
  subtitle_burn_concurrency: 1,
  repair_concurrency: 1,
  scheduler_poll_interval: 2,
  telegram_app_id: '',
  telegram_app_hash: '',
  telegram_threads: 4,
  telegram_data_dir: 'data/telegram',
  proxy_url: '',
  proxy_for_ytdlp: true,
})
const saving = ref(false)

// Telegram 登录状态与登录弹窗
const tgStatus = ref<TelegramStatus>({
  configured: false,
  ready: false,
  authenticated: false,
  login_status: '',
})
const loginDialogVisible = ref(false)

async function loadTelegramStatus(): Promise<void> {
  try {
    tgStatus.value = await getTelegramStatus()
  } catch {
    // 错误提示已由请求拦截器统一处理
  }
}

async function handleLogout(): Promise<void> {
  try {
    await ElMessageBox.confirm(t('telegram.login.logout_confirm'), t('common.confirm'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      type: 'warning',
    })
  } catch {
    return
  }

  try {
    await logoutTelegram()
    ElMessage.success(t('telegram.login.logout_success'))
    await loadTelegramStatus()
  } catch {
    // 错误提示已由请求拦截器统一处理
  }
}

async function handleSave(): Promise<void> {
  saving.value = true
  try {
    await settingsStore.saveSettings(form.value)
    ElMessage.success(t('settings.save.success'))
    // 凭据或代理可能已变化，刷新登录状态
    await loadTelegramStatus()
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  await settingsStore.init()
  form.value = { ...settingsStore.setting }
  await loadTelegramStatus()
})
</script>

<template>
  <div class="settings-view">
    <div class="vf-panel">
      <div class="vf-panel__footer"></div>

      <div class="vf-panel-header">
        <div class="vf-panel-header__title">
          <span class="vf-led vf-led--green"></span>
          <span>{{ $t('nav.settings') }}</span>
        </div>
        <div class="header__hint">
          <span class="vf-data-label">{{ $t('settings.hint.immediate') }}</span>
        </div>
      </div>

      <el-tabs v-model="activeTab" class="settings-tabs">
        <!-- System Settings Tab -->
        <el-tab-pane :label="$t('settings.tab.system')" name="system">
          <div class="panel-body">
            <el-form v-loading="settingsStore.loading" label-width="170px" class="config-form">
              <!-- 基础配置 -->
              <section class="config-section">
                <div class="section-marker">
                  <span class="section-marker__line"></span>
                  <span class="section-marker__label">{{ $t('settings.section.basic') }}</span>
                  <span class="section-marker__line"></span>
                </div>
                <div class="section-grid section-grid--2">
                  <el-form-item :label="$t('settings.label.video_dir')">
                    <el-input v-model="form.video_dir" :placeholder="$t('settings.label.video_dir')" />
                  </el-form-item>
                  <el-form-item :label="$t('settings.label.output_dir')">
                    <el-input v-model="form.output_dir" :placeholder="$t('settings.label.output_dir')" />
                  </el-form-item>
                  <el-form-item :label="$t('settings.label.scan_interval')">
                    <el-input-number v-model="form.scan_interval" :min="1" :max="86400" />
                  </el-form-item>
                </div>
              </section>

              <!-- 出站代理 -->
              <section class="config-section">
                <div class="section-marker">
                  <span class="section-marker__line"></span>
                  <span class="section-marker__label">{{ $t('settings.section.proxy') }}</span>
                  <span class="section-marker__line"></span>
                </div>
                <div class="section-grid section-grid--2">
                  <el-form-item :label="$t('settings.label.proxy_url')">
                    <el-input v-model="form.proxy_url" placeholder="socks5://127.0.0.1:1080" />
                  </el-form-item>
                  <el-form-item :label="$t('settings.label.proxy_for_ytdlp')">
                    <el-switch v-model="form.proxy_for_ytdlp" />
                  </el-form-item>
                </div>
                <div class="vf-field-hint proxy-hint">{{ $t('settings.hint.proxy') }}</div>
              </section>

              <!-- ASR 配置 -->
              <section class="config-section">
                <div class="section-marker">
                  <span class="section-marker__line"></span>
                  <span class="section-marker__label">{{ $t('settings.section.asr') }}</span>
                  <span class="section-marker__line"></span>
                </div>
                <div class="section-grid section-grid--2">
                  <el-form-item :label="$t('settings.label.asr_url')">
                    <el-input v-model="form.asr_url" :placeholder="$t('settings.label.asr_url')" />
                  </el-form-item>
                  <el-form-item :label="$t('settings.label.asr_language')">
                    <el-input v-model="form.asr_language" placeholder="zh" />
                  </el-form-item>
                  <el-form-item :label="$t('settings.label.asr_task')">
                    <el-radio-group v-model="form.asr_task">
                      <el-radio-button label="transcribe">{{ $t('settings.asr_task.transcribe') }}</el-radio-button>
                      <el-radio-button label="translate">{{ $t('settings.asr_task.translate') }}</el-radio-button>
                    </el-radio-group>
                  </el-form-item>
                  <el-form-item :label="$t('settings.label.asr_output')">
                    <el-select v-model="form.asr_output" style="width:100%">
                      <el-option label="JSON" value="json" />
                      <el-option label="SRT" value="srt" />
                      <el-option label="VTT" value="vtt" />
                      <el-option label="TXT" value="txt" />
                      <el-option label="TSV" value="tsv" />
                    </el-select>
                  </el-form-item>
                  <el-form-item :label="$t('settings.label.asr_prompt')">
                    <el-input v-model="form.asr_initial_prompt" type="textarea" :rows="2" />
                  </el-form-item>
                  <el-form-item :label="$t('settings.label.asr_options')">
                    <div class="checkbox-group">
                      <el-checkbox v-model="form.asr_vad_filter">{{ $t('settings.asr_option.vad') }}</el-checkbox>
                      <el-checkbox v-model="form.asr_encode">{{ $t('settings.asr_option.encode') }}</el-checkbox>
                      <el-checkbox v-model="form.asr_word_timestamps">{{ $t('settings.asr_option.word_timestamps') }}</el-checkbox>
                    </div>
                  </el-form-item>
                </div>
              </section>

              <!-- 去马赛克配置 -->
              <section class="config-section">
                <div class="section-marker">
                  <span class="section-marker__line"></span>
                  <span class="section-marker__label">{{ $t('settings.section.repair') }}</span>
                  <span class="section-marker__line"></span>
                </div>
                <div class="section-grid section-grid--2">
                  <el-form-item :label="$t('settings.label.repair_image')">
                    <el-input v-model="form.repair_docker_image" placeholder="ladaapp/lada:latest" />
                  </el-form-item>
                  <el-form-item :label="$t('settings.label.repair_device')">
                    <el-radio-group v-model="form.repair_device">
                      <el-radio-button label="cpu">CPU</el-radio-button>
                      <el-radio-button label="cuda:0">CUDA</el-radio-button>
                      <el-radio-button label="mps">MPS</el-radio-button>
                      <el-radio-button label="xpu:0">Intel XPU</el-radio-button>
                    </el-radio-group>
                  </el-form-item>
                </div>
              </section>

              <!-- 清晰度修复配置 -->
              <section class="config-section">
                <div class="section-marker">
                  <span class="section-marker__line"></span>
                  <span class="section-marker__label">{{ $t('settings.section.upscale') }}</span>
                  <span class="section-marker__line"></span>
                </div>
                <div class="section-grid section-grid--2">
                  <el-form-item :label="$t('settings.label.upscale_image')">
                    <el-input v-model="form.upscale_docker_image" placeholder="ghcr.io/k4yt3x/video2x:latest" />
                    <div class="vf-field-hint">{{ $t('settings.hint.upscale_image') }}</div>
                  </el-form-item>
                  <el-form-item :label="$t('settings.label.upscale_device')">
                    <el-radio-group v-model="form.upscale_device">
                      <el-radio-button label="cpu">CPU</el-radio-button>
                      <el-radio-button label="cuda:0">CUDA</el-radio-button>
                    </el-radio-group>
                    <div class="vf-field-hint">{{ $t('settings.hint.upscale_device') }}</div>
                  </el-form-item>
                </div>
              </section>

              <!-- 并发控制 -->
              <section class="config-section">
                <div class="section-marker">
                  <span class="section-marker__line"></span>
                  <span class="section-marker__label">{{ $t('settings.section.concurrency') }}</span>
                  <span class="section-marker__line"></span>
                </div>
                <div class="section-grid section-grid--2">
                  <el-form-item :label="$t('settings.label.subtitle_concurrency')">
                    <el-slider v-model="form.subtitle_concurrency" :min="1" :max="50" show-input />
                  </el-form-item>
                  <el-form-item :label="$t('settings.label.subtitle_burn_concurrency')">
                    <el-slider v-model="form.subtitle_burn_concurrency" :min="1" :max="50" show-input />
                  </el-form-item>
                  <el-form-item :label="$t('settings.label.repair_concurrency')">
                    <el-slider v-model="form.repair_concurrency" :min="1" :max="50" show-input />
                  </el-form-item>
                  <el-form-item :label="$t('settings.label.upscale_concurrency')">
                    <el-slider v-model="form.upscale_concurrency" :min="1" :max="50" show-input />
                  </el-form-item>
                  <el-form-item :label="$t('settings.label.scheduler_poll_interval')">
                    <el-slider v-model="form.scheduler_poll_interval" :min="1" :max="3600" show-input />
                  </el-form-item>
                </div>
              </section>

              <!-- Telegram 下载配置 -->
              <section class="config-section">
                <div class="section-marker">
                  <span class="section-marker__line"></span>
                  <span class="section-marker__label">{{ $t('settings.section.telegram') }}</span>
                  <span class="section-marker__line"></span>
                </div>
                <div class="section-grid section-grid--2">
                  <el-form-item :label="$t('settings.label.telegram_app_id')">
                    <el-input v-model="form.telegram_app_id" placeholder="1234567" />
                  </el-form-item>
                  <el-form-item :label="$t('settings.label.telegram_app_hash')">
                    <el-input
                      v-model="form.telegram_app_hash"
                      type="password"
                      show-password
                      placeholder="0123456789abcdef0123456789abcdef"
                    />
                  </el-form-item>
                  <el-form-item :label="$t('settings.label.telegram_threads')">
                    <el-slider v-model="form.telegram_threads" :min="1" :max="16" show-input />
                  </el-form-item>
                  <el-form-item :label="$t('settings.label.telegram_data_dir')">
                    <el-input v-model="form.telegram_data_dir" placeholder="data/telegram" />
                  </el-form-item>
                </div>
                <div class="vf-field-hint telegram-hint">{{ $t('settings.hint.telegram') }}</div>

                <!-- 登录状态与入口 -->
                <div class="telegram-account">
                  <span
                    class="vf-led"
                    :class="tgStatus.authenticated ? 'vf-led--green' : 'vf-led--cyan'"
                  ></span>
                  <span class="vf-data-label">
                    {{
                      tgStatus.authenticated
                        ? $t('telegram.login.logged_in_as', { account: tgStatus.account })
                        : $t('telegram.login.not_logged_in')
                    }}
                  </span>
                  <el-button size="small" type="primary" plain @click="loginDialogVisible = true">
                    {{ tgStatus.authenticated ? $t('telegram.login.relogin') : $t('telegram.login.action') }}
                  </el-button>
                  <el-button
                    v-if="tgStatus.authenticated"
                    size="small"
                    type="danger"
                    plain
                    @click="handleLogout"
                  >
                    {{ $t('telegram.login.logout') }}
                  </el-button>
                </div>
              </section>

              <el-form-item class="form-actions">
                <el-button type="primary" size="large" :loading="saving" @click="handleSave">
                  <el-icon><Check /></el-icon>{{ $t('settings.save') }}
                </el-button>
              </el-form-item>
            </el-form>
          </div>
        </el-tab-pane>

        <!-- Component Management Tab -->
        <el-tab-pane :label="$t('settings.tab.components')" name="components">
          <div class="panel-body">
            <ComponentManager />
          </div>
        </el-tab-pane>
      </el-tabs>
    </div>

    <TelegramLoginDialog v-model="loginDialogVisible" @success="loadTelegramStatus" />
  </div>
</template>

<style scoped>
.settings-view {
  padding: var(--vf-padding-page);
  min-height: 100%;
}

.vf-panel {
  min-height: calc(100vh - 92px);
  display: flex;
  flex-direction: column;
}

@media (max-width: 767px) {
  .vf-panel {
    min-height: calc(100vh - 52px - 56px);
  }
}

.header__hint {
  display: flex;
  align-items: center;
  gap: 8px;
}

.settings-tabs {
  padding: 0 16px;
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.settings-tabs :deep(.el-tabs__content) {
  flex: 1;
  overflow: hidden;
}

.settings-tabs :deep(.el-tab-pane) {
  height: 100%;
  overflow-y: auto;
}

.panel-body {
  padding: 16px 0;
}

.config-form :deep(.el-form-item__label) {
  color: var(--vf-text-secondary);
  font-family: var(--vf-font-display);
  font-weight: 500;
  letter-spacing: 0.02em;
}

/* 手机端：标签放在输入框上方 */
@media (max-width: 767px) {
  .config-form :deep(.el-form-item) {
    display: flex;
    flex-direction: column;
    align-items: stretch;
  }
  .config-form :deep(.el-form-item__label) {
    padding-bottom: 4px;
    justify-content: flex-start;
  }
  .config-form :deep(.el-form-item__content) {
    flex: none;
    width: 100%;
    /* 抵消 Element Plus 对无 label 表单项注入的 margin-left: label-width，否则内容会溢出视口 */
    margin-left: 0 !important;
  }
  .config-form :deep(.el-radio-group) {
    flex-wrap: wrap;
  }
  .config-form :deep(.el-slider__runway) {
    width: calc(100% - 150px);
  }
}

.config-form {
  /* 宽屏下表单约束最大宽度并水平居中，窄屏自适应收缩 */
  max-width: 900px;
  margin: 0 auto;
}

.config-section {
  margin-bottom: 28px;
}

.section-marker {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 18px;
}

.section-marker__line {
  flex: 1;
  height: 1px;
  background: linear-gradient(90deg, var(--vf-border), var(--vf-border-light), transparent);
}

.section-marker__line:last-child {
  background: linear-gradient(90deg, transparent, var(--vf-border-light), var(--vf-border));
}

.section-marker__label {
  font-family: var(--vf-font-display);
  font-size: 12px;
  font-weight: 600;
  color: var(--vf-accent);
  letter-spacing: 0.08em;
  text-transform: uppercase;
  white-space: nowrap;
}

.section-grid {
  display: grid;
  gap: 16px 24px;
}

.section-grid--2 {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

/* 两列网格下防止内容溢出 */
.section-grid--2 :deep(.el-form-item) {
  min-width: 0;
}

.section-grid--2 :deep(.el-form-item__content) {
  min-width: 0;
}

.section-grid--2 :deep(.el-select),
.section-grid--2 :deep(.el-slider),
.section-grid--2 :deep(.el-input-number) {
  width: 100%;
}

.section-grid--2 :deep(.el-radio-group) {
  flex-wrap: wrap;
  gap: 2px;
}

.section-grid--2 :deep(.el-radio-button__inner) {
  font-size: 12px;
  padding: 6px 10px;
}

.section-grid--2 :deep(.el-slider__runway) {
  width: calc(100% - 150px);
}

.checkbox-group {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding-top: 6px;
}

.form-actions {
  margin-top: 8px;
  padding-top: 20px;
  border-top: 1px solid var(--vf-border);
}

.form-actions :deep(.el-form-item__content) {
	justify-content: flex-end;
}

/* 手机端：保存按钮通栏居中，保证始终可见 */
@media (max-width: 767px) {
  .form-actions :deep(.el-form-item__content) {
    justify-content: center;
  }
  .form-actions :deep(.el-button) {
    width: 100%;
  }
}

.vf-field-hint {
  font-size: 11px;
  color: var(--vf-text-muted);
  line-height: 1.4;
  margin-top: 4px;
}

.telegram-hint {
  max-width: 720px;
  margin-bottom: 12px;
  line-height: 1.6;
}

.proxy-hint {
  max-width: 720px;
  line-height: 1.6;
}

.telegram-account {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  padding: 10px 12px;
  border: 1px solid var(--vf-border-light);
  border-radius: var(--vf-radius-sm);
  background: var(--vf-bg-elevated);
}

.option-with-desc {
  display: flex;
  flex-direction: column;
  line-height: 1.3;
}

.option-desc {
  font-size: 11px;
  color: var(--vf-text-muted);
  white-space: normal;
}

@media (max-width: 1200px) {
  .section-grid--2 {
    grid-template-columns: 1fr;
  }
}
</style>
