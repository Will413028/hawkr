package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/handler/delegation"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

type stubSessionResolver struct {
	calls    int
	activity []bool
	resp     *accountpb.GetCurrentSessionResponse
	err      error
}

func (s *stubSessionResolver) GetCurrentDelegationSession(_ context.Context, _ string, countsAsActivity bool, _ int) (*accountpb.GetCurrentSessionResponse, error) {
	s.calls++
	s.activity = append(s.activity, countsAsActivity)
	return s.resp, s.err
}

// stubStoreState 是 §8.1.3 第 8 項那一跳的替身。
type stubStoreState struct {
	calls int
	resp  *sellerpb.SellerResponse
	err   error
}

func (s *stubStoreState) GetStoreStateForGuard(_ context.Context, _ int64, _ int) (*sellerpb.SellerResponse, error) {
	s.calls++
	return s.resp, s.err
}

type stubRevoker struct {
	reqs []*accountpb.RevokeSessionRequest
	resp *accountpb.RevokeSessionResponse
	err  error
}

func (s *stubRevoker) RevokeDelegationSession(_ context.Context, req *accountpb.RevokeSessionRequest, _ int) (*accountpb.RevokeSessionResponse, error) {
	s.reqs = append(s.reqs, req)
	if s.resp == nil && s.err == nil {
		return &accountpb.RevokeSessionResponse{Code: 0}, nil
	}
	return s.resp, s.err
}

func operableStore(lifecycle string) *sellerpb.SellerResponse {
	return &sellerpb.SellerResponse{Code: 0, Data: &sellerpb.SellerInfo{LifecycleStatus: lifecycle}}
}

func okSession(status string, storeID string, caps ...string) *accountpb.GetCurrentSessionResponse {
	return &accountpb.GetCurrentSessionResponse{
		Code: 0,
		Data: &accountpb.OperationSessionSummary{
			SessionId:    "2701445588392017",
			Status:       status,
			Capabilities: caps,
			Target:       &accountpb.DelegationTarget{StoreId: storeID},
			Actor:        &accountpb.DelegationActor{AdminId: "7493952179963691009"},
		},
	}
}

// stubAudit 記下 guard 送出的稽核 RPC。斷言的是「送了什麼」而不是「有沒有送」：
// action 記錯面、capability 記成 read、三個 transport 欄位空著，這幾種壞法在任何
// 其他地方都看不出來——稽核列照樣落地、每個約束都過。
type stubAudit struct {
	intents   []*accountpb.RecordAuditIntentRequest
	confirms  []*accountpb.ConfirmAuditResultRequest
	intentErr error
}

const stubAuditLogID = "7495464541774413824"

func (s *stubAudit) RecordAuditIntent(_ context.Context, req *accountpb.RecordAuditIntentRequest, _ int) (*accountpb.RecordAuditIntentResponse, error) {
	s.intents = append(s.intents, req)
	if s.intentErr != nil {
		return nil, s.intentErr
	}
	return &accountpb.RecordAuditIntentResponse{
		Code: 0,
		Data: &accountpb.RecordAuditIntentData{AuditLogId: stubAuditLogID},
	}, nil
}

func (s *stubAudit) ConfirmAuditResult(_ context.Context, req *accountpb.ConfirmAuditResultRequest, _ int) (*accountpb.ConfirmAuditResultResponse, error) {
	s.confirms = append(s.confirms, req)
	return &accountpb.ConfirmAuditResultResponse{Code: 0}, nil
}

func (s *stubAudit) RecordAuditIntentBatch(_ context.Context, _ *accountpb.RecordAuditIntentBatchRequest, _ int) (*accountpb.RecordAuditIntentBatchResponse, error) {
	return &accountpb.RecordAuditIntentBatchResponse{Code: 0}, nil
}

func (s *stubAudit) ConfirmAuditResultBatch(_ context.Context, _ *accountpb.ConfirmAuditResultBatchRequest, _ int) (*accountpb.ConfirmAuditResultBatchResponse, error) {
	return &accountpb.ConfirmAuditResultBatchResponse{Code: 0}, nil
}

func newGuardEngine(stub *stubSessionResolver) *gin.Engine {
	return newGuardEngineWith(stub, &stubStoreState{resp: operableStore("active")}, &stubRevoker{})
}

func newGuardEngineWith(stub *stubSessionResolver, stores *stubStoreState, revoker *stubRevoker) *gin.Engine {
	return newGuardEngineWithAudit(stub, stores, revoker, nil)
}

