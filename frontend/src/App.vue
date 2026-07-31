<script setup lang="ts">
import { RouterView, RouterLink, useRoute } from 'vue-router'

const route = useRoute()

const menuItems = [
  { name: 'videos', label: '视频列表', icon: 'VideoCamera' },
  { name: 'tasks', label: '任务管理', icon: 'List' },
  { name: 'settings', label: '设置', icon: 'Setting' },
]
</script>

<template>
  <div class="app-layout">
    <!-- 侧边导航 -->
    <aside class="sidebar">
      <div class="logo">VideoFlow</div>
      <nav class="menu">
        <RouterLink
          v-for="item in menuItems"
          :key="item.name"
          :to="{ name: item.name }"
          class="menu-item"
          :class="{ active: route.name === item.name }"
        >
          <el-icon size="18">
            <component :is="item.icon" />
          </el-icon>
          <span>{{ item.label }}</span>
        </RouterLink>
      </nav>
    </aside>

    <!-- 主内容区 -->
    <main class="main-content">
      <RouterView />
    </main>
  </div>
</template>

<style scoped>
.app-layout {
  display: flex;
  min-height: 100vh;
}

.sidebar {
  width: 200px;
  background-color: #304156;
  color: #fff;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
}

.logo {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  font-weight: bold;
  border-bottom: 1px solid #1f2d3d;
}

.menu {
  padding: 12px 0;
}

.menu-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 20px;
  color: #bfcbd9;
  text-decoration: none;
  transition: all 0.2s;
}

.menu-item:hover {
  background-color: #263445;
  color: #fff;
}

.menu-item.active {
  background-color: #1f2d3d;
  color: #409eff;
}

.main-content {
  flex: 1;
  background-color: #f5f7fa;
  overflow: auto;
}
</style>
