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
    // 兼容性：默认目标会保留 ES2020 语法（?? / ?. 等），部分手机/国产浏览器内核解析失败导致白屏。
    // 降到 es2018 让 esbuild 转译这些语法，覆盖更老的 Android WebView / 国产浏览器。
    target: ['es2018'],
  },
})
