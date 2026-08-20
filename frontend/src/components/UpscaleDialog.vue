<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatFileSize } from '@/utils/format'

const { t } = useI18n()

const props = withDefaults(
  defineProps<{
    modelValue: boolean
    videoName: string
    files: Array<{
      name: string
      path: string
      width: number
      height: number
      size: number
      fileType: string
    }>
  }>(),
  {
    files: () => [],
  },
)

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'confirm', payload: { sourcePath: string; targetWidth: number; targetHeight: number; processor: string; model: string; noiseLevel: number }): void
}>()

interface TargetResolution {
  label: string
  width: number
  height: number
}

// 处理器/模型/降噪等级仅在创建清晰修复任务时选择，不再读取系统配置
const selectedSourcePath = ref<string>('')
const selectedTargetResolution = ref<TargetResolution | null>(null)
const selectedProcessor = ref<string>('realesrgan')
const selectedModel = ref<string>('realesr-animevideov3')
const selectedNoiseLevel = ref<number>(-1)

// 可用模型列表
const availableModels = computed(() => {
  const processor = selectedProcessor.value
  if (processor === 'realesrgan') {
    return [
      { label: 'realesr-animevideov3（动漫推荐，支持 x2/x3/x4）', value: 'realesr-animevideov3' },
      { label: 'realesrgan-plus-anime（动漫高质量，仅 x4）', value: 'realesrgan-plus-anime' },
      { label: 'realesrgan-plus（真人/自然场景，仅 x4）', value: 'realesrgan-plus' },
      { label: 'realesr-generalv3（通用，仅 x4，支持降噪）', value: 'realesr-generalv3' },
    ]
  } else if (processor === 'realcugan') {
    return [
      { label: 'models-se（高质量，含 SE 模块，x2/x3/x4）', value: 'models-se' },
      { label: 'models-pro（专业版，x2/x3）', value: 'models-pro' },
      { label: 'models-nose（轻量快速，仅 x2）', value: 'models-nose' },
    ]
  } else {
    return [
      { label: 'anime4k-v4-a（亮度放大）', value: 'anime4k-v4-a' },
      { label: 'anime4k-v4-a+a（亮度放大 + 抗锯齿）', value: 'anime4k-v4-a+a' },
      { label: 'anime4k-v4-b（色彩放大）', value: 'anime4k-v4-b' },
      { label: 'anime4k-v4-b+b（色彩放大 + 抗锯齿）', value: 'anime4k-v4-b+b' },
      { label: 'anime4k-v4-c（线条增强）', value: 'anime4k-v4-c' },
      { label: 'anime4k-v4-c+a（线条增强 + 抗锯齿）', value: 'anime4k-v4-c+a' },
      { label: 'anime4k-v4.1-gan（GAN 神经着色器，质量最高）', value: 'anime4k-v4.1-gan' },
    ]
  }
})

// 切换处理器时重置模型为默认选项
watch(selectedProcessor, (proc) => {
  const models = getModelList(proc)
  if (models.length > 0) {
    selectedModel.value = models[0].value
  }
})

function getModelList(processor: string): { label: string; value: string }[] {
  if (processor === 'realesrgan') {
    return [
      { label: 'realesr-animevideov3（动漫推荐，支持 x2/x3/x4）', value: 'realesr-animevideov3' },
      { label: 'realesrgan-plus-anime（动漫高质量，仅 x4）', value: 'realesrgan-plus-anime' },
      { label: 'realesrgan-plus（真人/自然场景，仅 x4）', value: 'realesrgan-plus' },
      { label: 'realesr-generalv3（通用，仅 x4，支持降噪）', value: 'realesr-generalv3' },
    ]
  } else if (processor === 'realcugan') {
    return [
      { label: 'models-se（高质量，含 SE 模块，x2/x3/x4）', value: 'models-se' },
      { label: 'models-pro（专业版，x2/x3）', value: 'models-pro' },
      { label: 'models-nose（轻量快速，仅 x2）', value: 'models-nose' },
    ]
  }
  return [
    { label: 'anime4k-v4-a（亮度放大）', value: 'anime4k-v4-a' },
    { label: 'anime4k-v4-a+a（亮度放大 + 抗锯齿）', value: 'anime4k-v4-a+a' },
    { label: 'anime4k-v4-b（色彩放大）', value: 'anime4k-v4-b' },
    { label: 'anime4k-v4-b+b（色彩放大 + 抗锯齿）', value: 'anime4k-v4-b+b' },
    { label: 'anime4k-v4-c（线条增强）', value: 'anime4k-v4-c' },
    { label: 'anime4k-v4-c+a（线条增强 + 抗锯齿）', value: 'anime4k-v4-c+a' },
    { label: 'anime4k-v4.1-gan（GAN 神经着色器，质量最高）', value: 'anime4k-v4.1-gan' },
  ]
}

