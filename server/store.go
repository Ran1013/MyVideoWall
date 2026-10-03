package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type Store struct {
	DB *sql.DB
}

type Video struct {
	ID         int64  `json:"id"`
	Fname      string `json:"fname"`
	Title      string `json:"title"`
	Category   string `json:"category"`
	Ext        string `json:"ext"`
	Size       int64  `json:"size"`
	Views      int64  `json:"views"`
	UploaderIP string `json:"uploader_ip,omitempty"`
	CreatedAt  string `json:"created_at"`
	URL        string `json:"url,omitempty"`
	Poster     string `json:"poster,omitempty"`
	OnDisk     bool   `json:"onDisk,omitempty"`
}

type Category struct {
	C string `json:"c"`
	N int64  `json:"n"`
	W int64  `json:"w"`
}

type LogEntry struct {
	ID      int64  `json:"id"`
	IP      string `json:"ip"`
	Region  string `json:"region"`
	UA      string `json:"ua"`
	Action  string `json:"action"`
	VideoID *int64 `json:"video_id"`
	TS      string `json:"ts"`
}

type Stats struct {
	Count    int64  `json:"count"`
	Size     int64  `json:"size"`
	Views    int64  `json:"views"`
	TodayIPs int64  `json:"today_ips"`
	SizeH    string `json:"size_h,omitempty"`
}

const videoCols = "id, fname, title, category, ext, size, views, uploader_ip, DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s') AS created_at"

