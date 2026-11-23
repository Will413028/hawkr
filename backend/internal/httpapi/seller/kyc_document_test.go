package seller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type ownDocModule struct {
	in      model.ReadOwnKYCDocumentInput
	content model.FileContent
	err     error
	calls   int
}

func (d *ownDocModule) ReadOwnKYCDocument(_ context.Context, in model.ReadOwnKYCDocumentInput) (model.FileContent, error) {
	d.in, d.calls = in, d.calls+1
	return d.content, d.err
}

func serveOwnDocument(m *ownDocModule, log *slog.Logger, ctx context.Context, path string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	mux.Handle(OwnDocumentRoute, NewOwnDocuments(m, log))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, path, nil))
	return rec
}

// 賣家讀自己證件的端點：只給顧客帳號（沒登入 401、管理員 403），帳號 ID 來自 session、位置來自路徑；格式不對的位置不呼叫模組；
// 回應不可快取、不讓瀏覽器猜型別，錯誤只有 HTTP 狀態。
func TestOwnDocumentEndpointAuthorizesAccountsAndMapsErrorsWithoutLeaking(t *testing.T) {
	customer := rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAccount, ID: 5})
	for name, c := range map[string]struct {
		ctx  context.Context
		want int
	}{"visitor": {context.Background(), 401}, "admin": {adminCtx(42), 403}} {
		m := &ownDocModule{}
		rec := serveOwnDocument(m, nil, c.ctx, "/api/files/seller/kyc/0")
		if rec.Code != c.want || m.calls != 0 || rec.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatalf("%s：code=%d calls=%d", name, rec.Code, m.calls)
		}
	}
	for _, bad := range []string{"x", "-1", "1.5", ""} {
		m := &ownDocModule{}
		if rec := serveOwnDocument(m, nil, customer, "/api/files/seller/kyc/"+bad); rec.Code != 404 || m.calls != 0 {
			t.Fatalf("%q：code=%d calls=%d", bad, rec.Code, m.calls)
		}
	}
	m := &ownDocModule{content: model.FileContent{Body: io.NopCloser(strings.NewReader("IMG")), ContentType: "image/png"}}
	rec := serveOwnDocument(m, nil, customer, "/api/files/seller/kyc/3")
	if rec.Code != 200 || rec.Body.String() != "IMG" || m.in.AccountID != 5 || m.in.Position != 3 {
		t.Fatalf("code=%d in=%+v", rec.Code, m.in)
	}
	if h := rec.Header(); h.Get("Content-Type") != "image/png" || h.Get("Cache-Control") != "private, no-store" || h.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("headers=%v", h)
	}
	for name, c := range map[string]struct {
		err   error
		code  int
		level string
	}{
		"missing in media": {fmt.Errorf("x：%w", model.ErrFileMissing), 404, "ERROR"},
		"not secure":       {fmt.Errorf("x：%w", media.ErrNotSecure), 404, "ERROR"},
		"not found":        {apperr.NotFound("找不到這份文件"), 404, ""},
		"forbidden":        {apperr.PermissionDenied("x"), 403, ""},
		"media down":       {errors.New("secret internal detail"), 500, "ERROR"},
	} {
		var logs strings.Builder
		rec := serveOwnDocument(&ownDocModule{err: c.err}, slog.New(slog.NewTextHandler(&logs, nil)), customer, "/api/files/seller/kyc/0")
		if rec.Code != c.code || strings.Contains(rec.Body.String(), "secret") {
			t.Fatalf("%s：code=%d body=%q", name, rec.Code, rec.Body.String())
		}
		if c.level == "" && logs.Len() != 0 || c.level != "" && !strings.Contains(logs.String(), "level="+c.level) {
			t.Fatalf("%s：log=%q，要 %q", name, logs.String(), c.level)
		}
	}
	if got := ownDocumentURI(2); got != "/api/files/seller/kyc/2" {
		t.Fatalf("uri=%q", got)
	}
}

type failingBody struct {
	head string
	err  error
	read bool
}

func (f *failingBody) Read(p []byte) (int, error) {
	if f.read || f.head == "" {
		return 0, f.err
	}
	f.read = true
	return copy(p, f.head), nil
}

func (f *failingBody) Close() error { return nil }

// 第一段讀不出來（解密、儲存或完整性失敗）時還沒送 header：回 500 加 error log，不是先送 200 再中斷。
func TestOwnDocumentFirstSegmentFailureReturns500BeforeAnyHeader(t *testing.T) {
	customer := rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAccount, ID: 5})
	var logs strings.Builder
	m := &ownDocModule{content: model.FileContent{Body: &failingBody{err: errors.New("secret chunk failure")}, ContentType: "image/png"}}
	rec := serveOwnDocument(m, slog.New(slog.NewTextHandler(&logs, nil)), customer, "/api/files/seller/kyc/0")
	if rec.Code != 500 || strings.Contains(rec.Body.String(), "secret") || rec.Header().Get("Content-Type") == "image/png" {
		t.Fatalf("code=%d body=%q headers=%v", rec.Code, rec.Body.String(), rec.Header())
	}
	if !strings.Contains(logs.String(), "level=ERROR") {
		t.Fatalf("log=%q", logs.String())
	}
}

// 送出中途讀取端失敗：狀態碼已是 200、改不了，中斷連線並記 error（不是 client 斷線的 warn）。
func TestOwnDocumentMidStreamReadFailureAbortsAndLogsError(t *testing.T) {
	customer := rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAccount, ID: 5})
	var logs strings.Builder
	m := &ownDocModule{content: model.FileContent{Body: &failingBody{head: "IMG", err: errors.New("segment 2 corrupt")}, ContentType: "image/png"}}
	defer func() {
		if r := recover(); r != any(http.ErrAbortHandler) {
			t.Fatalf("recover=%v，要 http.ErrAbortHandler", r)
		}
		if !strings.Contains(logs.String(), "level=ERROR") || strings.Contains(logs.String(), "level=WARN") {
			t.Fatalf("log=%q", logs.String())
		}
	}()
	serveOwnDocument(m, slog.New(slog.NewTextHandler(&logs, nil)), customer, "/api/files/seller/kyc/0")
}
