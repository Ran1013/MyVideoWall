package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

/* ---------- HMAC token（与 v3 PHP 版算法一致） ----------
 * token = base64url({"r":role,"e":expiry}) + "." + hex(hmac_sha256(payload, secret))
 * secret 由双密码派生：改密码 = 全部旧 token 立即失效。
 */

func tokenSecret(pw *PasswordStore) []byte {
	mac := hmac.New(sha256.New, []byte("videowall-next-token"))
	mac.Write([]byte(pw.ViewPassword() + "|" + pw.UploadPassword()))
	return mac.Sum(nil)
}

func issueToken(pw *PasswordStore, ttlDays int, role string) string {
	payload, _ := json.Marshal(map[string]any{"r": role, "e": time.Now().Add(time.Duration(ttlDays) * 24 * time.Hour).Unix()})
	p := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, tokenSecret(pw))
	mac.Write([]byte(p))
	return p + "." + strings.ToLower(hexEncode(mac.Sum(nil)))
}

func hexEncode(b []byte) string {
	const hexDigits = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hexDigits[v>>4]
		out[i*2+1] = hexDigits[v&0x0f]
	}
	return string(out)
}

func verifyToken(pw *PasswordStore, token string) (string, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return "", false
	}
	payload, sig := parts[0], parts[1]
	mac := hmac.New(sha256.New, tokenSecret(pw))
	mac.Write([]byte(payload))
	if !hmac.Equal([]byte(sig), []byte(hexEncode(mac.Sum(nil)))) {
		return "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return "", false
	}
	var data struct {
		R string `json:"r"`
		E int64  `json:"e"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return "", false
	}
	if data.E < time.Now().Unix() {
		return "", false
	}
	if data.R != "view" && data.R != "upload" {
		return "", false
	}
	return data.R, true
}

func requestToken(c *gin.Context) string {
	if h := c.GetHeader("Authorization"); h != "" {
		if _, rest, ok := strings.Cut(h, " "); ok && strings.HasPrefix(strings.ToLower(h), "bearer") {
			return strings.TrimSpace(rest)
		}
	}
	return c.Query("t")
}

func requireView(pw *PasswordStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, ok := verifyToken(pw, requestToken(c))
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"ok": false, "error": "需要访问密码登录"})
			return
		}
		c.Set("role", role)
		c.Next()
	}
}

func requireUpload(pw *PasswordStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if role, ok := verifyToken(pw, requestToken(c)); !ok || role != "upload" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"ok": false, "error": "需要上传密码登录"})
			return
		}
		c.Next()
	}
}

func clientIP(c *gin.Context) string {
	// gin 的 ClientIP 已按 X-Forwarded-For/X-Real-IP 链路解析；
	// CF Tunnel 场景补 CF-Connecting-IP 优先
	if ip := c.GetHeader("CF-Connecting-IP"); ip != "" {
		return strings.TrimSpace(ip)
	}
	return c.ClientIP()
}
