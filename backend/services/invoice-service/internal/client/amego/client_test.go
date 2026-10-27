package amego

import (
	"context"
	"crypto/md5" //nolint:gosec // 測試要重算 AMEGO 規定的簽章
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	testInvoiceBAN = "12345678" // ＝ AMEGO_TEST_INVOICE
	realInvoiceBAN = "45444444" // 一個真實統編
)

// recordingSink 收下每一筆稽核，供斷言「恰好寫一次」與遮罩正確。
type recordingSink struct {
	mu   sync.Mutex
	rows []Exchange
	// panicOnce 讓測試驗證「sink 爆掉不得影響開票」。
	panicOnce bool
}

func (s *recordingSink) Record(_ context.Context, ex Exchange) {
	s.mu.Lock()
	s.rows = append(s.rows, ex)
	shouldPanic := s.panicOnce
	s.mu.Unlock()
	if shouldPanic {
		panic("audit sink exploded")
	}
}

func (s *recordingSink) snapshot() []Exchange {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Exchange, len(s.rows))
	copy(out, s.rows)
	return out
}

// harness 是一個 httptest.Server ＋ 一個對它說話的 client。
type harness struct {
	t        *testing.T
	server   *httptest.Server
	client   *HTTPClient
	audit    *recordingSink
	requests atomic.Int64
	forms    chan url.Values
}

type handlerFunc func(form url.Values) (status int, contentType, body string)

func newHarness(t *testing.T, opts Options, h handlerFunc) *harness {
	t.Helper()
	hs := &harness{t: t, audit: &recordingSink{}, forms: make(chan url.Values, 64)}
	hs.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hs.requests.Add(1)
		_ = r.ParseForm()
		form := url.Values{}
		for k, v := range r.PostForm {
			form[k] = v
		}
		form.Set("__path", r.URL.Path)
		form.Set("__ctype", r.Header.Get("Content-Type"))
		select {
		case hs.forms <- form:
		default:
		}
		status, ctype, body := h(form)
		w.Header().Set("Content-Type", ctype)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(hs.server.Close)

	opts.BaseURL = hs.server.URL
	if opts.TestInvoice == "" {
		opts.TestInvoice = testInvoiceBAN
	}
	if opts.Deadlines == (Deadlines{}) {
		opts.Deadlines = Deadlines{Issue: 2 * time.Second, Query: 2 * time.Second, List: 2 * time.Second, Time: 2 * time.Second}
	}
	if opts.RateLimitQPS == 0 {
		opts.RateLimitQPS = 1000
	}
	if opts.Audit == nil {
		opts.Audit = hs.audit
	}
	c, err := New(opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	hs.client = c
	return hs
}

func okJSON(body string) handlerFunc {
	return func(url.Values) (int, string, string) { return http.StatusOK, "application/json; charset=utf-8", body }
}

func testCred(ban string) Credential {
	return Credential{IssuerID: 42, Invoice: ban, AppKey: officialAppKey}
}

func sampleIssueRequest() IssueRequest {
	return IssueRequest{
		OrderID:         "A20200817101021",
		BuyerIdentifier: "0000000000",
		BuyerName:       "客人",
		ProductItem: []IssueProductItem{
			{Description: "測試商品1", Quantity: "1", UnitPrice: "170", Amount: "170", TaxType: "1"},
		},
		SalesAmount: "170", FreeTaxSalesAmount: "0", ZeroTaxSalesAmount: "0",
		TaxType: "1", TaxRate: "0.05", TaxAmount: "0", TotalAmount: "170",
	}
}

// ── §4.5.8 第二層：guardEnvironment ─────────────────────────────────────────

// TestGuardEnvironmentBlocksBeforeAnyRequest 是本 package 最重要的一支測試。
//
// 🔴 斷言的是 **request count == 0**，不是只看回傳值：「回了 ENV_GUARD」在
// 「先打出去再判斷」的寫法下**一樣成立**，那樣的測試零鑑別力，而它守的正是
// 「絕不對真實公司開出一張無法回收的真發票」。
func TestGuardEnvironmentBlocksBeforeAnyRequest(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Options{AllowRealInvoice: false, TestInvoice: testInvoiceBAN}, okJSON(`{"code":0,"msg":""}`))

	_, err := h.client.IssueInvoice(context.Background(), testCred(realInvoiceBAN), sampleIssueRequest())
	if err == nil {
		t.Fatal("非測試統編在 ALLOW_REAL=false 下必須被擋")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("錯誤型別 = %T, want *APIError", err)
	}
	if apiErr.Code != CodeEnvGuard {
		t.Fatalf("Code = %q, want %q", apiErr.Code, CodeEnvGuard)
	}
	if apiErr.Class != ClassManual {
		t.Fatalf("Class = %s, want %s", apiErr.Class, ClassManual)
	}
	if n := h.requests.Load(); n != 0 {
		t.Fatalf("httptest.Server 收到 %d 個請求，必須是 0（閘門要在送出之前）", n)
	}
	if rows := h.audit.snapshot(); len(rows) != 0 {
		t.Fatalf("沒有往返就不該寫 provider audit，得到 %d 列", len(rows))
	}
}

