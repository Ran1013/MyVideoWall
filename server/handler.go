package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const mimeTypes = "mp4:video/mp4,m4v:video/mp4,webm:video/webm,mov:video/quicktime,mkv:video/x-matroska,jpg:image/jpeg,jpeg:image/jpeg,png:image/png"

func mimeOf(ext string) string {
	for _, pair := range strings.Split(mimeTypes, ",") {
		if k, v, ok := strings.Cut(pair, ":"); ok && k == ext {
			return v
		}
	}
	return ""
}

func failJSON(c *gin.Context, code int, msg string) {
	c.JSON(code, gin.H{"ok": false, "error": msg})
}

func withTokenURL(cfg *Config, c *gin.Context, fname string) string {
	return "stream.php?f=" + urlEncode(fname) + "&t=" + requestToken(c)
}

func urlEncode(s string) string {
	safe := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_.~"
	out := []byte{}
	for _, b := range []byte(s) {
		if strings.ContainsRune(safe, rune(b)) {
			out = append(out, b)
		} else {
			out = append(out, []byte(fmt.Sprintf("%%%02X", b))...)
		}
	}
	return string(out)
}

/* ---------- 健康检查 ---------- */

func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "status": "ok"})
}

/* ---------- 认证 ---------- */

func (h *Handler) authHandler(c *gin.Context) {
	var in struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.Password == "" {
		failJSON(c, 400, "请输入密码")
		return
	}
	if authFails.Blocked(clientIP(c)) {
		failJSON(c, 429, "尝试次数过多，请 1 分钟后再试")
		return
	}
	if h.pw.ViewPassword() != "" && in.Password == h.pw.ViewPassword() {
		authFails.Clear(clientIP(c))
		c.JSON(http.StatusOK, gin.H{"ok": true, "role": "view", "token": issueToken(h.pw, h.cfg.TokenTTLDays, "view"), "site": "我的游戏高光"})
		return
	}
	if h.pw.UploadPassword() != "" && in.Password == h.pw.UploadPassword() {
		authFails.Clear(clientIP(c))
		c.JSON(http.StatusOK, gin.H{"ok": true, "role": "upload", "token": issueToken(h.pw, h.cfg.TokenTTLDays, "upload"), "site": "我的游戏高光"})
		return
	}
	h.store.LogAdd(clientIP(c), regionForIP(clientIP(c)), c.GetHeader("User-Agent"), "login_fail", nil)
	authFails.Record(clientIP(c))
	failJSON(c, 401, "密码不对")
}

/* ---------- 视频 ---------- */

func (h *Handler) videosHandler(c *gin.Context) {
	act := c.Query("act")
	switch {
	case c.Request.Method == http.MethodGet && act == "list":
		h.videosList(c)
	case c.Request.Method == http.MethodGet && act == "get":
		h.videosGet(c)
	case c.Request.Method == http.MethodPost && act == "view":
		h.videosView(c)
	default:
		failJSON(c, 400, "未知操作")
	}
}

func (h *Handler) videosList(c *gin.Context) {
	h.store.SyncVideos(h.cfg.VideoDir, h.cfg.AllowedExt)
	cat := strings.TrimSpace(c.Query("cat"))
	q := strings.TrimSpace(c.Query("q"))
	sort := c.DefaultQuery("sort", "new")
	if sort != "hot" {
		sort = "new"
	}
	page := atoi(c.DefaultQuery("p", "1"))
	if page < 1 {
		page = 1
	}

	items, total, cats, err := h.store.VideoList(h.cfg, cat, q, sort, page)
	if err != nil {
		failJSON(c, 500, "查询失败: "+err.Error())
		return
	}
	for i := range items {
		items[i].URL = withTokenURL(h.cfg, c, items[i].Fname)
		// 只给封面图地址；没有封面则留空，前端用占位块（避免用 video 标签取帧抢带宽）
		poster := items[i].Fname + ".jpg"
		if fileExists(filepath.Join(h.cfg.VideoDir, poster)) {
			items[i].Poster = withTokenURL(h.cfg, c, poster)
		} else {
			items[i].Poster = ""
		}
		items[i].UploaderIP = ""
	}
	c.JSON(http.StatusOK, gin.H{
		"ok": true, "items": items, "total": total, "page": page,
		"pageSize": h.cfg.PageSize, "categories": cats,
		"trafficBlocked": h.traffic.Exceeded(),
	})
}