func newGuardEngineWithAudit(stub *stubSessionResolver, stores *stubStoreState, revoker *stubRevoker, audit *stubAudit) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// TracingMiddleware 的替身。真實請求上這兩個值恆存在（有 inbound header 就沿用、
	// 沒有就現生），不設的話「稽核列有沒有帶上追溯欄」這件事在測試裡驗不出來。
	r.Use(func(c *gin.Context) {
		c.Set("request_id", guardTestRequestID)
		c.Set("trace_id", guardTestCorrelationID)
		c.Next()
	})
	deps := delegationDeps{resolver: stub, stores: stores, revoker: revoker}
	if audit != nil {
		deps.audit = &delegation.Orchestrator{Audit: audit}
	}
	g := r.Group("/g")
	g.Handle(http.MethodPatch, "/sellers/:store_id/products/:product_id",
		requireDelegated(authz.AdminCapProductsWrite, deps),
		func(c *gin.Context) {
			if _, ok := middleware.DelegationSession(c); !ok {
				c.String(http.StatusInternalServerError, "session not in context")
				return
			}
			c.JSON(http.StatusOK, gin.H{"code": 0})
		})
	g.Handle(http.MethodPost, "/media/upload_sessions",
		requireDelegatedSessionOnly(deps),
		func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"code": 0}) })
	return r
}