// TestGuardEnvironmentAllowsTestInvoice：dev 與 prod 走同一條 code path，只是資料不同。
func TestGuardEnvironmentAllowsTestInvoice(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Options{AllowRealInvoice: false}, okJSON(`{"code":0,"msg":"","invoice_number":"AB00001111"}`))
	if _, err := h.client.IssueInvoice(context.Background(), testCred(testInvoiceBAN), sampleIssueRequest()); err != nil {
		t.Fatalf("測試統編應放行: %v", err)
	}
	if n := h.requests.Load(); n != 1 {
		t.Fatalf("request count = %d, want 1", n)
	}
}

// TestGuardEnvironmentAllowsRealWhenFlagOn：判定依據是 AMEGO_ALLOW_REAL_INVOICE，
// 不是 Env == "prod"（W-21）。
func TestGuardEnvironmentAllowsRealWhenFlagOn(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Options{AllowRealInvoice: true}, okJSON(`{"code":0,"msg":"","invoice_number":"AB00001111"}`))
	if _, err := h.client.IssueInvoice(context.Background(), testCred(realInvoiceBAN), sampleIssueRequest()); err != nil {
		t.Fatalf("ALLOW_REAL=true 應放行真實統編: %v", err)
	}
	if n := h.requests.Load(); n != 1 {
		t.Fatalf("request count = %d, want 1", n)
	}
}

// TestGuardAppliesToReadOnlyCalls：§4.5.8 說的是「每一次外呼」。
func TestGuardAppliesToReadOnlyCalls(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Options{AllowRealInvoice: false}, okJSON(`{"code":0,"msg":"","data":{}}`))
	if _, err := h.client.QueryInvoiceByOrderID(context.Background(), testCred(realInvoiceBAN), "X"); err == nil {
		t.Fatal("唯讀查詢也要過閘門")
	}
	if n := h.requests.Load(); n != 0 {
		t.Fatalf("request count = %d, want 0", n)
	}
}

// ── 序列化一次 ─────────────────────────────────────────────────────────────