func (h *Handler) videosGet(c *gin.Context) {
	id := int64(atoi(c.Query("id")))
	if id <= 0 {
		failJSON(c, 400, "参数不对")
		return
	}
	video, err := h.store.VideoGet(id)
	if err != nil {
		failJSON(c, 500, "查询失败")
		return
	}
	if video == nil || !fileExists(filepath.Join(h.cfg.VideoDir, video.Fname)) {
		failJSON(c, 404, "视频不存在")
		return
	}
	sort := c.DefaultQuery("sort", "new")
	if sort != "hot" {
		sort = "new"
	}
	prev, next, _ := h.store.VideoNeighbors(id, sort)
	related, _ := h.store.VideoRelated(id, video.Category, 6)

	video.URL = withTokenURL(h.cfg, c, video.Fname)
	poster := video.Fname + ".jpg"
	if fileExists(filepath.Join(h.cfg.VideoDir, poster)) {
		video.Poster = withTokenURL(h.cfg, c, poster)
	} else {
		video.Poster = ""
	}
	video.UploaderIP = ""
	for i := range related {
		rp := related[i].Fname + ".jpg"
		if fileExists(filepath.Join(h.cfg.VideoDir, rp)) {
			related[i].Poster = withTokenURL(h.cfg, c, rp)
		} else {
			related[i].Poster = ""
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "video": video, "prev": prev, "next": next, "related": related,
		"trafficBlocked": h.traffic.Exceeded()})
}

