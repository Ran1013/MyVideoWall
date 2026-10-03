package main

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
)

// TranscodeStore 转码画质参数：env 是初始默认值，管理端修改后持久化到 data/transcode.json（热生效）。
// 码率上限是流量消耗的主导项（流量 ≈ 码率 × 观看时长）；ffmpeg 的 bufsize 固定取码率的 2 倍。
type TranscodeStore struct {
	mu   sync.RWMutex
	path string
	p    TranscodeParams
}

type TranscodeParams struct {
	EdgePx   int `json:"edge_px"`      // 长边分辨率上限（720p=1280，1080p=1920）
	Fps      int `json:"fps"`          // 帧率
	Crf      int `json:"crf"`          // 质量基准（越小越清晰越耗流量）
	MaxrateK int `json:"maxrate_kbps"` // 码率上限 kbps
	MaxMB    int `json:"max_mb"`       // 单文件转码上限 MB（超过跳过压缩原样播出；0=不限制）
}

// 文件里出现越界值时的兜底范围
func saneTranscode(p TranscodeParams) bool {
	return p.EdgePx >= 480 && p.EdgePx <= 1920 &&
		p.Fps >= 10 && p.Fps <= 60 &&
		p.Crf >= 14 && p.Crf <= 35 &&
		p.MaxrateK >= 200 && p.MaxrateK <= 20000 &&
		(p.MaxMB == 0 || (p.MaxMB >= 200 && p.MaxMB <= 8192))
}

func NewTranscodeStore(path string) *TranscodeStore {
	t := &TranscodeStore{
		path: path,
		p: TranscodeParams{
			EdgePx:   targetEdgePx,
			Fps:      getenvInt("TRANSCODE_FPS", 30),
			Crf:      getenvInt("TRANSCODE_CRF", 28),
			MaxrateK: parseKbps(getenv("TRANSCODE_MAXRATE", "1200k")),
			MaxMB:    getenvInt("TRANSCODE_MAX_MB", 4096),
		},
	}
	if raw, err := os.ReadFile(path); err == nil {
		var d TranscodeParams
		if json.Unmarshal(raw, &d) == nil && saneTranscode(d) {
			t.p = d
		}
	}
	return t
}

func (t *TranscodeStore) Get() TranscodeParams {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.p
}

// Set 热生效并持久化（入参需先通过接口层的范围校验）
func (t *TranscodeStore) Set(p TranscodeParams) TranscodeParams {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.p = p
	raw, _ := json.MarshalIndent(p, "", "  ")
	tmp := t.path + ".tmp"
	if os.WriteFile(tmp, raw, 0o644) == nil {
		os.Rename(tmp, t.path)
	}
	return t.p
}

// parseKbps "1200k" → 1200
func parseKbps(s string) int {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimSuffix(s, "k")
	return atoi(s)
}
