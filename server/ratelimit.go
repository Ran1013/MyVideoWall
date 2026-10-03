package main

import (
	"sync"
	"time"
)

// FailLimiter 登录失败限速：同一 IP 一分钟内失败 5 次即封锁 1 分钟。
// 防止弱密码（如 4 位数字）被扫描器爆破。内存态，重启清零可接受。
type FailLimiter struct {
	mu    sync.Mutex
	fails map[string][]time.Time
	max   int
}

var authFails = &FailLimiter{fails: map[string][]time.Time{}, max: 5}

func (l *FailLimiter) cleanup(ip string, now time.Time) []time.Time {
	keep := l.fails[ip][:0]
	for _, t := range l.fails[ip] {
		if now.Sub(t) < time.Minute {
			keep = append(keep, t)
		}
	}
	l.fails[ip] = keep
	return keep
}

// Blocked 该 IP 是否处于封锁期（1 分钟内失败 >= max 次）
func (l *FailLimiter) Blocked(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.cleanup(ip, time.Now())) >= l.max
}

// Record 记录一次失败
func (l *FailLimiter) Record(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fails[ip] = append(l.cleanup(ip, time.Now()), time.Now())
}

// Clear 登录成功后清除该 IP 的失败记录
func (l *FailLimiter) Clear(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, ip)
}
