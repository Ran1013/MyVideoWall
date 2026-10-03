package main

import (
	"bufio"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed all:web-dist
var webDist embed.FS

//go:embed data/ip2region.xdb
var embeddedXDB embed.FS

// ensureXDB 数据目录缺 xdb 时从二进制内置副本解出（挂载卷为空时自愈）
func ensureXDB(path string) {
	if _, err := os.Stat(path); err == nil {
		return
	}
	os.MkdirAll(filepath.Dir(path), 0o755)
	if data, err := embeddedXDB.ReadFile("data/ip2region.xdb"); err == nil {
		if err := os.WriteFile(path, data, 0o644); err == nil {
			log.Printf("已从内置副本恢复 IP 归属地库: %s", path)
		}
	}
}

func main() {
	loadEnvFile(".env")
	cfg := loadConfig()

	// MySQL（宿主机宝塔库）
	store, err := NewStore(getenv("MYSQL_DSN", ""), cfg.DBName)
	if err != nil {
		log.Fatalf("MySQL 连接失败（检查 .env 的 MYSQL_DSN 与宝塔库是否运行）: %v", err)
	}
	defer store.DB.Close()
	log.Printf("MySQL 已连接，库 %s", cfg.DBName)

	// ip2region 归属地（可选：xdb 缺失时归属地显示未知，不影响主流程）
	initIP2Region(cfg.IP2RegionXDB)

	// 数据目录自愈：挂载卷里缺 xdb 时从内置副本补齐（容器重建不丢能力）
	ensureXDB(getenv("IP2REGION_XDB", "/app/data/ip2region.xdb"))

	// 月度流量熔断（默认 280GB，环境变量可调）
	traffic := NewTrafficCounter(getenv("TRAFFIC_FILE", "/app/data/traffic.json"), getenvInt("TRAFFIC_LIMIT_GB", 280))

	// 站点密码（env 为初始值，管理端修改后持久化到 data/config.json 热生效）
	pw := NewPasswordStore(getenv("PASSWORDS_FILE", "/app/data/config.json"), cfg.ViewPassword, cfg.UploadPassword)

	// 转码画质参数（env 为初始默认，管理端可调，持久化到 data/transcode.json 热生效）
	tc := NewTranscodeStore(getenv("TRANSCODE_FILE", "/app/data/transcode.json"))

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(corsMiddleware(cfg))

	registerAPI(r, cfg, store, traffic, pw, tc)
	registerStatic(r, cfg)

	log.Printf("视频站启动：http://localhost:%s（API /api/*，前端 embed 托管）", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}

func corsMiddleware(cfg *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := cfg.AllowOrigin
		if origin == "" {
			origin = c.GetHeader("Origin")
			if origin == "" {
				origin = "*"
			}
		}
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		c.Header("Access-Control-Max-Age", "86400")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// registerStatic：embed 的 Vue 前端 + SPA fallback（/api/* 未匹配返回 JSON 404）
func registerStatic(r *gin.Engine, cfg *Config) {
	dist, err := fs.Sub(webDist, "web-dist")
	if err != nil {
		return
	}
	if _, err := fs.Stat(dist, "index.html"); err != nil {
		log.Println("web-dist 无 index.html，跳过前端托管（仅 API 模式）")
		return
	}
	fileServer := http.FileServer(http.FS(dist))

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/") || path == "/api" || strings.HasPrefix(path, "/stream.php") {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "not found"})
			return
		}
		clean := strings.TrimPrefix(path, "/")
		if clean == "" {
			clean = "index.html"
		}
		if _, err := fs.Stat(dist, clean); err == nil {
			// 带哈希的构建产物长缓存；index.html 不缓存，保证更新后浏览器能拿到新入口
			if strings.HasPrefix(clean, "assets/") {
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
			} else if clean == "index.html" {
				c.Header("Cache-Control", "no-cache")
			}
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}
		// SPA fallback：非文件路径回 index.html
		c.Request.URL.Path = "/"
		c.Header("Cache-Control", "no-cache")
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}

// loadEnvFile 极简 .env 加载（KEY=VALUE，# 注释），无外部依赖
func loadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
}
