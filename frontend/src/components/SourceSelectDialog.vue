<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { formatFileSize } from '@/utils/format'

// 可选择的处理源（同名衍生视频），供任务类型复用时构建选项
export interface SourceSelectOption {
  name: string
  path: string
  size: number
  /** 文件类型 i18n key，如 videos.file.subtitled_video */
  labelKey: string
  /** el-tag 类型：primary/success/warning/danger/info */
  tag: string
}

const props = withDefaults(
  defineProps<{
    modelValue: boolean
    title: string
    /** 原视频文件名 */
    videoName: string
    /** 可选的处理源（同名衍生视频），为空时不弹窗直接走原视频 */
    options: SourceSelectOption[]
    /** 提示文字 i18n key，默认为去马赛克专用提示 */
    tipKey?: string
  }>(),
  {
    options: () => [],
    tipKey: 'videos.source.tip',
  },
)

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  /** 确认选择，path 为空串表示使用原视频；overwrite 表示是否覆盖所选的衍生视频 */
  (e: 'confirm', payload: { path: string; overwrite: boolean }): void
}>()

const selectedPath = ref<string>('')
const overwriteSelected = ref<boolean>(false)

// 每次打开弹窗时重置选中项为原视频
watch(
  () => props.modelValue,
  (visible) => {
    if (visible) {
      selectedPath.value = ''
      overwriteSelected.value = false
    }
  },
)

// 是否选择了衍生视频（非原视频），只有此时才提供“覆盖所选视频”勾选
const derivedSelected = computed(() => selectedPath.value !== '')

const selectedOption = computed<SourceSelectOption | null>(
  () => props.options.find((o) => o.path === selectedPath.value) || null,
)

const visible = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value),
})

function handleConfirm(): void {
  emit('confirm', { path: selectedPath.value, overwrite: overwriteSelected.value })
  visible.value = false
}
</script>

<template>
  <el-dialog
    v-model="visible"
    :title="title"
    width="640px"
    :close-on-click-modal="false"
    destroy-on-close
  >
    <div class="source-select">
      <div class="source-select__tip">{{ $t(props.tipKey) }}</div>
      <el-radio-group v-model="selectedPath" class="source-select__group">
        <!-- 原视频（默认） -->
        <el-radio class="source-select__item" value="" border>
          <div class="source-select__info">
            <span class="source-select__name" :title="videoName">{{ videoName }}</span>
            <el-tag type="primary" size="small">{{ $t('videos.source.original_video') }}</el-tag>
          </div>
        </el-radio>
        <!-- 同名衍生视频 -->
        <el-radio
          v-for="opt in options"
          :key="opt.path"
          class="source-select__item"
          :value="opt.path"
          border
        >
          <div class="source-select__info">
            <span class="source-select__name" :title="opt.name">{{ opt.name }}</span>
            <span class="source-select__meta">
              <el-tag :type="opt.tag" size="small">{{ $t(opt.labelKey) }}</el-tag>
              <span class="source-select__size">{{ formatFileSize(opt.size) }}</span>
            </span>
          </div>
        </el-radio>
      </el-radio-group>

      <!-- 覆盖所选视频：仅在选择了衍生视频（非原视频）时提供勾选 -->
      <div v-if="derivedSelected" class="source-select__overwrite">
        <el-checkbox v-model="overwriteSelected" class="source-select__overwrite-checkbox">
          {{ $t('videos.source.overwrite_label', { name: selectedOption?.name || '' }) }}
        </el-checkbox>
        <div class="source-select__overwrite-tip">{{ $t('videos.source.overwrite_tip') }}</div>
      </div>
    </div>
    <template #footer>
      <el-button @click="visible = false">{{ $t('common.cancel') }}</el-button>
      <el-button type="primary" @click="handleConfirm">{{ $t('common.confirm') }}</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.source-select__tip {
  font-size: 12px;
  color: var(--vf-text-muted);
  margin-bottom: 12px;
}

.source-select__group {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}

.source-select__item {
  height: auto;
  width: 100%;
  margin-right: 0;
  padding: 10px 12px;
  display: flex;
  align-items: center;
}

.source-select__info {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  flex: 1;
}

.source-select__name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: var(--vf-font-display);
  font-size: 13px;
}

.source-select__meta {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.source-select__size {
  font-family: var(--vf-font-mono);
  font-size: 11px;
  color: var(--vf-text-muted);
}

.source-select__overwrite {
  margin-top: 14px;
  padding: 10px 12px;
  border: 1px dashed var(--vf-border);
  border-radius: var(--vf-radius);
  background: var(--vf-bg-elevated);
}

.source-select__overwrite-checkbox {
  --el-checkbox-text-color: var(--vf-text-primary);
}

.source-select__overwrite-tip {
  margin-top: 4px;
  margin-left: 24px;
  font-size: 11px;
  color: var(--vf-text-muted);
  line-height: 1.5;
}
</style>
