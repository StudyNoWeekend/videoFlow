<script setup lang="ts">
import { RouterView, RouterLink, useRoute } from 'vue-router'
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'

const route = useRoute()

const menuItems = [
  { name: 'videos', label: '视频资源', icon: 'VideoCamera', code: '资源' },
  { name: 'tasks', label: '任务监控', icon: 'List', code: '监控' },
  { name: 'settings', label: '系统配置', icon: 'Setting', code: '配置' },
]

const now = ref(new Date())
let clockTimer: number | null = null

const timeString = computed(() => {
  return now.value.toLocaleTimeString('zh-CN', { hour12: false })
})

const dateString = computed(() => {
  return now.value.toLocaleDateString('zh-CN')
})

const theme = ref<'dark' | 'light'>('dark')
const isDark = computed(() => theme.value === 'dark')
const themeIcon = computed(() => (isDark.value ? 'Moon' : 'Sunny'))
const themeLabel = computed(() => (isDark.value ? '夜间' : '日间'))

function applyTheme(): void {
  document.documentElement.setAttribute('data-theme', theme.value)
  try {
    localStorage.setItem('videoflow-theme', theme.value)
  } catch {
    // 忽略存储异常
  }
}

function toggleTheme(): void {
  theme.value = isDark.value ? 'light' : 'dark'
}

watch(theme, applyTheme)

onMounted(() => {
  clockTimer = window.setInterval(() => {
    now.value = new Date()
  }, 1000)

  const saved = localStorage.getItem('videoflow-theme')
  if (saved === 'light' || saved === 'dark') {
    theme.value = saved
  } else {
    theme.value = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  }
  applyTheme()
})

onUnmounted(() => {
  if (clockTimer !== null) {
    clearInterval(clockTimer)
  }
})
</script>

<template>
  <div class="workbench">
    <!-- 顶部状态栏 -->
    <header class="status-bar">
      <div class="status-bar__brand">
        <div class="brand-logo">
          <span class="brand-logo__mark">VF</span>
          <span class="brand-logo__text">VideoFlow</span>
        </div>
        <span class="status-bar__version">v1.0.0</span>
      </div>

      <div class="status-bar__indicators">
        <div class="indicator">
          <span class="vf-led vf-led--green vf-led--pulse"></span>
          <span class="indicator__label">系统在线</span>
        </div>
        <div class="indicator">
          <span class="vf-led vf-led--amber"></span>
          <span class="indicator__label">工作台就绪</span>
        </div>
      </div>

      <div class="status-bar__actions">
        <button class="theme-toggle" :aria-label="themeLabel + '模式'" @click="toggleTheme">
          <el-icon size="16">
            <component :is="themeIcon" />
          </el-icon>
          <span class="theme-toggle__label">{{ themeLabel }}</span>
        </button>

        <div class="status-bar__clock">
          <span class="clock__date">{{ dateString }}</span>
          <span class="clock__time">{{ timeString }}</span>
        </div>
      </div>
    </header>

    <div class="workbench__body">
      <!-- 侧边导航 -->
      <aside class="side-rail">
        <nav class="rail-menu">
          <RouterLink
            v-for="item in menuItems"
            :key="item.name"
            :to="{ name: item.name }"
            class="rail-item"
            :class="{ active: route.name === item.name }"
          >
            <div class="rail-item__code">{{ item.code }}</div>
            <el-icon size="20">
              <component :is="item.icon" />
            </el-icon>
            <span class="rail-item__label">{{ item.label }}</span>
            <span v-if="route.name === item.name" class="rail-item__active-bar"></span>
          </RouterLink>
        </nav>

        <div class="rail-footer">
          <div class="rail-footer__line">
            <span class="vf-data-label">节点</span>
            <span class="vf-data-value">本地-01</span>
          </div>
          <div class="rail-footer__line">
            <span class="vf-data-label">运行时间</span>
            <span class="vf-data-value">--:--</span>
          </div>
        </div>
      </aside>

      <!-- 主内容区 -->
      <main class="main-stage tech-grid scanlines noise">
        <RouterView />
      </main>
    </div>
  </div>
</template>

<style scoped>
.workbench {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background: var(--vf-bg);
  color: var(--vf-text-primary);
}

/* 顶部状态栏 */
.status-bar {
  height: 52px;
  background: var(--vf-bg-elevated);
  border-bottom: 1px solid var(--vf-border);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  flex-shrink: 0;
  position: relative;
  z-index: 10;
}

