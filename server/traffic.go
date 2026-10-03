package main

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// TrafficCounter 月度流量计数器（只计视频流出字节）：
//
//	计数持久化到 data/traffic.json，每月 1 日自动清零；
//	达到阈值（TRAFFIC_LIMIT_GB，默认 280）后视频流熔断（返回 503），
//	管理端可查看用量、手动开/关限制、手动清零。
type TrafficCounter struct {
	mu      sync.Mutex
	path    string
	limitGB int
	month   string // "2026-10"，跨月自动清零
	bytes   int64
	enabled bool // 熔断开关（管理员可关）
}

func currentMonth() string { return time.Now().Format("2006-01") }

func NewTrafficCounter(path string, limitGB int) *TrafficCounter {
	tc := &TrafficCounter{path: path, limitGB: limitGB, enabled: true, month: currentMonth()}
	if raw, err := os.ReadFile(path); err == nil {
		var d struct {
			Month   string `json:"month"`
			Bytes   int64  `json:"bytes"`
			Enabled *bool  `json:"enabled"`
		}
		if json.Unmarshal(raw, &d) == nil {
			if d.Month == tc.month {
				tc.bytes = d.Bytes
			}
			if d.Enabled != nil {
				tc.enabled = *d.Enabled
			}
		}
	}
	return tc
}

func (t *TrafficCounter) save() {
	raw, _ := json.Marshal(map[string]any{"month": t.month, "bytes": t.bytes, "enabled": t.enabled})
	tmp := t.path + ".tmp"
	if os.WriteFile(tmp, raw, 0o644) == nil {
		os.Rename(tmp, t.path)
	}
}

// rollover 跨月自动清零
func (t *TrafficCounter) rollover() {
	if m := currentMonth(); m != t.month {
		t.month = m
		t.bytes = 0
		t.save()
	}
}

// Add 累加视频流出字节（每次 stream 响应后调用）
func (t *TrafficCounter) Add(n int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.rollover()
	t.bytes += n
	t.save()
}

// Exceeded 是否达到熔断阈值（管理员关闭限制时永不触发）
func (t *TrafficCounter) Exceeded() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.rollover()
	return t.enabled && t.bytes >= int64(t.limitGB)*1024*1024*1024
}

// Status 管理端展示用
func (t *TrafficCounter) Status() map[string]any {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.rollover()
	blocked := t.enabled && t.bytes >= int64(t.limitGB)*1024*1024*1024
	return map[string]any{
		"used_bytes": t.bytes,
		"used_gb":    float64(t.bytes) / 1073741824,
		"limit_gb":   t.limitGB,
		"enabled":    t.enabled,
		"blocked":    blocked,
	}
}

func (t *TrafficCounter) SetEnabled(on bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.enabled = on
	t.save()
}

func (t *TrafficCounter) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.bytes = 0
	t.save()
}

// LimitGB 供熔断提示文案读取
func (t *TrafficCounter) LimitGB() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.limitGB
}
