// Package amego is the ONLY place in this service that talks to 光貿 AMEGO.
//
// It performs no retries and makes no business decisions: it turns a domain
// request into AMEGO's four form fields, signs it, and turns the reply into
// either a typed result or an *APIError carrying an ErrorClass. Retry/backoff
// lives in internal/worker (pkg/outbox); the void-vs-allowance decision lives
// in internal/usecase. Mirrors the PayUni client convention (client 內零 retry:
// client 內重試會讓 lease 的「單筆最壞耗時」變成 deadline × retries，租約下界跟著
// 翻倍，而且重試次數對 ADM 不可見).
package amego

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	gobreaker "github.com/sony/gobreaker/v2"
	"google.golang.org/api/idtoken"
)

// Credential 是一個 ISSUER 的 AMEGO 身分——issuer 不是 seller：平台自己也是 issuer
// 而它沒有 store_id。
//
// 🔴 AppKey 不落 log、不進 metric label、不進 MQ、不進任何 error message（§4.5.4）。
// 它的活動範圍只有：repository.mapper（解密）→ usecase → Credential → signature 的
// md5 輸入。
type Credential struct {
	IssuerID int64  // invoice_issuers.id（TSID）；只用於稽核關聯，不參與簽章
	Invoice  string // 統編 8 碼（＝ invoice_provider_accounts.provider_tax_id）
	AppKey   string // 明文；repository 層解密後即時傳入，不快取於本 package
}

// String 遮住 AppKey，讓 %v / zap.Any 意外印出 Credential 時不會外洩金鑰。
func (c Credential) String() string {
	return fmt.Sprintf("amego.Credential{IssuerID:%d, Invoice:%s, AppKey:<redacted>}", c.IssuerID, c.Invoice)
}

// Client 是 AMEGO 的抗腐層介面。
//
// 介面涵蓋的端點集合 ＝ §4.5.9 的 nginx 白名單：16 個方法打 15 支端點
// （兩支 query 共用 /json/invoice_query）。見 Endpoints()。
type Client interface {
	// ── 寫入類（有副作用，逾時必須走 §4.5.7 對帳）──
	IssueInvoice(ctx context.Context, c Credential, req IssueRequest) (*IssueResult, error)           // /json/f0401
	VoidInvoice(ctx context.Context, c Credential, req VoidRequest) (*VoidAccepted, error)            // /json/f0501
	IssueAllowance(ctx context.Context, c Credential, req AllowanceRequest) (*AllowanceResult, error) // /json/g0401
	VoidAllowance(ctx context.Context, c Credential, req VoidAllowanceRequest) (*VoidAccepted, error) // /json/g0501

	// ── 查詢類（唯讀，可安心重試）──
	QueryInvoiceByOrderID(ctx context.Context, c Credential, orderID string) (*InvoiceDetail, error)    // invoice_query type=order
	QueryInvoiceByNumber(ctx context.Context, c Credential, number string) (*InvoiceDetail, error)      // invoice_query type=invoice
	ListInvoices(ctx context.Context, c Credential, req ListRequest) (*InvoicePage, error)              // invoice_list（無 180 天限制）
	QueryAllowance(ctx context.Context, c Credential, allowanceNumber string) (*AllowanceDetail, error) // allowance_query
	FetchInvoiceFileURL(ctx context.Context, c Credential, req FileRequest) (string, error)             // invoice_file（URL 僅 10 分鐘有效）

	// ── 驗證類（送出前的前置檢查，§4.5.5）──
	VerifyMobileBarcode(ctx context.Context, c Credential, barcode string) error                   // /json/barcode
	QueryCompanyNames(ctx context.Context, c Credential, bans []string) (map[string]string, error) // /json/ban_query

	// ── 資源與健康 ──
	ListTracks(ctx context.Context, c Credential, year, period int) ([]Track, error)                              // /json/track_all
	TrackStatus(ctx context.Context, c Credential, year, period int, trackAPICode string) ([]TrackBalance, error) // /json/track_status
	ServerTime(ctx context.Context) (time.Time, error)                                                            // GET /json/time（免簽章）

	// ── 上傳狀態（recheck worker 專用）──
	// 只能回答「財政部收到了沒有」，回填 provider_upload_status。
	// 🔴 不可用來回答「這張開過了沒有」——超出窗口的憑證靜默回 NOT_FOUND + code 0
	// + status -1。冪等判斷一律走 QueryInvoiceByOrderID。
	InvoiceUploadStatus(ctx context.Context, c Credential, number string) (*UploadStatus, error)            // /json/invoice_status
	AllowanceUploadStatus(ctx context.Context, c Credential, allowanceNumber string) (*UploadStatus, error) // /json/allowance_status
}

// Deadlines 是四個 per-call deadline（§4.5.12）。
//
// 🔴 List 不可併回 Query：併了 INV.op 的租約下界少算 10s，而那 20s 正是下界
// 60.4s 遠高於 INV.rc 20.3s 的唯一原因。
type Deadlines struct {
	Issue time.Duration // f0401／f0501／g0401／g0501
	Query time.Duration // 查詢／驗證／字軌類
	List  time.Duration // invoice_list（每頁獨立計時）
	Time  time.Duration // GET /json/time
}

// DefaultDeadlines 是 §4.5.12 常數表的初值。
func DefaultDeadlines() Deadlines {
	return Deadlines{Issue: 30 * time.Second, Query: 10 * time.Second, List: 20 * time.Second, Time: 5 * time.Second}
}

// BreakerSettings 照抄 pkg/sms/client.go 的參數，差別只在 IsExcluded。
type BreakerSettings struct {
	MaxRequests uint32
	Interval    time.Duration
	Timeout     time.Duration
	Failures    uint32
}