// 每次打开弹窗时重置选中项
watch(
  () => props.modelValue,
  (visible) => {
    if (visible) {
      selectedSourcePath.value = ''
      selectedTargetResolution.value = null
      selectedProcessor.value = 'realesrgan'
      selectedModel.value = 'realesr-animevideov3'
      selectedNoiseLevel.value = -1
    }
  },
)

const visible = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value),
})

// 当前选中源文件对象
const selectedSourceFile = computed(() => {
  if (!selectedSourcePath.value) return null
  return props.files.find((f) => f.path === selectedSourcePath.value) || null
})

// 各超分模型支持的放大倍数（Real-ESRGAN / Real-CUGAN 的 NCNN 模型按倍数分文件，
// video2x 镜像中只内置了部分倍数的模型，选择不支持的倍数会在执行时直接失败）
const MODEL_FACTORS: Record<string, number[]> = {
  'realesr-animevideov3': [2, 3, 4],
  'realesrgan-plus-anime': [4],
  'realesrgan-plus': [4],
  'realesr-generalv3': [4],
  'models-se': [2, 3, 4],
  'models-pro': [2, 3],
  'models-nose': [2],
}

// 可用的目标分辨率列表（使用弹窗中选中的处理器/模型）
const availableTargets = computed<TargetResolution[]>(() => {
  const file = selectedSourceFile.value
  if (!file) return []
  return resolveTargetResolutions(file.width, file.height, selectedProcessor.value, selectedModel.value)
})

// 切换源文件时重置目标分辨率
watch(selectedSourcePath, () => {
  selectedTargetResolution.value = null
})

/**
 * 根据源文件分辨率、处理器和模型计算可用的目标分辨率选项。
 * ML 处理器按放大倍数推导，且只列出所选模型支持的倍数。
 */
function resolveTargetResolutions(
  sourceWidth: number,
  sourceHeight: number,
  processor: string,
  model: string,
): TargetResolution[] {
  const targets = [
    { label: '720p', height: 720 },
    { label: '1080p', height: 1080 },
    { label: '2K', height: 1440 },
    { label: '4K', height: 2160 },
  ]
  const result: TargetResolution[] = []

  if (processor === 'realesrgan' || processor === 'realcugan') {
    // ML 处理器：基于放大因子，且仅列出所选模型支持的倍数
    const factors = MODEL_FACTORS[model] ?? [2, 3, 4]
    for (const target of targets) {
      if (target.height <= sourceHeight) continue
      for (const factor of factors) {
        if (sourceHeight * factor >= target.height) {
          const w = sourceWidth * factor
          const h = sourceHeight * factor
          result.push({
            label: `${target.label} (×${factor}, 输出 ${w}×${h})`,
            width: w,
            height: h,
          })
          break
        }
      }
    }
  } else {
    // libplacebo：任意缩放
    for (const target of targets) {
      if (target.height <= sourceHeight) continue
      const ratio = target.height / sourceHeight
      const w = Math.round(sourceWidth * ratio)
      const h = target.height
      result.push({
        label: `${target.label} (输出 ${w}×${h})`,
        width: w,
        height: h,
      })
    }
  }

  return result
}