func (h *Handler) videosView(c *gin.Context) {
	var in struct {
		ID int64 `json:"id"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.ID <= 0 {
		failJSON(c, 400, "参数不对")
		return
	}
	if !h.videoFileExists(in.ID) {
		failJSON(c, 404, "视频不存在")
		return
	}
	h.store.VideoIncrViews(in.ID)
	videoID := in.ID
	h.store.LogAdd(clientIP(c), regionForIP(clientIP(c)), c.GetHeader("User-Agent"), "play", &videoID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

/* ---------- 视频流 ---------- */

// countingWriter 统计实际写出的字节数（视频流流量计量）
type countingWriter struct {
	gin.ResponseWriter
	written int64
}

func (w *countingWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.written += int64(n)
	return n, err
}

func (h *Handler) streamHandler(c *gin.Context) {
	// 月度流量熔断：超限后拒绝视频流（下月 1 日自动恢复）
	if h.traffic.Exceeded() {
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"ok": false, "error": "本月视频流量已用完（" + itoa(h.traffic.LimitGB()) + "GB），下月 1 日自动恢复",
		})
		return
	}
	name := c.Query("f")
	if name == "" {
		http.NotFound(c.Writer, c.Request)
		return
	}
	baseReal, err := filepath.Abs(h.cfg.VideoDir)
	if err != nil {
		http.Error(c.Writer, "server error", 500)
		return
	}
	real := filepath.Clean(filepath.Join(baseReal, name))
	if !strings.HasPrefix(real, baseReal+string(filepath.Separator)) {
		http.NotFound(c.Writer, c.Request)
		return
	}
	info, err := os.Stat(real)
	if err != nil || info.IsDir() {
		http.NotFound(c.Writer, c.Request)
		return
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(real), "."))
	mime := mimeOf(ext)
	if mime == "" {
		http.Error(c.Writer, "Forbidden", 403)
		return
	}
	f, err := os.Open(real)
	if err != nil {
		http.Error(c.Writer, "server error", 500)
		return
	}
	defer f.Close()
	c.Header("Cache-Control", "private, max-age=3600")
	// http.ServeContent 原生处理 Range/206/416/If-Range；写出字节计入月度流量
	cw := &countingWriter{ResponseWriter: c.Writer}
	http.ServeContent(cw, c.Request, "", info.ModTime(), f)
	h.traffic.Add(cw.written)
}

/* ---------- 上传 ---------- */

func (h *Handler) uploadHandler(c *gin.Context) {
	switch act := c.Query("act"); act {
	case "init":
		h.uploadInit(c)
	case "status":
		h.uploadStatus(c)
	case "chunk":
		h.uploadChunk(c)
	case "finish":
		h.uploadFinish(c)
	default:
		failJSON(c, 400, "未知操作")
	}
}

func (h *Handler) chunkSizeBytes() int {
	return h.cfg.ChunkMB * 1024 * 1024
}

type uploadMeta struct {
	Orig    string `json:"orig"`
	Base    string `json:"base"`
	Ext     string `json:"ext"`
	Size    int64  `json:"size"`
	Chunks  int    `json:"chunks"`
	Created int64  `json:"created"`
	IP      string `json:"ip"`
}

func readMeta(dir string) (*uploadMeta, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	if err != nil {
		return nil, err
	}
	var m uploadMeta
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func sessionReceived(dir string) []int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []int{}
	}
	got := []int{}
	for _, e := range entries {
		var idx int
		if _, err := fmt.Sscanf(e.Name(), "%d.part", &idx); err == nil {
			got = append(got, idx)
		}
	}
	return got
}

func gcSessions(chunksDir string, ttlHours int) {
	entries, err := os.ReadDir(chunksDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		metaPath := filepath.Join(chunksDir, e.Name(), "meta.json")
		info, err := os.Stat(metaPath)
		if err != nil {
			continue
		}
		if info.ModTime().Unix()+int64(ttlHours)*3600 < time.Now().Unix() {
			dir := filepath.Dir(metaPath)
			sub, _ := os.ReadDir(dir)
			for _, f := range sub {
				os.Remove(filepath.Join(dir, f.Name()))
			}
			os.Remove(dir)
		}
	}
}

func (h *Handler) uploadInit(c *gin.Context) {
	var in struct {
		Name   string `json:"name"`
		Size   int64  `json:"size"`
		Chunks int    `json:"chunks"`
	}
	body, _ := io.ReadAll(c.Request.Body)
	if err := json.Unmarshal(body, &in); err != nil {
		log.Printf("upload init body 解析失败: %v, body=%s", err, string(body))
		failJSON(c, 400, "参数不对")
		return
	}
	gcSessions(h.cfg.ChunksDir, h.cfg.ChunkTTLHours)

	if in.Name == "" || in.Size <= 0 || in.Chunks <= 0 || in.Chunks > 100000 {
		// 记下原始请求体：这类 400 依赖客户端 File 对象，出问题时要靠它定位
		log.Printf("upload init 参数异常: name=%q size=%d chunks=%d body=%s", in.Name, in.Size, in.Chunks, string(body))
		failJSON(c, 400, "参数不对")
		return
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(in.Name), "."))
	allowed := false
	for _, a := range h.cfg.AllowedExt {
		if ext == a {
			allowed = true
			break
		}
	}
	if !allowed {
		failJSON(c, 400, "不支持的格式 ."+ext+"（允许："+strings.Join(h.cfg.AllowedExt, ",")+"）")
		return
	}

	// 断点续传：同名同大小的未完成会话复用
	subs, _ := os.ReadDir(h.cfg.ChunksDir)
	for _, e := range subs {
		meta, err := readMeta(filepath.Join(h.cfg.ChunksDir, e.Name()))
		if err == nil && meta.Size == in.Size && meta.Orig == in.Name {
			dir := filepath.Join(h.cfg.ChunksDir, e.Name())
			c.JSON(http.StatusOK, gin.H{
				"ok": true, "upload_id": e.Name(), "received": sessionReceived(dir),
				"chunk_size": h.chunkSizeBytes(), "resumed": true,
			})
			return
		}
	}

	uploadID := randomHex16()
	dir := filepath.Join(h.cfg.ChunksDir, uploadID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		failJSON(c, 500, "服务器无法创建上传会话目录（chunks/ 不可写）")
		return
	}
	meta := uploadMeta{Orig: in.Name, Base: cleanBase(in.Name), Ext: ext, Size: in.Size, Chunks: in.Chunks, Created: time.Now().Unix(), IP: clientIP(c)}
	raw, _ := json.Marshal(meta)
	os.WriteFile(filepath.Join(dir, "meta.json"), raw, 0o644)

	c.JSON(http.StatusOK, gin.H{"ok": true, "upload_id": uploadID, "received": []int{}, "chunk_size": h.chunkSizeBytes()})
}

func (h *Handler) uploadStatus(c *gin.Context) {
	var in struct {
		UploadID string `json:"upload_id"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || !validUploadID(in.UploadID) {
		failJSON(c, 400, "会话 ID 不对")
		return
	}
	dir := filepath.Join(h.cfg.ChunksDir, in.UploadID)
	m, err := readMeta(dir)
	if err != nil {
		failJSON(c, 400, "会话不存在或已过期，请重新选择文件")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "received": sessionReceived(dir), "chunks": m.Chunks, "name": m.Base + "." + m.Ext})
}