// DefaultBreakerSettings 回 pkg/sms 同值（MaxRequests 3／Interval 30s／Timeout 10s／
// ConsecutiveFailures >= 5）。
func DefaultBreakerSettings() BreakerSettings {
	return BreakerSettings{MaxRequests: 3, Interval: 30 * time.Second, Timeout: 10 * time.Second, Failures: 5}
}

// Options 是 New 的輸入。
type Options struct {
	BaseURL    string
	HTTPClient *http.Client
	Clock      *Clock
	Audit      AuditSink

	// AllowRealInvoice ＝ AMEGO_ALLOW_REAL_INVOICE。
	// 🔴 判定「現在准不准開真發票」的是它，**不是 Env == "prod"**（W-21）。
	AllowRealInvoice bool
	// TestInvoice ＝ AMEGO_TEST_INVOICE（不是 Go 常數：光貿換公開測試帳號時
	// 不該需要改 code 重新部署）。
	TestInvoice string

	RateLimitQPS int
	Breaker      BreakerSettings
	Deadlines    Deadlines

	// now 供測試注入；nil 時用 time.Now。
	now func() time.Time
}

// HTTPClient 是 Client 的唯一實作。
//
// Credential 逐次傳入、client 對憑證無狀態 ⇒ 連線池／斷路器／限速器全服務共用一份
// （AMEGO 是一個上游主機，不是 N 個）。
type HTTPClient struct {
	baseURL string
	http    *http.Client
	clock   *Clock
	limiter *rateLimiter
	breaker *gobreaker.CircuitBreaker[[]byte] // 全域（§7.3.4「全域斷路器另計」）

	// perIssuer 是**每個 store 一顆**斷路器（§7.3.4「per-store 斷路器」）。
	//
	// 🔴 先前只有一顆全域的、而且用的是 per-store 的跳脫條件（連 5 敗）⇒
	// **一家賣家的憑證壞掉會把全服務推向 open**，其餘賣家的開票一起停在
	// RETRY_SCHEDULED。而 invoice_provider_accounts.status='DEGRADED' 的語意是
	// 「per-store 斷路器的持久化投影」，投影的來源卻不是 per-store 的。
	perIssuerMu sync.Mutex
	perIssuer   map[int64]*gobreaker.CircuitBreaker[[]byte]
	breakerSet  BreakerSettings
	audit       AuditSink
	deadlines   Deadlines

	allowRealInvoice bool
	testInvoice      string
	now              func() time.Time
}

var _ Client = (*HTTPClient)(nil)

// NewTransport 建出兩種傳輸模式的 http.Client（§4.5.9，照抄 pkg/sms 的兩模式建構子）。
//
//   - proxyCallerKeyFile 為空：情境 A，直連 invoice-api.amego.tw。
//   - proxyCallerKeyFile 有值：情境 B，用 SA key 簽 Google ID token 打
//     IAM-protected 的 egress-proxy（baseURL 同時是 idtoken audience）。
//
// 程式碼差異為零——同一個建構子的兩個分支。
//
// backstop 是 http.Client.Timeout（取 deadlineIssue + 5s）：idtoken.NewClient 不設
// timeout，proxy 卡住會無限期掛住呼叫端。權威仍是 per-call ctx deadline。
func NewTransport(ctx context.Context, baseURL, proxyCallerKeyFile string, backstop time.Duration) (*http.Client, error) {
	var hc *http.Client
	if proxyCallerKeyFile == "" {
		hc = &http.Client{}
	} else {
		var err error
		hc, err = idtoken.NewClient(ctx, baseURL, idtoken.WithAuthCredentialsFile(idtoken.ServiceAccount, proxyCallerKeyFile))
		if err != nil {
			return nil, fmt.Errorf("amego idtoken client: %w", err)
		}
	}
	hc.Timeout = backstop
	return hc, nil
}

// New 建出 client。
func New(opts Options) (*HTTPClient, error) {
	if opts.BaseURL == "" {
		return nil, errors.New("amego: BaseURL is required")
	}
	if opts.TestInvoice == "" {
		// 沒有測試統編 ⇒ guardEnvironment 的第二個放行條件永遠不成立。
		// 這在 ALLOW_REAL=false 的環境等於全部外呼被擋，而那是靜默的
		// ⇒ 寧可在建構時就硬錯。Config.Validate 也擋同一件事（§4.5.8 第一層）。
		return nil, errors.New("amego: TestInvoice is required (AMEGO_TEST_INVOICE)")
	}
	nowFn := opts.now
	if nowFn == nil {
		nowFn = time.Now
	}
	clk := opts.Clock
	if clk == nil {
		clk = newClockWithSource(nowFn)
	}
	sink := opts.Audit
	if sink == nil {
		sink = NoopAuditSink{}
	}
	dl := opts.Deadlines
	if dl.Issue == 0 || dl.Query == 0 || dl.List == 0 || dl.Time == 0 {
		return nil, errors.New("amego: all four deadlines must be set (§4.5.12)")
	}
	bs := opts.Breaker
	if bs.Failures == 0 {
		bs = DefaultBreakerSettings()
	}
	hc := opts.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: dl.Issue + 5*time.Second}
	}
	// 全域斷路器（§7.3.4「全域斷路器另計」）：**60 秒滾動窗口內失敗率 ≥ 50% 且
	// 樣本 ≥ 20** ⇒ 全域 open。對應「code 10 停機維護」這種所有賣家一起壞的情況。
	//
	// 🔴 跳脫條件與 per-store 那顆**不同**，這是刻意的：先前全域那顆用的是
	// per-store 的「連 5 敗」，於是一家賣家壞掉就拖垮全部。
	cb := gobreaker.NewCircuitBreaker[[]byte](gobreaker.Settings{
		Name:        "amego-global",
		MaxRequests: bs.MaxRequests,
		Interval:    globalBreakerWindow,
		Timeout:     bs.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			if counts.Requests < globalBreakerMinSamples {
				return false
			}
			return float64(counts.TotalFailures)/float64(counts.Requests) >= globalBreakerFailureRatio
		},
		// 🔴 per-store 開路**不計入全域**：那是「這一家壞了」不是「AMEGO 壞了」。
		// 少了這一句，一家賣家持續開路會把全域的失敗率一路推上去 ⇒ 回到修正前的行為。
		IsExcluded: func(err error) bool {
			if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
				return true
			}
			return isExcludedFromBreaker(err)
		},
	})
	return &HTTPClient{
		baseURL:          strings.TrimRight(opts.BaseURL, "/"),
		http:             hc,
		clock:            clk,
		limiter:          newRateLimiter(opts.RateLimitQPS, nowFn),
		breaker:          cb,
		perIssuer:        make(map[int64]*gobreaker.CircuitBreaker[[]byte]),
		breakerSet:       bs,
		audit:            sink,
		deadlines:        dl,
		allowRealInvoice: opts.AllowRealInvoice,
		testInvoice:      opts.TestInvoice,
		now:              nowFn,
	}, nil
}

