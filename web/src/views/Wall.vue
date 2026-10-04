<script setup>
import { ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { videoList, videoPinned, isUpload, streamUrl } from '../api'
import ThemeToggle from '../components/ThemeToggle.vue'
import LogoutButton from '../components/LogoutButton.vue'

const router = useRouter()
const q = ref('')
const cat = ref('')
const sort = ref('new')
const page = ref(1)
const items = ref([])
const total = ref(0)
const cats = ref([])
const pinned = ref([])
const loading = ref(true)
const err = ref('')

async function load() {
  loading.value = true
  err.value = ''
  try {
    const d = await videoList({ q: q.value, cat: cat.value, sort: sort.value, p: page.value })
    items.value = d.items
    total.value = d.total
    cats.value = d.categories
  } catch (e) {
    err.value = e.message
  } finally {
    loading.value = false
  }
}

// 精选区与搜索/分类无关，进首页拉一次即可（置顶视频同时在下面的墙里出现）
async function loadPinned() {
  try {
    pinned.value = (await videoPinned()).items
  } catch {
    pinned.value = []
  }
}

watch([q, cat, sort], () => { page.value = 1; load() })
watch(page, load)
load()
loadPinned()

const pages = () => Math.max(1, Math.ceil(total.value / 60))

function fmtDate(s) { return (s || '').slice(0, 10) }
function fmtSize(b) {
  if (b >= 1073741824) return (b / 1073741824).toFixed(1) + ' GB'
  if (b >= 1048576) return (b / 1048576).toFixed(1) + ' MB'
  return Math.max(1, Math.round(b / 1024)) + ' KB'
}
function pickCat(c) { cat.value = cat.value === c ? '' : c }
</script>

<template>
  <header class="top">
    <h1>我的游戏高光</h1>
    <input v-model="q" class="search-box" type="search" placeholder="搜索片段…">
    <span class="sp"></span>
    <nav class="sort-switch">
      <button :class="{ cur: sort === 'new' }" @click="sort = 'new'">最新</button>
      <button :class="{ cur: sort === 'hot' }" @click="sort = 'hot'">最热</button>
    </nav>
    <router-link v-if="isUpload()" class="btn" to="/upload">上传</router-link>
    <router-link v-if="isUpload()" class="btn ghost" to="/admin">管理</router-link>
    <LogoutButton />
    <ThemeToggle />
  </header>

  <nav v-if="cats.length" class="cats">
    <button :class="{ cur: cat === '' }" @click="cat = ''">全部</button>
    <button v-for="c in cats" :key="c.c" :class="{ cur: cat === c.c }" @click="pickCat(c.c)">
      {{ c.c }} <i>{{ c.n }}</i>
    </button>
  </nav>

  <!-- 站长精选：置顶视频横排大卡；搜索/分类浏览时隐藏（那时用户在找特定内容） -->
  <section v-if="pinned.length && !q && !cat" class="featured" aria-label="站长精选">
    <h2 class="feat-title">📌 站长精选</h2>
    <div class="feat-row">
      <router-link v-for="it in pinned" :key="'p' + it.id" class="card feat-card" :to="'/play/' + it.id">
        <div class="thumb">
          <img v-if="it.poster" :src="streamUrl(it.poster)" :alt="it.title" loading="lazy">
          <div v-else class="thumb-placeholder">🎬</div>
          <span class="play">▶ 播放</span>
          <span v-if="it.category" class="tag">{{ it.category }}</span>
          <span class="pin-badge">📌</span>
        </div>
        <figcaption>
          <div class="name" :title="it.title">{{ it.title }}</div>
          <div class="meta">{{ fmtDate(it.created_at) }} · {{ fmtSize(it.size) }}</div>
        </figcaption>
      </router-link>
    </div>
  </section>

  <main>
    <div v-if="err" class="empty">{{ err }}</div>
    <div v-else-if="loading" class="empty">加载中…</div>
    <div v-else-if="total === 0" class="empty">
      <p>{{ q ? '没有匹配「' + q + '」的片段' : '还没有视频' }}</p>
      <p v-if="isUpload()" class="sub">去 <router-link to="/upload">上传页</router-link> 拖进去，刷新即可。</p>
    </div>
    <template v-else>
      <div class="grid">
        <router-link v-for="it in items" :key="it.id" class="card" :to="'/play/' + it.id" :data-name="it.title.toLowerCase()">
          <div class="thumb">
            <img v-if="it.poster" :src="streamUrl(it.poster)" :alt="it.title" loading="lazy">
            <div v-else class="thumb-placeholder">🎬</div>
            <span class="play">▶ 播放</span>
            <span v-if="it.category" class="tag">{{ it.category }}</span>
            <span v-if="it.pinned" class="pin-badge">📌</span>
          </div>
          <figcaption>
            <div class="name" :title="it.title">{{ it.title }}</div>
            <div class="meta">{{ fmtDate(it.created_at) }} · {{ fmtSize(it.size) }}</div>
          </figcaption>
        </router-link>
      </div>

      <nav v-if="pages() > 1" class="pager">
        <template v-for="i in pages()" :key="i">
          <button v-if="pages() <= 9 || i === 1 || i === pages() || Math.abs(i - page) <= 3"
                  :class="{ cur: i === page }" @click="page = i">{{ i }}</button>
          <span v-else-if="Math.abs(i - page) === 4" class="dots">…</span>
        </template>
      </nav>
    </template>
  </main>
</template>
