package httpapi

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
)

type recordingChargeEvents struct {
	calls int
	body  string
	err   error
}

func (r *recordingChargeEvents) ReceivePaymentEvent(_ context.Context, _ http.Header, body []byte) error {
	r.calls++
	r.body = string(body)
	return r.err
}

func chargeWebhookRouter(charge *recordingChargeEvents, log *slog.Logger) http.Handler {
	return withChargeWebhook(charge, log, http.NotFoundHandler())
}

// 付款是 snapshot event（內含完整物件），本文上限比 thin event 的收款帳戶寬；超過付款自己的上限仍在呼叫模組之前擋下。
// mutation：付款路由改回用收款帳戶的 64 KiB 上限 → 100 KiB 的本文回 413，失敗。
func TestChargeWebhookAcceptsSnapshotSizedBodiesButStillCapsThem(t *testing.T) {
	charge := &recordingChargeEvents{}
	h := chargeWebhookRouter(charge, slog.New(slog.DiscardHandler))
	if rec := post(h, "/webhooks/stripe-payments", strings.Repeat("x", 100<<10)); rec.Code != http.StatusNoContent || charge.calls != 1 {
		t.Fatalf("100 KiB 的 snapshot：%d calls=%d", rec.Code, charge.calls)
	}
	if rec := post(h, "/webhooks/stripe-payments", strings.Repeat("x", 257<<10)); rec.Code != http.StatusRequestEntityTooLarge || charge.calls != 1 {
		t.Fatalf("257 KiB：%d calls=%d", rec.Code, charge.calls)
	}
}

// 付款 webhook 失敗的 log 用付款自己的標籤，不是「收款」開頭的帳戶文字。
func TestChargeWebhookFailureLogNamesThePaymentDestination(t *testing.T) {
	var logs bytes.Buffer
	h := chargeWebhookRouter(&recordingChargeEvents{err: errors.New("db down")}, slog.New(slog.NewTextHandler(&logs, nil)))
	if rec := post(h, "/webhooks/stripe-payments", "{}"); rec.Code != http.StatusInternalServerError {
		t.Fatalf("%d", rec.Code)
	}
	if got := logs.String(); !strings.Contains(got, "付款 webhook 處理失敗") || strings.Contains(got, "收款") {
		t.Fatalf("log=%q", got)
	}
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