// Clock 回本 client 的時鐘，供校時排程與 NTP 告警使用。
func (c *HTTPClient) Clock() *Clock { return c.clock }

// BreakerOpen 回報斷路器是否處於 open。
//
// 🔴 INV.op 的閘門必須放在 outbox.RunOnce **之前**、不能放進 ProcessFunc：
// pkg/outbox.Worker.processRow 是寫死的 Claim → Process、無 pre-claim hook，
// 放進 ProcessFunc 時租約已經扣掉了，而「不消耗租約」正是這道閘門的理由。
// half-open 照跑（W-19：連 half-open 一起跳過，斷路器永遠不知道上游好了沒有）。
// BreakerOpen 回**全域**斷路器是否開路（§7.3.4「全域斷路器另計」）。
//
// ⚠️ 它**不是**「這個 client 現在拒不拒絕呼叫」——那要看該賣家自己那顆，見
// BreakerOpenFor。2026-08-31 之前只有一顆斷路器，兩者是同一件事；拆成 per-store
// ＋ 全域之後就不是了。
func (c *HTTPClient) BreakerOpen() bool { return c.breaker.State() == gobreaker.StateOpen }

// BreakerOpenFor 回該 issuer 的斷路器是否開路（§7.3.4「per-store 斷路器」）。
//
// 這才是「這一家現在打不打得出去」的答案，也是 invoice_provider_accounts.status
// = 'DEGRADED' 那個持久化投影的來源。
func (c *HTTPClient) BreakerOpenFor(issuerID int64) bool {
	return c.breakerFor(issuerID).State() == gobreaker.StateOpen
}

// guardEnvironment 是環境 fail-closed 的**第二層**（§4.5.8）。
//
// 啟動時的 Config.Validate 只涵蓋平台自身那一組憑證；per-seller 憑證是 runtime
// 從 DB 讀出來的，那時 Validate 早已跑完。測試與正式共用同一網域、回應無環境標記
// ⇒ 設定填錯會對真實公司開出無法回收的真發票，而且無法從回應辨識。
//
// 放行條件：AllowRealInvoice == true **或** cred.Invoice == TestInvoice。
// 否則回 ENV_GUARD 並且**不打出去**。
func (c *HTTPClient) guardEnvironment(cred Credential) *APIError {
	if c.allowRealInvoice {
		return nil
	}
	if c.testInvoice != "" && cred.Invoice == c.testInvoice {
		return nil
	}
	return &APIError{
		Code:  CodeEnvGuard,
		Class: ClassManual,
		Msg: fmt.Sprintf(
			"AMEGO_ALLOW_REAL_INVOICE=false 且賣家統編 %s 不是 AMEGO_TEST_INVOICE(%s)："+
				"拒絕外呼，避免對真實公司開出無法回收的真發票",
			cred.Invoice, c.testInvoice),
		KnownCode: true,
	}
}