function handleConfirm(): void {
  const target = selectedTargetResolution.value
  if (!selectedSourcePath.value || !target) return
  emit('confirm', {
    sourcePath: selectedSourcePath.value,
    targetWidth: target.width,
    targetHeight: target.height,
    processor: selectedProcessor.value,
    model: selectedModel.value,
    noiseLevel: selectedNoiseLevel.value,
  })
  visible.value = false
}

function getFileTypeDisplay(fileType: string): string {
  const map: Record<string, string> = {
    subtitle: t('videos.file.subtitle'),
    subtitled_video: t('videos.file.subtitled_video'),
    repaired_video: t('videos.file.repaired_video'),
    upscaled_video: t('videos.file.upscaled_video'),
    original: t('videos.source.original_video'),
  }
  return map[fileType] || fileType
}

function getFileTypeTag(fileType: string): string {
  const map: Record<string, string> = {
    subtitle: 'primary',
    subtitled_video: 'warning',
    source: 'info',
  }
  return map[fileType] || 'info'
}
</script>

<template>
  <el-dialog
    v-model="visible"
    :title="$t('videos.dialog.upscale_title')"
    width="640px"
    :close-on-click-modal="false"
    destroy-on-close
  >
    <div class="upscale-dialog">
      <!-- 处理源 -->
      <div class="upscale-dialog__section">
        <div class="upscale-dialog__section-title">{{ $t('videos.upscale.source_label') }}</div>
        <el-radio-group v-model="selectedSourcePath" class="upscale-dialog__group">
          <el-radio
            v-for="file in files"
            :key="file.path"
            class="upscale-dialog__item"
            :value="file.path"
            border
          >
            <div class="upscale-dialog__info">
              <span class="upscale-dialog__name" :title="file.name">{{ file.name }}</span>
              <span class="upscale-dialog__meta">
                <span class="upscale-dialog__resolution">{{ file.width }}×{{ file.height }}</span>
                <el-tag :type="getFileTypeTag(file.fileType)" size="small">{{ getFileTypeDisplay(file.fileType) }}</el-tag>
                <span class="upscale-dialog__size">{{ formatFileSize(file.size) }}</span>
              </span>
            </div>
          </el-radio>
        </el-radio-group>
      </div>

      <!-- 目标清晰度 -->
      <div v-if="selectedSourceFile" class="upscale-dialog__section">
        <div class="upscale-dialog__section-title">{{ $t('videos.upscale.target_label') }}</div>
        <el-select
          v-if="availableTargets.length > 0"
          v-model="selectedTargetResolution"
          :placeholder="$t('videos.upscale.select_source')"
          value-key="label"
          class="upscale-dialog__select"
        >
          <el-option
            v-for="t in availableTargets"
            :key="t.label"
            :label="t.label"
            :value="t"
          />
        </el-select>
        <div v-if="availableTargets.length === 0" class="upscale-dialog__empty-tip">
          {{ $t('videos.upscale.no_target') }}
        </div>
      </div>

      <!-- 清晰度修复处理器 -->
      <div class="upscale-dialog__section">
        <div class="upscale-dialog__section-title">{{ $t('videos.upscale.processor_label') }}</div>
        <el-select v-model="selectedProcessor" class="upscale-dialog__select">
          <el-option label="Real-ESRGAN" value="realesrgan">
            <div class="option-with-desc">
              <div>Real-ESRGAN</div>
              <div class="option-desc">{{ $t('settings.upscale_processor.realesrgan') }}</div>
            </div>
          </el-option>
          <el-option label="Real-CUGAN" value="realcugan">
            <div class="option-with-desc">
              <div>Real-CUGAN</div>
              <div class="option-desc">{{ $t('settings.upscale_processor.realcugan') }}</div>
            </div>
          </el-option>
          <el-option label="Libplacebo (Anime4K)" value="libplacebo">
            <div class="option-with-desc">
              <div>Libplacebo (Anime4K)</div>
              <div class="option-desc">{{ $t('settings.upscale_processor.libplacebo') }}</div>
            </div>
          </el-option>
        </el-select>
      </div>

      <!-- 清晰度修复模型/着色器 -->
      <div class="upscale-dialog__section">
        <div class="upscale-dialog__section-title">{{ $t('videos.upscale.model_label') }}</div>
        <el-select v-model="selectedModel" class="upscale-dialog__select">
          <!-- Real-ESRGAN 模型 -->
          <template v-if="selectedProcessor === 'realesrgan'">
            <el-option label="realesr-animevideov3" value="realesr-animevideov3">
              <div class="option-with-desc">
                <div>realesr-animevideov3</div>
                <div class="option-desc">动漫推荐，支持 x2/x3/x4 倍，轻量高效</div>
              </div>
            </el-option>
            <el-option label="realesrgan-plus-anime" value="realesrgan-plus-anime">
              <div class="option-with-desc">
                <div>realesrgan-plus-anime</div>
                <div class="option-desc">动漫高质量，仅 x4 倍，效果更好但速度较慢</div>
              </div>
            </el-option>
            <el-option label="realesrgan-plus" value="realesrgan-plus">
              <div class="option-with-desc">
                <div>realesrgan-plus</div>
                <div class="option-desc">真人/自然场景优化，仅 x4 倍</div>
              </div>
            </el-option>
            <el-option label="realesr-generalv3" value="realesr-generalv3">
              <div class="option-with-desc">
                <div>realesr-generalv3</div>
                <div class="option-desc">通用模型，仅 x4 倍，配合降噪等级使用</div>
              </div>
            </el-option>
          </template>
          <!-- Real-CUGAN 模型集 -->
          <template v-if="selectedProcessor === 'realcugan'">
            <el-option label="models-se" value="models-se">
              <div class="option-with-desc">
                <div>models-se</div>
                <div class="option-desc">高质量，含 SE 模块，支持 x2/x3/x4，动漫最佳</div>
              </div>
            </el-option>
            <el-option label="models-pro" value="models-pro">
              <div class="option-with-desc">
                <div>models-pro</div>
                <div class="option-desc">专业版，支持 x2/x3，品质与性能平衡</div>
              </div>
            </el-option>
            <el-option label="models-nose" value="models-nose">
              <div class="option-with-desc">
                <div>models-nose</div>
                <div class="option-desc">轻量快速版，仅 x2，无 SE 模块</div>
              </div>
            </el-option>
          </template>
          <!-- Libplacebo 着色器 -->
          <template v-if="selectedProcessor === 'libplacebo'">
            <el-option label="anime4k-v4-a" value="anime4k-v4-a">
              <div class="option-with-desc">
                <div>anime4k-v4-a</div>
                <div class="option-desc">亮度通道放大，基础清晰度修复着色器（默认）</div>
              </div>
            </el-option>
            <el-option label="anime4k-v4-a+a" value="anime4k-v4-a+a">
              <div class="option-with-desc">
                <div>anime4k-v4-a+a</div>
                <div class="option-desc">亮度放大 + 抗锯齿，画面更平滑</div>
              </div>
            </el-option>
            <el-option label="anime4k-v4-b" value="anime4k-v4-b">
              <div class="option-with-desc">
                <div>anime4k-v4-b</div>
                <div class="option-desc">色彩通道放大，改善颜色分辨率</div>
              </div>
            </el-option>
            <el-option label="anime4k-v4-b+b" value="anime4k-v4-b+b">
              <div class="option-with-desc">
                <div>anime4k-v4-b+b</div>
                <div class="option-desc">色彩放大 + 抗锯齿</div>
              </div>
            </el-option>
            <el-option label="anime4k-v4-c" value="anime4k-v4-c">
              <div class="option-with-desc">
                <div>anime4k-v4-c</div>
                <div class="option-desc">线条增强/暗化，提升画面锐度</div>
              </div>
            </el-option>
            <el-option label="anime4k-v4-c+a" value="anime4k-v4-c+a">
              <div class="option-with-desc">
                <div>anime4k-v4-c+a</div>
                <div class="option-desc">线条增强 + 抗锯齿</div>
              </div>
            </el-option>
            <el-option label="anime4k-v4.1-gan" value="anime4k-v4.1-gan">
              <div class="option-with-desc">
                <div>anime4k-v4.1-gan</div>
                <div class="option-desc">GAN 神经网络着色器，质量最高但性能开销最大</div>
              </div>
            </el-option>
          </template>
        </el-select>
      </div>

      <!-- 降噪等级（仅 ML 处理器） -->
      <div v-if="selectedProcessor === 'realesrgan' || selectedProcessor === 'realcugan'" class="upscale-dialog__section">
        <div class="upscale-dialog__section-title">{{ $t('videos.upscale.noise_label') }}</div>
        <el-select v-model="selectedNoiseLevel" class="upscale-dialog__select">
          <!-- Real-ESRGAN 噪声选项 -->
          <template v-if="selectedProcessor === 'realesrgan'">
            <el-option label="无降噪（默认）" :value="-1" />
            <el-option label="轻度降噪" :value="0" />
            <el-option label="最大降噪（使用 generalv3-wdn 模型）" :value="1" />
          </template>
          <!-- Real-CUGAN 噪声选项 -->
          <template v-if="selectedProcessor === 'realcugan'">
            <el-option label="保守模式（最大细节保留）" :value="-1" />
            <el-option label="无降噪" :value="0" />
            <el-option label="轻度降噪" :value="1" />
            <el-option label="中度降噪" :value="2" />
            <el-option label="强力降噪" :value="3" />
          </template>
        </el-select>
      </div>
    </div>
    <template #footer>
      <el-button @click="visible = false">{{ $t('common.cancel') }}</el-button>
      <el-button
        type="primary"
        :disabled="!selectedSourcePath || !selectedTargetResolution"
        @click="handleConfirm"
      >
        {{ $t('common.confirm') }}
      </el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.upscale-dialog__section {
  margin-bottom: 20px;
}

