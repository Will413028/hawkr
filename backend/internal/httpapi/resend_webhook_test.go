package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type recordingResend struct {
	calls int
	err   error
	body  string
}

func (r *recordingResend) ReceiveResendEvent(_ context.Context, _ http.Header, body []byte) error {
	r.calls++
	r.body = string(body)
	return r.err
}

func resendRouter(ev *recordingResend) (http.Handler, *int) {
	inner := 0
	csrf, err := newCSRF([]string{"https://app.test"})
	if err != nil {
		panic(err)
	}
	chain := csrf.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		inner++
		w.WriteHeader(http.StatusOK)
	}))
	return withResendWebhook(ev, slog.New(slog.DiscardHandler), chain), &inner
}

// 簽章就是驗證：跨來源、沒有 session 的 POST 也到得了 Resend webhook，其他路徑仍被 CSRF 擋下。
func TestResendWebhookBypassesCSRFAndSessionButOtherRoutesDoNot(t *testing.T) {
	ev := &recordingResend{}
	h, inner := resendRouter(ev)
	if rec := post(h, "/webhooks/resend", `{"type":"email.bounced"}`); rec.Code != http.StatusNoContent || ev.calls != 1 || ev.body != `{"type":"email.bounced"}` || *inner != 0 {
		t.Fatalf("webhook：%d calls=%d inner=%d", rec.Code, ev.calls, *inner)
	}
	if rec := post(h, "/api/connect/x", "{}"); rec.Code != http.StatusForbidden || ev.calls != 1 || *inner != 0 {
		t.Fatalf("其他路徑：%d calls=%d inner=%d", rec.Code, ev.calls, *inner)
	}
}

func TestResendWebhookStatusCodes(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want int
	}{
		"接受與不處理的事件回 2xx": {nil, http.StatusNoContent},
		"簽章或格式不合":        {apperr.InvalidInput("簽章不合"), http.StatusBadRequest},
		"沒有設定密鑰":         {apperr.FailedPrecondition("尚未設定"), http.StatusServiceUnavailable},
		"其他失敗":           {errors.New("db down"), http.StatusInternalServerError},
	} {
		h, _ := resendRouter(&recordingResend{err: tc.err})
		if rec := post(h, "/webhooks/resend", "{}"); rec.Code != tc.want {
			t.Errorf("%s：%d，want %d", name, rec.Code, tc.want)
		}
	}
}

func TestResendWebhookRejectsOversizedBodiesBeforeCallingTheModule(t *testing.T) {
	ev := &recordingResend{}
	h, _ := resendRouter(ev)
	if rec := post(h, "/webhooks/resend", strings.Repeat("x", 65<<10)); rec.Code != http.StatusRequestEntityTooLarge || ev.calls != 0 {
		t.Fatalf("%d calls=%d", rec.Code, ev.calls)
	}
}
