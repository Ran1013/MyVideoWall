package main

import (
	"encoding/json"
	"os"
	"sync"
)

// PasswordStore 站点密码存储：
//   env（VIEW_PASSWORD/UPLOAD_PASSWORD）是初始默认值；
//   管理端修改密码后持久化到 data/config.json，之后以文件为准（热生效）。
//   token 的 secret 由两个密码派生——改密码 = 所有已发 token 立即失效。
type PasswordStore struct {
	mu    sync.RWMutex
	path  string
	view  string
	upload string
}

func NewPasswordStore(path, defView, defUpload string) *PasswordStore {
	p := &PasswordStore{path: path, view: defView, upload: defUpload}
	if raw, err := os.ReadFile(path); err == nil {
		var d struct {
			View   string `json:"view_password"`
			Upload string `json:"upload_password"`
		}
		if json.Unmarshal(raw, &d) == nil {
			if d.View != "" {
				p.view = d.View
			}
			if d.Upload != "" {
				p.upload = d.Upload
			}
		}
	}
	return p
}

func (p *PasswordStore) save(view, upload string) {
	raw, _ := json.MarshalIndent(map[string]string{
		"view_password":   view,
		"upload_password": upload,
	}, "", "  ")
	tmp := p.path + ".tmp"
	if os.WriteFile(tmp, raw, 0o600) == nil {
		os.Rename(tmp, p.path)
	}
}

func (p *PasswordStore) ViewPassword() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.view
}

func (p *PasswordStore) UploadPassword() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.upload
}

// SetViewPassword 修改观看密码（写盘 + 热生效）
func (p *PasswordStore) SetViewPassword(pwd string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.view = pwd
	p.save(p.view, p.upload)
}

// SetUploadPassword 修改上传/管理密码（写盘 + 热生效）
func (p *PasswordStore) SetUploadPassword(pwd string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.upload = pwd
	p.save(p.view, p.upload)
}