func NewStore(dsn, dbName string) (*Store, error) {
	if dsn == "" {
		return nil, fmt.Errorf("MYSQL_DSN 未设置")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)
	if err := db.Ping(); err != nil {
		return nil, err
	}
	s := &Store{DB: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	for _, ddl := range []string{
		`CREATE TABLE IF NOT EXISTS videos (
			id         INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			fname      VARCHAR(255) NOT NULL UNIQUE,
			title      VARCHAR(120) NOT NULL DEFAULT '',
			category   VARCHAR(60)  NOT NULL DEFAULT '',
			ext        VARCHAR(10)  NOT NULL DEFAULT '',
			size       BIGINT UNSIGNED NOT NULL DEFAULT 0,
			views      INT UNSIGNED NOT NULL DEFAULT 0,
			uploader_ip VARCHAR(45) NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL,
			KEY idx_cat (category),
			KEY idx_created (created_at),
			KEY idx_views (views)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS access_log (
			id       BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			ip       VARCHAR(45)  NOT NULL DEFAULT '',
			region   VARCHAR(255) NOT NULL DEFAULT '',
			ua       VARCHAR(255) NOT NULL DEFAULT '',
			action   VARCHAR(20)  NOT NULL DEFAULT '',
			video_id INT UNSIGNED NULL,
			ts       DATETIME NOT NULL,
			KEY idx_ts (ts),
			KEY idx_ip (ip)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	} {
		if _, err := s.DB.Exec(ddl); err != nil {
			return fmt.Errorf("建表失败: %w", err)
		}
	}
	return nil
}

/* ---------- 视频列表 / 详情 ---------- */

func (s *Store) VideoList(cfg *Config, cat, q, sort string, page int) ([]Video, int64, []Category, error) {
	where := []string{}
	args := []any{}
	if cat != "" {
		where = append(where, "category = ?")
		args = append(args, cat)
	}
	if q != "" {
		where = append(where, "(title LIKE ? OR fname LIKE ?)")
		like := "%" + q + "%"
		args = append(args, like, like)
	}
	whereSQL := ""
	if len(where) > 0 {
		whereSQL = "WHERE " + strings.Join(where, " AND ")
	}
	order := "created_at DESC"
	if sort == "hot" {
		order = "views DESC, created_at DESC"
	}

	var total int64
	if err := s.DB.QueryRow("SELECT COUNT(*) FROM videos "+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, nil, err
	}

	query := "SELECT " + videoCols + " FROM videos " + whereSQL + " ORDER BY " + order + " LIMIT ? OFFSET ?"
	args = append(args, cfg.PageSize, (page-1)*cfg.PageSize)
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, 0, nil, err
	}
	defer rows.Close()

	items := []Video{}
	for rows.Next() {
		var v Video
		if err := rows.Scan(&v.ID, &v.Fname, &v.Title, &v.Category, &v.Ext, &v.Size, &v.Views, &v.UploaderIP, &v.CreatedAt); err != nil {
			return nil, 0, nil, err
		}
		items = append(items, v)
	}
	cats, err := s.Categories()
	if err != nil {
		return nil, 0, nil, err
	}
	return items, total, cats, nil
}

func (s *Store) scanVideo(row *sql.Row) (*Video, error) {
	var v Video
	err := row.Scan(&v.ID, &v.Fname, &v.Title, &v.Category, &v.Ext, &v.Size, &v.Views, &v.UploaderIP, &v.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *Store) VideoGet(id int64) (*Video, error) {
	return s.scanVideo(s.DB.QueryRow("SELECT "+videoCols+" FROM videos WHERE id = ?", id))
}

func (s *Store) VideoNeighbors(id int64, sort string) (*Video, *Video, error) {
	order := "created_at DESC"
	if sort == "hot" {
		order = "views DESC, created_at DESC"
	}
	rows, err := s.DB.Query("SELECT " + videoCols + " FROM videos ORDER BY " + order)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var all []Video
	for rows.Next() {
		var v Video
		if err := rows.Scan(&v.ID, &v.Fname, &v.Title, &v.Category, &v.Ext, &v.Size, &v.Views, &v.UploaderIP, &v.CreatedAt); err != nil {
			return nil, nil, err
		}
		all = append(all, v)
	}
	cur := -1
	for i := range all {
		if all[i].ID == id {
			cur = i
			break
		}
	}
	if cur < 0 {
		return nil, nil, nil
	}
	var prev, next *Video
	if cur > 0 {
		prev = &all[cur-1]
	}
	if cur < len(all)-1 {
		next = &all[cur+1]
	}
	return prev, next, nil
}

func (s *Store) VideoRelated(id int64, cat string, n int) ([]Video, error) {
	rows := []Video{}
	if cat != "" {
		rs, err := s.DB.Query("SELECT "+videoCols+" FROM videos WHERE category = ? AND id <> ? ORDER BY created_at DESC LIMIT ?",
			cat, id, n)
		if err != nil {
			return nil, err
		}
		for rs.Next() {
			var v Video
			if err := rs.Scan(&v.ID, &v.Fname, &v.Title, &v.Category, &v.Ext, &v.Size, &v.Views, &v.UploaderIP, &v.CreatedAt); err != nil {
				rs.Close()
				return nil, err
			}
			rows = append(rows, v)
		}
		rs.Close()
	}
	if len(rows) < n {
		rs, err := s.DB.Query("SELECT "+videoCols+" FROM videos WHERE id <> ? ORDER BY created_at DESC LIMIT ?", id, n)
		if err != nil {
			return nil, err
		}
		defer rs.Close()
		for rs.Next() {
			var v Video
			if err := rs.Scan(&v.ID, &v.Fname, &v.Title, &v.Category, &v.Ext, &v.Size, &v.Views, &v.UploaderIP, &v.CreatedAt); err != nil {
				return nil, err
			}
			dup := false
			for _, have := range rows {
				if have.ID == v.ID {
					dup = true
					break
				}
			}
			if !dup && len(rows) < n {
				rows = append(rows, v)
			}
		}
	}
	return rows, nil
}

func (s *Store) VideoIncrViews(id int64) error {
	_, err := s.DB.Exec("UPDATE videos SET views = views + 1 WHERE id = ?", id)
	return err
}

func (s *Store) VideoAdd(fname, title, category, ext string, size int64, ip string) (int64, error) {
	res, err := s.DB.Exec(
		"INSERT IGNORE INTO videos (fname, title, category, ext, size, uploader_ip, created_at) VALUES (?, ?, ?, ?, ?, ?, NOW())",
		fname, title, category, ext, size, ip)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) VideoUpdate(id int64, title, category string) error {
	_, err := s.DB.Exec("UPDATE videos SET title = ?, category = ? WHERE id = ?", truncateRunes(title, 120), truncateRunes(category, 60), id)
	return err
}

func (s *Store) VideoDelete(id int64) (string, error) {
	var fname string
	err := s.DB.QueryRow("SELECT fname FROM videos WHERE id = ?", id).Scan(&fname)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("视频不存在")
	}
	if err != nil {
		return "", err
	}
	_, err = s.DB.Exec("DELETE FROM videos WHERE id = ?", id)
	return fname, err
}

func (s *Store) Categories() ([]Category, error) {
	rows, err := s.DB.Query("SELECT category c, COUNT(*) n, COALESCE(SUM(views),0) w FROM videos WHERE category <> '' GROUP BY category ORDER BY n DESC, c ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Category{}
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.C, &c.N, &c.W); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func (s *Store) CategoryRename(old, new_ string) (int64, error) {
	res, err := s.DB.Exec("UPDATE videos SET category = ? WHERE category = ?", truncateRunes(new_, 60), old)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) CategoryClear(cat string) (int64, error) {
	res, err := s.DB.Exec("UPDATE videos SET category = '' WHERE category = ?", cat)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// SyncVideos：videos/ 目录里存在但库里没有的补登记；库里登记了硬盘上没有的删行
func (s *Store) SyncVideos(videoDir string, allowedExt []string) error {
	onDisk := map[string]string{} // fname -> mtime
	entries, err := os.ReadDir(videoDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(e.Name()), "."))
			allowed := false
			for _, a := range allowedExt {
				if ext == a {
					allowed = true
					break
				}
			}
			if !allowed {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			onDisk[e.Name()] = info.ModTime().Format("2006-01-02 15:04:05")
		}
	}

	rows, err := s.DB.Query("SELECT fname FROM videos")
	if err != nil {
		return err
	}
	inDb := map[string]bool{}
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			rows.Close()
			return err
		}
		inDb[f] = true
	}
	rows.Close()

	for f, mtime := range onDisk {
		if inDb[f] {
			continue
		}
		info, err := os.Stat(filepath.Join(videoDir, f))
		size := int64(0)
		if err == nil {
			size = info.Size()
		}
		if _, err := s.DB.Exec(
			"INSERT IGNORE INTO videos (fname, title, category, ext, size, created_at) VALUES (?, ?, '', ?, ?, ?)",
			f, strings.TrimSuffix(f, filepath.Ext(f)), strings.ToLower(strings.TrimPrefix(filepath.Ext(f), ".")), size, mtime); err != nil {
			return err
		}
	}
	for f := range inDb {
		if _, ok := onDisk[f]; !ok {
			if _, err := s.DB.Exec("DELETE FROM videos WHERE fname = ?", f); err != nil {
				return err
			}
		}
	}
	return nil
}

/* ---------- 访问日志 ---------- */

func logFilterWhere(f map[string]string, args []any) (string, []any) {
	where := []string{}
	if q := f["q"]; q != "" {
		where = append(where, "(ip LIKE ? OR region LIKE ?)")
		like := "%" + q + "%"
		args = append(args, like, like)
	}
	if a := f["action"]; a != "" {
		where = append(where, "action = ?")
		args = append(args, a)
	}
	switch f["range"] {
	case "today":
		where = append(where, "ts >= CURDATE()")
	case "week":
		where = append(where, "ts >= NOW() - INTERVAL 7 DAY")
	}
	if len(where) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(where, " AND "), args
}

func (s *Store) LogAdd(ip, region, ua, action string, videoID *int64) error {
	_, err := s.DB.Exec("INSERT INTO access_log (ip, region, ua, action, video_id, ts) VALUES (?, ?, ?, ?, ?, NOW())",
		ip, region, truncateRunes(ua, 255), action, videoID)
	return err
}

func (s *Store) LogByIP(limit int, f map[string]string) ([]map[string]any, error) {
	args := []any{}
	w, args := logFilterWhere(f, args)
	rows, err := s.DB.Query("SELECT ip, MAX(region) region, COUNT(*) n, MAX(ts) last FROM access_log "+w+" GROUP BY ip ORDER BY last DESC LIMIT "+itoa(limit), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var ip, region, last string
		var n int64
		if err := rows.Scan(&ip, &region, &n, &last); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"ip": ip, "region": region, "n": n, "last": last})
	}
	return out, nil
}

func (s *Store) LogRecent(limit int, f map[string]string) ([]LogEntry, error) {
	args := []any{}
	w, args := logFilterWhere(f, args)
	rows, err := s.DB.Query("SELECT id, ip, region, ua, action, video_id, DATE_FORMAT(ts, '%Y-%m-%d %H:%i:%s') ts FROM access_log "+w+" ORDER BY id DESC LIMIT "+itoa(limit), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LogEntry{}
	for rows.Next() {
		var e LogEntry
		if err := rows.Scan(&e.ID, &e.IP, &e.Region, &e.UA, &e.Action, &e.VideoID, &e.TS); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

func (s *Store) LogClear() error {
	_, err := s.DB.Exec("TRUNCATE TABLE access_log")
	return err
}

func (s *Store) LogCount() (int64, error) {
	var n int64
	err := s.DB.QueryRow("SELECT COUNT(*) FROM access_log").Scan(&n)
	return n, err
}

func (s *Store) SiteStats() (Stats, error) {
	st := Stats{}
	err := s.DB.QueryRow("SELECT COUNT(*) c, COALESCE(SUM(size),0) s, COALESCE(SUM(views),0) w FROM videos").Scan(&st.Count, &st.Size, &st.Views)
	if err != nil {
		return st, err
	}
	err = s.DB.QueryRow("SELECT COUNT(DISTINCT ip) FROM access_log WHERE ts >= CURDATE()").Scan(&st.TodayIPs)
	return st, err
}

/* ---------- 工具 ---------- */

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
