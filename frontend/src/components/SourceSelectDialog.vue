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
  }>(),
  {
    options: () => [],
  },
)

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  /** 确认选择，path 为空串表示使用原视频 */
  (e: 'confirm', path: string): void
}>()

const selectedPath = ref<string>('')

// 每次打开弹窗时重置选中项为原视频
watch(
  () => props.modelValue,
  (visible) => {
    if (visible) {
      selectedPath.value = ''
    }
  },
)

const visible = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value),
})

function handleConfirm(): void {
  emit('confirm', selectedPath.value)
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
      <div class="source-select__tip">{{ $t('videos.source.tip') }}</div>
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
</style>
