package handler

import (
	"bytes"
	"html/template"
	"net/http"
	"net/url"

	"github.com/Will413028/hawkr/go-pkg/payuni"
	"github.com/gin-gonic/gin"
)

// payUniRedirectForm 承載 PayUni auto-submit 中轉頁的動態欄位。
//
// #1394：舊實作把 client round-trip 回來的 base64 `?data=` 以 fmt.Sprintf 拼進
// HTML，無 escape / 無來源驗證 → 反射型 XSS + open form-action。現改為 GET `?id=`
// → gRPC 依 id 取回後端 canonical record → 這裡以 html/template context-aware escape
// 安全 render。所有欄位值皆來自後端 canonical record（非 client 可控），並疊加
// action host allowlist（isAllowedPayUniActionURL）作深度防禦。
type payUniRedirectForm struct {
	Title       string // 靜態文案（正在跳轉付款 / 開啟門市），非 client 可控
	FormID      string // 靜態表單 id（payuniForm / storeMapForm）
	ActionURL   string // Form POST 目標；render 前必經 host allowlist 驗證
	MerID       string
	Version     string
	EncryptInfo string
	HashInfo    string
	IsPlatForm  string // 已由 handler 依 canonical 值決定（payment: bool→"1"/"0"；storemap: 固定 "1"）
}

// payUniRedirectTemplate 是 auto-submit 中轉頁模板。靜態部分（spinner 動畫 CSS、
// auto-submit JS）保持原功能；所有動態欄位走 {{.Field}}，由 html/template 依所在
// context（action="" 的 URL context、value="" 的 attribute context、<script> 的
// JS context）自動 escape，杜絕反射 XSS。
var payUniRedirectTemplate = template.Must(template.New("payuniRedirect").Parse(`<!DOCTYPE html>
<html lang="zh-TW">
<head>
    <meta charset="UTF-8">
    <title>{{.Title}}</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, sans-serif; display: flex; justify-content: center; align-items: center; height: 100vh; margin: 0; background: #f5f5f5; }
        .loading { text-align: center; }
        .spinner { border: 4px solid #f3f3f3; border-top: 4px solid #3498db; border-radius: 50%; width: 40px; height: 40px; animation: spin 1s linear infinite; margin: 0 auto 20px; }
        @keyframes spin { 0% { transform: rotate(0deg); } 100% { transform: rotate(360deg); } }
    </style>
</head>
<body>
    <div class="loading">
        <div class="spinner"></div>
        <p>{{.Title}}，請稍候...</p>
    </div>
    <form id="{{.FormID}}" method="POST" action="{{.ActionURL}}" style="display:none;">
        <input type="hidden" name="MerID" value="{{.MerID}}">
        <input type="hidden" name="Version" value="{{.Version}}">
        <input type="hidden" name="EncryptInfo" value="{{.EncryptInfo}}">
        <input type="hidden" name="HashInfo" value="{{.HashInfo}}">
        <input type="hidden" name="IsPlatForm" value="{{.IsPlatForm}}">
    </form>
    <script>document.getElementById({{.FormID}}).submit();</script>
</body>
</html>`))

// payUniActionHosts 由 pkg/payuni 的 base URL 常數推導出合法 host 白名單，避免另行
// 硬編碼字面（§26 精神 + #1394 深度防禦）。
var payUniActionHosts = func() map[string]struct{} {
	hosts := make(map[string]struct{})
	for _, base := range []string{payuni.SandboxBaseURL, payuni.ProdBaseURL} {
		if u, err := url.Parse(base); err == nil && u.Host != "" {
			hosts[u.Host] = struct{}{}
		}
	}
	return hosts
}()

// isAllowedPayUniActionURL 斷言 Form POST 目標為 https 且 host 屬 PayUni 白名單。
// 即使 actionURL 來自後端 canonical record，仍在 render 前驗證（深度防禦），
// 阻擋 open form-action（例：http://evil.com、https://evil.com、javascript:）。
func isAllowedPayUniActionURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if u.Scheme != "https" {
		return false
	}
	_, ok := payUniActionHosts[u.Host]
	return ok
}

// renderPayUniRedirect 驗證 action host → html/template 安全 render 中轉頁。
// 先 render 進 buffer 再輸出，確保驗證/render 失敗時仍能回受控錯誤頁（不半寫）。
func renderPayUniRedirect(c *gin.Context, form payUniRedirectForm) {
	if !isAllowedPayUniActionURL(form.ActionURL) {
		renderPayUniRedirectError(c, http.StatusBadRequest, "無效的付款目標位址")
		return
	}

	var buf bytes.Buffer
	if err := payUniRedirectTemplate.Execute(&buf, form); err != nil {
		renderPayUniRedirectError(c, http.StatusInternalServerError, "頁面產生失敗")
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, buf.String())
}

// renderPayUniRedirectError 回受控錯誤頁（明確 4xx/5xx，非 500 leak、非空白）。
// message 皆為靜態常數，額外 HTMLEscapeString 為 belt-and-suspenders。
func renderPayUniRedirectError(c *gin.Context, status int, message string) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(status, `<!DOCTYPE html><html lang="zh-TW"><head><meta charset="UTF-8"><title>頁面無法顯示</title></head>`+
		`<body style="font-family:-apple-system,BlinkMacSystemFont,sans-serif;text-align:center;padding:48px;color:#333;">`+
		`<p>`+template.HTMLEscapeString(message)+`</p></body></html>`)
}
