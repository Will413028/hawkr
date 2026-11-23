package media

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 機敏檔分段上傳的單體端點（B1-D17，API 設計規範（Connect）§十三）：原始位元組 body，不經 Connect。錯誤沿用 /api/files/ 的做法：
// 只回 HTTP 狀態、純文字、不帶細節；唯一例外是 index 不符回 409 並帶 Hawkr-Next-Index。

const (
	// ChunkRoute 是段上傳端點的 ServeMux 樣式。
	ChunkRoute = "PUT /api/files/secure_uploads/{token}/chunks/{index}"
	// AdminChunkRoute 是代行營運的段上傳端點（B1-D16）：上傳者是開會話的管理員，後台路由，權限由 fileRoute.permission 宣告。
	AdminChunkRoute = "PUT /api/files/admin/secure_uploads/{token}/chunks/{index}"
	// PreviewRoute 是確認前暫存預覽端點的 ServeMux 樣式。
	PreviewRoute = "GET /api/files/secure_uploads/{token}"

	// NextIndexHeader 是 index 不符（409）時帶回下一個要送的段序號的回應 header。
	NextIndexHeader = "Hawkr-Next-Index"

	// ChunkReadTimeout 是每個段請求讀 body 的時限：4 MiB 在 512 kbit/s 上行約 66 秒，低於 Vercel 外部 rewrite 的 120 秒。
	ChunkReadTimeout = 90 * time.Second

	// metaNextIndex 是 media 模組放在 FailedPrecondition metadata 的鍵（upload.metaNextIndex）。
	metaNextIndex = "next_index"
)

// SecureFiles 是端點需要的模組方法（*media.Module）。
type SecureFiles interface {
	PutSecureChunk(ctx context.Context, uploader media.Uploader, token string, index int, body io.Reader, length int64) (media.SecureChunkResult, error)
	ReadStagedSecure(ctx context.Context, uploader media.Uploader, token string) (io.ReadCloser, error)
}

// SecureUploads 是 PUT 段上傳與 GET 預覽的 handler。
type SecureUploads struct {
	m           SecureFiles
	log         *slog.Logger
	readTimeout time.Duration
}

// NewSecureUploads 建立端點的 handler。
func NewSecureUploads(m SecureFiles, log *slog.Logger) *SecureUploads {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &SecureUploads{m: m, log: log, readTimeout: ChunkReadTimeout}
}

// Chunk 回 PUT 的 handler。
func (h *SecureUploads) Chunk() http.Handler { return http.HandlerFunc(h.putChunk) }

// AdminChunk 回後台 PUT 的 handler：段請求的語意與 Chunk 完全相同（時限、大小上限、409 與 Hawkr-Next-Index、錯誤分類），只有上傳者不同：
// 呼叫者是管理員，Uploader 是 {ID: 管理員 ID, Admin: true}，會話只有開它的管理員能續傳。
func (h *SecureUploads) AdminChunk() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { h.chunk(w, r, h.adminUploader) })
}

// Preview 回 GET 的 handler。
func (h *SecureUploads) Preview() http.Handler { return http.HandlerFunc(h.getPreview) }

// uploader 設好機敏回應的 header 並取出呼叫的顧客帳號；沒有登入 401、不是顧客帳號 403。
func (h *SecureUploads) uploader(w http.ResponseWriter, r *http.Request) (media.Uploader, bool) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	p, ok := rpc.PrincipalFrom(r.Context())
	switch {
	case !ok:
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return media.Uploader{}, false
	case p.Kind != identity.PrincipalAccount:
		http.Error(w, "forbidden", http.StatusForbidden)
		return media.Uploader{}, false
	}
	return media.Uploader{ID: p.ID}, true
}

// adminUploader 設好機敏回應的 header 並取出呼叫的管理員；沒有登入 401、不是管理員 403（角色是否允許由 adminauth.GuardFile 先擋）。
func (h *SecureUploads) adminUploader(w http.ResponseWriter, r *http.Request) (media.Uploader, bool) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	p, ok := rpc.PrincipalFrom(r.Context())
	switch {
	case !ok:
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return media.Uploader{}, false
	case p.Kind != identity.PrincipalAdmin:
		http.Error(w, "forbidden", http.StatusForbidden)
		return media.Uploader{}, false
	}
	return media.Uploader{ID: p.ID, Admin: true}, true
}

func (h *SecureUploads) putChunk(w http.ResponseWriter, r *http.Request) { h.chunk(w, r, h.uploader) }

