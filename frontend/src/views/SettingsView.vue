<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useSettingsStore } from '@/stores/settings'
import type { Setting } from '@/api/setting'
import ComponentManager from './ComponentManager.vue'

const { t } = useI18n()
const settingsStore = useSettingsStore()
const activeTab = ref('system')

const form = ref<Setting>({
  video_dir: '',
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
})
const saving = ref(false)

async function handleSave(): Promise<void> {
  saving.value = true
  try {
    await settingsStore.saveSettings(form.value)
    ElMessage.success(t('settings.save.success'))
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  await settingsStore.init()
  form.value = { ...settingsStore.setting }
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
                  <el-form-item :label="$t('settings.label.scan_interval')">
                    <el-input-number v-model="form.scan_interval" :min="1" :max="86400" />
                  </el-form-item>
                </div>
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
                    <el-select v-model="form.asr_output" style="width: 200px">
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
  </div>
</template>

<style scoped>
.settings-view {
  padding: 20px;
  min-height: 100%;
}

.vf-panel {
  min-height: calc(100vh - 92px);
  display: flex;
  flex-direction: column;
}

.header__hint {
  display: flex;
  align-items: center;
  gap: 8px;
}

.settings-tabs {
  padding: 0 16px;
}

.panel-body {
  padding: 16px 0;
  flex: 1;
}

.config-form :deep(.el-form-item__label) {
  color: var(--vf-text-secondary);
  font-family: var(--vf-font-display);
  font-weight: 500;
  letter-spacing: 0.02em;
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

.vf-field-hint {
  font-size: 11px;
  color: var(--vf-text-muted);
  line-height: 1.4;
  margin-top: 4px;
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
