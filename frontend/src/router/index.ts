import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/login',
      name: 'login',
      meta: { public: true },
      component: () => import('../views/LoginView.vue'),
    },
    {
      path: '/init',
      name: 'init',
      meta: { public: true },
      component: () => import('../views/InitView.vue'),
    },
    {
      path: '/reset-password',
      name: 'resetPassword',
      meta: { public: true },
      component: () => import('../views/ResetPasswordView.vue'),
    },
    {
      path: '/',
      name: 'videos',
      component: () => import('../views/VideosView.vue'),
    },
    {
      path: '/tasks',
      name: 'tasks',
      component: () => import('../views/TasksView.vue'),
    },
    {
      path: '/settings',
      name: 'settings',
      component: () => import('../views/SettingsView.vue'),
    },
  ],
})

// 路由守卫
router.beforeEach(async (to, _from, next) => {
  const auth = useAuthStore()

  // 非公开页面需要先加载初始化状态
  if (!to.meta.public && !auth.statusLoaded) {
    try {
      await auth.loadStatus()
    } catch {
      // 后端不可达，放行
    }
  }

  if (to.meta.public) {
    if (to.name === 'init') {
      // 初始化页：已初始化则跳转到登录页
      if (auth.statusLoaded && auth.isInitialized) {
        return next('/login')
      }
      return next()
    }
    if (to.name === 'login' || to.name === 'resetPassword') {
      // 已登录则跳转到首页
      if (auth.isAuthenticated && auth.isTokenValid()) {
        return next('/')
      }
      return next()
    }
    return next()
  }

  // 非公开页面：需要登录
  if (!auth.isAuthenticated || !auth.isTokenValid()) {
    if (auth.statusLoaded && !auth.isInitialized) {
      return next('/init')
    }
    return next('/login')
  }

  next()
})

export default router
