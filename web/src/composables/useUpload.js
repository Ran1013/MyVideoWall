// 分片上传（断点续传/重试/暂停）——从 v2 app.js 移植为 composable
import { reactive } from 'vue'
import { uploadApi } from '../api'

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
      const fin = await uploadApi.finish(row.uploadId, file.name.replace(/\.[^.]+$/, ''), row.category)
      if (!fin.ok) throw new Error(fin.error || '合并失败')
      row.state = 'done'
      row.stateText = '✓ 完成'
      row.pct = 100
    } catch (e) {
      row.state = 'fail'
      row.stateText = '✗ ' + (e.message || '失败')
    }
  }

  async function addFiles(files, category, chunkSize) {
    for (const f of files) {
      const row = reactive({
        name: f.name, file: f, category, chunkSize,
        pct: 0, state: 'waiting', stateText: '准备中…', paused: false,
        togglePause: () => {}, uploadId: '',
      })
      rows.unshift(row)
      await uploadOne(row)
    }
  }

  function retry(row) {
    row.state = 'waiting'
    uploadOne(row)
  }

  return { rows, addFiles, retry }
}