// TestSignIsComputedOverTheExactBytesSent 是「序列化一次」的端到端鑑別力測試。
//
// 伺服器端拿**收到的 data 字串**重算 md5(data+time+AppKey) 並與收到的 sign 比對——
// 這正是 AMEGO 做的事。payload 刻意含 & < >：encoding/json 的 Marshal 會轉義它們
// 而 encodeOnce 不會 ⇒ 任何「先 marshal 算 sign、再 marshal 一次送出」的寫法
// 會讓兩邊分歧，這支測試就轉紅。
func TestSignIsComputedOverTheExactBytesSent(t *testing.T) {
	t.Parallel()
	var mismatch atomic.Bool
	h := newHarness(t, Options{AllowRealInvoice: true}, func(form url.Values) (int, string, string) {
		data := form.Get("data")
		ts := form.Get("time")
		sum := md5.Sum([]byte(data + ts + officialAppKey)) //nolint:gosec
		if hex.EncodeToString(sum[:]) != form.Get("sign") {
			mismatch.Store(true)
		}
		return http.StatusOK, "application/json", `{"code":0,"msg":"","invoice_number":"AB00001111"}`
	})

	req := sampleIssueRequest()
	req.OrderID = "A&B<C>D-2026"
	req.BuyerName = "客人 & 夥伴 <測試>"
	if _, err := h.client.IssueInvoice(context.Background(), testCred(realInvoiceBAN), req); err != nil {
		t.Fatalf("IssueInvoice: %v", err)
	}
	if mismatch.Load() {
		t.Fatal("sign 與實際送出的 data 位元組不符 ⇒ 序列化被做了兩次（AMEGO 會回 code 16，且訊息不提示原因）")
	}
	form := <-h.forms
	if !strings.Contains(form.Get("data"), "A&B<C>D-2026") {
		t.Fatalf("送出的 data 被轉義了: %s", form.Get("data"))
	}
	if form.Get("invoice") != realInvoiceBAN {
		t.Fatalf("invoice 欄 = %q", form.Get("invoice"))
	}
	if ct := form.Get("__ctype"); !strings.HasPrefix(ct, "application/x-www-form-urlencoded") {
		t.Fatalf("Content-Type = %q，官方明令不得用 application/json（用錯回 code 11，訊息完全誤導）", ct)
	}
	if p := form.Get("__path"); p != EndpointIssue {
		t.Fatalf("path = %q, want %q", p, EndpointIssue)
	}
}

// ── 回應形狀 ───────────────────────────────────────────────────────────────

// TestIssueResultComesFromTopLevel：f0401 的業務結果攤在頂層、不放 data。
func TestIssueResultComesFromTopLevel(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Options{AllowRealInvoice: true}, okJSON(
		`{"code":0,"msg":"","invoice_number":"AB12345678","invoice_time":1683776130,"random_number":"1060","barcode":"BC","qrcode_left":"L","qrcode_right":"R"}`))
	got, err := h.client.IssueInvoice(context.Background(), testCred(realInvoiceBAN), sampleIssueRequest())
	if err != nil {
		t.Fatalf("IssueInvoice: %v", err)
	}
	if got.InvoiceNumber != "AB12345678" || got.RandomNumber != "1060" || got.InvoiceTime != 1683776130 {
		t.Fatalf("結果沒有從頂層讀到: %+v", got)
	}
}

// TestProviderCodeAcceptsStringAndNumber：code 在 g0401／g0501／f0701 回 String、
// 查詢類回 Number。用 int 綁會在**折讓的錯誤路徑**上 unmarshal 失敗，而那正是
// 最需要讀到錯誤碼的路徑。
func TestProviderCodeAcceptsStringAndNumber(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, body, wantCode string
	}{
		{"number", `{"code":3040174,"msg":"SalesAmount 計算錯誤"}`, "3040174"},
		{"string", `{"code":"4040145","msg":"第1筆 AllowanceType 不可為 1"}`, "4040145"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t, Options{AllowRealInvoice: true}, okJSON(tc.body))
			_, err := h.client.IssueInvoice(context.Background(), testCred(realInvoiceBAN), sampleIssueRequest())
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("want *APIError, got %v", err)
			}
			if apiErr.Code != tc.wantCode {
				t.Fatalf("Code = %q, want %q", apiErr.Code, tc.wantCode)
			}
			if apiErr.Class != ClassPermanent {
				t.Fatalf("Class = %s, want %s", apiErr.Class, ClassPermanent)
			}
		})
	}
}

