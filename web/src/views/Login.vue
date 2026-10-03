<script setup>
import { ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { login, saveSession } from '../api'

const router = useRouter()
const route = useRoute()
const pwd = ref('')
const err = ref('')
const busy = ref(false)
const needUpload = route.query.need === 'upload'

async function submit() {
  if (!pwd.value || busy.value) return
  busy.value = true
  err.value = ''
  try {
    const r = await login(pwd.value)
    saveSession(r.token, r.role)
    router.push(route.query.redirect || '/')
  } catch (e) {
    err.value = e.message
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="center-wrap">
    <form class="panel" @submit.prevent="submit">
      <h2>我的游戏高光</h2>
      <p class="tip">这是一个私享小站，输入访问密码继续</p>
      <p v-if="needUpload" class="err" style="margin: 0 0 12px">该页面需要管理密码：当前是观看身份，直接输入管理密码（上传/管理密码）即可切换。</p>
      <input v-model="pwd" type="password" placeholder="访问密码" autofocus required>
      <div v-if="err" class="err">{{ err }}</div>
      <button class="primary" type="submit" :disabled="busy">{{ busy ? '验证中…' : '进 入' }}</button>
    </form>
  </div>
</template>
