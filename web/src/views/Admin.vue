<script setup>
import { ref, onMounted } from 'vue'
import { adminApi, streamFor, clearSession } from '../api'
import { useRouter } from 'vue-router'
const router = useRouter()
import ThemeToggle from '../components/ThemeToggle.vue'
import LogoutButton from '../components/LogoutButton.vue'

const items = ref([])
const cats = ref([])
const stats = ref(null)
const traffic = ref(null)
const tc = ref(null)
const msg = ref('')
const err = ref('')
const newViewPwd = ref('')
const newUploadPwd = ref('')
const newViewPwd2 = ref('')
const newUploadPwd2 = ref('')
const pwdErr = ref('')

function fmtSize(b) {
  if (b >= 1073741824) return (b / 1073741824).toFixed(1) + ' GB'
  if (b >= 1048576) return (b / 1048576).toFixed(1) + ' MB'
  return Math.max(1, Math.round(b / 1024)) + ' KB'
}

async function load() {
  try {
    const d = await adminApi.list()
    items.value = d.items
    cats.value = d.categories.map(c => ({ ...c, newName: c.c }))
    stats.value = d.stats
    traffic.value = (await adminApi.traffic()).traffic
    tc.value = (await adminApi.transcode()).transcode
  } catch (e) { err.value = e.message }
}
onMounted(load)

async function trafficToggle() {
  await act(() => adminApi.trafficToggle(!traffic.value.enabled), traffic.value.enabled ? '已关闭流量熔断' : '已开启流量熔断')
}
async function trafficReset() {
  if (!confirm('确定清零本月流量计数？')) return
  await act(() => adminApi.trafficReset(), '流量计数已清零')
}

// 转码画质快捷档（填充下面四个输入框，点「保存转码参数」才生效）
const tcPresets = [
  { name: '流畅 · 最省流量', maxrate_kbps: 800, fps: 25, crf: 31, edge_px: 1280 },
  { name: '标准 · 推荐', maxrate_kbps: 2000, fps: 30, crf: 26, edge_px: 1280 },
  { name: '高清 1080p · 吃带宽', maxrate_kbps: 3000, fps: 30, crf: 25, edge_px: 1920 },
]
function applyPreset(p) {
  Object.assign(tc.value, { maxrate_kbps: p.maxrate_kbps, fps: p.fps, crf: p.crf, edge_px: p.edge_px })
}
async function saveTranscode() {
  await act(() => adminApi.saveTranscode({
    maxrate_kbps: tc.value.maxrate_kbps, fps: tc.value.fps, crf: tc.value.crf, edge_px: tc.value.edge_px, max_mb: tc.value.max_mb,
  }), '转码参数已保存')
}

function trafficPct() {
  if (!traffic.value) return 0
  return Math.min(100, traffic.value.used_gb / traffic.value.limit_gb * 100)
}

// 改密码后 token 由双密码派生立即失效（含当前浏览器），提示后回登录页
function clearAndGo(message) {
  clearSession()
  alert(message)
  router.push('/login')
}

async function changeViewPwd() {
  pwdErr.value = ''
  if (newViewPwd.value.length < 4) { pwdErr.value = '观看密码至少 4 位'; return }
  if (newViewPwd.value !== newViewPwd2.value) { pwdErr.value = '两次输入的观看密码不一致'; return }
  try {
    const r = await adminApi.changePassword('view', newViewPwd.value)
    clearAndGo(r.message || '观看密码已修改，请用新密码重新登录')
  } catch (e) { pwdErr.value = e.message }
}

async function changeUploadPwd() {
  pwdErr.value = ''
  if (newUploadPwd.value.length < 4) { pwdErr.value = '上传/管理密码至少 4 位'; return }
  if (newUploadPwd.value !== newUploadPwd2.value) { pwdErr.value = '两次输入的密码不一致'; return }
  try {
    const r = await adminApi.changePassword('upload', newUploadPwd.value)
    clearAndGo(r.message || '上传/管理密码已修改，请用新密码重新登录')
  } catch (e) { pwdErr.value = e.message }
}

async function act(fn, okMsg) {
  msg.value = ''; err.value = ''
  try {
    const r = await fn()
    msg.value = r.message || okMsg
    await load()
  } catch (e) { err.value = e.message }
}