// TestHTTP200NonJSONIsPermanentDecode：打錯路徑時 AMEGO 回 HTTP 200 ＋ text/html
// ＋ body 是字串 `error`，不是 404。
//
// 🔴 歸 PERMANENT 不歸 UNKNOWN：那是開機就全錯的我方 bug，要的是立刻告警，
// 不是每一筆都 fail-closed 停在 WAITING_CONFIRMATION 等一個永遠不會有結論的重查。
func TestHTTP200NonJSONIsPermanentDecode(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Options{AllowRealInvoice: true}, func(url.Values) (int, string, string) {
		return http.StatusOK, "text/html", "error"
	})
	_, err := h.client.IssueInvoice(context.Background(), testCred(realInvoiceBAN), sampleIssueRequest())
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *APIError, got %v", err)
	}
	if apiErr.Class != ClassPermanent {
		t.Fatalf("Class = %s, want %s", apiErr.Class, ClassPermanent)
	}
	if apiErr.Kind != ErrorKindDecode {
		t.Fatalf("Kind = %q, want %q（DECODE 必須與其他三者分開）", apiErr.Kind, ErrorKindDecode)
	}
	if apiErr.HTTPStatus != http.StatusOK {
		t.Fatalf("HTTPStatus = %d, want 200", apiErr.HTTPStatus)
	}
	rows := h.audit.snapshot()
	if len(rows) != 1 || rows[0].ErrorKind != ErrorKindDecode {
		t.Fatalf("稽核必須記下 DECODE: %+v", rows)
	}
}

// TestNon200IsRecordedAndReconciled：HTTP status 恆為 200，哪天不是就是重要訊號。
func TestNon200IsRecordedAndReconciled(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Options{AllowRealInvoice: true}, func(url.Values) (int, string, string) {
		return http.StatusBadGateway, "text/plain", "bad gateway"
	})
	_, err := h.client.IssueInvoice(context.Background(), testCred(realInvoiceBAN), sampleIssueRequest())
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *APIError, got %v", err)
	}
	if apiErr.HTTPStatus != http.StatusBadGateway {
		t.Fatalf("HTTPStatus = %d", apiErr.HTTPStatus)
	}
	if apiErr.Class != ClassReconcile {
		t.Fatalf("Class = %s, want %s（寫入類的結果未知一律交給對帳）", apiErr.Class, ClassReconcile)
	}
	rows := h.audit.snapshot()
	if len(rows) != 1 || rows[0].HTTPStatus != http.StatusBadGateway {
		t.Fatalf("稽核必須記下非 200 的 status: %+v", rows)
	}
}

// TestTransportTimeoutIsReconcileAndAudited：逾時的代價是「結果未知 ⇒ 要走對帳」。
//
// 同時驗證稽核用 context.WithoutCancel：外呼逾時時 ctx 已取消，直接沿用會讓
// **最需要留下證據的那一筆反而寫不進去**。
func TestTransportTimeoutIsReconcileAndAudited(t *testing.T) {
	t.Parallel()
	// handler 睡得比 deadline 久（不用「永遠阻塞」：httptest.Server.Close 會等
	// 未完成的 handler，配上 t.Cleanup 的 LIFO 順序會死鎖）。
	h := newHarness(t, Options{
		AllowRealInvoice: true,
		Deadlines:        Deadlines{Issue: 60 * time.Millisecond, Query: time.Second, List: time.Second, Time: time.Second},
	}, func(url.Values) (int, string, string) {
		time.Sleep(400 * time.Millisecond)
		return http.StatusOK, "application/json", `{"code":0}`
	})
	_, err := h.client.IssueInvoice(context.Background(), testCred(realInvoiceBAN), sampleIssueRequest())
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *APIError, got %v", err)
	}
	if apiErr.Class != ClassReconcile {
		t.Fatalf("Class = %s, want %s", apiErr.Class, ClassReconcile)
	}
	if apiErr.Kind != ErrorKindTimeout {
		t.Fatalf("Kind = %q, want %q", apiErr.Kind, ErrorKindTimeout)
	}
	rows := h.audit.snapshot()
	if len(rows) != 1 {
		t.Fatalf("逾時仍必須恰好寫一筆稽核，得到 %d", len(rows))
	}
	if rows[0].ErrorKind != ErrorKindTimeout {
		t.Fatalf("ErrorKind = %q", rows[0].ErrorKind)
	}
}

// ── 斷路器 ─────────────────────────────────────────────────────────────────