.status-bar::after {
  content: '';
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  height: 1px;
  background: linear-gradient(90deg, transparent, var(--vf-accent), transparent);
  opacity: 0.4;
}

.status-bar__brand {
  display: flex;
  align-items: center;
  gap: 16px;
}

.brand-logo {
  display: flex;
  align-items: center;
  gap: 10px;
}

.brand-logo__mark {
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--vf-accent-soft);
  border: 1px solid var(--vf-accent-border);
  color: var(--vf-accent);
  font-family: var(--vf-font-display);
  font-weight: 700;
  font-size: 13px;
  letter-spacing: 0.06em;
  border-radius: var(--vf-radius);
  box-shadow: var(--vf-glow-amber);
}

.brand-logo__text {
  font-family: var(--vf-font-display);
  font-weight: 600;
  font-size: 16px;
  letter-spacing: 0.06em;
}

.status-bar__version {
  font-family: var(--vf-font-mono);
  font-size: 11px;
  color: var(--vf-text-muted);
  border: 1px solid var(--vf-border);
  padding: 2px 8px;
  border-radius: var(--vf-radius-sm);
}

.status-bar__indicators {
  display: flex;
  align-items: center;
  gap: 24px;
}

.indicator {
  display: flex;
  align-items: center;
  gap: 8px;
}

.indicator__label {
  font-family: var(--vf-font-mono);
  font-size: 11px;
  color: var(--vf-text-secondary);
  letter-spacing: 0.06em;
}

.status-bar__actions {
  display: flex;
  align-items: center;
  gap: 16px;
}

.theme-toggle {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  background: var(--vf-bg-panel);
  border: 1px solid var(--vf-border-light);
  border-radius: var(--vf-radius);
  color: var(--vf-text-secondary);
  font-family: var(--vf-font-ui);
  font-size: 12px;
  font-weight: 500;
  letter-spacing: 0.04em;
  cursor: pointer;
  transition: all 0.2s ease;
}

.theme-toggle:hover {
  border-color: var(--vf-accent);
  color: var(--vf-accent);
}

.theme-toggle__label {
  min-width: 24px;
  text-align: center;
}

.status-bar__clock {
  display: flex;
  align-items: center;
  gap: 12px;
  font-family: var(--vf-font-mono);
}

.clock__date {
  font-size: 12px;
  color: var(--vf-text-muted);
}

.clock__time {
  font-size: 15px;
  color: var(--vf-accent);
  min-width: 72px;
  text-align: right;
}

/* 主体 */
.workbench__body {
  display: flex;
  flex: 1;
  overflow: hidden;
}

/* 侧边栏 */
.side-rail {
  width: 72px;
  background: var(--vf-bg-elevated);
  border-right: 1px solid var(--vf-border);
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  flex-shrink: 0;
}

.rail-menu {
  padding: 12px 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.rail-item {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 14px 0;
  color: var(--vf-text-muted);
  text-decoration: none;
  transition: all 0.2s ease;
}

.rail-item:hover {
  color: var(--vf-text-secondary);
  background: rgba(255, 255, 255, 0.02);
}

[data-theme='light'] .rail-item:hover {
  background: rgba(0, 0, 0, 0.03);
}

.rail-item.active {
  color: var(--vf-accent);
  background: var(--vf-accent-soft);
}

.rail-item__code {
  position: absolute;
  top: 6px;
  right: 8px;
  font-family: var(--vf-font-mono);
  font-size: 9px;
  letter-spacing: 0.08em;
  color: var(--vf-text-disabled);
}

.rail-item.active .rail-item__code {
  color: var(--vf-accent);
  opacity: 0.7;
}

.rail-item__label {
  font-family: var(--vf-font-ui);
  font-size: 11px;
  font-weight: 500;
  letter-spacing: 0.04em;
}

.rail-item__active-bar {
  position: absolute;
  left: 0;
  top: 50%;
  transform: translateY(-50%);
  width: 3px;
  height: 28px;
  background: var(--vf-accent);
  box-shadow: var(--vf-glow-amber);
  border-radius: 0 2px 2px 0;
}

.rail-footer {
  padding: 14px 10px;
  border-top: 1px solid var(--vf-border);
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.rail-footer__line {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.rail-footer__line .vf-data-value {
  font-size: 10px;
}

/* 主舞台 */
.main-stage {
  flex: 1;
  overflow: auto;
  position: relative;
  background-color: var(--vf-bg);
}

.main-stage > :deep(*) {
  position: relative;
  z-index: 1;
}
</style>
