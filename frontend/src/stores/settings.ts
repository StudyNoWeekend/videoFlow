import { ref } from 'vue'
import { defineStore } from 'pinia'
import { getSettings, updateSettings, type Setting } from '@/api/setting'

// 默认设置值
const defaultSetting: Setting = {
  video_dir: '',
  scan_interval: 60,
  asr_url: 'http://1.12.70.219:9999/asr',
  asr_language: 'zh',
  asr_vad_filter: false,
  asr_task: 'transcribe',
  asr_encode: true,
  asr_initial_prompt: '',
  asr_word_timestamps: false,
  asr_output: 'json',
  repair_docker_image: 'ladaapp/lada:latest',
  repair_device: 'cpu',
  subtitle_concurrency: 2,
  subtitle_burn_concurrency: 1,
  repair_concurrency: 1,
  translate_concurrency: 1,
}

// 设置中心 Store，管理统一配置对象
export const useSettingsStore = defineStore('settings', () => {
  // 统一设置对象
  const setting = ref<Setting>({ ...defaultSetting })
  // 加载状态
  const loading = ref<boolean>(false)

  /**
   * 加载设置
   */
  async function loadSettings(): Promise<void> {
    const res = await getSettings()
    setting.value = { ...defaultSetting, ...res }
  }

  /**
   * 保存设置
   * @param value 设置对象
   */
  async function saveSettings(value: Setting): Promise<Setting> {
    const res = await updateSettings(value)
    setting.value = { ...defaultSetting, ...res }
    return res
  }

  /**
   * 初始化设置
   */
  async function init(): Promise<void> {
    loading.value = true
    try {
      await loadSettings()
    } finally {
      loading.value = false
    }
  }

  return {
    setting,
    loading,
    loadSettings,
    saveSettings,
    init,
  }
})
