/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** 后端 API 基础地址（开发态留空走 Vite 代理，生产态可显式指定） */
  readonly VITE_API_BASE_URL?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