func doReq(t *testing.T, r *gin.Engine, method, path string, credential string) (int, int32) {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if credential != "" {
		req.Header.Set(headerOperationCredential, credential)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var env struct {
		Code int32 `json:"code"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	return w.Code, env.Code
}

func TestRequireDelegatedVerdicts(t *testing.T) {
	const path = "/g/sellers/2685914682/products/77"

	cases := []struct {
		name       string
		credential string
		resp       *accountpb.GetCurrentSessionResponse
		err        error
		wantHTTP   int
		wantCode   int32
	}{
		{
			name:     "缺 credential",
			resp:     okSession("active", "2685914682", "products.write"),
			wantHTTP: http.StatusUnauthorized, wantCode: bizcode.DelegationCredentialInvalid,
		},
		{
			name: "上游回 9274", credential: "dopc_x",
			resp:     &accountpb.GetCurrentSessionResponse{Code: bizcode.DelegationCredentialInvalid, Message: "無效"},
			wantHTTP: http.StatusUnauthorized, wantCode: bizcode.DelegationCredentialInvalid,
		},
		{
			name: "hard/idle 到期", credential: "dopc_x",
			resp:     okSession("expired", "2685914682", "products.write"),
			wantHTTP: http.StatusGone, wantCode: bizcode.DelegationSessionExpired,
		},
		{
			name: "已撤銷", credential: "dopc_x",
			resp:     okSession("revoked", "2685914682", "products.write"),
			wantHTTP: http.StatusGone, wantCode: bizcode.DelegationSessionRevoked,
		},
		{
			name: "target 不符", credential: "dopc_x",
			resp:     okSession("active", "2632049831", "products.write"),
			wantHTTP: http.StatusForbidden, wantCode: bizcode.DelegationTargetMismatch,
		},
		{
			name: "capability 不足", credential: "dopc_x",
			resp:     okSession("active", "2685914682", "products.read"),
			wantHTTP: http.StatusForbidden, wantCode: bizcode.DelegationCapabilityDenied,
		},
		{
			name: "放行", credential: "dopc_x",
			resp:     okSession("active", "2685914682", "products.read", "products.write"),
			wantHTTP: http.StatusOK, wantCode: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubSessionResolver{resp: tc.resp, err: tc.err}
			status, code := doReq(t, newGuardEngine(stub), http.MethodPatch, path, tc.credential)
			if status != tc.wantHTTP || code != tc.wantCode {
				t.Fatalf("HTTP %d / code %d，預期 HTTP %d / code %d", status, code, tc.wantHTTP, tc.wantCode)
			}
		})
	}
}

// 上游問不到答案 → 424 + 9280。不得用 503／404：前端把那兩者讀成「此環境尚未部署代行
// 營運服務」並永久隱藏入口；也不得用 5xx——Cloudflare 會把 5xx 的 body 換成自己的錯誤頁，
// 業務碼永遠到不了前端（見 response/bizhttp.go 的登錄表註解）。未接線的 resolver 同樣
// fail closed，不得讀成「已授權」。
func TestRequireDelegatedFailsClosedOnUpstream(t *testing.T) {
	stub := &stubSessionResolver{err: context.DeadlineExceeded}
	status, code := doReq(t, newGuardEngine(stub), http.MethodPatch, "/g/sellers/1/products/2", "dopc_x")
	if status != http.StatusFailedDependency || code != bizcode.DelegationUpstreamUnavailable {
		t.Fatalf("上游錯誤：HTTP %d / code %d，預期 424 / 9280", status, code)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Handle(http.MethodPatch, "/g/sellers/:store_id/products/:product_id",
		requireDelegated(authz.AdminCapProductsWrite, delegationDeps{}),
		func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"code": 0}) })
	// ⚠️ 這一段與上一段的 HTTP 相同（424）而**碼不同**，那正是本測試的價值所在：
	// 上一段是上游逾時（重試會好，9280），這一段是 resolver 根本沒被 wire 起來
	// （重試一萬次都一樣，9289，#2744）。兩者外部看起來完全相同，只有碼講不講真話。
	status, code = doReq(t, r, http.MethodPatch, "/g/sellers/1/products/2", "dopc_x")
	if status != http.StatusFailedDependency || code != bizcode.DelegationInternalContractMismatch {
		t.Fatalf("resolver 未接線：HTTP %d / code %d，預期 424 / 9289（未接線是永久失敗，不是 9280 的「稍後再試」）", status, code)
	}
	if bizcode.DelegationInternalContractMismatch == bizcode.DelegationUpstreamUnavailable {
		t.Fatal("兩個碼相同的話上面的斷言恆真，等於沒驗")
	}
}

// TestRequireDelegatedRestoresBizCodeFromUpstreamError 鎖住「憑證無效」與「上游不可用」
// 的分辨。
//
// account-service 對無效憑證回的是 gRPC OK ＋ 非零 envelope code，而
// EnvelopeGuardInterceptor 會把它轉成帶 "[9274] " 前綴的 gRPC error
// （gateway/internal/grpc/envelope_guard_interceptor.go:104）。少了還原這一步，
// 憑證無效會一律落進 9280/424 分支 —— 前端顯示「代行服務暫時不可用，請稍後再試」，
// 而重試永遠不會好，因為憑證不會因為再試一次就變有效。
//
// 這支測試的兩半缺一不可：只驗上半會讓「把所有 err 都當成業務碼」也通過。
func TestRequireDelegatedRestoresBizCodeFromUpstreamError(t *testing.T) {
	// ① 帶業務碼前綴的 error → 還原成該碼與它登錄的 HTTP status
	wrapped := grpcstatus.Error(codes.Unauthenticated, "[9274] 代行憑證無效，請重新建立工作階段")
	status, code := doReq(t, newGuardEngine(&stubSessionResolver{err: wrapped}),
		http.MethodPatch, "/g/sellers/1/products/2", "dopc_x")
	if code != bizcode.DelegationCredentialInvalid {
		t.Fatalf("憑證無效未被還原：code %d，預期 %d（會被誤報成服務故障）",
			code, bizcode.DelegationCredentialInvalid)
	}
	if status != http.StatusUnauthorized {
		t.Fatalf("憑證無效的 HTTP status = %d，預期 401（424 會讓前端顯示可重試）", status)
	}

	// ② 反向：沒有業務碼前綴的 error 仍必須是 9280/424，不得被誤判成業務拒絕
	status, code = doReq(t, newGuardEngine(&stubSessionResolver{err: context.DeadlineExceeded}),
		http.MethodPatch, "/g/sellers/1/products/2", "dopc_x")
	if status != http.StatusFailedDependency || code != bizcode.DelegationUpstreamUnavailable {
		t.Fatalf("真正的上游錯誤：HTTP %d / code %d，預期 424 / %d",
			status, code, bizcode.DelegationUpstreamUnavailable)
	}
}

// §8.1.3 第 8 項：目標店家在代行期間轉為不可代行 ⇒ 下一次請求即 fail-closed，且該
// 工作階段當場轉入終態。deny-by-default 白名單——只有 active 放行。
func TestRequireDelegatedRechecksStoreState(t *testing.T) {
	const path = "/g/sellers/2685914682/products/77"

	cases := []struct {
		name       string
		stores     *stubStoreState
		wantHTTP   int
		wantCode   int32
		wantRevoke bool
	}{
		{
			name:   "active 放行",
			stores: &stubStoreState{resp: operableStore("active")},
			// 放行時 revoke 一次都不能打：那會把正常代行的工作階段當場撤掉。
			wantHTTP: http.StatusOK, wantCode: 0,
		},
		{
			name:     "停權 → 9272 ＋ 轉終態",
			stores:   &stubStoreState{resp: operableStore("suspended")},
			wantHTTP: http.StatusUnprocessableEntity, wantCode: bizcode.DelegationStoreNotOperable,
			wantRevoke: true,
		},
		{
			name:     "治理停權 → 9272",
			stores:   &stubStoreState{resp: operableStore("suspended_governance")},
			wantHTTP: http.StatusUnprocessableEntity, wantCode: bizcode.DelegationStoreNotOperable,
			wantRevoke: true,
		},
		{
			// 日後新增的生命週期值走 deny-by-default：黑名單會靜默放行它。
			name:     "未知生命週期值 → 9272",
			stores:   &stubStoreState{resp: operableStore("dormant")},
			wantHTTP: http.StatusUnprocessableEntity, wantCode: bizcode.DelegationStoreNotOperable,
			wantRevoke: true,
		},
		{
			// 軟刪的店在 seller-service 的 lookup 上是 NotFound（濾 deleted_at）。
			name:     "已刪除／查無 → 9265",
			stores:   &stubStoreState{err: grpcstatus.Error(codes.NotFound, "seller not found")},
			wantHTTP: http.StatusForbidden, wantCode: bizcode.DelegationStoreNotDelegable,
			wantRevoke: true,
		},
		{
			// 已關店回 envelope 4040，經 EnvelopeGuardInterceptor 成為帶業務碼前綴的
			// FailedPrecondition——那是「答案」不是「答不出來」。
			name:     "已關店 → 9265",
			stores:   &stubStoreState{err: grpcstatus.Error(codes.FailedPrecondition, "[4040] 商家不存在或已關閉")},
			wantHTTP: http.StatusForbidden, wantCode: bizcode.DelegationStoreNotDelegable,
			wantRevoke: true,
		},
		{
			name:     "回應缺 data → 9265",
			stores:   &stubStoreState{resp: &sellerpb.SellerResponse{Code: 0}},
			wantHTTP: http.StatusForbidden, wantCode: bizcode.DelegationStoreNotDelegable,
			wantRevoke: true,
		},
		{
			// 問不到答案 ≠ 確知不可代行：拒絕本次請求，但工作階段必須維持原狀，否則
			// seller-service 抖動 30 秒就會把當下所有工作階段永久打死。
			name:     "逾時 → 9280 且不轉終態",
			stores:   &stubStoreState{err: grpcstatus.Error(codes.DeadlineExceeded, "context deadline exceeded")},
			wantHTTP: http.StatusFailedDependency, wantCode: bizcode.DelegationUpstreamUnavailable,
		},
		{
			name:     "上游不可用 → 9280 且不轉終態",
			stores:   &stubStoreState{err: grpcstatus.Error(codes.Unavailable, "connection refused")},
			wantHTTP: http.StatusFailedDependency, wantCode: bizcode.DelegationUpstreamUnavailable,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubSessionResolver{resp: okSession("active", "2685914682", "products.write")}
			revoker := &stubRevoker{}
			status, code := doReq(t, newGuardEngineWith(stub, tc.stores, revoker), http.MethodPatch, path, "dopc_x")
			if status != tc.wantHTTP || code != tc.wantCode {
				t.Fatalf("HTTP %d / code %d，預期 HTTP %d / code %d", status, code, tc.wantHTTP, tc.wantCode)
			}
			if tc.stores.calls != 1 {
				t.Fatalf("店家狀態被查 %d 次，預期恰好 1 次", tc.stores.calls)
			}
			if got := len(revoker.reqs) > 0; got != tc.wantRevoke {
				t.Fatalf("撤銷呼叫 = %v，預期 %v", got, tc.wantRevoke)
			}
			if tc.wantRevoke {
				req := revoker.reqs[0]
				if req.GetRevokeReason() != revokeReasonStoreNotOperable {
					t.Errorf("revokeReason = %q，預期 %q（稽核靠這個固定字面值分辨撤銷來源）",
						req.GetRevokeReason(), revokeReasonStoreNotOperable)
				}
				if req.GetSessionId() != "2701445588392017" || req.GetActorAdminId() != "7493952179963691009" {
					t.Errorf("撤銷對象／執行者不正確：session=%q actor=%q", req.GetSessionId(), req.GetActorAdminId())
				}
			}
		})
	}
}

// 撤銷失敗不得改變本次請求的結果：拒絕在判定成立的那一刻就已成立，撤銷只是讓 UI 與
// 排程跟上。把它改判成 5xx 會讓一道正常運作的防線看起來像後端壞掉。
func TestStoreNotOperableStillDeniesWhenRevokeFails(t *testing.T) {
	stub := &stubSessionResolver{resp: okSession("active", "2685914682", "products.write")}
	revoker := &stubRevoker{err: context.DeadlineExceeded}
	stores := &stubStoreState{resp: operableStore("suspended")}
	status, code := doReq(t, newGuardEngineWith(stub, stores, revoker),
		http.MethodPatch, "/g/sellers/2685914682/products/77", "dopc_x")
	if status != http.StatusUnprocessableEntity || code != bizcode.DelegationStoreNotOperable {
		t.Fatalf("HTTP %d / code %d，預期 422 / 9272", status, code)
	}
}

// 未接線的店家狀態依賴同樣 fail closed：一個問不到的授權判定不得讀成「已授權」。
func TestStoreStateResolverUnwiredFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	deps := delegationDeps{resolver: &stubSessionResolver{resp: okSession("active", "2685914682", "products.write")}}
	r.Handle(http.MethodPatch, "/g/sellers/:store_id/products/:product_id",
		requireDelegated(authz.AdminCapProductsWrite, deps),
		func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"code": 0}) })
	status, code := doReq(t, r, http.MethodPatch, "/g/sellers/2685914682/products/77", "dopc_x")
	if status != http.StatusFailedDependency || code != bizcode.DelegationInternalContractMismatch {
		t.Fatalf("stores 未接線：HTTP %d / code %d，預期 424 / 9289（#2744：未接線是永久失敗）", status, code)
	}
}

// 媒體 upload session 兩支不跑第 8 項：它們不碰 target store，沒有「這家店還能不能被
// 代行」可問（而它們仍然要算活動，見 TestCountsAsActivityIsDecidedByGuard）。
func TestSessionOnlyGuardSkipsStoreState(t *testing.T) {
	stub := &stubSessionResolver{resp: okSession("active", "2685914682", "products.write")}
	stores := &stubStoreState{resp: operableStore("suspended")}
	r := newGuardEngineWith(stub, stores, &stubRevoker{})
	if status, _ := doReq(t, r, http.MethodPost, "/g/media/upload_sessions", "dopc_x"); status != http.StatusOK {
		t.Fatalf("媒體 upload session 應放行，實得 %d", status)
	}
	if stores.calls != 0 {
		t.Fatalf("媒體 guard 查了 %d 次店家狀態，預期 0", stores.calls)
	}
}

// counts_as_activity 由「路由註冊在哪個 middleware 下」決定，填 true 的 guard 恰為兩個。
// 媒體兩支漏填是靜默的：換一批商品圖正是最容易停留超過 idle 窗口、又完全不打
// delegatedHandle 路由的動作，操作者會在按儲存時才被踢掉，中途沒有任何訊號。
func TestCountsAsActivityIsDecidedByGuard(t *testing.T) {
	stub := &stubSessionResolver{resp: okSession("active", "2685914682", "products.write")}
	r := newGuardEngine(stub)

	if status, _ := doReq(t, r, http.MethodPatch, "/g/sellers/2685914682/products/77", "dopc_x"); status != http.StatusOK {
		t.Fatalf("代行資源路由應放行，實得 %d", status)
	}
	if status, _ := doReq(t, r, http.MethodPost, "/g/media/upload_sessions", "dopc_x"); status != http.StatusOK {
		t.Fatalf("媒體 upload session 應放行，實得 %d", status)
	}
	if len(stub.activity) != 2 {
		t.Fatalf("resolver 被呼叫 %d 次，預期 2", len(stub.activity))
	}
	for i, got := range stub.activity {
		if !got {
			t.Errorf("第 %d 次呼叫 counts_as_activity=false，兩個 guard 都必須填 true", i+1)
		}
	}
}

const (
	guardTestRequestID     = "req-guard-1"
	guardTestCorrelationID = "corr-guard-1"
	guardTestUserAgent     = "ADM-E2E/1.0"
)

// doGuardReq 與 doReq 的差別只在多帶一個 User-Agent：稽核列的「裝置」欄就是它。
func doGuardReq(t *testing.T, r *gin.Engine, method, path string) (int, int32) {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set(headerOperationCredential, "dopc_x")
	req.Header.Set("User-Agent", guardTestUserAgent)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var env struct {
		Code int32 `json:"code"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	return w.Code, env.Code
}

// §6.5.3「這一列由誰、在哪一步寫」前兩列：Q2（9263 target 不符）與 Q3（9262 capability
// 不足）由 guard 自己在 domain call 之前寫一列 result='FAILED'。那不是瀏覽，是「有人拿
// 著一把 credential 去要不屬於他的資料」。
//
// 反向的四種同樣被鎖住，而且它們才是這支測試的重點：§7.1.2 行 5819／5821 明訂 Q1
// （9270／9271）與 Q3b（9272／9265）**不落**稽核——前者連 intent 都寫不進去（origin=
// interactive 的前置條件正是 session 有效），後者順序上也做不到。少了這半邊，一個「所有
// 拒絕都寫一列」的實作照樣全綠，而那正是 §6.1.14 警告會把安全訊號淹掉的那種寫法。
func TestRequireDelegatedRecordsQ2Q3Denial(t *testing.T) {
	const path = "/g/sellers/2685914682/products/77"
	operable := func() *stubStoreState { return &stubStoreState{resp: operableStore("active")} }

	cases := []struct {
		name      string
		resp      *accountpb.GetCurrentSessionResponse
		stores    *stubStoreState
		wantCode  int32
		wantAudit bool
		// wantCapability 是路由宣告的那一個（products.write），不是該面的 read：
		// §7.1.3「它記的正是『請求了一個不被授予的 capability』這件事本身」。
		wantCapability string
	}{
		{
			name: "Q2 target 不符 → 落一列 FAILED/9263",
			resp: okSession("active", "2632049831", "products.write"), stores: operable(),
			wantCode: bizcode.DelegationTargetMismatch, wantAudit: true, wantCapability: "products.write",
		},
		{
			name: "Q3 capability 不足 → 落一列 FAILED/9262",
			resp: okSession("active", "2685914682", "products.read"), stores: operable(),
			wantCode: bizcode.DelegationCapabilityDenied, wantAudit: true, wantCapability: "products.write",
		},
		{
			name: "Q1 到期（9270）不落",
			resp: okSession("expired", "2685914682", "products.write"), stores: operable(),
			wantCode: bizcode.DelegationSessionExpired,
		},
		{
			name: "Q1 已撤銷（9271）不落",
			resp: okSession("revoked", "2685914682", "products.write"), stores: operable(),
			wantCode: bizcode.DelegationSessionRevoked,
		},
		{
			name:     "Q3b 店家停權（9272）不落",
			resp:     okSession("active", "2685914682", "products.write"),
			stores:   &stubStoreState{resp: operableStore("suspended")},
			wantCode: bizcode.DelegationStoreNotOperable,
		},
		{
			name:     "Q3b 店家查無／已刪除（9265）不落",
			resp:     okSession("active", "2685914682", "products.write"),
			stores:   &stubStoreState{err: grpcstatus.Error(codes.NotFound, "seller not found")},
			wantCode: bizcode.DelegationStoreNotDelegable,
		},
		{
			name: "放行不落（成功的讀取本來就不記，記了會把稽核表淹掉）",
			resp: okSession("active", "2685914682", "products.read", "products.write"), stores: operable(),
			wantCode: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			audit := &stubAudit{}
			r := newGuardEngineWithAudit(&stubSessionResolver{resp: tc.resp}, tc.stores, &stubRevoker{}, audit)
			_, code := doGuardReq(t, r, http.MethodPatch, path)
			if code != tc.wantCode {
				t.Fatalf("業務碼 = %d，預期 %d", code, tc.wantCode)
			}

			if !tc.wantAudit {
				if len(audit.intents) != 0 || len(audit.confirms) != 0 {
					t.Fatalf("不該落稽核，實得 intent %d 列／confirm %d 列（§7.1.2 行 5819／5821）",
						len(audit.intents), len(audit.confirms))
				}
				return
			}

			if len(audit.intents) != 1 || len(audit.confirms) != 1 {
				t.Fatalf("預期恰好 1 對 intent+confirm，實得 %d／%d", len(audit.intents), len(audit.confirms))
			}
			in := audit.intents[0]
			if in.GetAction() != "PRODUCT_READ" {
				t.Errorf("action = %q，預期 PRODUCT_READ（四個 READ action 是拒絕列唯一的合法值）", in.GetAction())
			}
			if in.GetResourceType() != "product" || in.GetResourceId() != "77" {
				t.Errorf("resourceType/resourceId = %q/%q，預期 product/77", in.GetResourceType(), in.GetResourceId())
			}
			if in.GetCapability() != tc.wantCapability {
				t.Errorf("capability = %q，預期 %q（記成該面的 read 會把被拒的寫入低報成被拒的瀏覽）",
					in.GetCapability(), tc.wantCapability)
			}
			if in.GetSessionId() != "2701445588392017" {
				t.Errorf("sessionId = %q，預期 2701445588392017", in.GetSessionId())
			}
			if in.GetOperationReason() != "" || in.GetNotifySeller() {
				t.Errorf("operationReason = %q / notifySeller = %v，兩者都必須是零值（拒絕列沒有使用者輸入，也不得推播給賣家）",
					in.GetOperationReason(), in.GetNotifySeller())
			}
			if in.GetOrigin() != "interactive" {
				t.Errorf("origin = %q，預期 interactive", in.GetOrigin())
			}
			// 三個 transport 欄位：漏掉不會有任何錯誤，只會讓稽核面板對這一類列永遠
			// 顯示「—」，而它們正是最需要回答「誰、從哪裡試的」那一群。
			if in.GetRequestId() != guardTestRequestID || in.GetCorrelationId() != guardTestCorrelationID {
				t.Errorf("requestId/correlationId = %q/%q，預期 %q/%q",
					in.GetRequestId(), in.GetCorrelationId(), guardTestRequestID, guardTestCorrelationID)
			}
			if in.GetUserAgent() != guardTestUserAgent {
				t.Errorf("userAgent = %q，預期 %q", in.GetUserAgent(), guardTestUserAgent)
			}

			cf := audit.confirms[0]
			if cf.GetResult() != "FAILED" || cf.GetBizCode() != tc.wantCode {
				t.Errorf("confirm result/bizCode = %q/%d，預期 FAILED/%d", cf.GetResult(), cf.GetBizCode(), tc.wantCode)
			}
			if cf.GetAuditLogId() != stubAuditLogID {
				t.Errorf("confirm 的 auditLogId = %q，預期 %q（沒對上就是另一列被關掉了）",
					cf.GetAuditLogId(), stubAuditLogID)
			}
		})
	}
}

// 每個面各自的 action／resourceType／resourceId 都要對得上路由。映射寫錯不會有任何
// 錯誤——稽核列照樣落地，只是事後查「他試過讀誰的東西」時指向另一個面。
func TestGuardDenialFaceMatchesRoute(t *testing.T) {
	cases := []struct {
		name         string
		cap          authz.AdminCapability
		relPath      string
		reqPath      string
		wantAction   string
		wantResource string
		wantID       string
	}{
		{"店面", authz.AdminCapStorefrontWrite, "/sellers/:store_id/storefront",
			"/g/sellers/2685914682/storefront", "STOREFRONT_READ", "storefront", ""},
		{"商品", authz.AdminCapProductsPublish, "/sellers/:store_id/products/:product_id/publish",
			"/g/sellers/2685914682/products/77/publish", "PRODUCT_READ", "product", "77"},
		{"貼文", authz.AdminCapPostsWrite, "/sellers/:store_id/posts/:post_id",
			"/g/sellers/2685914682/posts/88", "POST_READ", "post", "88"},
		// 訂單編號不是數字，account-service 的 resourceNameForRow 會把它轉存進
		// resource_name，那一列因此仍指得回那筆訂單。
		{"訂單", authz.AdminCapOrdersRead, "/sellers/:store_id/orders/:order_no",
			"/g/sellers/2685914682/orders/O005001417", "ORDER_READ", "order", "O005001417"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			audit := &stubAudit{}
			r := gin.New()
			// target 不符（session 指向另一家店）⇒ 走 Q2，四個面都到得了。
			deps := delegationDeps{
				resolver: &stubSessionResolver{resp: okSession("active", "2632049831", string(tc.cap))},
				stores:   &stubStoreState{resp: operableStore("active")},
				revoker:  &stubRevoker{},
				audit:    &delegation.Orchestrator{Audit: audit},
			}
			g := r.Group("/g")
			g.Handle(http.MethodPatch, tc.relPath, requireDelegated(tc.cap, deps),
				func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"code": 0}) })

			if _, code := doGuardReq(t, r, http.MethodPatch, tc.reqPath); code != bizcode.DelegationTargetMismatch {
				t.Fatalf("業務碼 = %d，預期 %d", code, bizcode.DelegationTargetMismatch)
			}
			if len(audit.intents) != 1 {
				t.Fatalf("預期恰好 1 列 intent，實得 %d", len(audit.intents))
			}
			in := audit.intents[0]
			if in.GetAction() != tc.wantAction || in.GetResourceType() != tc.wantResource || in.GetResourceId() != tc.wantID {
				t.Fatalf("action/resourceType/resourceId = %q/%q/%q，預期 %q/%q/%q",
					in.GetAction(), in.GetResourceType(), in.GetResourceId(),
					tc.wantAction, tc.wantResource, tc.wantID)
			}
		})
	}
}