func (h *Handler) uploadChunk(c *gin.Context) {
	uploadID := c.PostForm("upload_id")
	index := c.PostForm("index")
	if !validUploadID(uploadID) {
		failJSON(c, 400, "会话 ID 不对")
		return
	}
	var idx int
	if _, err := fmt.Sscanf(index, "%d", &idx); err != nil {
		failJSON(c, 400, "分片序号不对")
		return
	}
	dir := filepath.Join(h.cfg.ChunksDir, uploadID)
	m, err := readMeta(dir)
	if err != nil {
		failJSON(c, 400, "会话不存在或已过期，请重新选择文件")
		return
	}
	if idx < 0 || idx >= m.Chunks {
		failJSON(c, 400, "分片序号越界")
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
		failJSON(c, 400, "没有收到分片数据")
		return
	}
	if err := c.SaveUploadedFile(fh, filepath.Join(dir, itoa(idx)+".part")); err != nil {
		failJSON(c, 500, "分片保存失败（chunks/ 不可写）")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "received": len(sessionReceived(dir))})
}

func (h *Handler) uploadFinish(c *gin.Context) {
	var in struct {
		UploadID string `json:"upload_id"`
		Title    string `json:"title"`
		Cat      string `json:"cat"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || !validUploadID(in.UploadID) {
		failJSON(c, 400, "会话 ID 不对")
		return
	}
	dir := filepath.Join(h.cfg.ChunksDir, in.UploadID)
	m, err := readMeta(dir)
	if err != nil {
		failJSON(c, 400, "会话不存在或已过期，请重新选择文件")
		return
	}

	got := sessionReceived(dir)
	if len(got) < m.Chunks {
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": "还有分片未传完", "received": got, "chunks": m.Chunks})
		return
	}
	sum := int64(0)
	for i := 0; i < m.Chunks; i++ {
		if info, err := os.Stat(filepath.Join(dir, itoa(i)+".part")); err == nil {
			sum += info.Size()
		}
	}
	if sum != m.Size {
		failJSON(c, 400, "分片总大小与预期不符，请重新上传")
		return
	}

	name := m.Base + "." + m.Ext
	for n := 2; fileExists(filepath.Join(h.cfg.VideoDir, name)); n++ {
		name = fmt.Sprintf("%s-%d.%s", m.Base, n, m.Ext)
	}
	dest := filepath.Join(h.cfg.VideoDir, name)
	out, err := os.Create(dest)
	if err != nil {
		failJSON(c, 500, "保存失败：videos/ 目录不可写")
		return
	}
	for i := 0; i < m.Chunks; i++ {
		part, err := os.Open(filepath.Join(dir, itoa(i)+".part"))
		if err != nil {
			out.Close()
			os.Remove(dest)
			failJSON(c, 400, "分片读取失败，请重新上传")
			return
		}
		buf := make([]byte, 2*1024*1024)
		for {
			n, err := part.Read(buf)
			if n > 0 {
				out.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
		part.Close()
	}
	out.Close()
	os.Chmod(dest, 0o644)

	if info, err := os.Stat(dest); err != nil || info.Size() != m.Size {
		os.Remove(dest)
		failJSON(c, 400, "拼接后大小校验失败，请重新上传")
		return
	}

	// 服务器自动压缩（适合小带宽流播放）；失败保留原文件，不影响上传。
	// 转码产物一律是 H.264 MP4：源不是 mp4 时把文件名与扩展名一并改成 .mp4，
	// 避免“mp4 内容顶着 .mkv/.mov 扩展名”被 Safari 等按容器误判而播放失败。
	finalSize := m.Size
	if ffmpegAvailable() {
		replaced, err := transcodeForWeb(dest, h.tc)
		if err != nil {
			log.Printf("自动压缩跳过：%v", err)
		}
		if replaced && m.Ext != "mp4" {
			base := strings.TrimSuffix(name, filepath.Ext(name))
			newName := base + ".mp4"
			for n := 2; fileExists(filepath.Join(h.cfg.VideoDir, newName)); n++ {
				newName = fmt.Sprintf("%s-%d.mp4", base, n)
			}
			if err := os.Rename(dest, filepath.Join(h.cfg.VideoDir, newName)); err == nil {
				name = newName
				m.Ext = "mp4"
			}
		}
		if fi, err := os.Stat(filepath.Join(h.cfg.VideoDir, name)); err == nil {
			finalSize = fi.Size()
		}
		generatePoster(filepath.Join(h.cfg.VideoDir, name)) // 封面图：首页卡片只加载它，避免视频请求抢占带宽
	}

	id, err := h.store.VideoAdd(name, defaultStr(in.Title, m.Base), truncateRunes(strings.TrimSpace(in.Cat), 60), m.Ext, finalSize, m.IP)
	if err != nil {
		failJSON(c, 500, "入库失败")
		return
	}
	vid := id
	h.store.LogAdd(clientIP(c), regionForIP(clientIP(c)), c.GetHeader("User-Agent"), "upload", &vid)

	sub, _ := os.ReadDir(dir)
	for _, f := range sub {
		os.Remove(filepath.Join(dir, f.Name()))
	}
	os.Remove(dir)

	c.JSON(http.StatusOK, gin.H{"ok": true, "id": id, "name": name})
}

/* ---------- 管理 ---------- */

func (h *Handler) adminHandler(c *gin.Context) {
	act := c.Query("act")
	if c.Request.Method == http.MethodGet {
		switch act {
		case "list":
			h.adminList(c)
		case "categories":
			cats, err := h.store.Categories()
			if err != nil {
				failJSON(c, 500, "查询失败")
				return
			}
			c.JSON(http.StatusOK, gin.H{"ok": true, "items": cats})
		case "traffic":
			c.JSON(http.StatusOK, gin.H{"ok": true, "traffic": h.traffic.Status()})
		case "transcode":
			c.JSON(http.StatusOK, gin.H{"ok": true, "transcode": h.tc.Get()})
		default:
			failJSON(c, 400, "未知操作")
		}
		return
	}
	switch act {
	case "save":
		h.adminSave(c)
	case "delete":
		h.adminDelete(c)
	case "cat-rename":
		h.adminCatRename(c)
	case "cat-clear":
		h.adminCatClear(c)
	case "traffic-toggle":
		var in struct {
			Enabled bool `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			failJSON(c, 400, "参数不对")
			return
		}
		h.traffic.SetEnabled(in.Enabled)
		c.JSON(http.StatusOK, gin.H{"ok": true, "traffic": h.traffic.Status()})
	case "traffic-reset":
		h.traffic.Reset()
		c.JSON(http.StatusOK, gin.H{"ok": true, "traffic": h.traffic.Status()})
	case "transcode":
		var in TranscodeParams
		if err := c.ShouldBindJSON(&in); err != nil {
			failJSON(c, 400, "参数不对")
			return
		}
		if !saneTranscode(in) {
			failJSON(c, 400, "参数超出范围：长边 480–1920，帧率 10–60，CRF 14–35，码率 200–20000")
			return
		}
		p := h.tc.Set(in)
		c.JSON(http.StatusOK, gin.H{"ok": true, "message": "转码参数已保存，之后上传的新视频按此压缩", "transcode": p})
	case "password-view", "password-upload":
		var in struct {
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			failJSON(c, 400, "参数不对")
			return
		}
		pwd := strings.TrimSpace(in.Password)
		if len(pwd) < 4 {
			failJSON(c, 400, "密码至少 4 位（建议 8 位以上混合字符）")
			return
		}
		if act == "password-view" {
			h.pw.SetViewPassword(pwd)
			c.JSON(http.StatusOK, gin.H{"ok": true, "message": "观看密码已修改，所有人（含本浏览器）需用新密码重新登录"})
		} else {
			h.pw.SetUploadPassword(pwd)
			c.JSON(http.StatusOK, gin.H{"ok": true, "message": "上传/管理密码已修改，请用新密码重新登录"})
		}
	default:
		failJSON(c, 400, "未知操作")
	}
}

