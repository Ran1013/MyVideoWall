<script setup>
import { ref } from 'vue'
import { uploadApi, adminApi } from '../api'
import { useUpload } from '../composables/useUpload'
import ThemeToggle from '../components/ThemeToggle.vue'
import LogoutButton from '../components/LogoutButton.vue'

const { rows, addFiles, retry } = useUpload()
const category = ref('')
const dragging = ref(false)
const fileInput = ref(null)
const catSuggestions = ref([])

// 已有分类提示
adminApi.categories().then(d => { catSuggestions.value = d.items.map(c => c.c) }).catch(() => {})

// 单片大小：先探一次 init 的 chunk_size（用 1 字节假文件太丑，直接约定：后端在错误信息里给不了，
// 就用 fetch 一个小 init 请求取 chunk_size —— init 需要真实文件名，改为上传前第一行初始化时感知）
let chunkSize = 4 * 1024 * 1024

function pickFiles() { fileInput.value?.click() }
function onFiles(e) { handle(e.target.files); e.target.value = '' }
function onDrop(e) {
  dragging.value = false
  handle(e.dataTransfer.files)
}
async function handle(files) {
  if (!files?.length) return
  // 先用第一个文件探单片大小（同时它也作为正常上传开始）
  try {
    const probe = await uploadApi.init(files[0].name, files[0].size, Math.ceil(files[0].size / chunkSize))
    if (probe.ok && probe.chunk_size) chunkSize = probe.chunk_size
  } catch { /* init 失败会在 useUpload 内部重试并报错 */ }
  addFiles(Array.from(files), category.value.trim(), chunkSize)
}
</script>

<template>
  <header class="top">
    <router-link class="btn" to="/">← 首页</router-link>
    <h1>上传视频</h1>
    <span class="sp"></span>
    <router-link class="btn ghost" to="/admin">管理</router-link>
    <LogoutButton />
    <ThemeToggle />
  </header>

  <main class="center-wrap" style="align-items: flex-start;">
    <div class="panel wide" style="margin-top: 30px;">
      <h2>拖进来就传</h2>
      <p class="tip">分片上传 · 断网/关页后重选同一文件自动续传 · 传完回 <router-link to="/">首页</router-link> 看</p>

      <div class="dz" :class="{ over: dragging }"
           @click="pickFiles"
           @dragover.prevent="dragging = true"
           @dragleave="dragging = false"
           @drop.prevent="onDrop">
        拖拽视频到这里，或点击选择文件（可多选）
        <input ref="fileInput" type="file" multiple hidden
               accept=".mp4,.webm,.m4v,.mov" @change="onFiles">
      </div>

      <div class="upload-fields">
        <input v-model="category" type="text" list="cat-list" placeholder="分类（可选，如：英雄联盟）" maxlength="60">
        <datalist id="cat-list">
          <option v-for="c in catSuggestions" :key="c" :value="c"></option>
        </datalist>
      </div>

      <div class="rows">
        <div v-for="(row, i) in rows" :key="row.name + i" class="row" :class="row.state">
          <span class="fn" :title="row.name">{{ row.name }}</span>
          <progress :value="row.pct" max="100"></progress>
          <span class="st">{{ row.stateText }}</span>
          <button v-if="row.state === 'uploading'" class="row-btn" @click="row.togglePause()">
            {{ row.paused ? '继续' : '暂停' }}
          </button>
          <button v-if="row.state === 'fail'" class="row-btn" @click="retry(row)">重试</button>
        </div>
      </div>

    </div>
  </main>
</template>
