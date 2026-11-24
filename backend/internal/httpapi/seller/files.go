package seller

import (
	"bufio"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 後台機敏檔端點（KYC 證件、治理佐證）共用的傳輸規則：回應（含錯誤）一律不可快取、不讓瀏覽器猜型別，錯誤只有 HTTP 狀態、不帶
// 細節。為什麼是普通的 HTTP GET、不是 Connect，見 admin_document.go。

// adminFile 設好機敏檔回應的 header，取出呼叫的管理員；沒有登入回 401、不是管理員回 403（權限已由 adminauth.GuardFile 檢查）。
func adminFile(w http.ResponseWriter, r *http.Request) (int64, bool) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	p, ok := rpc.PrincipalFrom(r.Context())
	switch {
	case !ok:
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return 0, false
	case p.Kind != identity.PrincipalAdmin:
		http.Error(w, "forbidden", http.StatusForbidden)
		return 0, false
	}
	return p.ID, true
}

// writeFile 串流回應檔案本身並關閉 Body；型別只放行圖片與 PDF（safeContentType）。先讀出第一段再送 header：第一段的
// 解密、儲存或完整性失敗還能回 500 加 error log，不會先送 200。送出中途失敗時回應已開始、狀態碼改不了，中斷連線讓 client 看到
// 截斷而不是一份看似完整的檔；client 斷線記 warn，讀取端（解密、儲存、完整性）的失敗記 error。
func writeFile(w http.ResponseWriter, r *http.Request, log *slog.Logger, content model.FileContent) {
	defer func() { _ = content.Body.Close() }()
	br := bufio.NewReader(content.Body)
	if _, err := br.Peek(1); err != nil && !errors.Is(err, io.EOF) {
		fileFailure(w, r, log, err, "檔案")
		return
	}
	w.Header().Set("Content-Type", safeContentType(content.ContentType))
	w.Header().Set("Content-Disposition", "inline")
	w.WriteHeader(http.StatusOK)
	cw := &clientWriter{w: w}
	if _, err := io.Copy(cw, br); err != nil {
		if cw.err != nil {
			log.WarnContext(r.Context(), "seller：送出機敏檔中斷（client 端）", "error", err)
		} else {
			log.ErrorContext(r.Context(), "seller：送出機敏檔中斷（讀取端失敗）", "error", err)
		}
		panic(http.ErrAbortHandler)
	}
}

// clientWriter 記下寫往 client 的錯誤，讓呼叫端分辨中斷來自 client 還是來自讀取端。
type clientWriter struct {
	w   io.Writer
	err error
}

func (c *clientWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	if err != nil {
		c.err = err
	}
	return n, err
}

// fileFailure 把模組的錯誤轉成 HTTP 狀態，不帶任何細節：沒有授權一律 403（不洩漏資源存在與否），查無 404；
// media 說不是機敏資產、或我們有這份檔案而 media 說沒有，是資料不一致，404 加 error log
// （要人處理，不是 client 能解的）；暫時性的問題 503；其餘（含沒有分類的錯誤）500 並記 error。
// what 是記 log 用的檔案名稱。
func fileFailure(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error, what string) {
	switch {
	case errors.Is(err, media.ErrNotSecure), errors.Is(err, model.ErrFileMissing):
		log.ErrorContext(r.Context(), "seller："+what+"在 media 讀不到，資料不一致", "error", err)
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	switch apperr.KindOf(err) {
	case apperr.KindPermissionDenied, apperr.KindUnauthenticated:
		http.Error(w, "forbidden", http.StatusForbidden)
	case apperr.KindNotFound, apperr.KindInvalidInput:
		http.Error(w, "not found", http.StatusNotFound)
	case apperr.KindUnavailable, apperr.KindResourceExhausted:
		log.WarnContext(r.Context(), "seller：讀取"+what+"暫時失敗", "error", err)
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	case apperr.KindInternal, apperr.KindVersionConflict, apperr.KindAlreadyExists, apperr.KindFailedPrecondition:
		log.ErrorContext(r.Context(), "seller：讀取"+what+"失敗", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// safeContentType 只放行圖片與 PDF 的型別；media 沒給或給了別的型別（例如 text/html）一律當成不透明的二進位，瀏覽器不會把
// 上傳的內容當網頁執行。
func safeContentType(ct string) string {
	base, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(ct)), ";")
	if strings.HasPrefix(base, "image/") && !strings.Contains(base, "svg") || base == "application/pdf" {
		return base
	}
	return "application/octet-stream"
}