func (h *Handler) adminList(c *gin.Context) {
	h.store.SyncVideos(h.cfg.VideoDir, h.cfg.AllowedExt)
	rows, err := h.store.DB.Query("SELECT " + videoCols + " FROM videos ORDER BY created_at DESC")
	if err != nil {
		failJSON(c, 500, "查询失败")
		return
	}
	defer rows.Close()
	items := []Video{}
	for rows.Next() {
		var v Video
		if err := rows.Scan(&v.ID, &v.Fname, &v.Title, &v.Category, &v.Ext, &v.Size, &v.Views, &v.UploaderIP, &v.CreatedAt); err != nil {
			failJSON(c, 500, "查询失败")
			return
		}
		v.OnDisk = fileExists(filepath.Join(h.cfg.VideoDir, v.Fname))
		items = append(items, v)
	}
	cats, _ := h.store.Categories()
	stats, _ := h.store.SiteStats()
	c.JSON(http.StatusOK, gin.H{"ok": true, "items": items, "categories": cats, "stats": stats})
}

func (h *Handler) adminSave(c *gin.Context) {
	var in struct {
		ID       int64  `json:"id"`
		Title    string `json:"title"`
		Category string `json:"category"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.ID <= 0 {
		failJSON(c, 400, "参数不对")
		return
	}
	if v, _ := h.store.VideoGet(in.ID); v == nil {
		failJSON(c, 404, "视频不存在")
		return
	}
	if err := h.store.VideoUpdate(in.ID, in.Title, in.Category); err != nil {
		failJSON(c, 500, "保存失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) adminDelete(c *gin.Context) {
	var in struct {
		ID int64 `json:"id"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.ID <= 0 {
		failJSON(c, 400, "参数不对")
		return
	}
	fname, err := h.store.VideoDelete(in.ID)
	if err != nil {
		failJSON(c, 404, err.Error())
		return
	}
	os.Remove(filepath.Join(h.cfg.VideoDir, fname))
	os.Remove(filepath.Join(h.cfg.VideoDir, fname+".jpg")) // 自动生成的封面一并清理（不存在时静默）
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) adminCatRename(c *gin.Context) {
	var in struct {
		Old string `json:"old"`
		New string `json:"new"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		failJSON(c, 400, "参数不对")
		return
	}
	in.Old = strings.TrimSpace(in.Old)
	in.New = strings.TrimSpace(in.New)
	exists := false
	if in.New != "" && in.New != in.Old {
		cats, _ := h.store.Categories()
		for _, cat := range cats {
			if cat.C == in.New {
				exists = true
				break
			}
		}
	}
	n, err := h.store.CategoryRename(in.Old, in.New)
	if err != nil {
		failJSON(c, 500, "操作失败")
		return
	}
	if n == 0 {
		c.JSON(http.StatusOK, gin.H{"ok": true, "renamed": 0, "merged": false, "message": "没有改动（新旧名字相同或为空）"})
		return
	}
	action := "改名为"
	if exists {
		action = "合并进"
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "renamed": n, "merged": exists,
		"message": fmt.Sprintf("已把「%s」%s「%s」（%d 个视频）", in.Old, action, in.New, n)})
}

func (h *Handler) adminCatClear(c *gin.Context) {
	var in struct {
		Cat string `json:"cat"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		failJSON(c, 400, "参数不对")
		return
	}
	n, err := h.store.CategoryClear(strings.TrimSpace(in.Cat))
	if err != nil {
		failJSON(c, 500, "操作失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "cleared": n, "message": fmt.Sprintf("已清空「%s」（%d 个视频移出分类，视频保留）", strings.TrimSpace(in.Cat), n)})
}

/* ---------- 访客记录 ---------- */

func (h *Handler) logsHandler(c *gin.Context) {
	act := c.Query("act")
	if c.Request.Method == http.MethodGet {
		switch act {
		case "stats":
			stats, err := h.store.SiteStats()
			if err != nil {
				failJSON(c, 500, "查询失败")
				return
			}
			total, _ := h.store.LogCount()
			c.JSON(http.StatusOK, gin.H{"ok": true, "stats": stats, "total": total})
		case "byip":
			items, err := h.store.LogByIP(100, h.logFilter(c))
			if err != nil {
				failJSON(c, 500, "查询失败")
				return
			}
			c.JSON(http.StatusOK, gin.H{"ok": true, "items": items})
		case "recent":
			items, err := h.store.LogRecent(200, h.logFilter(c))
			if err != nil {
				failJSON(c, 500, "查询失败")
				return
			}
			c.JSON(http.StatusOK, gin.H{"ok": true, "items": items})
		default:
			failJSON(c, 400, "未知操作")
		}
		return
	}
	if act == "clear" {
		h.store.LogClear()
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	failJSON(c, 400, "未知操作")
}
