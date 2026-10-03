<script setup>
import { ref, onMounted } from 'vue'
import { logsApi } from '../api'
import ThemeToggle from '../components/ThemeToggle.vue'
import LogoutButton from '../components/LogoutButton.vue'

const stats = ref(null)
const total = ref(0)
const byIp = ref([])
const recent = ref([])
const q = ref('')
const action = ref('')
const range = ref('')
const msg = ref('')
const loading = ref(true)

async function load() {
  loading.value = true
  const f = { ...(q.value ? { q: q.value } : {}), ...(action.value ? { action: action.value } : {}), ...(range.value ? { range: range.value } : {}) }
  try {
    const [s, ip, rc] = await Promise.all([logsApi.stats(), logsApi.byIp(f), logsApi.recent(f)])
    stats.value = s.stats
    total.value = s.total
    byIp.value = ip.items
    recent.value = rc.items
  } catch (e) { msg.value = '' } finally { loading.value = false }
}
onMounted(load)

function reset() { q.value = ''; action.value = ''; range.value = ''; load() }
async function clearAll() {
  if (!confirm('确定清空全部访问记录？')) return
  await logsApi.clear()
  load()
}
</script>

<template>
  <header class="top">
    <router-link class="btn" to="/admin">← 管理</router-link>
    <h1>访客记录</h1>
    <span class="sp"></span>
    <router-link class="btn ghost" to="/">首页</router-link>
    <LogoutButton />
    <ThemeToggle />
  </header>

  <main class="admin">
    <section class="panel-block">
      <h2>访客记录 <small>共 {{ total }} 条 · 按 IP 聚合，归属地为离线库查询结果</small></h2>

      <form class="filter-bar" @submit.prevent="load">
        <input v-model="q" type="search" placeholder="搜 IP 或归属地，如 223. 或 深圳">
        <select v-model="action">
          <option value="">全部动作</option>
          <option value="play">看视频</option>
          <option value="visit">访问站点</option>
          <option value="upload">上传</option>
          <option value="login_fail">密码错误</option>
        </select>
        <select v-model="range">
          <option value="">全部时间</option>
          <option value="today">今天</option>
          <option value="week">近 7 天</option>
        </select>
        <button class="go" type="submit">筛选</button>
        <a class="filter-clear" href="#" @click.prevent="reset">清除</a>
      </form>

      <p v-if="!byIp.length && !loading" class="tip">没有匹配筛选条件的访客记录。</p>
      <div v-else class="table-wrap">
        <table>
          <tr><th>IP</th><th>归属地</th><th>访问次数</th><th>最近访问</th></tr>
          <tr v-for="r in byIp" :key="r.ip">
            <td class="mono">{{ r.ip }}</td>
            <td><span v-if="r.region">{{ r.region }}</span><span v-else class="dim">未知</span></td>
            <td>{{ r.n }}</td>
            <td class="mono">{{ r.last }}</td>
          </tr>
        </table>
      </div>
    </section>

    <section class="panel-block">
      <h2>明细 <small>最近 200 条（随筛选变化）</small></h2>
      <p v-if="!recent.length && !loading" class="tip">没有匹配筛选条件的明细。</p>
      <div v-else class="table-wrap">
        <table>
          <tr><th>时间</th><th>IP</th><th>归属地</th><th>动作</th><th>视频</th></tr>
          <tr v-for="r in recent" :key="r.id">
            <td class="mono">{{ r.ts }}</td>
            <td class="mono">{{ r.ip }}</td>
            <td><span v-if="r.region">{{ r.region }}</span><span v-else class="dim">未知</span></td>
            <td>{{ r.action }}</td>
            <td class="mono">{{ r.video_id ? '#' + r.video_id : '—' }}</td>
          </tr>
        </table>
      </div>
      <div class="mini" style="text-align: right; margin-top: 14px;">
        <button @click="clearAll">清空访问记录</button>
      </div>
    </section>
  </main>
</template>