// 稽核寫不進去不得改變拒絕本身。拒絕在判定成立的那一刻就已成立；把它改判成別的碼會讓
// 一道正常運作的防線看起來像後端壞掉，而操作者拿到一個對應不到任何處置的錯誤。
func TestGuardDenialSurvivesAuditFailure(t *testing.T) {
	audit := &stubAudit{intentErr: context.DeadlineExceeded}
	r := newGuardEngineWithAudit(
		&stubSessionResolver{resp: okSession("active", "2685914682", "products.read")},
		&stubStoreState{resp: operableStore("active")}, &stubRevoker{}, audit)

	status, code := doGuardReq(t, r, http.MethodPatch, "/g/sellers/2685914682/products/77")
	if status != http.StatusForbidden || code != bizcode.DelegationCapabilityDenied {
		t.Fatalf("HTTP %d / code %d，預期 403 / %d", status, code, bizcode.DelegationCapabilityDenied)
	}
	if len(audit.confirms) != 0 {
		t.Fatalf("intent 失敗後不得 confirm，實得 %d 列", len(audit.confirms))
	}
}

// 未接線的稽核依賴不得讓拒絕變成別的結果：這一項與授權判定不同，它是附帶動作而不是
// 防線本身（deps.audit == nil 只發生在 holder 沒有 account client 的測試組裝上）。
func TestGuardDenialWithoutAuditClientStillDenies(t *testing.T) {
	status, code := doGuardReq(t,
		newGuardEngineWith(&stubSessionResolver{resp: okSession("active", "2632049831", "products.write")},
			&stubStoreState{resp: operableStore("active")}, &stubRevoker{}),
		http.MethodPatch, "/g/sellers/2685914682/products/77")
	if status != http.StatusForbidden || code != bizcode.DelegationTargetMismatch {
		t.Fatalf("HTTP %d / code %d，預期 403 / %d", status, code, bizcode.DelegationTargetMismatch)
	}
}