.upscale-dialog__section-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--vf-text-primary);
  margin-bottom: 10px;
}

.upscale-dialog__group {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}

.upscale-dialog__item {
  height: auto;
  width: 100%;
  margin-right: 0;
  padding: 10px 12px;
  display: flex;
  align-items: center;
}

.upscale-dialog__info {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  flex: 1;
}

.upscale-dialog__name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: var(--vf-font-display);
  font-size: 13px;
}

.upscale-dialog__meta {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.upscale-dialog__resolution {
  font-family: var(--vf-font-mono);
  font-size: 11px;
  color: var(--vf-text-secondary);
}

.upscale-dialog__size {
  font-family: var(--vf-font-mono);
  font-size: 11px;
  color: var(--vf-text-muted);
}

.upscale-dialog__select {
  width: 100%;
}

.upscale-dialog__empty-tip {
  font-size: 12px;
  color: var(--vf-text-muted);
  margin-top: 8px;
}

/* 修复 el-radio border 模式下白色文字看不清的问题 */
.upscale-dialog :deep(.el-radio.is-bordered) {
  background: var(--vf-bg-elevated);
  border-color: var(--vf-border);
  color: var(--vf-text-primary);
  min-height: 44px;
}

.upscale-dialog :deep(.el-radio.is-bordered.is-checked) {
  border-color: var(--vf-accent);
  background: var(--vf-bg-elevated);
}

.upscale-dialog :deep(.el-radio.is-bordered .el-radio__label) {
  color: var(--vf-text-primary);
  white-space: normal;
  width: 100%;
}

.upscale-dialog :deep(.el-radio.is-bordered .el-radio__input) {
  flex-shrink: 0;
}

/* 处理器/模型选项的描述样式 */
:deep(.option-with-desc) {
  display: flex;
  flex-direction: column;
  line-height: 1.4;
  padding: 2px 0;
}

:deep(.option-desc) {
  font-size: 11px;
  color: var(--vf-text-muted);
  white-space: normal;
}
</style>
