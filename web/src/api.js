// API 前缀：dev 与 embed 同源模式都是 '/api'；CF Pages 用 VITE_API_BASE 指向后端源站
const BASE = import.meta.env.VITE_API_BASE || '/api'

const TOKEN_KEY = 'vw_token'
const ROLE_KEY = 'vw_role'

export function getToken() { return localStorage.getItem(TOKEN_KEY) || '' }
export function getRole() { return localStorage.getItem(ROLE_KEY) || '' }
export function isUpload() { return getRole() === 'upload' }
export function saveSession(token, role) {
  localStorage.setItem(TOKEN_KEY, token)
  localStorage.setItem(ROLE_KEY, role)
}
export function clearSession() {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(ROLE_KEY)
}

export class ApiError extends Error {
  constructor(message, status) { super(message); this.status = status }
}

async function request(path, { method = 'GET', body, raw = false } = {}) {
  const headers = {}
  const token = getToken()
  if (token) headers.Authorization = 'Bearer ' + token
  if (body && !(body instanceof FormData)) headers['Content-Type'] = 'application/json'

  const res = await fetch(BASE + path, {
    method,
    headers,
    body: body instanceof FormData ? body : body ? JSON.stringify(body) : undefined,
  })

  if (raw) return res  // 视频流等场景由调用方处理

  let data = null
  try { data = await res.json() } catch { /* 非 JSON */ }
  if (res.status === 401) {
    clearSession()
    throw new ApiError(data?.error || '登录已过期，请重新输入密码', 401)
  }
  if (!res.ok || !data || data.ok === false) {
    throw new ApiError(data?.error || '请求失败（' + res.status + '）', res.status)
  }
  return data
}

/* ---------- 认证 ---------- */
export function login(password) {
  return request('/auth.php', { method: 'POST', body: { password } })
}

/* ---------- 视频 ---------- */
export function videoList({ q = '', cat = '', sort = 'new', p = 1 } = {}) {
  const qs = new URLSearchParams({ act: 'list', q, cat, sort, p })
  return request('/videos.php?' + qs)
}
export function videoGet(id) {
  return request('/videos.php?act=get&id=' + id)
}
export function videoMarkViewed(id) {
  return request('/videos.php?act=view', { method: 'POST', body: { id } })
}
export function streamUrl(backendUrl) {
  // 后端返回的 url/poster 形如 "stream.php?f=..&t=.."，拼上 API 前缀
  return BASE + '/' + backendUrl
}

// 用裸文件名直接构造流地址（管理页等后端未拼 url 的场景）
export function streamFor(fname) {
  return BASE + '/stream.php?f=' + encodeURIComponent(fname) + '&t=' + getToken()
}

/* ---------- 上传（upload 角色） ---------- */
export const uploadApi = {
  init: (name, size, chunks) => request('/upload.php?act=init', { method: 'POST', body: { name, size, chunks } }),
  status: (upload_id) => request('/upload.php?act=status', { method: 'POST', body: { upload_id } }),
  chunk: (upload_id, index, blob) => {
    const fd = new FormData()
    fd.append('upload_id', upload_id)
    fd.append('index', index)
    fd.append('file', blob, 'blob')
    return request('/upload.php?act=chunk', { method: 'POST', body: fd })
  },
  finish: (upload_id, title, cat) => request('/upload.php?act=finish', { method: 'POST', body: { upload_id, title, cat } }),
}

/* ---------- 管理（upload 角色） ---------- */
export const adminApi = {
  list: () => request('/admin.php?act=list'),
  save: (id, title, category) => request('/admin.php?act=save', { method: 'POST', body: { id, title, category } }),
  remove: (id) => request('/admin.php?act=delete', { method: 'POST', body: { id } }),
  categories: () => request('/admin.php?act=categories'),
  catRename: (old, new_) => request('/admin.php?act=cat-rename', { method: 'POST', body: { old, new: new_ } }),
  catClear: (cat) => request('/admin.php?act=cat-clear', { method: 'POST', body: { cat } }),
  traffic: () => request('/admin.php?act=traffic'),
  trafficToggle: (enabled) => request('/admin.php?act=traffic-toggle', { method: 'POST', body: { enabled } }),
  trafficReset: () => request('/admin.php?act=traffic-reset', { method: 'POST', body: {} }),
  transcode: () => request('/admin.php?act=transcode'),
  saveTranscode: (p) => request('/admin.php?act=transcode', { method: 'POST', body: p }),
  changePassword: (type, password) => request('/admin.php?act=password-' + type, { method: 'POST', body: { password } }),
}

/* ---------- 访客记录（upload 角色） ---------- */
export const logsApi = {
  stats: () => request('/logs.php?act=stats'),
  byIp: (f = {}) => request('/logs.php?act=byip&' + new URLSearchParams(f)),
  recent: (f = {}) => request('/logs.php?act=recent&' + new URLSearchParams(f)),
  clear: () => request('/logs.php?act=clear', { method: 'POST', body: {} }),
}
