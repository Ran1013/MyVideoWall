package main

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// Handler 聚合配置与存储，供全部端点方法使用
type Handler struct {
	cfg     *Config
	store   *Store
	traffic *TrafficCounter
	pw      *PasswordStore
	tc      *TranscodeStore
}

func registerAPI(r *gin.Engine, cfg *Config, store *Store, traffic *TrafficCounter, pw *PasswordStore, tc *TranscodeStore) {
	h := &Handler{cfg: cfg, store: store, traffic: traffic, pw: pw, tc: tc}

	api := r.Group("/api")
	{
		api.GET("/health", healthHandler)
		api.POST("/auth.php", h.authHandler)

		videos := api.Group("/videos.php", requireView(pw))
		videos.GET("", h.videosHandler)
		videos.POST("", h.videosHandler)

		api.GET("/stream.php", requireView(pw), h.streamHandler)

		api.POST("/upload.php", requireUpload(pw), h.uploadHandler)

		admin := api.Group("/admin.php", requireUpload(pw))
		admin.GET("", h.adminHandler)
		admin.POST("", h.adminHandler)

		logs := api.Group("/logs.php", requireUpload(pw))
		logs.GET("", h.logsHandler)
		logs.POST("", h.logsHandler)
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func randomHex16() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func cleanBase(orig string) string {
	base := filepath.Base(orig)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	replacer := strings.NewReplacer(
		"\\", "_", "/", "_", ":", "_", "*", "_", "?", "_",
		"\"", "_", "<", "_", ">", "_", "|", "_",
	)
	base = replacer.Replace(base)
	// 去控制字符
	clean := strings.Map(func(r rune) rune {
		if r < 0x20 {
			return '_'
		}
		return r
	}, base)
	clean = strings.TrimSpace(clean)
	if clean == "" {
		clean = "clip"
	}
	return truncateRunes(clean, 80)
}

func defaultStr(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func atoi(s string) int {
	n := 0
	neg := false
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	}
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0
		}
		n = n*10 + int(ch-'0')
	}
	if neg {
		return -n
	}
	return n
}

func (h *Handler) logFilter(c *gin.Context) map[string]string {
	f := map[string]string{}
	if q := strings.TrimSpace(c.Query("q")); q != "" {
		f["q"] = q
	}
	if a := c.Query("action"); a == "play" || a == "visit" || a == "upload" || a == "login_fail" {
		f["action"] = a
	}
	if r := c.Query("range"); r == "today" || r == "week" {
		f["range"] = r
	}
	return f
}

// videosView 的文件存在检查（库里有记录且硬盘上有文件）
func (h *Handler) videoFileExists(id int64) bool {
	v, err := h.store.VideoGet(id)
	if err != nil || v == nil {
		return false
	}
	return fileExists(filepath.Join(h.cfg.VideoDir, v.Fname))
}

// validUploadID：16 位十六进制，杜绝路径注入
func validUploadID(id string) bool {
	if len(id) != 16 {
		return false
	}
	for _, ch := range id {
		ok := (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')
		if !ok {
			return false
		}
	}
	return true
}
