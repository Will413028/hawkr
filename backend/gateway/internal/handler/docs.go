package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Will413028/hawkr/gateway/internal/config"
	"github.com/gin-gonic/gin"
)

// docsBaseDir 文件根目錄（相對於工作目錄）
const docsBaseDir = "docs"

const (
	docsCookieKey  = "docs_session"
	docsSessionTTL = 24 * time.Hour
)

// Docs 是文件檢視器。帳號、密碼與 session 簽章金鑰都來自環境變數
// （DOCS_USERNAME／DOCS_PASSWORD／DOCS_SIGN_KEY），任一為空就停用：路由不註冊。
type Docs struct {
	username string
	password string
	signKey  []byte
}

// NewDocs 依設定建立文件檢視器；設定不完整時回傳停用的檢視器。
func NewDocs(cfg config.DocsConfig) *Docs {
	return &Docs{username: cfg.Username, password: cfg.Password, signKey: []byte(cfg.SignKey)}
}

// Enabled 回報帳號、密碼與簽章金鑰是否都已設定。
func (d *Docs) Enabled() bool {
	return d != nil && d.username != "" && d.password != "" && len(d.signKey) > 0
}

// AuthMiddleware 驗證文件檢視器的登入狀態
func (d *Docs) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Param("filepath")
		if path == "/login" {
			c.Next()
			return
		}
		cookie, err := c.Cookie(docsCookieKey)
		if err != nil || !d.validateSession(cookie) {
			c.Redirect(http.StatusFound, "/docs/login")
			c.Abort()
			return
		}
		c.Next()
	}
}

// Login 顯示登入頁面
func (d *Docs) Login(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, docsLoginTemplate)
}

// LoginPost 處理登入表單
func (d *Docs) LoginPost(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	if d.credentialsMatch(username, password) {
		token := d.generateSession()
		c.SetCookie(docsCookieKey, token, int(docsSessionTTL.Seconds()), "/docs", "", false, true)
		c.Redirect(http.StatusFound, "/docs/")
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, docsLoginErrorTemplate)
}

// Logout 登出
func (d *Docs) Logout(c *gin.Context) {
	c.SetCookie(docsCookieKey, "", -1, "/docs", "", false, true)
	c.Redirect(http.StatusFound, "/docs/login")
}

// Viewer 處理 /docs/* 路由
// Docsify 模式：/ 回傳 index.html，其餘回傳原始 .md 或靜態檔案
func (d *Docs) Viewer(c *gin.Context) {
	reqPath := c.Param("filepath")

	// 登入/登出
	if reqPath == "/login" {
		d.Login(c)
		return
	}
	if reqPath == "/logout" {
		d.Logout(c)
		return
	}

	// 根路徑 → Docsify index.html
	if reqPath == "" || reqPath == "/" {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, docsifyIndexHTML)
		return
	}

	// 其餘：提供 docs/ 下的原始檔案（.md 或其他）
	cleanPath := filepath.Clean(strings.TrimPrefix(reqPath, "/"))
	fullPath := filepath.Join(docsBaseDir, cleanPath)

	// 安全性：防止路徑穿越
	if !strings.HasPrefix(filepath.Clean(fullPath), docsBaseDir) {
		c.String(http.StatusForbidden, "forbidden")
		return
	}

	// 檢查檔案是否存在
	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
		c.String(http.StatusNotFound, "not found")
		return
	}

	// Docsify 透過 AJAX 讀取 .md，Content-Type 用 text/plain 避免瀏覽器干擾
	switch {
	case strings.HasSuffix(fullPath, ".md"):
		c.Header("Content-Type", "text/plain; charset=utf-8")
	case strings.HasSuffix(fullPath, ".json"):
		c.Header("Content-Type", "application/json; charset=utf-8")
	}

	c.File(fullPath)
}

// --- Session 管理 ---

func (d *Docs) credentialsMatch(username, password string) bool {
	userOK := subtle.ConstantTimeCompare([]byte(username), []byte(d.username)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(password), []byte(d.password)) == 1
	return userOK && passOK
}

func (d *Docs) generateSession() string {
	expiry := time.Now().Add(docsSessionTTL).Unix()
	payload := strconv.FormatInt(expiry, 10)
	mac := hmac.New(sha256.New, d.signKey)
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return payload + "|" + sig
}

func (d *Docs) validateSession(token string) bool {
	parts := strings.SplitN(token, "|", 2)
	if len(parts) != 2 {
		return false
	}
	expiry, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix() > expiry {
		return false
	}
	mac := hmac.New(sha256.New, d.signKey)
	mac.Write([]byte(parts[0]))
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(parts[1]), []byte(expectedSig))
}
