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

func (h *SecureUploads) putChunk(w http.ResponseWriter, r *http.Request) {
	u, ok := h.uploader(w, r)
	if !ok {
		return
	}
	index, err := strconv.Atoi(r.PathValue("index"))
	if err != nil || index < 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if r.ContentLength < 0 {
		http.Error(w, "length required", http.StatusLengthRequired)
		return
	}
	if err := http.NewResponseController(w).SetReadDeadline(time.Now().Add(h.readTimeout)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		h.log.WarnContext(r.Context(), "media：設定段 body 的讀取時限失敗", "error", err)
	}
	body := http.MaxBytesReader(w, r.Body, media.SecureChunkSize)
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
	head, _ := br.Peek(512)
	w.Header().Set("Content-Type", safeContentType(http.DetectContentType(head)))
	w.Header().Set("Content-Disposition", "inline")
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, br); err != nil {
		h.log.WarnContext(r.Context(), "media：送出機敏暫存預覽中斷", "error", err)
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

// safeContentType 只放行圖片與 PDF 的型別，其餘（含 SVG）一律當不透明的二進位。
func safeContentType(ct string) string {
	base, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(ct)), ";")
	if strings.HasPrefix(base, "image/") && !strings.Contains(base, "svg") || base == "application/pdf" {
		return base
	}
	return "application/octet-stream"
}
