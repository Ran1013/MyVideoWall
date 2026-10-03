<script setup>
import { ref } from 'vue'

// 初始值由 main.js 挂载前写好（手动选择优先，否则跟随系统）
const theme = ref(document.documentElement.dataset.theme || 'dark')

function toggle() {
  theme.value = theme.value === 'light' ? 'dark' : 'light'
  localStorage.setItem('vw_theme', theme.value)
  document.documentElement.dataset.theme = theme.value
}
</script>

<template>
  <button class="btn ghost theme-toggle" :title="theme === 'light' ? '切换到暗色主题' : '切换到亮色主题'" @click="toggle">
    {{ theme === 'light' ? '🌙 暗色' : '☀️ 亮色' }}
  </button>
</template>

<style scoped>
.theme-toggle { cursor: pointer; }
</style>
