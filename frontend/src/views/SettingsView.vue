<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { useSettingsStore } from '@/stores/settings'
import type { Setting } from '@/api/setting'

const settingsStore = useSettingsStore()

// 本地表单数据
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

/**
 * 保存统一配置
 */
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
  <div class="settings-page">
    <h2>设置</h2>

    <el-card v-loading="settingsStore.loading" class="setting-card">
      <template #header>
        <span>统一配置</span>
      </template>
      <el-form label-width="160px">
        <el-form-item label="本地视频目录">
          <el-input v-model="form.video_dir" placeholder="请输入本地视频目录绝对路径" />
        </el-form-item>
        <el-form-item label="扫描间隔（秒）">
          <el-input-number v-model="form.scan_interval" :min="1" :max="86400" />
        </el-form-item>

        <el-divider content-position="left">ASR 配置</el-divider>
        <el-form-item label="ASR 服务地址">
          <el-input v-model="form.asr_url" placeholder="例如 http://1.12.70.219:9999/asr" />
        </el-form-item>
        <el-form-item label="ASR 语言">
          <el-input v-model="form.asr_language" placeholder="例如 zh" />
        </el-form-item>
        <el-form-item label="ASR VAD 过滤">
          <el-checkbox v-model="form.asr_vad_filter">启用 VAD 过滤</el-checkbox>
        </el-form-item>
        <el-form-item label="ASR 任务">
          <el-radio-group v-model="form.asr_task">
            <el-radio-button label="transcribe">转录</el-radio-button>
            <el-radio-button label="translate">翻译为英文</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="音频预编码">
          <el-checkbox v-model="form.asr_encode">通过 FFmpeg 编码音频</el-checkbox>
        </el-form-item>
        <el-form-item label="初始提示词">
          <el-input v-model="form.asr_initial_prompt" type="textarea" :rows="2" placeholder="可选，用于引导识别风格或术语" />
        </el-form-item>
        <el-form-item label="词级时间戳">
          <el-checkbox v-model="form.asr_word_timestamps">启用词级时间戳</el-checkbox>
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

        <el-divider content-position="left">视频修复配置</el-divider>
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

        <el-divider content-position="left">并发配置</el-divider>
        <el-form-item label="字幕并发数">
          <el-slider v-model="form.subtitle_concurrency" :min="1" :max="50" show-input />
        </el-form-item>
        <el-form-item label="修复并发数">
          <el-slider v-model="form.repair_concurrency" :min="1" :max="50" show-input />
        </el-form-item>

        <el-form-item>
          <el-button type="primary" :loading="saving" @click="handleSave">保存配置</el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<style scoped>
.settings-page {
  padding: 20px;
}

.setting-card {
  margin-bottom: 20px;
}
</style>
