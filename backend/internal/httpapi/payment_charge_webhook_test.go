package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"testing"
)

type recordingChargeEvents struct {
	calls int
	body  string
}

func (r *recordingChargeEvents) ReceivePaymentEvent(_ context.Context, _ http.Header, body []byte) error {
	r.calls++
	r.body = string(body)
	return nil
}

// 付款事件有自己的路由：它到達付款的處理者而不是收款帳戶的處理者（兩個 destination 兩把簽章密鑰），同樣不經 CSRF 與 session，其他路徑仍被擋下。
func TestChargeWebhookIsSeparateFromTheAccountWebhook(t *testing.T) {
	account, charge := &recordingEvents{}, &recordingChargeEvents{}
	csrf, err := newCSRF([]string{"https://app.test"})
	if err != nil {
		t.Fatal(err)
	}
	inner := 0
	chain := csrf.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { inner++; w.WriteHeader(http.StatusOK) }))
	h := withChargeWebhook(charge, slog.New(slog.DiscardHandler), withPaymentWebhook(account, slog.New(slog.DiscardHandler), chain))
	if rec := post(h, "/webhooks/stripe-payments", `{"id":"evt_p"}`); rec.Code != http.StatusNoContent || charge.calls != 1 || charge.body != `{"id":"evt_p"}` || account.calls != 0 || inner != 0 {
		t.Fatalf("付款 webhook：%d charge=%d account=%d inner=%d", rec.Code, charge.calls, account.calls, inner)
	}
	if rec := post(h, "/webhooks/stripe", `{"id":"evt_a"}`); rec.Code != http.StatusNoContent || charge.calls != 1 || account.calls != 1 {
		t.Fatalf("收款帳戶 webhook：%d charge=%d account=%d", rec.Code, charge.calls, account.calls)
	}
	if rec := post(h, "/api/connect/x", "{}"); rec.Code != http.StatusForbidden || inner != 0 {
		t.Fatalf("其他路徑：%d inner=%d", rec.Code, inner)
	}
}
