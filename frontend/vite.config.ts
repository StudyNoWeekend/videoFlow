import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'

// 与后端共用 APP_HTTP_PORT 作为端口唯一真源（后端 viper AutomaticEnv 同名覆盖 http.port）。
// 启动前端时：APP_HTTP_PORT=9090 npm run dev；不设则默认 8080。
const backendPort = process.env.APP_HTTP_PORT || '8080'

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    vue(),
    vueDevTools(),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    proxy: {
      '/api': {
        target: `http://localhost:${backendPort}`,
        changeOrigin: true,
      },
    },
  },
})
