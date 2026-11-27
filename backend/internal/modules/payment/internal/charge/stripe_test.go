package charge

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/stripe"
)

const (
	stripeTestKey       = "sk_test_unit"
	stripeTestWebhook   = "whsec_payments_unit"
	stripeTestAttemptID = "0192f0a0-0000-7000-8000-000000000001"
)

var stripeTestCreatedAfter = time.Date(2026, 11, 23, 4, 0, 0, 0, time.UTC)

type fixture struct {
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body"`
}

func loadFixture(t *testing.T, name string) fixture {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "platform", "stripe", "testdata", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var f fixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f fixture) object(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(f.Body, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func (f fixture) respond(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(f.Status)
	_, _ = w.Write(f.Body)
}

type recordedRequest struct {
	Method string
	Path   string
	Query  url.Values
	Form   url.Values
	Header http.Header
}

type deadlineTransport struct {
	next http.RoundTripper
	mu   sync.Mutex
	with int
	none int
}

func (d *deadlineTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	_, ok := req.Context().Deadline()
	d.mu.Lock()
	if ok {
		d.with++
	} else {
		d.none++
	}
	d.mu.Unlock()
	return d.next.RoundTrip(req)
}

type stripeRig struct {
	adapter  *Stripe
	deadline *deadlineTransport
	mu       sync.Mutex
	requests []recordedRequest
}

func (r *stripeRig) last(t *testing.T) recordedRequest {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.requests) == 0 {
		t.Fatal("沒有送出任何請求")
	}
	return r.requests[len(r.requests)-1]
}

func (r *stripeRig) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.requests)
}

