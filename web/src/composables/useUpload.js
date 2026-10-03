// 分片上传（断点续传/重试/暂停）——从 v2 app.js 移植为 composable
import { reactive } from 'vue'
import { uploadApi, adminApi } from '../api'

const sleep = ms => new Promise(res => setTimeout(res, ms))

export function useUpload() {
  const rows = reactive([])  // {name, pct, state, stateText, paused, file, uploadId}

  function updateRow(row, patch) { Object.assign(row, patch) }

  async function uploadOne(row) {
    const file = row.file
    row.pct = 0
    row.state = 'uploading'

    let paused = false
    row.togglePause = () => {
      paused = !paused
      row.paused = paused
    }
    const waitResume = () => new Promise(res => {
      (function check() { paused ? setTimeout(check, 200) : res() })()
    })

    const CHUNK = row.chunkSize
    const total = Math.ceil(file.size / CHUNK)
    const receivedMap = {}

    try {
      const init = await uploadApi.init(file.name, file.size, total)
      if (!init.ok) throw new Error(init.error || '初始化失败')
      row.uploadId = init.upload_id
      ;(init.received || []).forEach(i => { receivedMap[i] = true })
      if (init.resumed && Object.keys(receivedMap).length > 0) {
        row.stateText = `检测到未完成的传输，续传中 ${Object.keys(receivedMap).length}/${total}`
      } else {
        row.stateText = `0/${total} 片`
      }

      const sendChunk = async i => {
        const blob = file.slice(i * CHUNK, Math.min((i + 1) * CHUNK, file.size))
        const r = await uploadApi.chunk(row.uploadId, i, blob)
        if (!r.ok) throw new Error(r.error || '分片上传失败')
        receivedMap[i] = true
        row.pct = Math.round(Object.keys(receivedMap).length / total * 100)
        if (!paused) row.stateText = `${Object.keys(receivedMap).length}/${total} 片`
      }

      const retryChunk = async (i, round) => {
        try {
          await sendChunk(i)
        } catch (e) {
          if (round >= 3 || /密码|过期|格式|不支持|会话/.test(e.message || '')) throw e
          await sleep(1000 * Math.pow(2, round - 1))
          await retryChunk(i, round + 1)
        }
      }

      for (let i = 0; i < total; i++) {
        if (receivedMap[i]) continue
        await waitResume()
        await retryChunk(i, 1)
      }

      row.stateText = '服务器压缩中，请稍候…'
      row.pct = 100
      const title = file.name.replace(/\.[^.]+$/, '')
      // 分类读取行内控件的实时值：上传前/上传中/传完选都算数
      const fin = await uploadApi.finish(row.uploadId, title, effectiveCat(row))
      if (!fin.ok) throw new Error(fin.error || '合并失败')
      row.state = 'done'
      row.stateText = '✓ 完成'
      row.pct = 100
      row.videoId = fin.id
      row.title = title
      row.savedCat = effectiveCat(row)
      if (row.savedCat) row.catMsg = '分类「' + row.savedCat + '」已随视频保存'
      if (!fin.ok) throw new Error(fin.error || '合并失败')
      row.state = 'done'
      row.stateText = '✓ 完成'
      row.pct = 100
    } catch (e) {
      row.state = 'fail'
      row.stateText = '✗ ' + (e.message || '失败')
    }
  }

// 允许的扩展名（与服务端 ALLOWED_EXT 默认值一致；服务端仍会再校验）
const ALLOWED_EXT = ['mp4', 'webm', 'm4v', 'mov', 'mkv']

function extOf(name) {
  const m = /\.([a-z0-9]+)$/i.exec(name || '')
  return m ? m[1].toLowerCase() : ''
}

function failRow(f, msg) {
  rows.unshift(reactive({
    name: f && f.name ? f.name : '(未命名文件)', file: f,
    catPick: '', newCat: '', catMsg: '', savedCat: '',
    pct: 0, state: 'fail', stateText: '✗ ' + msg, paused: false,
    togglePause: () => {}, uploadId: '',
  }))
}

async function addFiles(files, chunkSize) {
    for (const f of files) {
      // 坏文件前置拦截：0 字节 / 扩展名不在白名单，直接标失败，不发起请求
      if (!f.size) {
        failRow(f, '文件大小为 0（可能选择失败或文件损坏），请重选')
        continue
      }
      const ext = extOf(f.name)
      if (!ALLOWED_EXT.includes(ext)) {
        failRow(f, '不支持的格式 .' + (ext || '未知') + '（允许：' + ALLOWED_EXT.join(',') + '）')
        continue
      }
      const row = reactive({
        name: f.name, file: f, catPick: '', newCat: '', catMsg: '', savedCat: '', chunkSize,
        pct: 0, state: 'waiting', stateText: '准备中…', paused: false,
        togglePause: () => {}, uploadId: '', videoId: 0, title: '',
      })
      rows.unshift(row)
      await uploadOne(row)
    }
  }

// 行内分类控件的当前取值（下拉选已有 / 选「＋新建」后输入）
function effectiveCat(row) {
  return (row.catPick === '__new__' ? (row.newCat || '') : (row.catPick || '')).trim()
}

// 保存分类：已完成的上传直接调管理接口更新；上传中的会被 finish 自动带上
// 返回成功保存的分类名（清空返回空串），失败返回 null
async function saveRowCategory(row) {
  const cat = effectiveCat(row)
  if (row.catPick === '__new__' && !cat) {
    row.catMsg = '✗ 请先输入新分类名'
    return null
  }
  if (row.state !== 'done' || !row.videoId) {
    row.catMsg = cat ? ('上传完成后将自动保存「' + cat + '」') : '未选择分类'
    return null
  }
  try {
    await adminApi.save(row.videoId, row.title || row.name.replace(/\.[^.]+$/, ''), cat)
    row.savedCat = cat
    row.catMsg = cat ? ('已保存「' + cat + '」') : '分类已清空'
    return cat
  } catch (e) {
    row.catMsg = '✗ 保存失败：' + (e.message || '')
    return null
  }
}

  function retry(row) {
    row.state = 'waiting'
    uploadOne(row)
  }

  return { rows, addFiles, retry, saveRowCategory }
}