// do 執行一次帶簽章的 POST，回原始回應 body。
//
// 稽核用 defer 包住整個流程 ⇒ 無論從哪一條 return 出去都恰好寫一次
// （PayUniHttpClient 用 finally 達成的同一件事）。
func (c *HTTPClient) do(ctx context.Context, cred Credential, endpoint string, payload any, deadline time.Duration) (raw []byte, retErr error) {
	if gerr := c.guardEnvironment(cred); gerr != nil {
		gerr.Endpoint = endpoint
		// 沒有任何交互發生 ⇒ 不寫 provider audit（那張表的 error_kind 只有
		// TIMEOUT／CONNECTION／PROVIDER_ERROR／DECODE 四值，記錄一次不存在的往返
		// 只會污染「這筆送了幾遍」的對帳）。
		return nil, gerr
	}

	// 🔴 序列化一次。dataJSON 之後同時餵給 signature 與 buildForm，
	// 中間絕對不可再 marshal 一次 payload。
	dataJSON, err := encodeOnce(payload)
	if err != nil {
		return nil, &APIError{Endpoint: endpoint, Code: "ENCODE", Class: ClassPermanent, Kind: ErrorKindDecode, Msg: err.Error(), KnownCode: true, wrapped: err}
	}
	unixSec := c.clock.Unix()
	sign := signature(dataJSON, unixSec, cred.AppKey)
	form := buildForm(cred.Invoice, dataJSON, unixSec, sign)

	opID, attempt := auditFromContext(ctx)
	// #3031：保證這一次往返有 correlation_id。
	//
	// 🔴 為什麼補在這裡而不是「有就用、沒有就算了」：`invoice_provider_audit_logs`
	// 的**絕大多數列是排程 worker 寫的**（INV.op／recheck／pdf），而 worker 沒有
	// 請求可種、本服務也沒有任何地方起 OTel span（`InitTracer` 只裝了 provider，
	// 全 repo 零 `tracer.Start`）⇒ 只讀不補的話那一欄恆為 NULL。
	// 2026-09-02 實測即如此：修復部署之後新寫的 6 列 correlation_id 仍全空，
	// 因為它們全都來自 `/json/invoice_query` 的排程路徑。
	//
	// ⚠️ **誠實標明它是什麼**：有請求脈絡時它就是那條請求的 correlation id
	// （可跨服務追）；排程路徑上它是**這一次往返自己的識別碼**——足以讓 ADM 的
	// 服務商往來列表指名某一列、讓客服對話有東西可引用，但**不是**跨系統 trace key。
	// 要讓後者成立，得先讓 worker 起 span，那是 observability 的工程不是查詢層補得出來的。
	ctx = ensureCorrelationID(ctx)
	started := c.now()
	ex := Exchange{
		Endpoint:    endpoint,
		IssuerID:    cred.IssuerID,
		OperationID: opID,
		AttemptSeq:  attempt,
		RequestMasked: map[string]string{
			"data": dataJSON,
			"time": strconv.FormatInt(unixSec, 10),
			"sign": maskSign(sign),
		},
		OccurredAt: started,
	}
	defer func() {
		elapsed := c.now().Sub(started)
		ex.LatencyMS = elapsed.Milliseconds()
		var apiErr *APIError
		if errors.As(retErr, &apiErr) {
			ex.ProviderCode = apiErr.Code
			ex.ProviderMessage = apiErr.Msg
			ex.ErrorKind = apiErr.Kind
			if apiErr.HTTPStatus != 0 {
				ex.HTTPStatus = apiErr.HTTPStatus
			}
		}
		// M2／M3／M4 掛在**同一個 defer** 上，與 provider audit 落庫同一個理由：
		// 無論從哪一條 return 出去都恰好觀測一次。掛在各 return 前面的話，
		// 新增一條 return 就會少一次觀測，而少的那次沒有任何東西看得出來。
		observeCall(endpoint, apiErr, elapsed.Seconds())
		safeRecord(ctx, c.audit, ex)
	}()

	callCtx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()

	// limiter 吃**這次呼叫自己的 ctx**：排隊時間因此被該支 API 的 deadline 涵蓋，
	// 租約下界才不必為它另計一項。
	if werr := c.limiter.Wait(callCtx); werr != nil {
		// 排隊排到超過 deadline ⇒ 一個字都沒送出去 ⇒ 開票延後，不遺失。
		// M4 source=local：這是設計中的正常行為（與 source=amego 的「限速沒守住」
		// 分開記，混在一起會讓告警 ① 在尖峰長期紅著而被忽略）。
		observeLocalRateLimited()
		return nil, &APIError{Endpoint: endpoint, Code: "RATE_LIMIT_LOCAL", Class: ClassRetryable, Kind: ErrorKindConn, Msg: werr.Error(), KnownCode: true, wrapped: werr}
	}

	// 雙層：全域先擋（AMEGO 整體壞掉時所有人一起停），再走該賣家自己那顆。
	// 順序不可對調——顛倒的話全域 open 期間仍會為每個賣家各建一顆並各打一次。
	perStore := c.breakerFor(cred.IssuerID)
	body, berr := c.breaker.Execute(func() ([]byte, error) {
		return perStore.Execute(func() ([]byte, error) {
			return c.roundTrip(callCtx, endpoint, form, &ex)
		})
	})
	if berr != nil {
		if open := asOpenStateError(endpoint, berr); open != nil {
			return nil, open
		}
		return nil, berr
	}
	return body, nil
}

// roundTrip 送出一次請求並把回應翻成 (raw, *APIError)。它不重試。
func (c *HTTPClient) roundTrip(ctx context.Context, endpoint string, form url.Values, ex *Exchange) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, &APIError{Endpoint: endpoint, Code: "REQUEST", Class: ClassReconcile, Kind: ErrorKindConn, Msg: err.Error(), KnownCode: true, wrapped: err}
	}
	// 官方明令「請勿使用 Content-Type: application/json」——用錯會回 code 11
	// 「invoice 不可為空」，訊息完全誤導。
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		kind := ErrorKindConn
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			kind = ErrorKindTimeout
		}
		// 傳輸層失敗／ctx deadline ⇒ **結果未知**，走 §4.5.7 對帳決策樹。
		// 絕不可直接當成失敗：「沒有回應」與「沒有開票」是兩件事，中間差一張真發票。
		return nil, &APIError{Endpoint: endpoint, Code: "TRANSPORT", Class: ClassReconcile, Kind: kind, Msg: err.Error(), KnownCode: true, wrapped: err}
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &APIError{Endpoint: endpoint, Code: "TRANSPORT", Class: ClassReconcile, Kind: ErrorKindConn, HTTPStatus: resp.StatusCode, Msg: err.Error(), KnownCode: true, wrapped: err}
	}
	ex.HTTPStatus = resp.StatusCode
	ex.ResponseMasked = string(raw)

	// HTTP status 恆為 200，但仍要記：哪天不是 200 就是重要訊號（例如上游換了反代）。
	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{Endpoint: endpoint, Code: "HTTP_" + strconv.Itoa(resp.StatusCode), Class: ClassReconcile, Kind: ErrorKindConn, HTTPStatus: resp.StatusCode, Msg: string(raw), KnownCode: false}
	}

	var head struct {
		Code providerCode `json:"code"`
		Msg  string       `json:"msg"`
	}
	if uerr := json.Unmarshal(raw, &head); uerr != nil {
		// 🔴 HTTP 200 但 body 非 JSON ⇒ **PERMANENT 不是 UNKNOWN**（§7.5.1）。
		// 打錯路徑時 AMEGO 回 200 ＋ text/html ＋ body 是字串 `error`，不是 404。
		// 那是開機就全錯的我方 bug，要的是立刻告警，不是每一筆都 fail-closed
		// 停在 WAITING_CONFIRMATION 等一個永遠不會有結論的重查。
		return nil, &APIError{Endpoint: endpoint, Code: ErrorKindDecode, Class: ClassPermanent, Kind: ErrorKindDecode, HTTPStatus: resp.StatusCode, Msg: truncateForLog(string(raw)), KnownCode: true, wrapped: uerr}
	}
	code := string(head.Code)
	if code != "0" {
		apiErr := newProviderError(endpoint, code, head.Msg, resp.StatusCode)
		if code == "21" {
			// 我方限速自動降檔。AMEGO 沒有 Retry-After／X-RateLimit-*。
			c.limiter.Degrade()
		}
		return nil, apiErr
	}
	return raw, nil
}