// TestBreakerTripsAndStopsSendingRequests 斷言跳閘後**不再發出請求**（request count），
// 而不是只看回傳值。
func TestBreakerTripsAndStopsSendingRequests(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Options{
		AllowRealInvoice: true,
		Breaker:          BreakerSettings{MaxRequests: 3, Interval: 30 * time.Second, Timeout: 10 * time.Second, Failures: 5},
	}, okJSON(`{"code":18,"msg":"無法建立資料庫連線"}`))

	cred := testCred(realInvoiceBAN)
	for i := 0; i < 5; i++ {
		if _, err := h.client.IssueInvoice(context.Background(), cred, sampleIssueRequest()); err == nil {
			t.Fatalf("第 %d 次應失敗", i+1)
		}
	}
	before := h.requests.Load()
	if before != 5 {
		t.Fatalf("跳閘前 request count = %d, want 5", before)
	}
	// 2026-08-31 起斷路器拆成 per-store ＋ 全域（§7.3.4）：連 5 敗跳的是**該賣家那顆**。
	// 這個測試守的不變式（連 5 次失敗後不再送出請求）不變，只是換了層級。
	if !h.client.BreakerOpenFor(cred.IssuerID) {
		t.Fatal("連續 5 次計入的失敗之後該賣家的斷路器必須 open")
	}
	// 🔴 同時斷言全域**仍然關著**：一家賣家壞掉不得拖垮全服務。
	// 這一格是修正前壞掉的地方——那時只有一顆全域的、用 per-store 的跳脫條件，
	// 於是任何一家連 5 敗就讓所有賣家的開票停在 RETRY_SCHEDULED。
	if h.client.BreakerOpen() {
		t.Fatal("一家賣家連敗不得讓全域斷路器 open（全域是 60s 窗口失敗率 ≥50% 且樣本 ≥20）")
	}
	_, err := h.client.IssueInvoice(context.Background(), cred, sampleIssueRequest())
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *APIError, got %v", err)
	}
	if apiErr.Class != ClassRetryable {
		t.Fatalf("開路時 Class = %s, want %s（開路時 operation 不得標成失敗，W-18）", apiErr.Class, ClassRetryable)
	}
	if after := h.requests.Load(); after != before {
		t.Fatalf("開路後 request count = %d, want %d（不得再搥上游）", after, before)
	}
}

// TestExcludedFailuresNeverTripBreaker：一個賣家送錯統編不該讓其他賣家開不出發票
// （pkg/sms 因 issue #2108 學到的同一課）。
func TestExcludedFailuresNeverTripBreaker(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Options{
		AllowRealInvoice: true,
		Breaker:          BreakerSettings{MaxRequests: 3, Interval: 30 * time.Second, Timeout: 10 * time.Second, Failures: 5},
	}, okJSON(`{"code":12,"msg":"invoice(統編)錯誤"}`))

	cred := testCred(realInvoiceBAN)
	for i := 0; i < 12; i++ {
		if _, err := h.client.IssueInvoice(context.Background(), cred, sampleIssueRequest()); err == nil {
			t.Fatalf("第 %d 次應失敗", i+1)
		}
	}
	if h.client.BreakerOpen() {
		t.Fatal("帳號層的 code 12 連續 12 次也不得跳閘")
	}
	if n := h.requests.Load(); n != 12 {
		t.Fatalf("request count = %d, want 12", n)
	}
}

// ── 稽核 ───────────────────────────────────────────────────────────────────