func (h *SecureUploads) chunk(w http.ResponseWriter, r *http.Request, who func(http.ResponseWriter, *http.Request) (media.Uploader, bool)) {
	u, ok := who(w, r)
	if !ok {
		return
	}
	rawIndex := r.PathValue("index")
	index, err := strconv.Atoi(rawIndex)
	if err != nil || index < 0 || strconv.Itoa(index) != rawIndex {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if r.ContentLength < 0 {
		http.Error(w, "length required", http.StatusLengthRequired)
		return
	}
	if r.ContentLength > media.SecureChunkSize {
		http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
		return
	}
	if err := http.NewResponseController(w).SetReadDeadline(time.Now().Add(h.readTimeout)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		h.log.WarnContext(r.Context(), "media：設定段 body 的讀取時限失敗", "error", err)
	}
	body := &originReader{r: http.MaxBytesReader(w, r.Body, media.SecureChunkSize)}
	if _, err := h.m.PutSecureChunk(r.Context(), u, r.PathValue("token"), index, body, r.ContentLength); err != nil {
		h.failure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SecureUploads) getPreview(w http.ResponseWriter, r *http.Request) {
	u, ok := h.uploader(w, r)
	if !ok {
		return
	}
	rc, err := h.m.ReadStagedSecure(r.Context(), u, r.PathValue("token"))
	if err != nil {
		h.failure(w, r, err)
		return
	}
	defer func() { _ = rc.Close() }()
	br := bufio.NewReader(rc)
	head, err := br.Peek(512)
	if err != nil && !errors.Is(err, io.EOF) {
		h.failure(w, r, err)
		return
	}
	w.Header().Set("Content-Type", safeContentType(http.DetectContentType(head)))
	w.Header().Set("Content-Disposition", "inline")
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, br); err != nil {
		h.log.WarnContext(r.Context(), "media：送出機敏暫存預覽中斷", "error", err)
		panic(http.ErrAbortHandler)
	}
}

// failure 把錯誤轉成 HTTP 狀態：讀 body 的問題（超過上限 413、逾時 408、client 中斷 400）；index 不符 409 加 Hawkr-Next-Index；
// 其他的 FailedPrecondition（過期、已完成、不能預覽）412；沒有授權與不存在一律 404 或 403 不可區分；滿載與暫時性問題 503（滿載帶
// Retry-After）；其餘 500 並記 error。
func (h *SecureUploads) failure(w http.ResponseWriter, r *http.Request, err error) {
	var tooLarge *http.MaxBytesError
	var ae *apperr.Error
	switch {
	case errors.As(err, &tooLarge):
		http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
		return
	case errors.Is(err, os.ErrDeadlineExceeded):
		http.Error(w, "request timeout", http.StatusRequestTimeout)
		return
	case isClientAbort(r, err):
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	case errors.Is(err, media.ErrNoKeyring):
		h.log.ErrorContext(r.Context(), "media：機敏上傳需要 KEK 但模組沒有設定", "error", err)
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	case errors.Is(err, media.ErrNotSecure):
		http.Error(w, "not found", http.StatusNotFound)
		return
	case errors.As(err, &ae) && ae.Kind() == apperr.KindFailedPrecondition:
		if next, ok := ae.Metadata[metaNextIndex]; ok {
			w.Header().Set(NextIndexHeader, next)
			http.Error(w, "conflict", http.StatusConflict)
			return
		}
		http.Error(w, "precondition failed", http.StatusPreconditionFailed)
		return
	}
	switch apperr.KindOf(err) {
	case apperr.KindPermissionDenied, apperr.KindUnauthenticated:
		http.Error(w, "forbidden", http.StatusForbidden)
	case apperr.KindNotFound:
		http.Error(w, "not found", http.StatusNotFound)
	case apperr.KindInvalidInput:
		http.Error(w, "bad request", http.StatusBadRequest)
	case apperr.KindResourceExhausted:
		if errors.As(err, &ae) && ae.RetryAfter > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int((ae.RetryAfter+time.Second-1)/time.Second)))
		}
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	case apperr.KindUnavailable:
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	case apperr.KindInternal, apperr.KindVersionConflict, apperr.KindAlreadyExists, apperr.KindFailedPrecondition:
		h.log.ErrorContext(r.Context(), "media：機敏上傳端點失敗", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	default:
		if errors.Is(err, context.Canceled) {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		h.log.ErrorContext(r.Context(), "media：機敏上傳端點失敗", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// bodyReadError 標記錯誤來自讀請求 body。
type bodyReadError struct{ err error }

func (e *bodyReadError) Error() string { return e.err.Error() }
func (e *bodyReadError) Unwrap() error { return e.err }

// originReader 把讀 body 的錯誤標成 bodyReadError，讓 failure 分辨 client 斷線與儲存或資料庫的失敗。
type originReader struct{ r io.Reader }

func (o *originReader) Read(p []byte) (int, error) {
	n, err := o.r.Read(p)
	if err != nil && err != io.EOF {
		err = &bodyReadError{err: err}
	}
	return n, err
}

// isClientAbort 判斷是 client 中途斷線或取消（不是伺服器的錯，不記 error）：請求的 context 已結束，或錯誤出自讀 body。
func isClientAbort(r *http.Request, err error) bool {
	var be *bodyReadError
	return r.Context().Err() != nil || errors.As(err, &be)
}

// safeContentType 只放行圖片與 PDF 的型別，其餘（含 SVG）一律當不透明的二進位。
func safeContentType(ct string) string {
	base, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(ct)), ";")
	if strings.HasPrefix(base, "image/") && !strings.Contains(base, "svg") || base == "application/pdf" {
		return base
	}
	return "application/octet-stream"
}
