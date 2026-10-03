package main

import (
	"log"
	"os"
	"strconv"
)

// Config 全部来自环境变量（.env / compose 注入），与 PHP 版 config.php 一一对应
type Config struct {
	Port           string
	ViewPassword   string
	UploadPassword string
	VideoDir       string
	ChunksDir      string
	PageSize       int
	AllowedExt     []string
	ChunkMB        int
	ChunkTTLHours  int
	TokenTTLDays   int
	AllowOrigin    string
	IP2RegionXDB   string
	DBName         string
	TrafficLimitGB int
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func loadConfig() *Config {
	c := &Config{
		Port:           getenv("PORT", "8080"),
		ViewPassword:   getenv("VIEW_PASSWORD", "view123"),
		UploadPassword: getenv("UPLOAD_PASSWORD", "upload123"),
		VideoDir:       getenv("VIDEO_DIR", "/app/videos"),
		ChunksDir:      getenv("CHUNKS_DIR", "/app/chunks"),
		PageSize:       getenvInt("PAGE_SIZE", 60),
		ChunkMB:        getenvInt("CHUNK_MB", 4),
		ChunkTTLHours:  getenvInt("CHUNK_TTL_HOURS", 24),
		TokenTTLDays:   getenvInt("TOKEN_TTL_DAYS", 30),
		AllowOrigin:    os.Getenv("ALLOW_ORIGIN"),
		IP2RegionXDB:   getenv("IP2REGION_XDB", "/app/data/ip2region.xdb"),
		DBName:         getenv("DB_NAME", "MyVideos"),
		TrafficLimitGB: getenvInt("TRAFFIC_LIMIT_GB", 280),
	}
	for _, e := range splitCSV(getenv("ALLOWED_EXT", "mp4,webm,m4v,mov,mkv")) {
		if e != "" {
			c.AllowedExt = append(c.AllowedExt, e)
		}
	}
	for _, d := range []string{c.VideoDir, c.ChunksDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			log.Fatalf("创建目录失败 %s: %v", d, err)
		}
	}
	return c
}

func splitCSV(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			part := trimSpace(s[start:i])
			out = append(out, part)
			start = i + 1
		}
	}
	return out
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