// TestAuditRecordsOnceWithMaskedSignAndNoAppKey。
func TestAuditRecordsOnceWithMaskedSignAndNoAppKey(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Options{AllowRealInvoice: true}, okJSON(`{"code":0,"msg":"","invoice_number":"AB00001111"}`))
	ctx := WithAuditContext(context.Background(), 987654321, 3)
	if _, err := h.client.IssueInvoice(ctx, testCred(realInvoiceBAN), sampleIssueRequest()); err != nil {
		t.Fatalf("IssueInvoice: %v", err)
	}
	rows := h.audit.snapshot()
	if len(rows) != 1 {
		t.Fatalf("稽核必須恰好一筆，得到 %d", len(rows))
	}
	r := rows[0]
	if r.Endpoint != EndpointIssue {
		t.Errorf("endpoint = %q, want 路徑 %q（同一個數字碼跨 API 意義不同）", r.Endpoint, EndpointIssue)
	}
	if r.IssuerID != 42 {
		t.Errorf("issuer_id = %d, want 42（不是 store_id：平台自身開票時沒有 store_id）", r.IssuerID)
	}
	if r.OperationID != 987654321 || r.AttemptSeq != 3 {
		t.Errorf("operation_id/attempt_seq = %d/%d", r.OperationID, r.AttemptSeq)
	}
	if r.HTTPStatus != http.StatusOK {
		t.Errorf("http_status = %d", r.HTTPStatus)
	}
	sign := r.RequestMasked["sign"]
	if len(sign) != 8 {
		t.Errorf("sign 必須只留前 8 碼（全長 sign ＋ 已落庫的 data／time ＝ 破解 AppKey 的 oracle），得到 %q", sign)
	}
	// 由稽核留下的 data／time ＋ 前 8 碼，重算完整 sign 前 8 碼必須一致（對單用）。
	unix, err := strconv.ParseInt(r.RequestMasked["time"], 10, 64)
	if err != nil {
		t.Fatalf("time 欄不是整數: %v", err)
	}
	if want := maskSign(signature(r.RequestMasked["data"], unix, officialAppKey)); want != sign {
		t.Errorf("遮罩後的 sign 對不上重算值: %q vs %q", sign, want)
	}
	for k, v := range r.RequestMasked {
		if strings.Contains(v, officialAppKey) {
			t.Errorf("request_masked[%s] 含 AppKey", k)
		}
	}
	if strings.Contains(r.ResponseMasked, officialAppKey) {
		t.Error("response_masked 含 AppKey")
	}
	if r.ResponseMasked == "" {
		t.Error("response_masked 必須留原文（稅務舉證需要）")
	}
	if r.OccurredAt.IsZero() {
		t.Error("occurred_at 必須填")
	}
}

// TestAuditSinkPanicDoesNotFailTheCall：稽核寫入失敗永遠不可以讓一次開票失敗。
func TestAuditSinkPanicDoesNotFailTheCall(t *testing.T) {
	t.Parallel()
	sink := &recordingSink{panicOnce: true}
	h := newHarness(t, Options{AllowRealInvoice: true, Audit: sink}, okJSON(`{"code":0,"msg":"","invoice_number":"AB00001111"}`))
	got, err := h.client.IssueInvoice(context.Background(), testCred(realInvoiceBAN), sampleIssueRequest())
	if err != nil {
		t.Fatalf("sink panic 不得外洩到開票路徑: %v", err)
	}
	if got.InvoiceNumber != "AB00001111" {
		t.Fatalf("結果 = %+v", got)
	}
}

// ── 其餘端點行為 ───────────────────────────────────────────────────────────

// TestCode71IsNotAnError：71 是「180 天內確定沒有」，與 51「不確定」語意完全不同。
func TestCode71IsNotAnError(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Options{AllowRealInvoice: true}, okJSON(`{"code":71,"msg":"查無資料"}`))
	_, err := h.client.QueryInvoiceByOrderID(context.Background(), testCred(realInvoiceBAN), "NOPE")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *APIError, got %v", err)
	}
	if !apiErr.NotFound {
		t.Fatal("code 71 必須標成 NotFound（可安全重送 f0401，沿用同一個 OrderId）")
	}
	h2 := newHarness(t, Options{AllowRealInvoice: true}, okJSON(`{"code":51,"msg":"該發票超過查詢期限"}`))
	_, err = h2.client.QueryInvoiceByOrderID(context.Background(), testCred(realInvoiceBAN), "OLD")
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *APIError, got %v", err)
	}
	if apiErr.NotFound {
		t.Fatal("code 51 是『不確定』，絕不可與 71『確定沒有』混為一談")
	}
}

// TestListInvoicesEmptyPageIsNotAnError：分頁超界回 code 0 ＋ 空陣列、不報錯
// ⇒ 終止條件只能是「Records 為空」的顯式判定。
func TestListInvoicesEmptyPageIsNotAnError(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Options{AllowRealInvoice: true}, okJSON(`{"code":0,"msg":"","page_total":187,"page_now":999,"data_total":3738,"data":[]}`))
	page, err := h.client.ListInvoices(context.Background(), testCred(realInvoiceBAN), ListRequest{DateSelect: 1, DateStart: "20230101", DateEnd: "20230228", Limit: 20, Page: 999})
	if err != nil {
		t.Fatalf("ListInvoices: %v", err)
	}
	if len(page.Records) != 0 {
		t.Fatalf("Records = %d, want 0", len(page.Records))
	}
	if page.PageTotal != 187 || page.DataTotal != 3738 {
		t.Fatalf("分頁欄與 data 平級、不在 data 之內: %+v", page)
	}
}