func truncateForLog(s string) string {
	const max = 512
	if len(s) <= max {
		return s
	}
	return s[:max] + "…(truncated)"
}

// ── 寫入類 ────────────────────────────────────────────────────────────────

// IssueInvoice 開立發票。data 是**單一物件**（送陣列回 3040112）。
func (c *HTTPClient) IssueInvoice(ctx context.Context, cred Credential, req IssueRequest) (*IssueResult, error) {
	raw, err := c.do(ctx, cred, EndpointIssue, req, c.deadlines.Issue)
	if err != nil {
		return nil, err
	}
	var env issueEnvelope
	if uerr := json.Unmarshal(raw, &env); uerr != nil {
		return nil, decodeErr(EndpointIssue, uerr)
	}
	return &IssueResult{
		InvoiceNumber: env.InvoiceNumber,
		InvoiceTime:   env.InvoiceTime,
		RandomNumber:  env.RandomNumber,
		Barcode:       env.Barcode,
		QRCodeLeft:    env.QRCodeLeft,
		QRCodeRight:   env.QRCodeRight,
	}, nil
}

// VoidInvoice 作廢發票。
//
// 一次只送一張：多張時錯誤是「整批第 n 筆」形式的單一錯誤，無法得知部分成功／
// 部分失敗。回 code 0 只代表**受理**，完成判定靠 recheck worker 看 invoice_type
// 變 C0501 或 wait[] 清空。
func (c *HTTPClient) VoidInvoice(ctx context.Context, cred Credential, req VoidRequest) (*VoidAccepted, error) {
	if _, err := c.do(ctx, cred, EndpointVoid, []VoidRequest{req}, c.deadlines.Issue); err != nil {
		return nil, err
	}
	return &VoidAccepted{Accepted: true}, nil
}

// IssueAllowance 開立折讓。AMEGO 不回折讓單號——那是我方指定的冪等鍵，
// 送出前必須已落庫，否則折讓永遠查不回來（allowance_query 只能用單號查）。
func (c *HTTPClient) IssueAllowance(ctx context.Context, cred Credential, req AllowanceRequest) (*AllowanceResult, error) {
	if _, err := c.do(ctx, cred, EndpointAllowance, []AllowanceRequest{req}, c.deadlines.Issue); err != nil {
		return nil, err
	}
	return &AllowanceResult{AllowanceNumber: req.AllowanceNumber, Accepted: true}, nil
}

// VoidAllowance 作廢折讓。同 VoidInvoice，code 0 只代表受理。
func (c *HTTPClient) VoidAllowance(ctx context.Context, cred Credential, req VoidAllowanceRequest) (*VoidAccepted, error) {
	if _, err := c.do(ctx, cred, EndpointVoidAllowance, []VoidAllowanceRequest{req}, c.deadlines.Issue); err != nil {
		return nil, err
	}
	return &VoidAccepted{Accepted: true}, nil
}

// ── 查詢類 ────────────────────────────────────────────────────────────────

type invoiceQueryRequest struct {
	Type          string `json:"type"`
	OrderID       string `json:"order_id,omitempty"`
	InvoiceNumber string `json:"invoice_number,omitempty"`
}

// QueryInvoiceByOrderID 是**唯一**的冪等對帳入口（§4.5.7）。
func (c *HTTPClient) QueryInvoiceByOrderID(ctx context.Context, cred Credential, orderID string) (*InvoiceDetail, error) {
	return c.queryInvoice(ctx, cred, invoiceQueryRequest{Type: "order", OrderID: orderID})
}

// QueryInvoiceByNumber 以發票號碼查詢（同樣受 180 天窗口限制）。
func (c *HTTPClient) QueryInvoiceByNumber(ctx context.Context, cred Credential, number string) (*InvoiceDetail, error) {
	return c.queryInvoice(ctx, cred, invoiceQueryRequest{Type: "invoice", InvoiceNumber: number})
}

func (c *HTTPClient) queryInvoice(ctx context.Context, cred Credential, req invoiceQueryRequest) (*InvoiceDetail, error) {
	raw, err := c.do(ctx, cred, EndpointInvoiceQuery, req, c.deadlines.Query)
	if err != nil {
		return nil, err
	}
	var env stdEnvelope
	if uerr := json.Unmarshal(raw, &env); uerr != nil {
		return nil, decodeErr(EndpointInvoiceQuery, uerr)
	}
	var detail InvoiceDetail
	if len(env.Data) > 0 {
		if uerr := json.Unmarshal(env.Data, &detail); uerr != nil {
			return nil, decodeErr(EndpointInvoiceQuery, uerr)
		}
	}
	return &detail, nil
}

