<script setup>
import { useRouter } from 'vue-router'
import { clearSession, getRole } from '../api'

const router = useRouter()

function logout() {
  clearSession()
  router.push('/login')
}
</script>

<template>
  <button class="btn ghost logout-btn" :title="getRole() === 'upload' ? '当前：管理身份' : '当前：观看身份'" @click="logout">
    退出{{ getRole() === 'upload' ? '（管理）' : '' }}
  </button>
</template>

<style scoped>
.logout-btn { cursor: pointer; }
</style>
