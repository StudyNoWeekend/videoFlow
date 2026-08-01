<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { useSettingsStore } from '@/stores/settings'
import type { Setting } from '@/api/setting'

const settingsStore = useSettingsStore()

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
  subtitle_concurrency: 2,
  repair_concurrency: 1,
})
const saving = ref<boolean>(false)

async function handleSave(): Promise<void> {
  saving.value = true
  try {
    await settingsStore.saveSettings(form.value)
    ElMessage.success('配置已保存')
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
          <span>系统参数配置</span>
        </div>
        <div class="header__hint">
          <span class="vf-data-label">所有参数保存后即时生效</span>
        </div>
      </div>

      <div class="panel-body">
        <el-form v-loading="settingsStore.loading" label-width="170px" class="config-form">
          <!-- 基础配置 -->
          <section class="config-section">
            <div class="section-marker">
              <span class="section-marker__line"></span>
              <span class="section-marker__label">基础参数</span>
              <span class="section-marker__line"></span>
            </div>
            <div class="section-grid section-grid--2">
              <el-form-item label="本地视频目录">
                <el-input v-model="form.video_dir" placeholder="请输入本地视频目录绝对路径" />
              </el-form-item>
              <el-form-item label="扫描间隔（秒）">
                <el-input-number v-model="form.scan_interval" :min="1" :max="86400" />
              </el-form-item>
            </div>
          </section>

          <!-- ASR 配置 -->
          <section class="config-section">
            <div class="section-marker">
              <span class="section-marker__line"></span>
              <span class="section-marker__label">ASR 配置</span>
              <span class="section-marker__line"></span>
            </div>
            <div class="section-grid section-grid--2">
              <el-form-item label="ASR 服务地址">
                <el-input v-model="form.asr_url" placeholder="例如 http://1.12.70.219:9999/asr" />
              </el-form-item>
              <el-form-item label="ASR 语言">
                <el-input v-model="form.asr_language" placeholder="例如 zh" />
              </el-form-item>
              <el-form-item label="ASR 任务">
                <el-radio-group v-model="form.asr_task">
                  <el-radio-button label="transcribe">转录</el-radio-button>
                  <el-radio-button label="translate">翻译为英文</el-radio-button>
                </el-radio-group>
              </el-form-item>
              <el-form-item label="输出格式">
                <el-select v-model="form.asr_output" style="width: 200px">
                  <el-option label="JSON" value="json" />
                  <el-option label="SRT" value="srt" />
                  <el-option label="VTT" value="vtt" />
                  <el-option label="TXT" value="txt" />
                  <el-option label="TSV" value="tsv" />
                </el-select>
              </el-form-item>
              <el-form-item label="初始提示词">
                <el-input v-model="form.asr_initial_prompt" type="textarea" :rows="2" placeholder="可选，用于引导识别风格或术语" />
              </el-form-item>
              <el-form-item label="选项开关">
                <div class="checkbox-group">
                  <el-checkbox v-model="form.asr_vad_filter">启用 VAD 过滤</el-checkbox>
                  <el-checkbox v-model="form.asr_encode">FFmpeg 音频编码</el-checkbox>
                  <el-checkbox v-model="form.asr_word_timestamps">词级时间戳</el-checkbox>
                </div>
              </el-form-item>
            </div>
          </section>

          <!-- 视频修复配置 -->
          <section class="config-section">
            <div class="section-marker">
              <span class="section-marker__line"></span>
              <span class="section-marker__label">视频修复配置</span>
              <span class="section-marker__line"></span>
            </div>
            <div class="section-grid section-grid--2">
              <el-form-item label="修复 Docker 镜像">
                <el-input v-model="form.repair_docker_image" placeholder="例如 ladaapp/lada:latest" />
              </el-form-item>
              <el-form-item label="修复设备">
                <el-radio-group v-model="form.repair_device">
                  <el-radio-button label="cpu">CPU</el-radio-button>
                  <el-radio-button label="cuda:0">CUDA</el-radio-button>
                  <el-radio-button label="mps">MPS</el-radio-button>
                  <el-radio-button label="xpu:0">Intel XPU</el-radio-button>
                </el-radio-group>
              </el-form-item>
            </div>
          </section>

          <!-- 并发配置 -->
          <section class="config-section">
            <div class="section-marker">
              <span class="section-marker__line"></span>
              <span class="section-marker__label">并发控制</span>
              <span class="section-marker__line"></span>
            </div>
            <div class="section-grid section-grid--2">
              <el-form-item label="字幕并发数">
                <el-slider v-model="form.subtitle_concurrency" :min="1" :max="50" show-input />
              </el-form-item>
              <el-form-item label="修复并发数">
                <el-slider v-model="form.repair_concurrency" :min="1" :max="50" show-input />
              </el-form-item>
            </div>
          </section>

          <el-form-item class="form-actions">
            <el-button type="primary" size="large" :loading="saving" @click="handleSave">
              <el-icon><Check /></el-icon>保存配置
            </el-button>
          </el-form-item>
        </el-form>
      </div>
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

.panel-body {
  padding: 20px 24px;
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

@media (max-width: 1200px) {
  .section-grid--2 {
    grid-template-columns: 1fr;
  }
}
</style>