// ListInvoices 是超過 180 天時的 fallback（無日期限制），**每頁獨立計時**。
//
// 🔴 終止條件只能是「Records 為空」的顯式判定：分頁超界回 code 0 ＋ 空陣列、不報錯。
// 呼叫端每輪最多翻 1 頁並把游標落庫——頁數無自然上界，無上限地放進租約內跑等於讓
// 「單筆最壞耗時」失去上界（《多副本排程搶佔規範》C1 的原形 ⇒ 重複開票）。
func (c *HTTPClient) ListInvoices(ctx context.Context, cred Credential, req ListRequest) (*InvoicePage, error) {
	raw, err := c.do(ctx, cred, EndpointInvoiceList, req, c.deadlines.List)
	if err != nil {
		return nil, err
	}
	var env pageEnvelope
	if uerr := json.Unmarshal(raw, &env); uerr != nil {
		return nil, decodeErr(EndpointInvoiceList, uerr)
	}
	page := &InvoicePage{PageTotal: env.PageTotal, PageNow: env.PageNow, DataTotal: env.DataTotal}
	if len(env.Data) > 0 {
		if uerr := json.Unmarshal(env.Data, &page.Records); uerr != nil {
			return nil, decodeErr(EndpointInvoiceList, uerr)
		}
	}
	return page, nil
}

// QueryAllowance 以折讓單號查詢（折讓沒有「用訂單編號查」的選項）。
func (c *HTTPClient) QueryAllowance(ctx context.Context, cred Credential, allowanceNumber string) (*AllowanceDetail, error) {
	payload := struct {
		AllowanceNumber string `json:"allowance_number"`
	}{AllowanceNumber: allowanceNumber}
	raw, err := c.do(ctx, cred, EndpointAllowanceQuery, payload, c.deadlines.Query)
	if err != nil {
		return nil, err
	}
	var env stdEnvelope
	if uerr := json.Unmarshal(raw, &env); uerr != nil {
		return nil, decodeErr(EndpointAllowanceQuery, uerr)
	}
	var detail AllowanceDetail
	if len(env.Data) > 0 {
		if uerr := json.Unmarshal(env.Data, &detail); uerr != nil {
			return nil, decodeErr(EndpointAllowanceQuery, uerr)
		}
	}
	return &detail, nil
}

// validDownloadStyles 是我方自訂的白名單。
//
// AMEGO 對 download_style **不做值域驗證**（對照：allowance_file 有錯誤碼 34，
// invoice_file 沒有）⇒ 錯誤值會靜默產生非預期版面的 PDF。
var validDownloadStyles = map[int]bool{0: true, 1: true, 2: true, 3: true, 5: true}

// FetchInvoiceFileURL 取 PDF 連結。**URL 僅 10 分鐘有效**，取得後必須立即下載。
func (c *HTTPClient) FetchInvoiceFileURL(ctx context.Context, cred Credential, req FileRequest) (string, error) {
	if !validDownloadStyles[req.DownloadStyle] {
		return "", &APIError{
			Endpoint: EndpointInvoiceFile, Code: "DOWNLOAD_STYLE", Class: ClassPermanent,
			Msg: fmt.Sprintf("download_style=%d 不在白名單 {0,1,2,3,5}", req.DownloadStyle), KnownCode: true,
		}
	}
	raw, err := c.do(ctx, cred, EndpointInvoiceFile, req, c.deadlines.Query)
	if err != nil {
		return "", err
	}
	var env stdEnvelope
	if uerr := json.Unmarshal(raw, &env); uerr != nil {
		return "", decodeErr(EndpointInvoiceFile, uerr)
	}
	var data struct {
		FileURL string `json:"file_url"`
	}
	if len(env.Data) > 0 {
		if uerr := json.Unmarshal(env.Data, &data); uerr != nil {
			return "", decodeErr(EndpointInvoiceFile, uerr)
		}
	}
	return data.FileURL, nil
}

// ── 驗證類 ────────────────────────────────────────────────────────────────

// VerifyMobileBarcode 驗手機條碼。它是唯一的載具驗證端點，且只驗手機條碼。
// 成功時回應沒有任何載具資訊 ⇒ 回傳值只有 error。
func (c *HTTPClient) VerifyMobileBarcode(ctx context.Context, cred Credential, barcode string) error {
	payload := struct {
		BarCode string `json:"barCode"`
	}{BarCode: barcode}
	_, err := c.do(ctx, cred, EndpointBarcode, payload, c.deadlines.Query)
	return err
}

// QueryCompanyNames 批次查統編對應公司名。
//
// 兩個實測事實決定了回傳型別是 map 不是 slice：① 送重複統編只回一筆（去重）
// ② 批次是 all-or-nothing（一顆壞蘋果整批失敗）⇒ 必須以 ban 為鍵 join，
// 不可索引對位。
//
// ⚠️ name 為空字串**不代表統編無效**（管委會、會計事務所等不在資料集內），
// 統編有效性要用財政部檢查碼演算法自己算。
func (c *HTTPClient) QueryCompanyNames(ctx context.Context, cred Credential, bans []string) (map[string]string, error) {
	payload := make([]struct {
		Ban string `json:"ban"`
	}, 0, len(bans))
	for _, b := range bans {
		payload = append(payload, struct {
			Ban string `json:"ban"`
		}{Ban: b})
	}
	raw, err := c.do(ctx, cred, EndpointBanQuery, payload, c.deadlines.Query)
	if err != nil {
		return nil, err
	}
	var env stdEnvelope
	if uerr := json.Unmarshal(raw, &env); uerr != nil {
		return nil, decodeErr(EndpointBanQuery, uerr)
	}
	var rows []struct {
		Ban  string `json:"ban"`
		Name string `json:"name"`
	}
	if len(env.Data) > 0 {
		if uerr := json.Unmarshal(env.Data, &rows); uerr != nil {
			return nil, decodeErr(EndpointBanQuery, uerr)
		}
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.Ban] = r.Name
	}
	return out, nil
}

