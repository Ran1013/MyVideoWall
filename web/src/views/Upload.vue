<script setup>
import { ref, computed } from 'vue'
import { uploadApi, adminApi } from '../api'
import { useUpload } from '../composables/useUpload'
import ThemeToggle from '../components/ThemeToggle.vue'
import LogoutButton from '../components/LogoutButton.vue'

const { rows, addFiles, retry, saveRowInfo, publishRow } = useUpload()

// 发布按钮包装：新分类并入下拉
async function pubRow(row) {
  const cat = await publishRow(row)
  if (cat && !catSuggestions.value.includes(cat)) {
    catSuggestions.value = [...catSuggestions.value, cat].sort()
  }
}

// 保存条目名称+分类；若新建了分类名，成功后并入下拉选项
async function saveRowCat(row) {
  const cat = await saveRowInfo(row)
  if (cat && !catSuggestions.value.includes(cat)) {
    catSuggestions.value = [...catSuggestions.value, cat].sort()
  }
}
// 从下拉里选了已有分类 → 立即保存；选「新建」/「不设分类」则等点保存
function onCatPick(row) {
  if (row.catPick && row.catPick !== '__new__') saveRowCat(row)
}

// 待发布列表与批量发布
const pendingRows = computed(() => rows.filter(r => r.state === 'done' && !r.published))
async function publishAll() {
  for (const row of pendingRows.value) {
    await pubRow(row)
  }
}
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
  // 传 getter：分类在 finish 时实时读取，先选文件后填分类也能生效
  addFiles(Array.from(files), chunkSize)
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
      <p class="tip">分片上传 · 断网/关页后重选同一文件自动续传 · 名称与分类在每个视频下方填写（传完再改也行）· 传完回 <router-link to="/">首页</router-link> 看</p>

      <div class="dz" :class="{ over: dragging }"
           @click="pickFiles"
           @dragover.prevent="dragging = true"
           @dragleave="dragging = false"
           @drop.prevent="onDrop">
        点击选择视频上传（手机可直接选相册/文件，iPhone 的 .MOV、OBS 的 .mkv 都支持）· 电脑支持拖拽，可多选
        <input ref="fileInput" type="file" multiple hidden
               accept="video/*,.mp4,.webm,.m4v,.mov,.mkv" @change="onFiles">
      </div>

      <div v-if="pendingRows.length" class="traffic-actions" style="margin-top: 14px">
        <button class="btn" @click="publishAll">发布全部（{{ pendingRows.length }} 个待发布）</button>
      </div>

      <div class="rows">
        <div v-for="(row, i) in rows" :key="row.name + i" class="row" :class="row.state">
          <div class="row-line1">
            <span class="fn" :title="row.name">{{ row.name }}</span>
            <progress :value="row.pct" max="100"></progress>
            <span class="st">{{ row.stateText }}</span>
            <button v-if="row.state === 'uploading'" class="row-btn" @click="row.togglePause()">
              {{ row.paused ? '继续' : '暂停' }}
            </button>
            <button v-if="row.state === 'fail'" class="row-btn" @click="retry(row)">重试</button>
          </div>
          <input v-model="row.titleInput" class="row-title" type="text" maxlength="120"
                 placeholder="视频名称（默认同文件名，可修改）" @keyup.enter="saveRowCat(row)">
          <div class="row-cat-line">
            <select v-model="row.catPick" class="row-cat-select" @change="onCatPick(row)">
              <option value="">（不设分类）</option>
              <option v-for="c in catSuggestions" :key="c" :value="c">{{ c }}</option>
              <option value="__new__">＋ 新建分类…</option>
            </select>
            <input v-if="row.catPick === '__new__'" v-model="row.newCat" class="row-cat-new" type="text"
                   maxlength="60" placeholder="输入新分类名" @keyup.enter="saveRowCat(row)">
            <button class="row-btn" @click="row.published ? saveRowCat(row) : pubRow(row)">
              {{ row.published ? '保存' : '发布' }}
            </button>
            <span v-if="row.catMsg" class="cat-msg">{{ row.catMsg }}</span>
          </div>
        </div>
      </div>

    </div>
  </main>
</template>
