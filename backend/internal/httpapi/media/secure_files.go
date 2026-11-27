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

	httpdelegation "github.com/Will413028/hawkr/backend/internal/httpapi/delegation"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/media/v1alpha1/mediav1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)


const (
	// ChunkRoute 是段上傳端點的 ServeMux 樣式。
	ChunkRoute = "PUT /api/files/secure_uploads/{token}/chunks/{index}"
	// DelegatedChunkRoute 是代行營運的段上傳端點（B1-D16）：只以代行 session 憑證授權，不是後台路由（沒有管理員 Bearer 與權限）。
	DelegatedChunkRoute = "PUT /api/files/delegated/secure_uploads/{token}/chunks/{index}"
	// PreviewRoute 是確認前暫存預覽端點的 ServeMux 樣式。
	PreviewRoute = "GET /api/files/secure_uploads/{token}"
	// DelegatedPreviewRoute 是代行營運的暫存預覽端點（B1-D16）：只有開會話的那個代行 session 能看。
	DelegatedPreviewRoute = "GET /api/files/delegated/secure_uploads/{token}"

	// NextIndexHeader 是 index 不符（409）時帶回下一個要送的段序號的回應 header。
	NextIndexHeader = "Hawkr-Next-Index"

	// ChunkReadTimeout 是每個段請求讀 body 的時限：4 MiB 在 512 kbit/s 上行約 66 秒，低於 Vercel 外部 rewrite 的 120 秒。
	ChunkReadTimeout = 90 * time.Second

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
	sessions DelegationAccess
}

// DelegationAccess 是代行路由需要的 delegation 公開 API（*delegation.Module）：憑證解析與授權（httpdelegation.Sessions），加上店家可代行的檢查（DelegatedRead）。
type DelegationAccess interface {
	httpdelegation.Sessions
	DelegationReader
}

// NewSecureUploads 建立端點的 handler。
func NewSecureUploads(m SecureFiles, log *slog.Logger) *SecureUploads {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &SecureUploads{m: m, log: log, readTimeout: ChunkReadTimeout}
}

// WithDelegation 接上代行路由驗代行 session 的來源。
func (h *SecureUploads) WithDelegation(sessions DelegationAccess) *SecureUploads {
	h.sessions = sessions
	return h
}

// Chunk 回 PUT 的 handler。
func (h *SecureUploads) Chunk() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { h.chunk(w, r, h.uploader) })
}

// DelegatedChunk 回代行營運 PUT 的 handler：段請求的語意與 Chunk 完全相同（時限、大小上限、409 與 Hawkr-Next-Index、錯誤分類），
// 只有授權不同：憑證只有 X-HK-Operation-Session（website 代行工作區的 BFF 轉送，沒有管理員 Bearer 也沒有顧客登入），Uploader 是
// {ID: session 的管理員, Admin: true, Store: session 的店家}，見 delegatedUploader。
func (h *SecureUploads) DelegatedChunk() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { h.chunk(w, r, h.delegatedUploader) })
}

// Preview 回 GET 的 handler。
func (h *SecureUploads) Preview() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { h.preview(w, r, h.uploader) })
}

// DelegatedPreview 回代行營運 GET 預覽的 handler：預覽語意與 Preview 相同，授權與 DelegatedChunk 相同。
func (h *SecureUploads) DelegatedPreview() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { h.preview(w, r, h.delegatedUploader) })
}

type authenticate func(w http.ResponseWriter, r *http.Request) (u media.Uploader, touch func(), ok bool)

func sensitiveHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}

func (h *SecureUploads) uploader(w http.ResponseWriter, r *http.Request) (media.Uploader, func(), bool) {
	sensitiveHeaders(w)
	p, ok := rpc.PrincipalFrom(r.Context())
	switch {
	case !ok:
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return media.Uploader{}, nil, false
	case p.Kind != identity.PrincipalAccount:
		http.Error(w, "forbidden", http.StatusForbidden)
		return media.Uploader{}, nil, false
	}
	return media.Uploader{ID: p.ID}, nil, true
}

func (h *SecureUploads) delegatedUploader(w http.ResponseWriter, r *http.Request) (media.Uploader, func(), bool) {
	sensitiveHeaders(w)
	if h.sessions == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return media.Uploader{}, nil, false
	}
	c, err := httpdelegation.Authorize(r.Context(), h.sessions, h.log, r.Pattern, delegatedUploadMethod, r.Header)
	var u media.Uploader
	if err == nil {
		err = h.sessions.DelegatedRead(r.Context(), c, func(_ context.Context, st seller.Store) error {
			u = media.Uploader{ID: c.Session.Actor.AdminID, Admin: true, Store: st.StoreID}
			return nil
		})
	}
	if err == nil {
		touch := func() { httpdelegation.Touch(r.Context(), h.sessions, h.log, c.Session.ID) }
		return u, touch, true
	}
	switch apperr.KindOf(err) {
	case apperr.KindUnauthenticated:
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
	case apperr.KindPermissionDenied, apperr.KindFailedPrecondition:
		http.Error(w, "forbidden", http.StatusForbidden)
	case apperr.KindNotFound, apperr.KindInternal, apperr.KindInvalidInput, apperr.KindVersionConflict, apperr.KindAlreadyExists,
		apperr.KindUnavailable, apperr.KindResourceExhausted:
		h.log.ErrorContext(r.Context(), "media：代行上傳驗證代行 session 失敗", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	default:
		h.log.ErrorContext(r.Context(), "media：代行上傳驗證代行 session 失敗", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
	return media.Uploader{}, nil, false
}

var delegatedUploadMethod = DelegatedDeclaration.Methods[mediav1alpha1connect.MediaDelegatedServiceBeginSecureUploadProcedure]

func (h *SecureUploads) chunk(w http.ResponseWriter, r *http.Request, who authenticate) {
	u, touch, ok := who(w, r)
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
	if touch != nil {
		touch()
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SecureUploads) preview(w http.ResponseWriter, r *http.Request, who authenticate) {
	u, touch, ok := who(w, r)
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
	if touch != nil {
		touch()
	}
	w.Header().Set("Content-Type", safeContentType(http.DetectContentType(head)))
	w.Header().Set("Content-Disposition", "inline")
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, br); err != nil {
		h.log.WarnContext(r.Context(), "media：送出機敏暫存預覽中斷", "error", err)
		panic(http.ErrAbortHandler)
	}
}

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

type bodyReadError struct{ err error }

func (e *bodyReadError) Error() string { return e.err.Error() }
func (e *bodyReadError) Unwrap() error { return e.err }

type originReader struct{ r io.Reader }

func (o *originReader) Read(p []byte) (int, error) {
	n, err := o.r.Read(p)
	if err != nil && err != io.EOF {
		err = &bodyReadError{err: err}
	}
	return n, err
}

func isClientAbort(r *http.Request, err error) bool {
	var be *bodyReadError
	return r.Context().Err() != nil || errors.As(err, &be)
}

func safeContentType(ct string) string {
	base, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(ct)), ";")
	if strings.HasPrefix(base, "image/") && !strings.Contains(base, "svg") || base == "application/pdf" {
		return base
	}
	return "application/octet-stream"
}
