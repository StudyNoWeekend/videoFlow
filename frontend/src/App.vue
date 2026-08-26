<script setup lang="ts">
import { RouterView, RouterLink, useRoute, useRouter } from 'vue-router'
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { i18n, elementPlusLocales } from '@/i18n'
import { useAuthStore } from '@/stores/auth'
import { changePassword } from '@/api/auth'
import { getVersion } from '@/api/version'
import { validatePassword } from '@/utils/validate'
import { useResponsive } from '@/composables/useResponsive'

const route = useRoute()
const router = useRouter()
const { t, locale } = useI18n()
const auth = useAuthStore()
const { isMobileOnly, isMobileOrTablet } = useResponsive()

// 应用版本号，从后端获取
const appVersion = ref('')

// 判断是否为公开认证页面（不显示工作台布局）
const isPublicPage = computed(() => {
  return route.meta?.public === true
})

	const menuItems = computed(() => [
	  { name: 'videos', label: t('nav.videos'), icon: 'VideoCamera', code: t('nav.code.videos') },
	  { name: 'tasks', label: t('nav.tasks'), icon: 'List', code: t('nav.code.tasks') },
	  { name: 'downloads', label: t('nav.downloads'), icon: 'Download', code: t('nav.code.downloads') },
	  { name: 'settings', label: t('nav.settings'), icon: 'Setting', code: t('nav.code.settings') },
	])

// i18n locale -> BCP 47 tag for date/time formatting
const dateTimeLocaleMap: Record<string, string> = {
  zh: 'zh-CN',
  'zh-TW': 'zh-TW',
  en: 'en-US',
  ja: 'ja-JP',
}
const dateTimeLocale = computed(() => dateTimeLocaleMap[locale.value] || 'zh-CN')

const now = ref(new Date())
let clockTimer: number | null = null

const timeString = computed(() => {
  return now.value.toLocaleTimeString(dateTimeLocale.value, { hour12: false })
})

const dateString = computed(() => {
  return now.value.toLocaleDateString(dateTimeLocale.value)
})

const theme = ref<'dark' | 'light'>('dark')
const isDark = computed(() => theme.value === 'dark')
const themeIcon = computed(() => (isDark.value ? 'Moon' : 'Sunny'))
const themeLabel = computed(() => (isDark.value ? t('theme.dark') : t('theme.light')))

// Element Plus locale (reactive)
const epLocale = computed(() => elementPlusLocales[locale.value] || elementPlusLocales.zh)

// 当前语言显示标签
const langLabels: Record<string, string> = {
  zh: '简体中文',
  'zh-TW': '繁體中文',
  en: 'English',
  ja: '日本語',
}
const currentLangLabel = computed(() => langLabels[locale.value] || '简体中文')

// 切换语言（显式操作全局 i18n locale）
function switchLanguage(lang: string): void {
  const langKey = lang as 'zh' | 'zh-TW' | 'en' | 'ja'
  i18n.global.locale.value = langKey
  locale.value = langKey
  localStorage.setItem('videoflow-lang', lang)
}

// ---- 用户菜单 ----
const userDropdownVisible = ref(false)
const changePwdDialogVisible = ref(false)
const changePwdLoading = ref(false)
const changePwdForm = ref({
  old_password: '',
  new_password: '',
  confirm_password: '',
})

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

