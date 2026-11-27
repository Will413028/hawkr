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

func safeContentType(ct string) string {
	base, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(ct)), ";")
	if strings.HasPrefix(base, "image/") && !strings.Contains(base, "svg") || base == "application/pdf" {
		return base
	}
	return "application/octet-stream"
}
