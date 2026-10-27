package rpc

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"

	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/content/v1alpha1/contentv1alpha1connect"
)

// records 收集 log，給測試比對。
type records struct {
	mu   sync.Mutex
	msgs []string
}

func (r *records) Enabled(context.Context, slog.Level) bool { return true }
func (r *records) WithAttrs([]slog.Attr) slog.Handler       { return r }
func (r *records) WithGroup(string) slog.Handler            { return r }
func (r *records) Handle(_ context.Context, rec slog.Record) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msgs = append(r.msgs, rec.Message)
	return nil
}

// 每個錯誤回應都有且只有一筆 log：interceptor 記過的不再記，interceptor 之前的由 LogUnhandled 補上。
func TestEveryErrorResponseIsLoggedOnce(t *testing.T) {
	rec := &records{}
	log := slog.New(rec)
	path, h := contentv1alpha1connect.NewContentPublicServiceHandler(
		contentv1alpha1connect.UnimplementedContentPublicServiceHandler{},
		connect.WithInterceptors(NewInterceptor(log)))
	srv := httptest.NewServer(LogUnhandled(log, h))
	defer srv.Close()

	cases := []struct {
		name   string
		body   string
		status int
		want   string
	}{
		// Unimplemented handler 回的是 *connect.Error，經轉換點變成 internal（ADR D5）。
		{"handler 回的錯誤由 interceptor 記", `{}`, http.StatusInternalServerError, "rpc"},
		{"請求無法解碼：interceptor 之前", `{"page_size":"x"`, http.StatusBadRequest, "rpc：interceptor 之前的錯誤"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec.msgs = nil
			req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+path+"ListLiveBanners", strings.NewReader(c.body))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			if err := resp.Body.Close(); err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != c.status {
				t.Fatalf("status = %d，預期 %d", resp.StatusCode, c.status)
			}
			if len(rec.msgs) != 1 || rec.msgs[0] != c.want {
				t.Fatalf("log = %q，預期一筆 %q", rec.msgs, c.want)
			}
		})
	}
}
