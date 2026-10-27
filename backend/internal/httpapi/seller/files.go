package seller

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
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

// writeFile 回應檔案本身；型別只放行圖片與 PDF（safeContentType）。
func writeFile(w http.ResponseWriter, content model.FileContent) {
	w.Header().Set("Content-Type", safeContentType(content.ContentType))
	w.Header().Set("Content-Disposition", "inline")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content.Data)
}

// retryAfterSeconds 是檔案的 media 引用還沒同步完時，請 client 多久之後重試（同步 job 的退避在秒到分鐘之間）。
const retryAfterSeconds = 30

// fileFailure 把模組的錯誤轉成 HTTP 狀態，不帶任何細節：沒有授權一律 403（不洩漏資源存在與否），查無 404；引用還沒同步完 503
// 加 Retry-After（warn）；media 說不是機敏資產、或我們有這份檔案而 media 說沒有（引用已同步），是資料不一致，404 加 error log
// （要人處理，不是 client 能解的）；暫時性的問題 503；其餘（含沒有分類的錯誤）500 並記 error。
// what 是記 log 用的檔案名稱。
func fileFailure(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error, what string) {
	switch {
	case errors.Is(err, model.ErrFileNotSynced):
		log.WarnContext(r.Context(), "seller："+what+"的 media 引用還沒同步完", "error", err)
		w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
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
