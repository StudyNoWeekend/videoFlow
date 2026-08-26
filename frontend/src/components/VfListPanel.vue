<script setup lang="ts">
import { ref, computed } from 'vue'
import { useResponsive } from '@/composables/useResponsive'

const props = withDefaults(defineProps<{
  title: string
  count?: number
  ledColor?: 'amber' | 'cyan' | 'green' | 'red'
  ledPulse?: boolean
  loading?: boolean
  // Pagination
  showPagination?: boolean
  total?: number
  page?: number
  pageSize?: number
  pageSizes?: number[]
  // Polling
  showPolling?: boolean
  pollingInterval?: number
  pollingOptions?: { value: number; label: string }[]
}>(), {
  count: undefined,
  ledColor: 'amber',
  ledPulse: false,
  loading: false,
  showPagination: true,
  total: 0,
  page: 1,
  pageSize: 10,
  pageSizes: () => [10, 20, 50, 100],
  showPolling: false,
  pollingInterval: 0,
  pollingOptions: () => [],
})

const emit = defineEmits<{
  'update:page': [value: number]
  'update:pageSize': [value: number]
  'page-change': [value: number]
  'size-change': [value: number]
  'update:pollingInterval': [value: number]
  refresh: []
}>()

const { isMobileOnly, isMobileOrTablet } = useResponsive()

const localPage = ref(props.page)
const localPageSize = ref(props.pageSize)
const localPollingInterval = ref(props.pollingInterval)

const ledClass = computed(() => {
  const color = props.ledColor === 'amber' ? 'vf-led--amber'
    : props.ledColor === 'cyan' ? 'vf-led--cyan'
    : props.ledColor === 'green' ? 'vf-led--green'
    : props.ledColor === 'red' ? 'vf-led--red'
    : ''
  return `vf-led ${color}${props.ledPulse ? ' vf-led--pulse' : ''}`
})

function handleCurrentChange(currentPage: number): void {
  localPage.value = currentPage
  emit('update:page', currentPage)
  emit('page-change', currentPage)
}

function handleSizeChange(size: number): void {
  localPageSize.value = size
  localPage.value = 1
  emit('update:pageSize', size)
  emit('size-change', size)
}

function handlePollingChange(value: number): void {
  localPollingInterval.value = value
  emit('update:pollingInterval', value)
}

function handleRefresh(): void {
  emit('refresh')
}
</script>

<template>
  <div class="vf-panel vf-list-panel">
    <div class="vf-panel__footer"></div>

    <!-- ====== Header ====== -->
    <div class="vf-panel-header vlf-header">
      <div class="vlf-header__title">
        <span :class="ledClass"></span>
        <span>{{ title }}</span>
        <span v-if="count !== undefined" class="vlf-header__count">{{ count }}</span>
      </div>
      <div class="vlf-header__extra">
        <slot name="header-extra" />
      </div>
    </div>

    <!-- ====== Toolbar ====== -->
    <div v-if="$slots['toolbar-left'] || $slots['toolbar-right'] || (showPolling && pollingOptions.length > 0)" class="vlf-toolbar">
      <div class="vlf-toolbar__left">
        <slot name="toolbar-left" />
      </div>
      <div class="vlf-toolbar__right">
        <!-- Polling control -->
        <div v-if="showPolling && pollingOptions.length > 0" class="vlf-poll hide-mobile">
          <span class="vf-data-label">{{ $t('tasks.polling.label') }}</span>
          <el-select
            :model-value="localPollingInterval"
            size="small"
            style="width: 110px"
            @update:model-value="handlePollingChange"
          >
            <el-option
              v-for="opt in pollingOptions"
              :key="opt.value"
              :label="$t(opt.label)"
              :value="opt.value"
            />
          </el-select>
          <span v-if="localPollingInterval > 0" class="vf-led vf-led--green vf-led--pulse"></span>
        </div>
        <slot name="toolbar-right" />
      </div>
    </div>

    <!-- ====== Body ====== -->
    <div class="vlf-body">
      <slot />
    </div>

    <!-- ====== Pagination ====== -->
    <div v-if="showPagination && total > 0" class="vlf-footer">
      <el-pagination
        :model-value="localPage"
        :page-size="localPageSize"
        :total="total"
        :page-sizes="pageSizes"
        :layout="isMobileOnly ? 'prev, pager, next' : 'total, sizes, prev, pager, next'"
        size="small"
        @update:model-value="handleCurrentChange"
        @update:page-size="handleSizeChange"
        @current-change="handleCurrentChange"
        @size-change="handleSizeChange"
      />
    </div>
  </div>
</template>

<style scoped>
.vf-list-panel {
  min-height: calc(100vh - 92px);
  display: flex;
  flex-direction: column;
}

@media (max-width: 767px) {
  .vf-list-panel {
    min-height: calc(100vh - 52px - 56px);
  }
}

/* ====== Header ====== */
.vlf-header__title {
  display: flex;
  align-items: center;
  gap: 10px;
}

.vlf-header__count {
  font-family: var(--vf-font-mono);
  font-size: 11px;
  color: var(--vf-text-muted);
  border: 1px solid var(--vf-border);
  padding: 2px 8px;
  border-radius: var(--vf-radius-sm);
  margin-left: 4px;
}

.vlf-header__extra {
  display: flex;
  align-items: center;
  gap: 10px;
}

@media (max-width: 480px) {
  .vlf-header__extra {
    gap: 6px;
  }
}

/* 极窄屏幕：header 纵向排列 */
@media (max-width: 480px) {
  .vlf-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }
}

/* ====== Toolbar ====== */
.vlf-toolbar {
  padding: 12px 16px;
  border-bottom: 1px solid var(--vf-border);
  display: flex;
  align-items: center;
  justify-content: space-between;
}

@media (max-width: 767px) {
  .vlf-toolbar {
    padding: 10px 12px;
  }
}

/* 极窄屏幕：toolbar 纵向堆叠 */
@media (max-width: 480px) {
  .vlf-toolbar {
    flex-direction: column;
    gap: 6px;
    align-items: stretch;
  }
  .vlf-toolbar__left {
    flex-wrap: wrap;
    gap: 6px;
  }
  .vlf-toolbar__right {
    flex-wrap: wrap;
    gap: 6px;
  }
}

.vlf-toolbar__left {
  display: flex;
  align-items: center;
  gap: 14px;
}

@media (max-width: 767px) {
  .vlf-toolbar__left {
    gap: 8px;
  }
}

.vlf-toolbar__right {
  display: flex;
  align-items: center;
  gap: 14px;
}

.vlf-poll {
  display: flex;
  align-items: center;
  gap: 8px;
}

/* ====== Body ====== */
.vlf-body {
  flex: 1;
  padding: 0;
}

/* ====== Footer (Pagination) ====== */
.vlf-footer {
  padding: 12px 16px;
  border-top: 1px solid var(--vf-border);
  display: flex;
  justify-content: flex-end;
}

@media (max-width: 767px) {
  .vlf-footer {
    padding: 10px 12px;
    justify-content: center;
  }
}
</style>
