import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// API 地址：开发环境走 vite 代理（同源免 CORS），生产用环境变量 VITE_API_BASE
export default defineConfig({
  plugins: [vue()],
  server: {
    proxy: {
      // Go 后端全部路由挂在 /api 前缀下，代理保留前缀不 rewrite
      '/api': {
        target: 'http://localhost:8082',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
  },
})
