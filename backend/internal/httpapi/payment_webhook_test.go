package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type recordingEvents struct {
	calls int
	err   error
	body  string
}

func (r *recordingEvents) ReceiveEvent(_ context.Context, _ http.Header, body []byte) error {
	r.calls++
	r.body = string(body)
	return r.err
}

func webhookRouter(events *recordingEvents) (http.Handler, *int) {
	inner := 0
	csrf, err := newCSRF([]string{"https://app.test"})
	if err != nil {
		panic(err)
	}
	chain := csrf.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		inner++
		w.WriteHeader(http.StatusOK)
	}))
	return withPaymentWebhook(events, slog.New(slog.DiscardHandler), chain), &inner
}

func post(h http.Handler, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Origin", "https://evil.test")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// 簽章就是驗證：跨來源、沒有 session 的 POST 也到得了 webhook，而其他路徑仍然被 CSRF 擋下。
func TestPaymentWebhookBypassesCSRFAndSessionButOtherRoutesDoNot(t *testing.T) {
	ev := &recordingEvents{}
	h, inner := webhookRouter(ev)
	if rec := post(h, "/webhooks/stripe", `{"id":"evt_1"}`); rec.Code != http.StatusNoContent || ev.calls != 1 || ev.body != `{"id":"evt_1"}` || *inner != 0 {
		t.Fatalf("webhook：%d calls=%d inner=%d", rec.Code, ev.calls, *inner)
	}
	if rec := post(h, "/api/connect/x", "{}"); rec.Code != http.StatusForbidden || ev.calls != 1 || *inner != 0 {
		t.Fatalf("其他路徑：%d calls=%d inner=%d", rec.Code, ev.calls, *inner)
	}
}

func TestPaymentWebhookStatusCodes(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want int
	}{
		"接受、重送與不處理的事件都回 2xx": {nil, http.StatusNoContent},
		"簽章或格式不合":            {apperr.InvalidInput("簽章不合"), http.StatusBadRequest},
		"沒有設定 provider":      {apperr.FailedPrecondition("尚未設定"), http.StatusServiceUnavailable},
		"其他失敗":               {errors.New("db down"), http.StatusInternalServerError},
	} {
		h, _ := webhookRouter(&recordingEvents{err: tc.err})
		if rec := post(h, "/webhooks/stripe", "{}"); rec.Code != tc.want {
			t.Errorf("%s：%d，want %d", name, rec.Code, tc.want)
		}
	}
}

func TestPaymentWebhookRejectsOversizedBodiesBeforeCallingTheModule(t *testing.T) {
	ev := &recordingEvents{}
	h, _ := webhookRouter(ev)
	if rec := post(h, "/webhooks/stripe", strings.Repeat("x", 65<<10)); rec.Code != http.StatusRequestEntityTooLarge || ev.calls != 0 {
		t.Fatalf("%d calls=%d", rec.Code, ev.calls)
	}
}