// ── 資源與健康 ────────────────────────────────────────────────────────────

type trackRequest struct {
	Year         int    `json:"Year"`
	Period       int    `json:"Period"`
	TrackAPICode string `json:"TrackApiCode,omitempty"`
}

// ListTracks 查該公司在加值中心的所有字軌（三層巢狀，Track.Data 保留巢狀結構）。
//
// ⚠️ period 是 **0–5 的期別代碼**（0=01-02月 … 5=11-12月），不是民國年月：
// 送 {"Period":"11508"} 當場回 code 99「Period 錯誤」。
func (c *HTTPClient) ListTracks(ctx context.Context, cred Credential, year, period int) ([]Track, error) {
	raw, err := c.do(ctx, cred, EndpointTrackAll, trackRequest{Year: year, Period: period}, c.deadlines.Query)
	if err != nil {
		return nil, err
	}
	var env stdEnvelope
	if uerr := json.Unmarshal(raw, &env); uerr != nil {
		return nil, decodeErr(EndpointTrackAll, uerr)
	}
	var tracks []Track
	if len(env.Data) > 0 {
		if uerr := json.Unmarshal(env.Data, &tracks); uerr != nil {
			return nil, decodeErr(EndpointTrackAll, uerr)
		}
	}
	return tracks, nil
}

// TrackStatus 查字軌配號狀態（只回「API 配號」類型的字軌）。
//
// ⚠️ 簽章與 §4.5.2 的介面草稿不同：草稿只收一個 trackID，而 AMEGO 的
// track_status **必填 Year 與 Period**（缺了回 code 99）。見交付報告的矛盾清單。
func (c *HTTPClient) TrackStatus(ctx context.Context, cred Credential, year, period int, trackAPICode string) ([]TrackBalance, error) {
	raw, err := c.do(ctx, cred, EndpointTrackStatus, trackRequest{Year: year, Period: period, TrackAPICode: trackAPICode}, c.deadlines.Query)
	if err != nil {
		return nil, err
	}
	var env stdEnvelope
	if uerr := json.Unmarshal(raw, &env); uerr != nil {
		return nil, decodeErr(EndpointTrackStatus, uerr)
	}
	var rows []TrackBalance
	if len(env.Data) > 0 {
		if uerr := json.Unmarshal(env.Data, &rows); uerr != nil {
			return nil, decodeErr(EndpointTrackStatus, uerr)
		}
	}
	return rows, nil
}

// ServerTime 是唯一免簽章、免 Credential 的端點（GET，Content-Type 是 text/plain）。
//
// 它刻意**不走斷路器**：校時是恢復手段之一，斷路器開路期間仍需要它才能修掉
// code 15 那條路徑；而每 15 分鐘一次唯讀 GET 對「不要搥打上游」的目的無影響。
// 也不寫 provider audit——沒有 issuer、沒有 data／sign 可遮罩。
func (c *HTTPClient) ServerTime(ctx context.Context) (time.Time, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.deadlines.Time)
	defer cancel()
	if werr := c.limiter.Wait(callCtx); werr != nil {
		return time.Time{}, &APIError{Endpoint: EndpointServerTime, Code: "RATE_LIMIT_LOCAL", Class: ClassRetryable, Kind: ErrorKindConn, Msg: werr.Error(), KnownCode: true, wrapped: werr}
	}
	req, err := http.NewRequestWithContext(callCtx, http.MethodGet, c.baseURL+EndpointServerTime, nil)
	if err != nil {
		return time.Time{}, &APIError{Endpoint: EndpointServerTime, Code: "REQUEST", Class: ClassReconcile, Kind: ErrorKindConn, Msg: err.Error(), wrapped: err}
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return time.Time{}, &APIError{Endpoint: EndpointServerTime, Code: "TRANSPORT", Class: ClassReconcile, Kind: ErrorKindConn, Msg: err.Error(), wrapped: err}
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return time.Time{}, &APIError{Endpoint: EndpointServerTime, Code: "TRANSPORT", Class: ClassReconcile, Kind: ErrorKindConn, HTTPStatus: resp.StatusCode, Msg: err.Error(), wrapped: err}
	}
	var env timeEnvelope
	if uerr := json.Unmarshal(raw, &env); uerr != nil {
		return time.Time{}, decodeErr(EndpointServerTime, uerr)
	}
	if env.Timestamp == 0 {
		return time.Time{}, &APIError{Endpoint: EndpointServerTime, Code: ErrorKindDecode, Class: ClassPermanent, Kind: ErrorKindDecode, Msg: "timestamp 缺席", KnownCode: true}
	}
	// 只用 timestamp，不解析 text：text 是台北時間、文件未載明時區。
	return time.Unix(env.Timestamp, 0).UTC(), nil
}

// ── 上傳狀態 ──────────────────────────────────────────────────────────────