const newCat = ref('')
async function saveVideo(v) {
  await act(() => adminApi.save(v.id, v.title, v.category), '已保存')
}
async function delVideo(v) {
  if (!confirm(`删除《${v.title}》？文件也会一并删除，不可恢复。`)) return
  await act(() => adminApi.remove(v.id), '已删除')
}
async function renameCat(c) {
  await act(() => adminApi.catRename(c.c, c.newName ?? c.c))
  cats.value.forEach(x => { x.newName = undefined })
}
async function publishVideo(v) {
  if (!v.title.trim()) { err.value = '标题不能为空'; return }
  await act(() => adminApi.publish(v.id, v.title, v.category), '已发布上线')
}
async function clearCat(c) {
  if (!confirm(`清空「${c.c}」？${c.n} 个视频会移出分类（视频保留）。`)) return
  await act(() => adminApi.catClear(c.c))
}
</script>

<template>
  <header class="top">
    <router-link class="btn" to="/">← 首页</router-link>
    <h1>管理</h1>
    <span class="sp"></span>
    <router-link class="btn" to="/logs">访客记录</router-link>
    <LogoutButton />
    <ThemeToggle />
    <router-link class="btn" to="/upload">上传</router-link>
  </header>

  <main class="admin">
    <div v-if="msg" class="ok">{{ msg }}</div>
    <div v-if="err" class="err">{{ err }}</div>

    <section v-if="stats" class="stat-row">
      <div class="stat"><b>{{ stats.count }}</b><span>个视频</span></div>
      <div class="stat"><b>{{ fmtSize(stats.size) }}</b><span>占用空间</span></div>
      <div class="stat"><b>{{ stats.views }}</b><span>总播放</span></div>
      <div class="stat"><b>{{ stats.today_ips }}</b><span>今日访客 IP</span></div>
    </section>

    <section v-if="traffic" class="panel-block">
      <h2>本月流量 <small>仅统计视频播放流出；达到上限自动暂停播放，下月 1 日自动恢复</small></h2>
      <div v-if="traffic.blocked" class="err" style="margin: 0 0 12px">
        ⚠️ 已达 {{ traffic.limit_gb }} GB 上限，视频播放已熔断。可点下方按钮清零计数或暂停熔断。
      </div>
      <div class="traffic-line">
        <b>{{ traffic.used_gb.toFixed(2) }}</b> / {{ traffic.limit_gb }} GB（{{ trafficPct().toFixed(1) }}%）
        <span v-if="!traffic.enabled"> · 熔断已暂停（当前不限制）</span>
      </div>
      <div class="traffic-bar"><i :class="{ full: traffic.blocked }" :style="{ width: trafficPct() + '%' }"></i></div>
      <div class="traffic-actions">
        <button class="btn" @click="trafficToggle">{{ traffic.enabled ? '暂停熔断' : '恢复熔断' }}</button>
        <button class="btn" @click="trafficReset">清零本月计数</button>
      </div>
    </section>

    <section v-if="tc" class="panel-block">
      <h2>转码画质 <small>上传后自动压缩的规格，只影响之后上传的新视频；流量 ≈ 码率上限 × 观看时长</small></h2>
      <div class="traffic-actions">
        <button v-for="p in tcPresets" :key="p.name" class="btn" @click="applyPreset(p)">{{ p.name }}</button>
      </div>
      <div class="pwd-grid">
        <div class="pwd-item">
          <label>码率上限 kbps（流量主要取决于它，200–20000）</label>
          <input v-model.number="tc.maxrate_kbps" type="number" min="200" max="20000" step="100">
        </div>
        <div class="pwd-item">
          <label>帧率 fps（10–60，游戏画面建议 30）</label>
          <input v-model.number="tc.fps" type="number" min="10" max="60" step="1">
        </div>
        <div class="pwd-item">
          <label>质量 CRF（14–35，越小越清晰越耗流量）</label>
          <input v-model.number="tc.crf" type="number" min="14" max="35" step="1">
        </div>
        <div class="pwd-item">
          <label>长边分辨率（720p=1280，1080p=1920）</label>
          <input v-model.number="tc.edge_px" type="number" min="480" max="1920" step="80">
        </div>
        <div class="pwd-item">
          <label>单文件转码上限 MB（超过则跳过压缩、原样播出；0=不限制，200–8192）</label>
          <input v-model.number="tc.max_mb" type="number" min="0" max="8192" step="100">
        </div>
      </div>
      <div class="traffic-actions">
        <button class="btn" @click="saveTranscode">保存转码参数</button>
      </div>
    </section>

    <section class="panel-block">
      <details class="fold" :open="cats.length === 0">
        <summary><h2>分类管理 <small>{{ cats.length }} 个 · 改名同步更新归属视频，改成已有名即合并</small></h2></summary>
        <p v-if="!cats.length" class="tip">还没有分类。在下方视频管理里给视频填分类。</p>
        <div v-else class="table-wrap">
          <table>
            <tr><th>分类名</th><th>视频数</th><th>总播放</th><th colspan="2">操作</th></tr>
            <tr v-for="c in cats" :key="c.c">
              <td><input v-model="c.newName" class="cell-input" type="text" maxlength="60"></td>
              <td class="mono">{{ c.n }}</td>
              <td class="mono">{{ c.w }}</td>
              <td><button @click="renameCat(c)">保存改名</button></td>
              <td><button class="danger" @click="clearCat(c)">清空分类</button></td>
            </tr>
          </table>
        </div>
      </details>
    </section>

    <section class="panel-block">
      <h2>视频管理 <small>{{ items.length }} 个 · 点预览图新窗口播放</small></h2>
      <div class="table-wrap">
        <table>
          <tr><th>预览</th><th>标题</th><th>分类</th><th>状态</th><th>大小</th><th>播放</th><th>时间</th><th colspan="2">操作</th></tr>
          <tr v-for="v in items" :key="v.id">
            <td>
              <a class="thumb-mini" :href="'/play/' + v.id" target="_blank">
                <video :src="streamFor(v.fname)" preload="metadata" muted playsinline></video>
              </a>
            </td>
            <td><input v-model="v.title" class="cell-input" type="text" maxlength="120"></td>
            <td><input v-model="v.category" class="cell-input" type="text" maxlength="60" :list="'cats-' + v.id"></td>
            <td><span :class="v.published ? 'dim' : 'err'">{{ v.published ? '已发布' : '未发布' }}</span></td>
            <td class="mono">{{ fmtSize(v.size) }}</td>
            <td class="mono">{{ v.views }}</td>
            <td class="mono">{{ v.created_at }}</td>
            <td><button v-if="!v.published" @click="publishVideo(v)">发布</button><button v-else @click="saveVideo(v)">保存</button></td>
            <td><button class="danger" @click="delVideo(v)">删除</button></td>
          </tr>
        </table>
        <datalist v-for="v in items" :key="'dl' + v.id" :id="'cats-' + v.id">
          <option v-for="c in cats" :key="c.c" :value="c.c"></option>
        </datalist>
      </div>
    </section>

    <section class="panel-block">
      <details class="fold">
        <summary><h2>密码管理 <small>改完立即生效，所有登录（含当前浏览器）失效，需用新密码重新登录</small></h2></summary>
        <div v-if="pwdErr" class="err" style="margin: 0 0 12px">{{ pwdErr }}</div>
        <div class="pwd-grid">
          <div class="pwd-item">
            <label>观看密码（访客进入本站用）</label>
            <input v-model="newViewPwd" type="password" maxlength="64" autocomplete="new-password" placeholder="新密码（至少 4 位）">
            <input v-model="newViewPwd2" type="password" maxlength="64" autocomplete="new-password" placeholder="再输一次确认">
            <button @click="changeViewPwd">修改观看密码</button>
          </div>
          <div class="pwd-item">
            <label>上传 / 管理密码（上传与管理入口用）</label>
            <input v-model="newUploadPwd" type="password" maxlength="64" autocomplete="new-password" placeholder="新密码（至少 4 位）">
            <input v-model="newUploadPwd2" type="password" maxlength="64" autocomplete="new-password" placeholder="再输一次确认">
            <button @click="changeUploadPwd">修改上传密码</button>
          </div>
        </div>
      </details>
    </section>
  </main>
</template>