func newStripeRig(t *testing.T, handler func(w http.ResponseWriter, req recordedRequest)) *stripeRig {
	t.Helper()
	rig := &stripeRig{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))
		rec := recordedRequest{Method: r.Method, Path: r.URL.Path, Query: r.URL.Query(), Form: form, Header: r.Header.Clone()}
		rig.mu.Lock()
		rig.requests = append(rig.requests, rec)
		rig.mu.Unlock()
		handler(w, rec)
	}))
	t.Cleanup(srv.Close)
	rig.deadline = &deadlineTransport{next: http.DefaultTransport}
	client, err := stripe.New(stripe.Config{
		BaseURL: srv.URL, SecretKey: stripeTestKey, WebhookSecret: stripeTestWebhook, AccountNamespace: "unit",
		HTTPClient: &http.Client{Transport: rig.deadline},
		Wait:       func(context.Context, time.Duration) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	rig.adapter, err = NewStripe(client)
	if err != nil {
		t.Fatal(err)
	}
	return rig
}

func fixed(f fixture) func(http.ResponseWriter, recordedRequest) {
	return func(w http.ResponseWriter, _ recordedRequest) { f.respond(w) }
}

func listBody(t *testing.T, hasMore bool, items ...map[string]any) []byte {
	t.Helper()
	if items == nil {
		items = []map[string]any{}
	}
	b, err := json.Marshal(map[string]any{"object": "list", "url": "/v1/x", "has_more": hasMore, "data": items})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func withMeta(obj map[string]any, attempt string) map[string]any {
	cp := make(map[string]any, len(obj)+1)
	for k, v := range obj {
		cp[k] = v
	}
	cp["metadata"] = map[string]any{stripe.MetaAttemptID: attempt}
	return cp
}

func withID(obj map[string]any, id string) map[string]any {
	cp := make(map[string]any, len(obj))
	for k, v := range obj {
		cp[k] = v
	}
	cp["id"] = id
	return cp
}

func stripeCalls(a *Stripe) map[string]func(context.Context) error {
	return map[string]func(context.Context) error{
		"CreatePayment": func(ctx context.Context) error {
			_, err := a.CreatePayment(ctx, CreateInput{PaymentID: uuid.NewV4(), OrderNos: []string{"order_A"}, Amount: 10000, TransferGroup: "group_A", IdempotencyKey: stripeTestAttemptID})
			return err
		},
		"CreateRefund": func(ctx context.Context) error {
			_, err := a.CreateRefund(ctx, RefundInput{ExternalPaymentID: "pi_x", Amount: 3000, IdempotencyKey: stripeTestAttemptID})
			return err
		},
		"CreateTransfer": func(ctx context.Context) error {
			_, err := a.CreateTransfer(ctx, TransferInput{Amount: 9000, SourceChargeID: "ch_x", DestinationAccountID: "acct_x", TransferGroup: "order_A", IdempotencyKey: stripeTestAttemptID})
			return err
		},
		"ReverseTransfer": func(ctx context.Context) error {
			_, err := a.ReverseTransfer(ctx, ReversalInput{ExternalTransferID: "tr_x", Amount: 3000, IdempotencyKey: stripeTestAttemptID})
			return err
		},
	}
}

type errClass int

const (
	classUnknown errClass = iota
	classPermanent
	classRetryable
)

func classOf(err error) errClass {
	switch {
	case errors.Is(err, ErrPermanent):
		return classPermanent
	case errors.Is(err, ErrRetryableRefusal):
		return classRetryable
	}
	return classUnknown
}

func (c errClass) String() string { return [...]string{"unknown", "permanent", "retryable"}[c] }

// 錯誤三分（D10）：樣本回應與合成的狀態碼，四種搬錢呼叫都要得到同一個分類。
func TestStripeMoneyErrorsAreClassifiedIntoThreeWays(t *testing.T) {
	cases := []struct {
		name string
		resp func(t *testing.T) fixture
		want errClass
	}{
		{"platform balance insufficient is a retryable refusal", func(t *testing.T) fixture { return loadFixture(t, "transfer_platform_insufficient_funds") }, classRetryable},
		{"429 is a retryable refusal", func(*testing.T) fixture {
			return fixture{Status: 429, Body: json.RawMessage(`{"error":{"type":"rate_limit_error","message":"slow down"}}`)}
		}, classRetryable},
		{"400 reversal above the remaining amount is permanent", func(t *testing.T) fixture { return loadFixture(t, "transfer_reversal_exceeds_remaining") }, classPermanent},
		{"400 transfer above the refunded source is permanent", func(t *testing.T) fixture { return loadFixture(t, "transfer_exceeds_refunded_source") }, classPermanent},
		{"400 currency mismatch is permanent", func(t *testing.T) fixture { return loadFixture(t, "transfer_currency_mismatch") }, classPermanent},
		{"409 idempotency_error is permanent", func(t *testing.T) fixture { return loadFixture(t, "create_provider_idem_conflict") }, classPermanent},
		{"404 is permanent", func(*testing.T) fixture {
			return fixture{Status: 404, Body: json.RawMessage(`{"error":{"code":"resource_missing","type":"invalid_request_error","message":"No such object"}}`)}
		}, classPermanent},
		{"409 without idempotency_error is unknown", func(*testing.T) fixture {
			return fixture{Status: 409, Body: json.RawMessage(`{"error":{"code":"idempotency_key_in_use","message":"in progress"}}`)}
		}, classUnknown},
		{"500 is unknown", func(*testing.T) fixture {
			return fixture{Status: 500, Body: json.RawMessage(`{"error":{"message":"boom"}}`)}
		}, classUnknown},
		{"503 is unknown", func(*testing.T) fixture { return fixture{Status: 503, Body: json.RawMessage(`{}`)} }, classUnknown},
		{"408 is unknown", func(*testing.T) fixture { return fixture{Status: 408, Body: json.RawMessage(`{}`)} }, classUnknown},
		{"401 is unknown", func(*testing.T) fixture {
			return fixture{Status: 401, Body: json.RawMessage(`{"error":{"type":"invalid_request_error","message":"Invalid API Key"}}`)}
		}, classUnknown},
		{"403 is unknown", func(*testing.T) fixture {
			return fixture{Status: 403, Body: json.RawMessage(`{"error":{"message":"denied"}}`)}
		}, classUnknown},
	}
	for _, tc := range cases {
		for call := range stripeCalls(nil) {
			t.Run(tc.name+"/"+call, func(t *testing.T) {
				resp := tc.resp(t)
				rig := newStripeRig(t, fixed(resp))
				err := stripeCalls(rig.adapter)[call](t.Context())
				if err == nil {
					t.Fatal("want an error")
				}
				if got := classOf(err); got != tc.want {
					t.Fatalf("class = %v, want %v (err: %v)", got, tc.want, err)
				}
			})
		}
	}
}

// 沒有 HTTP 回應的失敗（連線中斷、逾時）是不明，不得被歸成拒絕。
func TestStripeTransportFailuresAreUnknown(t *testing.T) {
	for call := range stripeCalls(nil) {
		t.Run("connection reset/"+call, func(t *testing.T) {
			rig := newStripeRig(t, func(w http.ResponseWriter, _ recordedRequest) {
				hj, ok := w.(http.Hijacker)
				if !ok {
					t.Error("no hijacker")
					return
				}
				conn, _, _ := hj.Hijack()
				_ = conn.Close()
			})
			err := stripeCalls(rig.adapter)[call](t.Context())
			if err == nil || classOf(err) != classUnknown {
				t.Fatalf("err = %v (class %v), want an unknown error", err, classOf(err))
			}
		})
		t.Run("deadline exceeded/"+call, func(t *testing.T) {
			client, err := stripe.New(stripe.Config{
				BaseURL: "http://127.0.0.1:1", SecretKey: stripeTestKey, WebhookSecret: stripeTestWebhook, AccountNamespace: "unit",
				HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, context.DeadlineExceeded })},
			})
			if err != nil {
				t.Fatal(err)
			}
			a, _ := NewStripe(client)
			err = stripeCalls(a)[call](t.Context())
			if err == nil || classOf(err) != classUnknown || !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("err = %v (class %v), want an unknown deadline error", err, classOf(err))
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// 2xx 但讀不懂（沒有 id）是不明：provider 可能已執行，不能當成功也不能當拒絕。
func TestStripeAnUnreadableSuccessIsUnknown(t *testing.T) {
	for call := range stripeCalls(nil) {
		t.Run(call, func(t *testing.T) {
			rig := newStripeRig(t, fixed(fixture{Status: 200, Body: json.RawMessage(`{}`)}))
			err := stripeCalls(rig.adapter)[call](t.Context())
			if err == nil || classOf(err) != classUnknown {
				t.Fatalf("err = %v (class %v), want an unknown error", err, classOf(err))
			}
		})
	}
}

// 每個出站呼叫都帶 deadline，即使呼叫方傳進沒有 deadline 的 ctx（port 的每個方法）。
func TestStripeEveryOutboundCallCarriesADeadline(t *testing.T) {
	pi := loadFixture(t, "payment_intent_confirmed_expanded")
	re := loadFixture(t, "refund_after_transfer")
	tr := loadFixture(t, "transfer_source_transaction")
	rv := loadFixture(t, "transfer_reversal_partial")
	rig := newStripeRig(t, func(w http.ResponseWriter, req recordedRequest) {
		p := req.Path
		list := func(obj map[string]any) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(listBody(t, false, obj))
		}
		switch {
		case strings.HasSuffix(p, "/reversals") && req.Method == http.MethodGet:
			list(withMeta(rv.object(t), stripeTestAttemptID))
		case strings.HasSuffix(p, "/reversals"):
			rv.respond(w)
		case strings.HasPrefix(p, "/v1/transfers") && req.Method == http.MethodGet:
			list(withMeta(tr.object(t), stripeTestAttemptID))
		case strings.HasPrefix(p, "/v1/transfers"):
			tr.respond(w)
		case strings.HasPrefix(p, "/v1/refunds") && req.Method == http.MethodGet && p == "/v1/refunds":
			list(withMeta(re.object(t), stripeTestAttemptID))
		case strings.HasPrefix(p, "/v1/refunds"):
			re.respond(w)
		case p == "/v1/payment_intents" && req.Method == http.MethodGet:
			list(withMeta(pi.object(t), stripeTestAttemptID))
		default:
			pi.respond(w)
		}
	})
	a, ctx := rig.adapter, context.Background()
	piID, _ := pi.object(t)["id"].(string)
	calls := map[string]func() error{
		"CreatePayment": func() error { return stripeCalls(a)["CreatePayment"](ctx) },
		"FindPayment":   func() error { _, _, err := a.FindPayment(ctx, stripeTestAttemptID, stripeTestCreatedAfter); return err },
		"GetPayment":    func() error { _, err := a.GetPayment(ctx, piID); return err },
		"CancelPayment": func() error { _, err := a.CancelPayment(ctx, piID); return err },
		"CreateRefund":  func() error { return stripeCalls(a)["CreateRefund"](ctx) },
		"FindRefund": func() error {
			_, _, err := a.FindRefund(ctx, piID, stripeTestAttemptID, stripeTestCreatedAfter)
			return err
		},
		"GetRefund":      func() error { _, err := a.GetRefund(ctx, "re_x"); return err },
		"CreateTransfer": func() error { return stripeCalls(a)["CreateTransfer"](ctx) },
		"FindTransfer": func() error {
			_, _, err := a.FindTransfer(ctx, "group_A", stripeTestAttemptID, stripeTestCreatedAfter)
			return err
		},
		"ReverseTransfer": func() error { return stripeCalls(a)["ReverseTransfer"](ctx) },
		"FindReversal":    func() error { _, _, err := a.FindReversal(ctx, "tr_x", stripeTestAttemptID); return err },
	}
	for name, call := range calls {
		before := rig.count()
		if err := call(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if rig.count() == before {
			t.Fatalf("%s: 沒有送出請求", name)
		}
	}
	rig.deadline.mu.Lock()
	defer rig.deadline.mu.Unlock()
	if rig.deadline.none != 0 || rig.deadline.with != rig.count() {
		t.Fatalf("%d 個請求沒有 deadline（共 %d 個）", rig.deadline.none, rig.count())
	}
}

func TestStripeCreatePaymentSendsCentsUSDGroupMetadataAndKey(t *testing.T) {
	pi := loadFixture(t, "payment_intent_confirmed_expanded")
	rig := newStripeRig(t, fixed(pi))
	pid := uuid.NewV4()
	snap, err := rig.adapter.CreatePayment(t.Context(), CreateInput{PaymentID: pid, OrderNos: []string{"order_A", "order_B"}, Amount: 10000, Description: "desc", TransferGroup: "group_A", IdempotencyKey: stripeTestAttemptID})
	if err != nil {
		t.Fatal(err)
	}
	req := rig.last(t)
	want := map[string]string{
		"amount": "10000", "currency": "usd", "transfer_group": "group_A", "description": "desc",
		"metadata[attempt_id]": stripeTestAttemptID, "metadata[payment_id]": pid.String(), "metadata[order_count]": "2", "metadata[order_nos]": "order_A,order_B",
		"automatic_payment_methods[enabled]": "true",
	}
	for k, v := range want {
		if got := req.Form.Get(k); got != v {
			t.Errorf("form %s = %q, want %q", k, got, v)
		}
	}
	if req.Method != http.MethodPost || req.Path != "/v1/payment_intents" || req.Header.Get("Idempotency-Key") != stripeTestAttemptID {
		t.Errorf("request = %s %s key %q", req.Method, req.Path, req.Header.Get("Idempotency-Key"))
	}
	obj := pi.object(t)
	if snap.ExternalID != obj["id"] || snap.State != StateSucceeded || snap.Amount != 10000 || snap.ClientSecret == "" {
		t.Errorf("snapshot = %+v", snap)
	}
	charge := obj["latest_charge"].(map[string]any)
	if snap.ChargeID != charge["id"] || snap.PaidAt.Unix() != int64(charge["created"].(float64)) {
		t.Errorf("charge %q paid %v, want %v %v", snap.ChargeID, snap.PaidAt, charge["id"], charge["created"])
	}
}

func TestStripeTransferRefundAndReversalRequestsAndSnapshots(t *testing.T) {
	tr := loadFixture(t, "transfer_source_transaction")
	re := loadFixture(t, "refund_after_transfer")
	rv := loadFixture(t, "transfer_reversal_partial")

	rig := newStripeRig(t, fixed(tr))
	ts, err := rig.adapter.CreateTransfer(t.Context(), TransferInput{Amount: 9000, SourceChargeID: "ch_src", DestinationAccountID: "acct_dst", TransferGroup: "order_A", IdempotencyKey: stripeTestAttemptID})
	if err != nil {
		t.Fatal(err)
	}
	req := rig.last(t)
	for k, v := range map[string]string{"amount": "9000", "currency": "usd", "destination": "acct_dst", "source_transaction": "ch_src", "transfer_group": "order_A", "metadata[attempt_id]": stripeTestAttemptID} {
		if req.Form.Get(k) != v {
			t.Errorf("transfer form %s = %q, want %q", k, req.Form.Get(k), v)
		}
	}
	if req.Path != "/v1/transfers" || req.Header.Get("Idempotency-Key") != stripeTestAttemptID {
		t.Errorf("transfer request = %s key %q", req.Path, req.Header.Get("Idempotency-Key"))
	}
	if obj := tr.object(t); ts.ExternalID != obj["id"] || ts.Amount != 9000 || ts.ReversedAmount != 0 {
		t.Errorf("transfer snapshot = %+v", ts)
	}

	rig = newStripeRig(t, fixed(re))
	rs, err := rig.adapter.CreateRefund(t.Context(), RefundInput{ExternalPaymentID: "pi_src", Amount: 3000, IdempotencyKey: stripeTestAttemptID})
	if err != nil {
		t.Fatal(err)
	}
	req = rig.last(t)
	if req.Form.Get("payment_intent") != "pi_src" || req.Form.Get("amount") != "3000" || req.Form.Get("metadata[attempt_id]") != stripeTestAttemptID || req.Form.Get("reverse_transfer") != "" {
		t.Errorf("refund form = %v", req.Form)
	}
	if obj := re.object(t); rs.ExternalID != obj["id"] || rs.State != RefundSucceeded || rs.Amount != 3000 {
		t.Errorf("refund snapshot = %+v", rs)
	}

	rig = newStripeRig(t, fixed(rv))
	rvs, err := rig.adapter.ReverseTransfer(t.Context(), ReversalInput{ExternalTransferID: "tr_src", Amount: 3000, IdempotencyKey: stripeTestAttemptID})
	if err != nil {
		t.Fatal(err)
	}
	req = rig.last(t)
	if req.Path != "/v1/transfers/tr_src/reversals" || req.Form.Get("amount") != "3000" || req.Form.Get("metadata[attempt_id]") != stripeTestAttemptID || req.Header.Get("Idempotency-Key") != stripeTestAttemptID {
		t.Errorf("reversal request = %s %v key %q", req.Path, req.Form, req.Header.Get("Idempotency-Key"))
	}
	if obj := rv.object(t); rvs.ExternalID != obj["id"] || rvs.Amount != 3000 {
		t.Errorf("reversal snapshot = %+v", rvs)
	}
}

func TestStripePaymentIntentStatusMapping(t *testing.T) {
	cases := map[string]State{
		"requires_payment_method": StateAwaitingPayment, "requires_confirmation": StateAwaitingPayment, "requires_action": StateAwaitingPayment,
		"processing": StateProcessing, "succeeded": StateSucceeded, "canceled": StateCanceled,
		"requires_capture": StateUnknown, "": StateUnknown, "something_new": StateUnknown,
	}
	base := loadFixture(t, "payment_intent_confirmed_expanded").object(t)
	for status, want := range cases {
		obj := withID(base, "pi_status")
		obj["status"] = status
		body, _ := json.Marshal(obj)
		rig := newStripeRig(t, fixed(fixture{Status: 200, Body: body}))
		snap, err := rig.adapter.GetPayment(t.Context(), "pi_status")
		if err != nil {
			t.Fatal(err)
		}
		if snap.State != want {
			t.Errorf("status %q -> %q, want %q", status, snap.State, want)
		}
		if want != StateSucceeded && (snap.ChargeID != "" || !snap.PaidAt.IsZero()) {
			t.Errorf("status %q carries a charge", status)
		}
	}
}

func TestStripePaymentIntentWithoutExpandedChargeHasNoPaidAt(t *testing.T) {
	rig := newStripeRig(t, fixed(loadFixture(t, "payment_intent_confirmed")))
	snap, err := rig.adapter.GetPayment(t.Context(), "pi_x")
	if err != nil {
		t.Fatal(err)
	}
	if snap.State != StateSucceeded || snap.ChargeID == "" || !snap.PaidAt.IsZero() {
		t.Fatalf("snapshot = %+v", snap)
	}
	if err := checkSucceededSnapshot(snap); !errors.Is(err, errIncompleteSuccess) {
		t.Fatalf("an unexpanded success must be flagged as incomplete, got %v", err)
	}
	req := newStripeRig(t, fixed(loadFixture(t, "payment_intent_confirmed")))
	_, _ = req.adapter.GetPayment(t.Context(), "pi_x")
	if got := expandValues(req.last(t).Query); len(got) != 1 || got[0] != "latest_charge" {
		t.Fatalf("GetPayment must expand latest_charge, expand = %v", got)
	}
}

func expandValues(q url.Values) []string {
	var out []string
	for k, v := range q {
		if strings.HasPrefix(k, "expand") {
			out = append(out, v...)
		}
	}
	return out
}

func TestStripeLastPaymentErrorCodeIsCarried(t *testing.T) {
	obj := withID(loadFixture(t, "payment_intent_confirmed_expanded").object(t), "pi_failed")
	obj["status"] = "requires_payment_method"
	obj["latest_charge"] = nil
	obj["last_payment_error"] = map[string]any{"code": "card_declined", "decline_code": "insufficient_funds", "type": "card_error"}
	body, _ := json.Marshal(obj)
	rig := newStripeRig(t, fixed(fixture{Status: 200, Body: body}))
	snap, err := rig.adapter.GetPayment(t.Context(), "pi_failed")
	if err != nil {
		t.Fatal(err)
	}
	if snap.State != StateAwaitingPayment || snap.LastError != "card_declined" {
		t.Fatalf("snapshot = %+v", snap)
	}
}

func TestStripeRefundStatusMapping(t *testing.T) {
	cases := map[string]RefundState{"pending": RefundPending, "succeeded": RefundSucceeded, "failed": RefundFailed, "canceled": RefundFailed, "requires_action": RefundUnknown, "": RefundUnknown}
	base := loadFixture(t, "refund_after_transfer").object(t)
	for status, want := range cases {
		obj := withID(base, "re_status")
		obj["status"] = status
		if status == "failed" {
			obj["failure_reason"] = "lost_or_stolen_card"
		}
		body, _ := json.Marshal(obj)
		rig := newStripeRig(t, fixed(fixture{Status: 200, Body: body}))
		snap, err := rig.adapter.GetRefund(t.Context(), "re_status")
		if err != nil {
			t.Fatal(err)
		}
		if snap.State != want {
			t.Errorf("refund status %q -> %q, want %q", status, snap.State, want)
		}
		if want == RefundFailed && snap.FailureReason == "" {
			t.Errorf("refund status %q has no failure reason", status)
		}
	}
}

// 讀取：不存在是永久失敗，限流與 5xx 不是。
func TestStripeReadErrors(t *testing.T) {
	missing := fixture{Status: 404, Body: json.RawMessage(`{"error":{"code":"resource_missing","type":"invalid_request_error","message":"No such payment_intent"}}`)}
	rig := newStripeRig(t, fixed(missing))
	if _, err := rig.adapter.GetPayment(t.Context(), "pi_none"); !errors.Is(err, ErrPermanent) {
		t.Errorf("GetPayment on a missing PaymentIntent = %v, want ErrPermanent", err)
	}
	if _, err := rig.adapter.GetRefund(t.Context(), "re_none"); !errors.Is(err, ErrPermanent) {
		t.Errorf("GetRefund on a missing refund = %v, want ErrPermanent", err)
	}
	for _, status := range []int{429, 500, 503} {
		rig := newStripeRig(t, fixed(fixture{Status: status, Body: json.RawMessage(`{}`)}))
		_, err := rig.adapter.GetPayment(t.Context(), "pi_x")
		if err == nil || errors.Is(err, ErrPermanent) || errors.Is(err, ErrRetryableRefusal) {
			t.Errorf("GetPayment status %d = %v, want a plain error", status, err)
		}
	}
}

func TestStripeCancelPayment(t *testing.T) {
	base := loadFixture(t, "payment_intent_confirmed_expanded").object(t)
	unexpected := fixture{Status: 400, Body: json.RawMessage(`{"error":{"code":"payment_intent_unexpected_state","type":"invalid_request_error","message":"already"}}`)}
	respondWith := func(status string, cancelOK bool) func(http.ResponseWriter, recordedRequest) {
		return func(w http.ResponseWriter, req recordedRequest) {
			if strings.HasSuffix(req.Path, "/cancel") && !cancelOK {
				unexpected.respond(w)
				return
			}
			obj := withID(base, "pi_c")
			obj["status"] = status
			body, _ := json.Marshal(obj)
			fixture{Status: 200, Body: body}.respond(w)
		}
	}
	t.Run("cancels an unpaid intent", func(t *testing.T) {
		rig := newStripeRig(t, respondWith("canceled", true))
		snap, err := rig.adapter.CancelPayment(t.Context(), "pi_c")
		if err != nil || snap.State != StateCanceled {
			t.Fatalf("snap %+v err %v", snap, err)
		}
		if rig.last(t).Path != "/v1/payment_intents/pi_c/cancel" {
			t.Errorf("path = %s", rig.last(t).Path)
		}
	})
	t.Run("an already canceled intent is success", func(t *testing.T) {
		rig := newStripeRig(t, respondWith("canceled", false))
		snap, err := rig.adapter.CancelPayment(t.Context(), "pi_c")
		if err != nil || snap.State != StateCanceled {
			t.Fatalf("snap %+v err %v", snap, err)
		}
	})
	t.Run("a paid intent returns the paid snapshot, not an error", func(t *testing.T) {
		rig := newStripeRig(t, respondWith("succeeded", false))
		snap, err := rig.adapter.CancelPayment(t.Context(), "pi_c")
		if err != nil || snap.State != StateSucceeded || snap.ChargeID == "" {
			t.Fatalf("snap %+v err %v", snap, err)
		}
	})
	t.Run("a refusal with another state is an error", func(t *testing.T) {
		rig := newStripeRig(t, respondWith("processing", false))
		if snap, err := rig.adapter.CancelPayment(t.Context(), "pi_c"); err == nil {
			t.Fatalf("want an error, got %+v", snap)
		}
	})
	t.Run("other 400s are permanent", func(t *testing.T) {
		rig := newStripeRig(t, fixed(fixture{Status: 400, Body: json.RawMessage(`{"error":{"code":"parameter_invalid_empty","message":"x"}}`)}))
		if _, err := rig.adapter.CancelPayment(t.Context(), "pi_c"); !errors.Is(err, ErrPermanent) {
			t.Fatalf("err = %v", err)
		}
	})
}

// 查回：transfer_group 篩選、只比對 attempt_id；沒有該筆是 found=false 且沒有錯誤。
func TestStripeFindTransferFiltersByTransferGroupAndMatchesTheAttempt(t *testing.T) {
	tr := loadFixture(t, "transfer_source_transaction").object(t)
	inGroup := withMeta(withID(tr, "tr_in_group"), stripeTestAttemptID)
	otherAttempt := withMeta(withID(tr, "tr_other_attempt"), "another-attempt")
	otherGroup := withMeta(withID(tr, "tr_other_group"), stripeTestAttemptID)
	rig := newStripeRig(t, func(w http.ResponseWriter, req recordedRequest) {
		items := []map[string]any{otherAttempt}
		if req.Query.Get("transfer_group") == "group_A" {
			items = append(items, inGroup)
		} else {
			items = append(items, otherGroup, inGroup)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(listBody(t, false, items...))
	})
	got, found, err := rig.adapter.FindTransfer(t.Context(), "group_A", stripeTestAttemptID, stripeTestCreatedAfter)
	if err != nil || !found || got.ExternalID != "tr_in_group" || got.Amount != 9000 {
		t.Fatalf("got %+v found %v err %v", got, found, err)
	}
	if rig.last(t).Path != "/v1/transfers" || rig.last(t).Query.Get("transfer_group") != "group_A" {
		t.Errorf("request = %s %v", rig.last(t).Path, rig.last(t).Query)
	}
	if _, found, err = rig.adapter.FindTransfer(t.Context(), "group_A", "no-such-attempt", stripeTestCreatedAfter); err != nil || found {
		t.Fatalf("a missing attempt must be found=false without an error, got found %v err %v", found, err)
	}
	if _, _, err = rig.adapter.FindTransfer(t.Context(), "", stripeTestAttemptID, stripeTestCreatedAfter); err == nil {
		t.Fatal("a lookup without a transfer_group must fail instead of listing every transfer")
	}
}

func TestStripeFindRefundAndReversalListTheirParent(t *testing.T) {
	re := loadFixture(t, "refund_after_transfer").object(t)
	rv := loadFixture(t, "transfer_reversal_partial").object(t)
	rig := newStripeRig(t, func(w http.ResponseWriter, req recordedRequest) {
		w.Header().Set("Content-Type", "application/json")
		if req.Path == "/v1/refunds" {
			if req.Query.Get("payment_intent") != "pi_src" {
				_, _ = w.Write(listBody(t, false, withMeta(withID(re, "re_other_payment"), stripeTestAttemptID)))
				return
			}
			_, _ = w.Write(listBody(t, false, withMeta(withID(re, "re_hit"), stripeTestAttemptID)))
			return
		}
		_, _ = w.Write(listBody(t, false, withMeta(withID(rv, "trr_hit"), stripeTestAttemptID)))
	})
	r, found, err := rig.adapter.FindRefund(t.Context(), "pi_src", stripeTestAttemptID, stripeTestCreatedAfter)
	if err != nil || !found || r.ExternalID != "re_hit" || r.State != RefundSucceeded {
		t.Fatalf("refund %+v found %v err %v", r, found, err)
	}
	v, found, err := rig.adapter.FindReversal(t.Context(), "tr_src", stripeTestAttemptID)
	if err != nil || !found || v.ExternalID != "trr_hit" || v.Amount != 3000 {
		t.Fatalf("reversal %+v found %v err %v", v, found, err)
	}
	if rig.last(t).Path != "/v1/transfers/tr_src/reversals" {
		t.Errorf("reversal list path = %s", rig.last(t).Path)
	}
}

// FindPayment 沒有 metadata 可篩選：在 created 下限之後的範圍內跨頁比對，一直翻到列表結束；沒看完不能回「找不到」。
func TestStripeFindPaymentPagesAndNeverClaimsAbsenceWhenItCannotFinish(t *testing.T) {
	base := loadFixture(t, "payment_intent_confirmed_expanded").object(t)
	t.Run("finds the attempt on a later page", func(t *testing.T) {
		rig := newStripeRig(t, func(w http.ResponseWriter, req recordedRequest) {
			w.Header().Set("Content-Type", "application/json")
			if req.Query.Get("starting_after") == "pi_page1" {
				_, _ = w.Write(listBody(t, false, withMeta(withID(base, "pi_page2"), stripeTestAttemptID)))
				return
			}
			_, _ = w.Write(listBody(t, true, withMeta(withID(base, "pi_page1"), "someone-else")))
		})
		snap, found, err := rig.adapter.FindPayment(t.Context(), stripeTestAttemptID, stripeTestCreatedAfter)
		if err != nil || !found || snap.ExternalID != "pi_page2" || snap.State != StateSucceeded {
			t.Fatalf("snap %+v found %v err %v", snap, found, err)
		}
		if got := expandValues(rig.last(t).Query); len(got) != 1 || got[0] != "data.latest_charge" {
			t.Errorf("expand = %v", got)
		}
	})
	t.Run("a finished scan without the attempt is found=false", func(t *testing.T) {
		rig := newStripeRig(t, func(w http.ResponseWriter, _ recordedRequest) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(listBody(t, false, withMeta(withID(base, "pi_a"), "someone-else")))
		})
		if _, found, err := rig.adapter.FindPayment(t.Context(), stripeTestAttemptID, stripeTestCreatedAfter); err != nil || found {
			t.Fatalf("found %v err %v", found, err)
		}
	})
	t.Run("a page that fails midway is an error, not found=false", func(t *testing.T) {
		var n int
		rig := newStripeRig(t, func(w http.ResponseWriter, _ recordedRequest) {
			n++
			w.Header().Set("Content-Type", "application/json")
			if n > 3 {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":{"type":"api_error","message":"boom"}}`))
				return
			}
			_, _ = w.Write(listBody(t, true, withMeta(withID(base, "pi_"+strconv.Itoa(n)), "someone-else")))
		})
		snap, found, err := rig.adapter.FindPayment(t.Context(), stripeTestAttemptID, stripeTestCreatedAfter)
		if err == nil || found {
			t.Fatalf("snap %+v found %v err %v, want an error", snap, found, err)
		}
		if n < 4 {
			t.Fatalf("only %d pages were requested: the scan must keep paging past any fixed page cap", n)
		}
	})
	t.Run("a long list is read to its end with no page cap", func(t *testing.T) {
		const pages = 60
		var n int
		rig := newStripeRig(t, func(w http.ResponseWriter, _ recordedRequest) {
			n++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(listBody(t, n < pages, withMeta(withID(base, "pi_"+strconv.Itoa(n)), "someone-else")))
		})
		if _, found, err := rig.adapter.FindPayment(t.Context(), stripeTestAttemptID, stripeTestCreatedAfter); err != nil || found || n != pages {
			t.Fatalf("found %v err %v pages %d, want a finished scan over %d pages", found, err, n, pages)
		}
	})
}

// 查回一律帶 created 下限（created[gte]，Unix 秒）：PaymentIntent、退款、轉帳三種列表都帶；沒有下限回錯誤，不發請求（沒有下限的列表是整個平台的歷史）。
// mutation：createdFilter 不帶下限（platform/stripe）→ 這個測試失敗。
func TestStripeFindsListOnlyObjectsCreatedAfterTheLowerBoundAndRefuseWithoutOne(t *testing.T) {
	base := loadFixture(t, "payment_intent_confirmed_expanded").object(t)
	rig := newStripeRig(t, func(w http.ResponseWriter, _ recordedRequest) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(listBody(t, false, withMeta(withID(base, "pi_a"), "someone-else")))
	})
	want := strconv.FormatInt(stripeTestCreatedAfter.Unix(), 10)
	ctx := t.Context()
	for name, call := range map[string]func() error{
		"payment_intents": func() error {
			_, _, err := rig.adapter.FindPayment(ctx, stripeTestAttemptID, stripeTestCreatedAfter)
			return err
		},
		"refunds": func() error {
			_, _, err := rig.adapter.FindRefund(ctx, "pi_src", stripeTestAttemptID, stripeTestCreatedAfter)
			return err
		},
		"transfers": func() error {
			_, _, err := rig.adapter.FindTransfer(ctx, "group_A", stripeTestAttemptID, stripeTestCreatedAfter)
			return err
		},
	} {
		if err := call(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := rig.last(t).Query.Get("created[gte]"); got != want {
			t.Errorf("%s: created[gte] = %q, want %q", name, got, want)
		}
	}
	before := rig.count()
	for name, call := range map[string]func() error{
		"payment_intents": func() error { _, _, err := rig.adapter.FindPayment(ctx, stripeTestAttemptID, time.Time{}); return err },
		"refunds": func() error {
			_, _, err := rig.adapter.FindRefund(ctx, "pi_src", stripeTestAttemptID, time.Time{})
			return err
		},
		"transfers": func() error {
			_, _, err := rig.adapter.FindTransfer(ctx, "group_A", stripeTestAttemptID, time.Time{})
			return err
		},
	} {
		if err := call(); err == nil {
			t.Errorf("%s: a lookup without a created lower bound must fail", name)
		}
	}
	if rig.count() != before {
		t.Errorf("a lookup without a lower bound sent %d requests", rig.count()-before)
	}
}

// 401／403 仍是「不明」（同鍵重送），但帶上 AuthRejectedError 讓呼叫方告警（D-F）。
// mutation：classifyMoney 拿掉 401／403 的分支 → 這個測試失敗。
func TestStripeAuthFailuresStayUnknownButCarryTheAuthRejectedError(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		err := classifyMoney(&stripe.APIError{Status: status, Code: "invalid_api_key", Message: "x"})
		var rejected *AuthRejectedError
		if !errors.As(err, &rejected) || rejected.Status != status || rejected.Code != "invalid_api_key" {
			t.Fatalf("status %d: err = %v, want an AuthRejectedError", status, err)
		}
		if classifyCall(err) != callUnknown {
			t.Errorf("status %d: classified as %v, want callUnknown (same-key resend, no reclassification)", status, classifyCall(err))
		}
	}
}

func signedPaymentEvent(t *testing.T, secret string, payload map[string]any) (http.Header, []byte) {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(bytes.Join([][]byte{[]byte(ts), body}, []byte(".")))
	h := http.Header{}
	h.Set("Stripe-Signature", "t="+ts+",v1="+hex.EncodeToString(mac.Sum(nil)))
	return h, body
}

func paymentEventPayload(id, typ, objectType, objectID string) map[string]any {
	return map[string]any{"id": id, "object": "event", "type": typ, "data": map[string]any{"object": map[string]any{"id": objectID, "object": objectType, "status": "claimed-by-the-event"}}}
}

func TestStripeParseEvent(t *testing.T) {
	rig := newStripeRig(t, fixed(fixture{Status: 200, Body: json.RawMessage(`{}`)}))
	a := rig.adapter
	h, body := signedPaymentEvent(t, stripeTestWebhook, paymentEventPayload("evt_1", "payment_intent.succeeded", "payment_intent", "pi_1"))
	if ev, err := a.ParseEvent(h, body); err != nil || ev != (Event{ID: "evt_1", Kind: EventPayment, ObjectID: "pi_1"}) {
		t.Errorf("payment event = %+v, %v", ev, err)
	}
	h, body = signedPaymentEvent(t, stripeTestWebhook, paymentEventPayload("evt_2", "refund.updated", "refund", "re_1"))
	if ev, err := a.ParseEvent(h, body); err != nil || ev != (Event{ID: "evt_2", Kind: EventRefund, ObjectID: "re_1"}) {
		t.Errorf("refund event = %+v, %v", ev, err)
	}
	for _, objectType := range []string{"transfer", "transfer_reversal", "charge"} {
		h, body = signedPaymentEvent(t, stripeTestWebhook, paymentEventPayload("evt_3", objectType+".created", objectType, "x_1"))
		if _, err := a.ParseEvent(h, body); !errors.Is(err, ErrIgnoredEvent) {
			t.Errorf("%s event err = %v, want ErrIgnoredEvent", objectType, err)
		}
	}
	h, body = signedPaymentEvent(t, "whsec_the_account_destination", paymentEventPayload("evt_4", "payment_intent.succeeded", "payment_intent", "pi_1"))
	if _, err := a.ParseEvent(h, body); !errors.Is(err, ErrInvalidEvent) {
		t.Errorf("a signature from another destination err = %v, want ErrInvalidEvent", err)
	}
	if _, err := a.ParseEvent(http.Header{}, body); !errors.Is(err, ErrInvalidEvent) {
		t.Errorf("a missing signature err = %v, want ErrInvalidEvent", err)
	}
	for name, payload := range map[string]map[string]any{
		"no event id":  paymentEventPayload("", "payment_intent.succeeded", "payment_intent", "pi_1"),
		"no object id": paymentEventPayload("evt_5", "payment_intent.succeeded", "payment_intent", ""),
	} {
		h, body = signedPaymentEvent(t, stripeTestWebhook, payload)
		if _, err := a.ParseEvent(h, body); !errors.Is(err, ErrInvalidEvent) {
			t.Errorf("%s err = %v, want ErrInvalidEvent", name, err)
		}
	}
}

func TestStripeNameIsAValidProviderCodeDistinctFromTheAccountAdapter(t *testing.T) {
	if StripeName == "stripe" || StripeName == FakeName || StripeName == "" {
		t.Fatalf("name %q collides with another inbox provider", StripeName)
	}
	for _, r := range StripeName {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			t.Fatalf("name %q has a character outside lowercase alphanumerics and underscore", StripeName)
		}
	}
	if _, err := NewStripe(nil); err == nil {
		t.Fatal("a nil client must be refused")
	}
}
