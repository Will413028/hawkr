package handler

import (
	"bytes"
	"html/template"
	"net/http"
	"net/url"

	"github.com/Will413028/hawkr/go-pkg/payuni"
	"github.com/gin-gonic/gin"
)

type payUniRedirectForm struct {
	Title       string
	FormID      string
	ActionURL   string
	MerID       string
	Version     string
	EncryptInfo string
	HashInfo    string
	IsPlatForm  string
}

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

var payUniActionHosts = func() map[string]struct{} {
	hosts := make(map[string]struct{})
	for _, base := range []string{payuni.SandboxBaseURL, payuni.ProdBaseURL} {
		if u, err := url.Parse(base); err == nil && u.Host != "" {
			hosts[u.Host] = struct{}{}
		}
	}
	return hosts
}()

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

func renderPayUniRedirectError(c *gin.Context, status int, message string) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(status, `<!DOCTYPE html><html lang="zh-TW"><head><meta charset="UTF-8"><title>頁面無法顯示</title></head>`+
		`<body style="font-family:-apple-system,BlinkMacSystemFont,sans-serif;text-align:center;padding:48px;color:#333;">`+
		`<p>`+template.HTMLEscapeString(message)+`</p></body></html>`)
}