onMounted(async () => {
  // 获取版本号
  try {
    appVersion.value = await getVersion()
  } catch {
    // 忽略错误，使用空值
  }

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

// 退出登录
function handleLogout(): void {
  auth.logout()
  ElMessage.success(t('auth.success.logout'))
  router.push('/login')
}

// 打开修改密码弹窗
function openChangePwdDialog(): void {
  changePwdForm.value = { old_password: '', new_password: '', confirm_password: '' }
  changePwdDialogVisible.value = true
}

// 提交修改密码
async function handleChangePwd(): Promise<void> {
  const form = changePwdForm.value
  if (!form.old_password || !form.new_password || !form.confirm_password) {
    ElMessage.warning(t('auth.error.fill_fields'))
    return
  }
  if (form.new_password !== form.confirm_password) {
    ElMessage.warning(t('auth.error.password_mismatch'))
    return
  }
  // 校验新密码格式（与后端 auth_validate.go 规则一致）
  const pwdErr = validatePassword(form.new_password)
  if (pwdErr) {
    ElMessage.warning(t(pwdErr))
    return
  }
  if (form.old_password === form.new_password) {
    ElMessage.warning(t('auth.error.same_password'))
    return
  }
  changePwdLoading.value = true
  try {
    await changePassword(form)
    changePwdDialogVisible.value = false
    ElMessage.success(t('auth.success.change_password'))
  } catch {
    // 错误已在拦截器中处理
  } finally {
    changePwdLoading.value = false
  }
}
</script>

<template>
  <el-config-provider :locale="epLocale">
    <!-- 公开认证页面（登录、初始化、忘记密码） -->
    <template v-if="isPublicPage">
      <RouterView />
    </template>

    <!-- 主工作台 -->
    <template v-else>
      <div class="workbench">
        <!-- 顶部状态栏 -->
        <header class="status-bar">
          <div class="status-bar__brand">
            <div class="brand-logo">
              <span class="brand-logo__mark">VF</span>
              <span class="brand-logo__text hide-mobile">VideoFlow</span>
            </div>
            <span class="status-bar__version hide-mobile">{{ appVersion }}</span>
          </div>

          <div class="status-bar__actions">
            <button class="theme-toggle" :aria-label="themeLabel + ' ' + $t('theme.mode')" @click="toggleTheme">
              <el-icon size="16">
                <component :is="themeIcon" />
              </el-icon>
              <span class="theme-toggle__label hide-mobile">{{ themeLabel }}</span>
            </button>

            <!-- 语言切换 -->
            <el-dropdown trigger="click" @command="switchLanguage">
              <button class="lang-toggle">
                <el-icon size="16"><Global /></el-icon>
                <span class="lang-toggle__label">{{ currentLangLabel }}</span>
                <el-icon size="10"><ArrowDown /></el-icon>
              </button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item :class="{ active: locale.value === 'zh' }" command="zh">简体中文</el-dropdown-item>
                  <el-dropdown-item :class="{ active: locale.value === 'zh-TW' }" command="zh-TW">繁體中文</el-dropdown-item>
                  <el-dropdown-item :class="{ active: locale.value === 'en' }" command="en">English</el-dropdown-item>
                  <el-dropdown-item :class="{ active: locale.value === 'ja' }" command="ja">日本語</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>

            <!-- 用户菜单 -->
            <el-dropdown trigger="click" @command="(cmd: string) => { if (cmd === 'change-pwd') openChangePwdDialog(); if (cmd === 'logout') handleLogout(); }">
              <button class="user-menu">
                <el-icon size="16"><User /></el-icon>
                <span class="user-menu__name hide-mobile">{{ auth.user?.username || 'User' }}</span>
                <el-icon size="12"><ArrowDown /></el-icon>
              </button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="change-pwd">
                    <el-icon size="14"><Lock /></el-icon>
                    {{ $t('auth.change_password.title') }}
                  </el-dropdown-item>
                  <el-dropdown-item command="logout" divided>
                    <el-icon size="14"><SwitchButton /></el-icon>
                    {{ $t('auth.btn.logout') }}
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>

            <div class="status-bar__clock">
              <span class="clock__date hide-mobile">{{ dateString }}</span>
              <span class="clock__time">{{ timeString }}</span>
            </div>
          </div>
        </header>

        <div class="workbench__body">
          <!-- 侧边导航（桌面端/平板端） -->
          <aside class="side-rail">
            <nav class="rail-menu">
              <RouterLink
                v-for="item in menuItems"
                :key="item.name"
                :to="{ name: item.name }"
                class="rail-item"
                :class="{ active: route.name === item.name }"
              >
                <div class="rail-item__code hide-tablet">{{ item.code }}</div>
                <el-icon size="20">
                  <component :is="item.icon" />
                </el-icon>
                <span class="rail-item__label hide-mobile">{{ item.label }}</span>
                <span v-if="route.name === item.name" class="rail-item__active-bar"></span>
              </RouterLink>
            </nav>
          </aside>

          <!-- 主内容区 -->
          <main class="main-stage tech-grid scanlines noise">
            <RouterView />
          </main>
        </div>

        <!-- 底部导航（手机端 < 768px） -->
        <nav class="bottom-nav">
          <RouterLink
            v-for="item in menuItems"
            :key="item.name"
            :to="{ name: item.name }"
            class="bottom-nav__item"
            :class="{ active: route.name === item.name }"
          >
            <el-icon size="20">
              <component :is="item.icon" />
            </el-icon>
            <span class="bottom-nav__label">{{ item.label }}</span>
          </RouterLink>
        </nav>

        <!-- 修改密码对话框 -->
        <el-dialog
          v-model="changePwdDialogVisible"
          :title="t('auth.change_password.title')"
          width="420px"
          :close-on-click-modal="false"
        >
          <el-form
            ref="changePwdFormRef"
            :model="changePwdForm"
            label-position="top"
            @submit.prevent="handleChangePwd"
          >
            <el-form-item :label="t('auth.placeholder.old_password')">
              <el-input
                v-model="changePwdForm.old_password"
                type="password"
                :placeholder="t('auth.placeholder.old_password')"
                show-password
              />
            </el-form-item>
            <el-form-item :label="t('auth.placeholder.new_password')">
              <el-input
                v-model="changePwdForm.new_password"
                type="password"
                :placeholder="t('auth.placeholder.new_password')"
                show-password
              />
            </el-form-item>
            <el-form-item :label="t('auth.placeholder.confirm_password')">
              <el-input
                v-model="changePwdForm.confirm_password"
                type="password"
                :placeholder="t('auth.placeholder.confirm_password')"
                show-password
              />
            </el-form-item>
          </el-form>
          <template #footer>
            <el-button @click="changePwdDialogVisible = false">{{ t('common.cancel') }}</el-button>
            <el-button type="primary" :loading="changePwdLoading" @click="handleChangePwd">
              {{ t('common.confirm') }}
            </el-button>
          </template>
        </el-dialog>
      </div>
    </template>
  </el-config-provider>
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

@media (max-width: 767px) {
  .status-bar {
    padding: 0 12px;
  }
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

.status-bar__actions {
  display: flex;
  align-items: center;
  gap: 16px;
}

@media (max-width: 767px) {
  .status-bar__actions {
    gap: 8px;
  }
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

@media (max-width: 767px) {
  .theme-toggle {
    padding: 6px 8px;
  }
}

.theme-toggle:hover {
  border-color: var(--vf-accent);
  color: var(--vf-accent);
}

.theme-toggle__label {
  min-width: 24px;
  text-align: center;
}

/* 语言切换按钮 */
.lang-toggle {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 6px 10px;
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

@media (max-width: 767px) {
  .lang-toggle {
    padding: 6px 8px;
  }
}

.lang-toggle:hover {
  border-color: var(--vf-accent);
  color: var(--vf-accent);
}

.lang-toggle__label {
  min-width: 48px;
  text-align: center;
}

/* 用户菜单 */
.user-menu {
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

@media (max-width: 767px) {
  .user-menu {
    padding: 6px 8px;
  }
}

.user-menu:hover {
  border-color: var(--vf-accent);
  color: var(--vf-accent);
}

.user-menu__name {
  max-width: 80px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.status-bar__clock {
  display: flex;
  align-items: center;
  gap: 12px;
  font-family: var(--vf-font-mono);
}

@media (max-width: 767px) {
  .status-bar__clock {
    gap: 6px;
  }
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
  width: var(--vf-side-rail-width);
  background: var(--vf-bg-elevated);
  border-right: 1px solid var(--vf-border);
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  flex-shrink: 0;
  transition: width 0.2s ease;
}

/* 平板端（768~1023px）：侧栏折叠为图标模式，hover 展开 */
@media (min-width: 768px) and (max-width: 1023px) {
  .side-rail {
    width: 56px;
    overflow: hidden;
  }
  .side-rail:hover {
    width: 160px;
  }
}

/* 手机端（< 768px）：侧栏完全隐藏 */
@media (max-width: 767px) {
  .side-rail {
    display: none;
  }
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

/* 平板 hover 展开时，侧栏项改为水平布局 */
@media (min-width: 768px) and (max-width: 1023px) {
  .rail-item {
    flex-direction: row;
    gap: 10px;
    padding: 12px 16px;
    justify-content: flex-start;
  }
  .rail-item__code {
    display: none;
  }
  .rail-item__label {
    display: none;
  }
  .side-rail:hover .rail-item__label {
    display: block;
    white-space: nowrap;
  }
  .side-rail:hover .rail-item {
    flex-direction: row;
  }
  .rail-item__active-bar {
    display: none;
  }
  .side-rail:hover .rail-item__active-bar {
    display: block;
  }
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
  overflow-y: auto;
  /* 禁止整页横向滑动，避免内容被滑出视口；超宽内容由各组件内部滚动 */
  overflow-x: hidden;
  position: relative;
  background-color: var(--vf-bg);
}

@media (max-width: 767px) {
  .main-stage {
    padding-bottom: 64px; /* 底部导航预留空间 */
  }
}

.main-stage > :deep(*) {
  position: relative;
  z-index: 1;
}

/* ===== 底部导航栏（手机端 < 768px） ===== */
.bottom-nav {
  display: none;
}

@media (max-width: 767px) {
  .bottom-nav {
    display: flex;
    position: fixed;
    bottom: 0;
    left: 0;
    right: 0;
    height: 56px;
    background: var(--vf-bg-elevated);
    border-top: 1px solid var(--vf-border);
    align-items: center;
    justify-content: space-around;
    z-index: 100;
    padding: 0 4px;
    padding-bottom: env(safe-area-inset-bottom, 0);
  }
}

.bottom-nav__item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  padding: 6px 12px;
  color: var(--vf-text-muted);
  text-decoration: none;
  transition: all 0.2s ease;
  border-radius: var(--vf-radius);
}

.bottom-nav__item.active {
  color: var(--vf-accent);
}

.bottom-nav__item:hover {
  color: var(--vf-text-secondary);
}

.bottom-nav__label {
  font-family: var(--vf-font-ui);
  font-size: 10px;
  font-weight: 500;
  letter-spacing: 0.03em;
}
</style>