// InvoiceUploadStatus 回填 provider_upload_status（status=99 才是「上傳完成」）。
func (c *HTTPClient) InvoiceUploadStatus(ctx context.Context, cred Credential, number string) (*UploadStatus, error) {
	payload := []struct {
		InvoiceNumber string `json:"InvoiceNumber"`
	}{{InvoiceNumber: number}}
	raw, err := c.do(ctx, cred, EndpointInvoiceStatus, payload, c.deadlines.Query)
	if err != nil {
		return nil, err
	}
	return decodeUploadStatus(EndpointInvoiceStatus, raw, "invoice_number", number)
}

// AllowanceUploadStatus 同上，用於折讓單。
func (c *HTTPClient) AllowanceUploadStatus(ctx context.Context, cred Credential, allowanceNumber string) (*UploadStatus, error) {
	payload := []struct {
		AllowanceNumber string `json:"AllowanceNumber"`
	}{{AllowanceNumber: allowanceNumber}}
	raw, err := c.do(ctx, cred, EndpointAllowanceStatus, payload, c.deadlines.Query)
	if err != nil {
		return nil, err
	}
	return decodeUploadStatus(EndpointAllowanceStatus, raw, "allowance_number", allowanceNumber)
}

// decodeUploadStatus 容忍新版（data 為陣列）與舊版（data 為以單號為 key 的物件）
// 兩種形狀：文件同時列出兩者卻沒說怎麼切換，只認一種的話 AMEGO 任何時候切回舊版
// 就 parse 失敗。
func decodeUploadStatus(endpoint string, raw []byte, numberField, want string) (*UploadStatus, error) {
	var env stdEnvelope
	if uerr := json.Unmarshal(raw, &env); uerr != nil {
		return nil, decodeErr(endpoint, uerr)
	}
	type row struct {
		InvoiceNumber   string `json:"invoice_number"`
		AllowanceNumber string `json:"allowance_number"`
		Type            string `json:"type"`
		Status          int    `json:"status"`
		TotalAmount     int64  `json:"total_amount"`
		TaxAmount       int64  `json:"tax_amount"`
	}
	pick := func(r row, key string) *UploadStatus {
		number := r.InvoiceNumber
		if numberField == "allowance_number" {
			number = r.AllowanceNumber
		}
		if number == "" {
			number = key
		}
		return &UploadStatus{Number: number, Type: r.Type, Status: r.Status, TotalAmount: r.TotalAmount, TaxAmount: r.TaxAmount}
	}
	trimmed := strings.TrimSpace(string(env.Data))
	if trimmed == "" || trimmed == "null" {
		return nil, &APIError{Endpoint: endpoint, Code: ErrorKindDecode, Class: ClassPermanent, Kind: ErrorKindDecode, Msg: "data 缺席", KnownCode: true}
	}
	if trimmed[0] == '[' {
		var rows []row
		if uerr := json.Unmarshal(env.Data, &rows); uerr != nil {
			return nil, decodeErr(endpoint, uerr)
		}
		for _, r := range rows {
			if s := pick(r, want); s.Number == want {
				return s, nil
			}
		}
		if len(rows) == 1 {
			return pick(rows[0], want), nil
		}
		return nil, &APIError{Endpoint: endpoint, Code: ErrorKindDecode, Class: ClassPermanent, Kind: ErrorKindDecode, Msg: "回應不含查詢的單號 " + want, KnownCode: true}
	}
	var m map[string]row
	if uerr := json.Unmarshal(env.Data, &m); uerr != nil {
		return nil, decodeErr(endpoint, uerr)
	}
	r, ok := m[want]
	if !ok {
		return nil, &APIError{Endpoint: endpoint, Code: ErrorKindDecode, Class: ClassPermanent, Kind: ErrorKindDecode, Msg: "回應不含查詢的單號 " + want, KnownCode: true}
	}
	return pick(r, want), nil
}

// decodeErr 把「回應是合法 JSON、但形狀不是我方預期」歸成 DECODE + PERMANENT，
// 與 HTTP 200 非 JSON 同一分支：兩者都是我方 bug，要的是立刻告警。
func decodeErr(endpoint string, err error) *APIError {
	return &APIError{Endpoint: endpoint, Code: ErrorKindDecode, Class: ClassPermanent, Kind: ErrorKindDecode, Msg: err.Error(), KnownCode: true, wrapped: err}
}

// 全域斷路器的跳脫參數（§7.3.4「全域斷路器另計」逐字）。
const (
	globalBreakerWindow       = 60 * time.Second
	globalBreakerMinSamples   = 20
	globalBreakerFailureRatio = 0.5
)

// breakerFor 回該 issuer 的斷路器，沒有就建一顆。
//
// 🔴 用 IssuerID 當 key 而不是 store_id：本層拿得到的是 Credential.IssuerID，
// 而 invoice_issuers 與 store 是一對一（issuer_type='SELLER' 時 issuer_ref_id
// 就是 store_id）⇒ 兩者等價。用 IssuerID 可以少一次查詢。
//
// map 會隨賣家數線性成長而不回收：那是刻意的。一顆 gobreaker 只有幾個計數器，
// 而「回收之後那家的連敗計數歸零」會讓斷路器在賣家間歇性呼叫時永遠跳不起來。
func (c *HTTPClient) breakerFor(issuerID int64) *gobreaker.CircuitBreaker[[]byte] {
	c.perIssuerMu.Lock()
	defer c.perIssuerMu.Unlock()
	if b, ok := c.perIssuer[issuerID]; ok {
		return b
	}
	b := gobreaker.NewCircuitBreaker[[]byte](gobreaker.Settings{
		Name:        "amego-issuer",
		MaxRequests: c.breakerSet.MaxRequests,
		Interval:    c.breakerSet.Interval,
		Timeout:     c.breakerSet.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= c.breakerSet.Failures
		},
		IsExcluded: isExcludedFromBreaker,
	})
	c.perIssuer[issuerID] = b
	return b
}
