import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import './style.css'
import { getToken, getRole } from './api'

// 主题：本地手动选择优先，否则跟随系统偏好（在挂载前写好，避免闪错主题）
const savedTheme = localStorage.getItem('vw_theme')
document.documentElement.dataset.theme =
  savedTheme || (matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark')

import Login from './views/Login.vue'
import Wall from './views/Wall.vue'
import Play from './views/Play.vue'
import Upload from './views/Upload.vue'
import Admin from './views/Admin.vue'
import Logs from './views/Logs.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'wall', component: Wall },
    { path: '/play/:id', name: 'play', component: Play },
    { path: '/upload', name: 'upload', component: Upload, meta: { role: 'upload' } },
    { path: '/admin', name: 'admin', component: Admin, meta: { role: 'upload' } },
    { path: '/logs', name: 'logs', component: Logs, meta: { role: 'upload' } },
    { path: '/login', name: 'login', component: Login },
  ],
})

router.beforeEach(to => {
  if (to.name === 'login') return true
  if (!getToken()) return { name: 'login', query: { redirect: to.fullPath } }
  if (to.meta.role === 'upload' && getRole() !== 'upload') return { name: 'wall' }
  return true
})

createApp(App).use(router).mount('#app')