// TestUploadStatusToleratesBothShapes：文件同時列新版陣列與舊版物件，卻沒說怎麼切換。
func TestUploadStatusToleratesBothShapes(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, body string }{
		{"新版陣列", `{"code":0,"msg":"","data":[{"invoice_number":"AB00001112","type":"C0401","status":99,"total_amount":1580}]}`},
		{"舊版物件", `{"code":0,"msg":"","data":{"AB00001112":{"type":"C0401","status":99,"total_amount":1580}}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t, Options{AllowRealInvoice: true}, okJSON(tc.body))
			got, err := h.client.InvoiceUploadStatus(context.Background(), testCred(realInvoiceBAN), "AB00001112")
			if err != nil {
				t.Fatalf("InvoiceUploadStatus: %v", err)
			}
			if got.Status != 99 || got.Type != "C0401" || got.Number != "AB00001112" {
				t.Fatalf("got = %+v", got)
			}
		})
	}
}

// TestUploadStatusNotFoundIsSilentlyReturned：超出窗口的憑證靜默回 NOT_FOUND +
// code 0 + status -1 ⇒ client 原樣回報，讓呼叫端知道它**不能**用來判「開過沒有」。
func TestUploadStatusNotFoundIsSilentlyReturned(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Options{AllowRealInvoice: true},
		okJSON(`{"code":0,"msg":"","data":[{"invoice_number":"AB00001111","type":"NOT_FOUND","status":-1,"total_amount":0}]}`))
	got, err := h.client.InvoiceUploadStatus(context.Background(), testCred(realInvoiceBAN), "AB00001111")
	if err != nil {
		t.Fatalf("NOT_FOUND 不是錯誤（code 仍是 0）: %v", err)
	}
	if got.Type != "NOT_FOUND" || got.Status != -1 {
		t.Fatalf("got = %+v", got)
	}
}

// TestQueryCompanyNamesJoinsByBan：送重複統編只回一筆（去重）⇒ 必須用 ban 鍵值 join，
// 不可索引對位。
func TestQueryCompanyNamesJoinsByBan(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Options{AllowRealInvoice: true},
		okJSON(`{"code":0,"msg":"","data":[{"ban":"45444444","name":"光貿科技股份有限公司"},{"ban":"11111101","name":""}]}`))
	got, err := h.client.QueryCompanyNames(context.Background(), testCred(realInvoiceBAN), []string{"45444444", "45444444", "11111101"})
	if err != nil {
		t.Fatalf("QueryCompanyNames: %v", err)
	}
	if got["45444444"] != "光貿科技股份有限公司" {
		t.Fatalf("got = %+v", got)
	}
	if v, ok := got["11111101"]; !ok || v != "" {
		t.Fatal("name 空字串不代表統編無效（管委會、會計事務所等不在資料集內），仍要回傳該鍵")
	}
}

// TestDownloadStyleWhitelist：AMEGO 對 download_style 不做值域驗證，
// 錯誤值會靜默產生非預期版面的 PDF ⇒ 白名單由我方守，且不打出去。
func TestDownloadStyleWhitelist(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Options{AllowRealInvoice: true}, okJSON(`{"code":0,"msg":"","data":{"file_url":"https://x/y"}}`))
	if _, err := h.client.FetchInvoiceFileURL(context.Background(), testCred(realInvoiceBAN), FileRequest{Type: "order", OrderID: "A1", DownloadStyle: 9}); err == nil {
		t.Fatal("download_style=9 必須被擋")
	}
	if n := h.requests.Load(); n != 0 {
		t.Fatalf("被擋的請求不得送出，request count = %d", n)
	}
	got, err := h.client.FetchInvoiceFileURL(context.Background(), testCred(realInvoiceBAN), FileRequest{Type: "order", OrderID: "A1", DownloadStyle: 0})
	if err != nil {
		t.Fatalf("FetchInvoiceFileURL: %v", err)
	}
	if got != "https://x/y" {
		t.Fatalf("file_url = %q", got)
	}
}

// TestCode21DegradesLocalRateLimit：AMEGO 沒有 Retry-After／X-RateLimit-*
// ⇒ 只能靠我方限速自動降檔。
func TestCode21DegradesLocalRateLimit(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Options{AllowRealInvoice: true, RateLimitQPS: 100}, okJSON(`{"code":21,"msg":"人數過多"}`))
	before := h.client.limiter.currentQPS()
	if _, err := h.client.IssueInvoice(context.Background(), testCred(realInvoiceBAN), sampleIssueRequest()); err == nil {
		t.Fatal("code 21 應回錯")
	}
	if after := h.client.limiter.currentQPS(); after >= before {
		t.Fatalf("code 21 之後上限應降檔: %v → %v", before, after)
	}
}

// TestServerTimeUsesTimestampOnly：GET /json/time 沒有 code／msg、Content-Type 是
// text/plain，且 text 是台北時間 ⇒ 程式一律只用 timestamp。
func TestServerTimeUsesTimestampOnly(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Options{AllowRealInvoice: true}, func(url.Values) (int, string, string) {
		return http.StatusOK, "text/plain; charset=utf-8", `{"timestamp":1787578776,"text":"2026\/08\/24 21:39:36","year":2026}`
	})
	got, err := h.client.ServerTime(context.Background())
	if err != nil {
		t.Fatalf("ServerTime: %v", err)
	}
	if got.Unix() != 1787578776 {
		t.Fatalf("ServerTime = %v", got)
	}
}

// TestVoidReturnsAcceptedNotDone：f0501 回 code 0 只代表**受理**。
func TestVoidReturnsAcceptedNotDone(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Options{AllowRealInvoice: true}, okJSON(`{"code":0,"msg":""}`))
	got, err := h.client.VoidInvoice(context.Background(), testCred(realInvoiceBAN), VoidRequest{CancelInvoiceNumber: "AB00001111", CancelReason: "買家取消"})
	if err != nil {
		t.Fatalf("VoidInvoice: %v", err)
	}
	if !got.Accepted {
		t.Fatal("want Accepted")
	}
	form := <-h.forms
	if !strings.HasPrefix(form.Get("data"), "[") {
		t.Fatalf("f0501 的 data 必須是陣列: %s", form.Get("data"))
	}
	if form.Get("__path") != EndpointVoid {
		t.Fatalf("path = %q", form.Get("__path"))
	}
}

// TestIssueDataIsObjectNotArray：f0401 的 data 是**單一物件**（送陣列回 3040112）。
func TestIssueDataIsObjectNotArray(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Options{AllowRealInvoice: true}, okJSON(`{"code":0,"msg":"","invoice_number":"A"}`))
	if _, err := h.client.IssueInvoice(context.Background(), testCred(realInvoiceBAN), sampleIssueRequest()); err != nil {
		t.Fatalf("IssueInvoice: %v", err)
	}
	form := <-h.forms
	if !strings.HasPrefix(form.Get("data"), "{") {
		t.Fatalf("f0401 的 data 必須是物件: %s", form.Get("data"))
	}
}

// TestNewRejectsMissingTestInvoice：沒有測試統編 ⇒ guardEnvironment 的第二個放行
// 條件永遠不成立，而那是靜默的 ⇒ 建構時就硬錯。
func TestNewRejectsMissingTestInvoice(t *testing.T) {
	t.Parallel()
	_, err := New(Options{BaseURL: "https://x", Deadlines: DefaultDeadlines()})
	if err == nil {
		t.Fatal("缺 TestInvoice 必須拒絕建構")
	}
	_, err = New(Options{BaseURL: "https://x", TestInvoice: testInvoiceBAN})
	if err == nil {
		t.Fatal("缺 deadline 必須拒絕建構（禁止無 deadline 的跨服務呼叫）")
	}
}
