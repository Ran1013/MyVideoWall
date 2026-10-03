<script setup>
import { ref, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { videoGet, videoMarkViewed, streamUrl } from '../api'
import ThemeToggle from '../components/ThemeToggle.vue'
import LogoutButton from '../components/LogoutButton.vue'

const route = useRoute()
const router = useRouter()
const video = ref(null)
const prev = ref(null)
const next = ref(null)
const related = ref([])
const err = ref('')
const blocked = ref(false)
const sort = ref(route.query.sort === 'hot' ? 'hot' : 'new')

function fmtSize(b) {
  if (!b) return ''
  if (b >= 1073741824) return (b / 1073741824).toFixed(1) + ' GB'
  if (b >= 1048576) return (b / 1048576).toFixed(1) + ' MB'
  return Math.max(1, Math.round(b / 1024)) + ' KB'
}

async function load() {
  err.value = ''
  try {
    const id = Number(route.params.id)
    const d = await videoGet(id)
    blocked.value = !!d.trafficBlocked
    video.value = d.video
    prev.value = d.prev
    next.value = d.next
    related.value = d.related
    await videoMarkViewed(id)
  } catch (e) {
    err.value = e.message
  }
}
onMounted(load)
watch(() => route.params.id, load)
</script>

<template>
  <header class="top">
    <router-link class="btn" to="/">← 首页</router-link>
    <h1>我的游戏高光</h1>
    <span class="sp"></span>
    <LogoutButton />
    <ThemeToggle />
  </header>

  <main class="watch">
    <div v-if="blocked" class="err" style="text-align:center;max-width:640px;margin:20px auto">
      ⚠️ 本月视频流量已用完，播放暂停。管理员可在管理页查看用量，下月 1 日自动恢复。
    </div>
    <div v-if="err" class="empty">{{ err }} <router-link to="/">回首页</router-link></div>
    <template v-else-if="video && !blocked">
      <div class="player-box">
        <video id="pv" :src="streamUrl(video.url)" :poster="video.poster ? streamUrl(video.poster) : undefined"
               controls playsinline autoplay preload="auto"></video>

        <div class="watch-info">
          <div class="watch-title">
            <h2>{{ video.title }}</h2>
            <router-link v-if="video.category" class="badge" to="/" @click="router.push({ name: 'wall', query: { cat: video.category } })">
              {{ video.category }}
            </router-link>
          </div>
          <div class="watch-meta">
            <span>{{ video.created_at }}</span>
            <span>{{ fmtSize(video.size) }}</span>
          </div>
          <div class="watch-nav">
            <router-link v-if="prev" class="btn" :to="'/play/' + prev.id">← 上一部</router-link>
            <span v-else></span>
            <router-link v-if="next" class="btn" :to="'/play/' + next.id">下一部 →</router-link>
            <span v-else></span>
          </div>
        </div>
      </div>

      <section v-if="related.length && !blocked" class="related">
        <h3>继续看</h3>
        <div class="grid">
          <router-link v-for="r in related" :key="r.id" class="card" :to="'/play/' + r.id">
            <div class="thumb">
              <img v-if="r.poster" :src="streamUrl(r.poster)" :alt="r.title" loading="lazy">
              <div v-else class="thumb-placeholder">🎬</div>
              <span class="play">▶</span>
            </div>
            <figcaption>
              <div class="name" :title="r.title">{{ r.title }}</div>
              <div class="meta">{{ r.category }}</div>
            </figcaption>
          </router-link>
        </div>
      </section>
    </template>
    <div v-else class="empty">加载中…</div>
  </main>
</template>