// credentialHandle 不掛任何 guard：那三支的存在意義就是如實報告工作階段現在是什麼
// 狀態，包含終態。掛上 delegatedHandle 會讓終態摘要再也回不去，前端永遠停在「結果不明」。
func TestCredentialHandleAttachesNoGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	reached := false
	credentialHandle(r.Group("/api/v1/admin"), http.MethodGet, "/delegated-operations/current",
		func(c *gin.Context) { reached = true; c.JSON(http.StatusOK, gin.H{"code": 0}) })

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/delegated-operations/current", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// registry 是 map 且這條路徑本來就在白名單內，重複註冊不改變任何集合，
	// 因此不需要善後。
	if !reached || w.Code != http.StatusOK {
		t.Fatalf("credentialHandle 不得攔下請求：reached=%v status=%d", reached, w.Code)
	}
}

func TestStripDelegationReservedHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.StripDelegationReservedHeaders())
	var seen []string
	r.GET("/x", func(c *gin.Context) {
		for _, h := range middleware.DelegationReservedHeaders {
			if c.GetHeader(h) != "" {
				seen = append(seen, h)
			}
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	for _, h := range middleware.DelegationReservedHeaders {
		req.Header.Set(h, "spoofed")
	}
	r.ServeHTTP(httptest.NewRecorder(), req)

	if len(seen) != 0 {
		t.Fatalf("保留 header 未被刪除：%v", seen)
	}
}

// ── 稽核先於回應：順序本身要有東西守 ────────────────────────────────────────────

// responseWriteOrderRecorder 記下「回應第一次被寫出」時的序號。
//
// 用序號而不是時間戳：兩者可能落在同一個奈秒刻度上，而我們要判的是先後而非間隔。
type responseWriteOrderRecorder struct {
	http.ResponseWriter
	seq       *int
	writtenAt int // 0 ＝ 尚未寫出任何位元組
}

func (w *responseWriteOrderRecorder) mark() {
	if w.writtenAt == 0 {
		*w.seq++
		w.writtenAt = *w.seq
	}
}

func (w *responseWriteOrderRecorder) WriteHeader(code int) {
	w.mark()
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseWriteOrderRecorder) Write(b []byte) (int, error) {
	w.mark()
	return w.ResponseWriter.Write(b)
}

// intentOrderAudit 記下 RecordAuditIntent 被呼叫時的序號，其餘行為沿用 stubAudit。
type intentOrderAudit struct {
	stubAudit
	seq      *int
	intentAt int
}

func (a *intentOrderAudit) RecordAuditIntent(ctx context.Context, req *accountpb.RecordAuditIntentRequest, ms int) (*accountpb.RecordAuditIntentResponse, error) {
	if a.intentAt == 0 {
		*a.seq++
		a.intentAt = *a.seq
	}
	return a.stubAudit.RecordAuditIntent(ctx, req, ms)
}

// Q2／Q3 的拒絕列必須在回應被寫出**之前**寫完。
//
// 為什麼這個順序需要一支測試守：它讀起來像風格偏好，實際上不是。取消 request context
// 的不是 c.Abort()（gin 只把 c.index 設成 abortIndex），而是回應送達後客戶端關掉連線
// ——net/http 會在那一刻取消 Request.Context()。RecordReadDenial 的第一跳
// RecordAuditIntent 正是跑在那個 context 上（只有第二跳 confirm 是 WithoutCancel），
// 所以把兩行對調，這一列就會在「操作者立刻關掉分頁」時消失，而且完全靜默：拒絕照樣
// 回 403，只是那個「有人試圖跨店讀取」的訊號不見了。
//
// 這支測試不量延遲，只量先後——順序是不變式，多付的那兩趟 gRPC 是已知且刻意付的代價。
func TestGuardDenialIsRecordedBeforeTheResponse(t *testing.T) {
	seq := 0
	audit := &intentOrderAudit{seq: &seq}
	// 就地組 engine 而不是用 newGuardEngineWithAudit：那支的 audit 參數是 *stubAudit，
	// 這裡要塞的是包過一層的計序版本。
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("request_id", guardTestRequestID)
		c.Set("trace_id", guardTestCorrelationID)
		c.Next()
	})
	deps := delegationDeps{
		resolver: &stubSessionResolver{resp: okSession("active", "2632049831", "products.write")},
		stores:   &stubStoreState{resp: operableStore("active")},
		revoker:  &stubRevoker{},
		audit:    &delegation.Orchestrator{Audit: audit},
	}
	r.Group("/g").Handle(http.MethodPatch, "/sellers/:store_id/products/:product_id",
		requireDelegated(authz.AdminCapProductsWrite, deps),
		func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"code": 0}) })

	req := httptest.NewRequest(http.MethodPatch, "/g/sellers/2685914682/products/77", nil)
	req.Header.Set(headerOperationCredential, "dopc_x")
	rec := httptest.NewRecorder()
	w := &responseWriteOrderRecorder{ResponseWriter: rec, seq: &seq}
	r.ServeHTTP(w, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("HTTP %d，預期 403", rec.Code)
	}
	if audit.intentAt == 0 {
		t.Fatal("Q2 拒絕沒有落任何稽核 intent —— 這支測試守的順序失去對象")
	}
	if w.writtenAt == 0 {
		t.Fatal("回應從未被寫出，順序無從比較")
	}
	if audit.intentAt > w.writtenAt {
		t.Fatalf("稽核 intent 在回應之後才送出（intent=%d, response=%d）—— "+
			"RecordAuditIntent 跑在 request context 上，客戶端一斷線這一列就沒了",
			audit.intentAt, w.writtenAt)
	}
}
