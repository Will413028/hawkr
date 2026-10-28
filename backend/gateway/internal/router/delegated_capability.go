package router

import (
	"context"
	"sort"
	"strconv"
	"sync"

	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/handler/delegation"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/gateway/internal/response"
	pkgdelegation "github.com/Will413028/hawkr/go-pkg/delegation"
	"github.com/Will413028/hawkr/go-pkg/envelope"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

// headerOperationCredential carries the opaque operation credential minted by S6 and
// redeemed by S6. It is the identity of the www host group in full — those requests
// carry no ADM cookie (host-only, the workspace lives on another origin) and no JWT.
//
// The name itself lives in handler (the S3/S4 handlers read the same header): two
// copies of one header name is the shape that drifts, and the symptom of a drift is
// every workspace request answering 9274 while looking like an expired credential.
const headerOperationCredential = handler.HeaderOperationCredential

// adminNoAuthRegistry records every route mounted on the credential-only sibling of
// the /api/v1/admin group (A31). Since A31 the /api/v1/admin prefix no longer implies
// an admin identity, so "which routes skipped requireAdmin()" has to be a fact the
// process can read back — otherwise a governance endpoint mounted on the wrong group
// variable is an endpoint with NO identity requirement at all, and in review it looks
// exactly like every other /admin/ route.
var (
	adminNoAuthMu       sync.Mutex
	adminNoAuthRegistry = map[string]struct{}{}
)

func registerAdminNoAuth(method, path string) {
	adminNoAuthMu.Lock()
	adminNoAuthRegistry[method+" "+path] = struct{}{}
	adminNoAuthMu.Unlock()
}

// registeredAdminNoAuth returns the credential-only routes, sorted for stable
// panic-message and test output.
func registeredAdminNoAuth() []string {
	adminNoAuthMu.Lock()
	out := make([]string, 0, len(adminNoAuthRegistry))
	for k := range adminNoAuthRegistry {
		out = append(out, k)
	}
	adminNoAuthMu.Unlock()
	sort.Strings(out)
	return out
}

// delegationSessionResolver is the slice of the account gRPC client the guards need
// (an interface so tests can stub it without a live delegation module).
type delegationSessionResolver interface {
	GetCurrentDelegationSession(ctx context.Context, credential string, countsAsActivity bool, deadlineMs int) (*accountpb.GetCurrentSessionResponse, error)
}

// delegationStoreStateResolver is the seller module slice §8.1.3 第 8 項 needs.
type delegationStoreStateResolver interface {
	GetStoreStateForGuard(ctx context.Context, storeID int64, deadlineMs int) (*sellerpb.SellerResponse, error)
}

// delegationSessionRevoker is the delegation module slice used to put the session into
// its terminal state once the target store stops being operable. It is the SAME RPC
// the ADM governance action uses — a system-triggered revocation is told apart from a
// human one by revoke_reason, not by a second endpoint (§7.2.7 ①).
type delegationSessionRevoker interface {
	RevokeDelegationSession(ctx context.Context, req *accountpb.RevokeSessionRequest, deadlineMs int) (*accountpb.RevokeSessionResponse, error)
}

// delegationDeps is what both guards need from the holder, resolved once at mount
// time. The typed-nil normalisation matters: a bare &handler.Holder{} in tests has a
// nil *AccountClient, and assigning that to an interface yields a NON-nil interface
// holding a nil pointer — calling through it would panic instead of failing closed.
type delegationDeps struct {
	resolver             delegationSessionResolver
	stores               delegationStoreStateResolver
	revoker              delegationSessionRevoker
	deadlineMs           int
	storeStateDeadlineMs int
	revokeDeadlineMs     int
	// audit writes the FAILED row for the two denials this guard decides by itself
	// (§6.5.3「這一列由誰、在哪一步寫」的前兩列). It is the same Orchestrator the
	// handlers use — a second implementation of the intent→confirm pair would be a
	// second place for「拒絕列長什麼樣」to drift.
	audit *delegation.Orchestrator
}

func newDelegationDeps(h *handler.Holder) delegationDeps {
	d := delegationDeps{}
	if h != nil {
		if h.Account != nil {
			d.resolver = h.Account
			d.revoker = h.Account
			d.audit = &delegation.Orchestrator{Audit: h.Account, Cfg: h.Delegation}
		}
		if h.Seller != nil {
			d.stores = h.Seller
		}
		d.deadlineMs = h.Delegation.GrpcDeadlineCurrentMs
		d.storeStateDeadlineMs = h.Delegation.GrpcDeadlineStoreStateMs
		// 撤銷沿用 CreateMs（§7.3.1：本項命中時多付的那一次 RevokeSession 不另立參數）。
		d.revokeDeadlineMs = h.Delegation.GrpcDeadlineCreateMs
	}
	return d
}

// delegationDenyMessage is the human-readable half of a delegated denial. The front
// end never shows these (§6.1.0: message is replaced client-side, only code and
// traceId survive) — they exist for logs and for anyone reading a raw response, which
// is also why every distinct failure has its own numeric code rather than its own
// sentence.
func delegationDenyMessage(code int32) string {
	switch code {
	case bizcode.DelegationCredentialInvalid:
		return "代行憑證無效，請重新建立工作階段"
	case bizcode.DelegationSessionExpired:
		return "代行工作階段已到期"
	case bizcode.DelegationSessionRevoked:
		return "代行工作階段已撤銷"
	case bizcode.DelegationTargetMismatch:
		return "此工作階段的代行對象不是這家店家"
	case bizcode.DelegationCapabilityDenied:
		return "此工作階段未取得執行該操作的能力"
	case bizcode.DelegationStoreNotOperable:
		return "此店家已停權或關閉，無法代行"
	case bizcode.DelegationStoreNotDelegable:
		return "此店家目前不可代行"
	case bizcode.DelegationUpstreamUnavailable:
		return "代行服務暫時不可用，請稍後再試"
	default:
		return "代行請求遭拒"
	}
}

// resolveDelegationSession turns the request's credential into a session snapshot.
//
// countsAsActivity is fixed by the caller — i.e. by WHICH guard, i.e. by which
// middleware the route was mounted under. It is never read from the request: a
// browser-controllable idle flag is the same thing as no idle timeout at all.
//
// A resolver that is not wired fails closed with 9280 rather than passing through:
// an unwired authorization dependency must never read as "authorized".
func resolveDelegationSession(c *gin.Context, deps delegationDeps, countsAsActivity bool) (*accountpb.OperationSessionSummary, bool) {
	credential := c.GetHeader(headerOperationCredential)
	// 讀完就從請求上拿掉：credential 只在 Gateway 這一跳有意義，往下游轉發等於
	// 讓每個領域服務都持有一把可直接冒用的憑證，而它們沒有任何理由需要它。
	c.Request.Header.Del(headerOperationCredential)
	if credential == "" {
		response.AbortBizStatus(c, bizcode.DelegationCredentialInvalid, delegationDenyMessage(bizcode.DelegationCredentialInvalid))
		return nil, false
	}
	// 9289 而不是 9280（#2744）：未接線是永久失敗，重試不會好。
	if deps.resolver == nil {
		zap.L().Error("Delegation.dependency.unwired", zap.String("dep", "resolver"))
		response.AbortBizStatus(c, bizcode.DelegationInternalContractMismatch, delegationDenyMessage(bizcode.DelegationInternalContractMismatch))
		return nil, false
	}

	resp, err := deps.resolver.GetCurrentDelegationSession(c.Request.Context(), credential, countsAsActivity, deps.deadlineMs)
	if err != nil {
		// 先問一句：這個 err 是「上游真的不可用」，還是「上游明確拒絕了這把憑證」？
		//
		// delegation 模組對無效憑證回的是 gRPC OK ＋ 非零 envelope code（9274 等），
		// 而 EnvelopeGuardInterceptor 會把非零碼轉成 gRPC error 並在 message 前面加上
		// "[9274] " 前綴（gateway/internal/grpc/envelope_guard_interceptor.go:104）。
		// 少了這一步還原，下面那個 resp.GetCode() != 0 分支永遠進不去（碼在到達前就
		// 變成 err 了），憑證無效會一律回 9280/424。
		//
		// 那個差別對使用者是實質的：424「代行服務暫時不可用，請稍後再試」會讓前端
		// 顯示可重試的錯誤，而重試永遠不會好——憑證不會因為再試一次就變有效。
		// 401 才會把人導回 ADM 重新建立工作階段。
		if code, bizMsg := envelope.ParseBizMessage(grpcstatus.Convert(err).Message()); code > 0 {
			if bizMsg == "" {
				bizMsg = delegationDenyMessage(code)
			}
			response.AbortBizStatus(c, code, bizMsg)
			return nil, false
		}
		// 沒有業務碼前綴 ⇒ 真的是上游問不到答案 → 424 + 9280，不得用 500／502／503：
		// 前端把 503 與 404 讀成「此環境尚未部署代行營運服務」並永久隱藏入口（C12），
		// 而任何 5xx 的 body 都會被 Cloudflare 換掉、業務碼根本到不了前端。
		response.AbortBizStatus(c, bizcode.DelegationUpstreamUnavailable, delegationDenyMessage(bizcode.DelegationUpstreamUnavailable))
		return nil, false
	}
	// 保留這一段：interceptor 的 envelopeCodePreserveData 白名單內的碼會以 gRPC OK
	// ＋非零 code 直接抵達（不轉 error），那條路徑仍走這裡。
	if resp.GetCode() != 0 {
		msg := resp.GetMessage()
		if msg == "" {
			msg = delegationDenyMessage(resp.GetCode())
		}
		response.AbortBizStatus(c, resp.GetCode(), msg)
		return nil, false
	}
	data := resp.GetData()
	if data == nil {
		response.AbortBizStatus(c, bizcode.DelegationCredentialInvalid, delegationDenyMessage(bizcode.DelegationCredentialInvalid))
		return nil, false
	}
	return data, true
}

// revokeReasonStoreNotOperable is the fixed literal that tells an auditor why a
// session ended (§8.1.3 第 8 項 / §12.4 ①). The other sources of the same terminal
// state — admin_logout, delegation_disabled, a human revocation carrying a ticket
// number — are told apart by exactly this column, so the value is not free text.
//
// 值取自 pkgdelegation 的封閉字面值集合，不在這裡手抄第二份：這一欄的判讀是跨服務的
// （Gateway 寫 store_not_operable、delegation 模組寫 authorization_revoked），
// 各寫各的字面值就會出現只差一個字的兩種值而稽核查詢完全看不出來。
const revokeReasonStoreNotOperable = pkgdelegation.RevokeReasonStoreNotOperable

// enforceStoreOperable is §8.1.3 第 8 項: the target store must still be operable on
// EVERY delegated resource request, reads included.
//
// Reads are in scope on purpose. Excluding them would leave the one window that
// §8.1.5 exists to close — an operator who can no longer write, but can still pull
// the recipient contact details on a deleted store's orders (orders.read is one of
// the nine capabilities).
//
// 「確知不可代行」與「問不到答案」走兩條不同的路，這是本項最容易寫錯的地方：
//   - 確知（seller 模組明確回覆非 active，或該店查無／已關閉）→ 拒絕 ＋ 轉終態
//   - 不知道（逾時／UNAVAILABLE）→ 拒絕，但工作階段**維持原狀**
//
// 轉終態是不可逆的，而 seller 模組的一次抖動是暫時的；把「問不到」也拿去轉終態，
// 一次 30 秒的不可用就會把當下所有進行中的工作階段永久打死。fail-closed 要求的是
// 「這次請求不准通過」，不是「把授權銷毀」。
//
// ⚠️ 刻意不加快取。停權是即時的風險控制，而快取的每一秒都是「治理端已按下停權、平台
// 帳號仍在替該店寫入」的窗口；46 號那份 seller:suspended Redis 快取更是 fail-open、
// 值域只涵蓋手動停權、且以 accountId 為鍵（§8.1.3 第 8 項的三條否決理由）。這一跳的
// 預算是 DELEGATION_GRPC_DEADLINE_STORE_STATE_MS（800 ms），已計入 §7.3.1 的兩式總
// 預算，由 config.validateDelegation() 在啟動時斷言。
func enforceStoreOperable(c *gin.Context, deps delegationDeps, session *accountpb.OperationSessionSummary, storeID int64) bool {
	// 未接線的授權依賴一律 fail closed：一個問不到的判定不得讀成「已授權」。
	if deps.stores == nil {
		zap.L().Error("Delegation.dependency.unwired", zap.String("dep", "stores"))
		response.AbortBizStatus(c, bizcode.DelegationInternalContractMismatch, delegationDenyMessage(bizcode.DelegationInternalContractMismatch))
		return false
	}

	resp, err := deps.stores.GetStoreStateForGuard(c.Request.Context(), storeID, deps.storeStateDeadlineMs)

	var denied int32
	switch {
	case err != nil:
		if !storeStateIsBusinessVerdict(err) {
			pkglogger.Warn(c.Request.Context(), "delegation.store_state.unavailable",
				zap.Int64("store_id", storeID), zap.Error(err))
			response.AbortBizStatus(c, bizcode.DelegationUpstreamUnavailable, delegationDenyMessage(bizcode.DelegationUpstreamUnavailable))
			return false
		}
		denied = bizcode.DelegationStoreNotDelegable
	case resp.GetData() == nil:
		denied = bizcode.DelegationStoreNotDelegable
	default:
		// deleted 恆傳 false：查得到的列必然未軟刪，上面兩個分支已經吃掉刪除的情況。
		if ok, code := pkgdelegation.StoreOperable(resp.GetData().GetLifecycleStatus(), false); !ok {
			denied = code
		}
	}
	if denied == 0 {
		return true
	}

	revokeForStoreNotOperable(c, deps, session)
	response.AbortBizStatus(c, denied, delegationDenyMessage(denied))
	return false
}

// storeStateIsBusinessVerdict tells "the seller module answered" apart from
// "the seller module could not be reached", which on this hop is NOT the same as
// err == nil: EnvelopeGuardInterceptor turns every non-zero envelope code into a
// gRPC error, so a business rejection and a transport failure both arrive as err.
// Reading err alone as "unknown" would make a deleted store answer 9280 forever and
// never revoke the session.
//
//	NotFound           查無此店（lookup 濾 deleted_at ⇒ 軟刪的店走這裡）
//	FailedPrecondition envelope 業務碼的預設映射（已關店的 4040 走這裡）
//
// Everything else — Unavailable / DeadlineExceeded / Canceled / Internal — is "we do
// not know", and 不知道的事不得拿去轉終態.
func storeStateIsBusinessVerdict(err error) bool {
	switch grpcstatus.Code(err) {
	case codes.NotFound, codes.FailedPrecondition:
		return true
	default:
		return false
	}
}

// revokeForStoreNotOperable puts the session into its terminal state so that the two
// things a bare rejection cannot fix do get fixed: the workspace still rendering
// 「進行中」 (its GET current poll keeps reading active), and the posts this session
// scheduled still going out at their appointed time (§7.6.4).
//
// Failure here does NOT change the outcome of the request. The rejection was already
// decided; revocation only lets the UI and the scheduler catch up. Turning a correctly
// refused request into a 5xx because the follow-up call failed would make a working
// defence look like a broken backend — and the next request re-runs both halves.
func revokeForStoreNotOperable(c *gin.Context, deps delegationDeps, session *accountpb.OperationSessionSummary) {
	if deps.revoker == nil {
		// 這一支刻意不回錯（見上方註解：連帶撤銷失敗不得把一次成功的防禦講成 5xx），
		// 但「未接線」與「呼叫失敗」是兩件事，前者從來沒有任何訊號 —— 補一行讓它查得到。
		zap.L().Error("Delegation.dependency.unwired", zap.String("dep", "revoker"))
		return
	}
	sessionID := session.GetSessionId()
	resp, err := deps.revoker.RevokeDelegationSession(c.Request.Context(), &accountpb.RevokeSessionRequest{
		SessionId: sessionID,
		// 填當前這位 ADM 自己，與「ADM 登出連動」完全同形；系統觸發的撤銷因此不需要
		// 另外定義一種空 actor 語意（§8.1.3 第 8 項）。
		ActorAdminId:   session.GetActor().GetAdminId(),
		RevokeReason:   revokeReasonStoreNotOperable,
		IdempotencyKey: revokeReasonStoreNotOperable + ":" + sessionID,
		RequestId:      c.GetString("request_id"),
	}, deps.revokeDeadlineMs)
	if err != nil {
		pkglogger.Warn(c.Request.Context(), "delegation.store_not_operable.revoke_failed",
			zap.String("session_id", sessionID), zap.Error(err))
		return
	}
	if resp.GetCode() != 0 {
		pkglogger.Warn(c.Request.Context(), "delegation.store_not_operable.revoke_denied",
			zap.String("session_id", sessionID), zap.Int32("code", resp.GetCode()))
	}
}

// recordGuardDenial writes the FAILED audit row for the two refusals this guard
// decides on its own: Q2 (target mismatch, 9263) and Q3 (capability denied, 9262).
// §6.5.3 assigns those two to the guard explicitly — they are settled before any
// domain call, so there is no orchestrator sequence to hang them off.
//
// 這兩碼是全部，不是「先做兩個」：
//   - Q1（9270 到期／9271 撤銷）與 Q3b（9272／9265 店家不可營運）依 §7.1.2 行 5819、
//     5821 明訂**不落**稽核，而且順序上也寫不進去——`origin=interactive` 的 intent
//     前置條件正是「session 有效」，已終止的工作階段連 intent 都會被打回。
//   - 反向的界線同樣硬：§6.1.14 行 4091 禁止「打一支必然 403 的端點來探測」，因為那會
//     製造一批 READ ＋ FAILED 的列，把「有人試圖跨店讀取」這個安全訊號淹掉。這四個
//     action 存在的唯一理由就是留下那個訊號，多寫一列就是稀釋它一次。
//
// 失敗不改變結果：拒絕在判定成立的那一刻就已成立，稽核寫不進去只留 log。把一道正常
// 運作的防線因為附帶動作失敗而改判成別的碼，會讓 403 變成一個對應不到任何處置的錯誤。
func recordGuardDenial(c *gin.Context, deps delegationDeps, session *accountpb.OperationSessionSummary, cap authz.AdminCapability, denied int32) {
	if denied != bizcode.DelegationTargetMismatch && denied != bizcode.DelegationCapabilityDenied {
		return
	}
	if deps.audit == nil {
		return
	}
	face := delegatedFace(cap)
	if face == "" {
		return
	}
	// ⚠️ 這一列落在 session 自己的 target store 名下（delegation 模組以
	// session.TargetStoreID 填列），不是 URL 上那家被試探的店。記的是「這個工作階段想
	// 去別家」，看報表時別把它讀成「那家店被動了手腳」。
	deps.audit.RecordReadDenial(c.Request.Context(), delegation.ReadDenial{
		Session:      session,
		ResourceType: face,
		ResourceID:   guardResourceID(c, face),
		// 記路由宣告的那一個，不是該面的 read：§7.1.3 明寫拒絕列「記的正是『請求了一個
		// 不被授予的 capability』這件事本身」。記成 products.read 會把一次被拒的寫入
		// 低報成一次被拒的瀏覽。
		Capability:    cap,
		BizCode:       denied,
		RequestID:     c.GetString("request_id"),
		CorrelationID: c.GetString("trace_id"),
		UserAgent:     c.GetHeader("User-Agent"),
	})
}

// delegatedFace maps the route's declared capability to the audit row's resource
// type. The nine capabilities cover exactly the four faces; an unknown token yields
// "" and the row is skipped rather than written with a resource_type the CHECK
// constraint would reject.
func delegatedFace(cap authz.AdminCapability) string {
	switch cap {
	case authz.AdminCapStorefrontRead, authz.AdminCapStorefrontWrite:
		return "storefront"
	case authz.AdminCapProductsRead, authz.AdminCapProductsWrite, authz.AdminCapProductsPublish:
		return "product"
	case authz.AdminCapPostsRead, authz.AdminCapPostsWrite, authz.AdminCapPostsPublish:
		return "post"
	case authz.AdminCapOrdersRead:
		return "order"
	default:
		return ""
	}
}

// guardResourceID is the resource named in the URL, empty on the collection routes
// (list / create / batch) where the request names no single resource.
//
// 店面沒有自己的 id：它就是那家店，而 store_id 已在 session 快照上。訂單編號不是數字，
// delegation 模組的 resourceNameForRow 會把它轉存進 resource_name——那一列因此仍指
// 得回那筆訂單。
func guardResourceID(c *gin.Context, face string) string {
	switch face {
	case "product":
		return c.Param("product_id")
	case "post":
		return c.Param("post_id")
	case "order":
		return c.Param("order_no")
	default:
		return ""
	}
}

// requireDelegated is the guard on the 20 delegated resource routes. capability is a
// REQUIRED parameter, so a new delegated route cannot compile without declaring one
// — the compile-time half of deny-by-default, with the boot assertion as the backstop.
//
// It covers §8.1.3 items 1, 2, 3, 5, 6 and 8; item 4 is a permanently reserved空號
// since A31 (a request on the www host carries no second identity, so comparing the
// credential against something derived from the credential would be an identity, not
// a check). Item 7 (授權來源仍有效) needs a field the session snapshot does not carry
// and is not decided here.
//
// Every request through this guard counts as operator activity — reads included.
// Excluding reads would mean "browse for 16 minutes, then hit save" ends in a kick.
func requireDelegated(cap authz.AdminCapability, deps delegationDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		session, ok := resolveDelegationSession(c, deps, true)
		if !ok {
			return
		}

		// 目標比對的對象是工作階段快照的 target_store_id，不是 resolver 的結果——
		// 代行路徑刻意完全不走 storeid resolver（§8.1.4(c)）。
		targetStoreID, _ := strconv.ParseInt(session.GetTarget().GetStoreId(), 10, 64)
		requestedStoreID, _ := strconv.ParseInt(c.Param("store_id"), 10, 64)

		caps := make([]authz.AdminCapability, 0, len(session.GetCapabilities()))
		for _, s := range session.GetCapabilities() {
			caps = append(caps, authz.AdminCapability(s))
		}

		if _, denied := authz.GrantedAdminCapabilities(authz.AdminDelegationContext{
			SessionStatus:      session.GetStatus(),
			Capabilities:       caps,
			TargetStoreID:      targetStoreID,
			RequestedStoreID:   requestedStoreID,
			RequiredCapability: cap,
		}); denied != 0 {
			// 稽核先寫、再回應。⚠️ 取消 context 的**不是** c.Abort()——gin 的 Abort
			// 只把 c.index 設成 abortIndex，request context 一個字都沒動，它後面的
			// 程式照跑。真正會取消它的是「回應送達之後客戶端把連線關掉」（net/http
			// 在 client 斷線時取消 Request.Context()）與 ServeHTTP 返回。
			//
			// 而 RecordReadDenial 的第一跳 RecordAuditIntent 用的正是本次請求的
			// context——只有第二跳 confirm 是 WithoutCancel。先回 403 再寫，等於把這
			// 一列的存活押在「操作者不會立刻關掉分頁或中止請求」上，而症狀完全靜默：
			// 拒絕照樣回 403，只是那一列不見了，而那正是它存在的唯一理由（有人試圖
			// 跨店讀取的訊號）。
			//
			// 代價也要寫出來：這個順序讓 403 在答覆前多付兩趟同步 gRPC（intent ＋
			// confirm）。刻意付——一次被拒的請求慢幾十毫秒，換稽核訊號不漏。
			// 順序由 TestGuardDenialIsRecordedBeforeTheResponse 釘住。
			recordGuardDenial(c, deps, session, cap, denied)
			response.AbortBizStatus(c, denied, delegationDenyMessage(denied))
			return
		}

		// 第 8 項排在最後：前面幾項只讀 session 快照與記憶體 registry，這一項是本鏈上
		// 第一次跨服務往返，排在最後才問最省。它同時必須在 before 快照與
		// RecordAuditIntent 之前（§7.1.2）——本 guard 在 handler 之前執行即滿足。
		if !enforceStoreOperable(c, deps, session, targetStoreID) {
			return
		}

		middleware.SetDelegationSession(c, session)
		c.Next()
	}
}

// requireDelegatedSessionOnly guards the two media upload-session endpoints
// (§6.1.13): credential valid and session not terminal, nothing else. They carry no
// capability (uploading only inserts a row in media's own table and touches no target
// store state — what actually needs authorizing is attaching the image, which
// storefront.write / products.write / posts.write already guard) and no :store_id to
// compare against.
//
// "不驗 capability" and "算不算活動" are two different things: these routes still
// count as activity. Swapping a batch of product images is the single easiest way to
// sit past the idle window while touching no delegatedHandle route at all — every
// step of it (picking files, cropping, uploading straight to R2) skips the Gateway.
// Getting this wrong is silent: the operator is kicked out at save time with nothing
// having warned them.
func requireDelegatedSessionOnly(deps delegationDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		session, ok := resolveDelegationSession(c, deps, true)
		if !ok {
			return
		}
		if denied := authz.AdminSessionDenial(session.GetStatus()); denied != 0 {
			response.AbortBizStatus(c, denied, delegationDenyMessage(denied))
			return
		}
		middleware.SetDelegationSession(c, session)
		c.Next()
	}
}

// ── 掛載 helper（四個，§6.1.0）────────────────────────────────────────────────
//
// adminHandle          只要 requireAdmin()，不掛 capability guard
// credentialHandle     不掛 requireAdmin()；credential 由 handler 自行讀取與解析
// delegatedHandle      不掛 requireAdmin()；解析 credential ＋ 驗 capability／target／到期
// sessionOnlyHandle    掛 requireDelegatedSessionOnly()（媒體兩支）

// 五個 helper 一律呼叫 middleware.RegisterDelegatedRoute：限流命中時要回代行專用的
// 9273（§6.1.0 行 2317）而不是裸 429，而限流跑在所有 handler 之前，唯一問得到的問題是
// 「c.FullPath() 是不是代行路由」。登錄制與 capability 登錄同理由——20 支資源端點掛在
// /sellers/:store_id/ 這一段，而同一段也可以掛非代行的端點；任何前綴規則都分不出兩者，
// 會多吃非代行的、或漏吃形狀不符前綴的代行端點，而兩種錯法都靜默（照樣回 429，
// 只是業務碼不對）。

// adminHandle mounts an ADM-host route on the group that already carries JWTAuth +
// requireAdmin. Nothing else to do: those five endpoints are authenticated by the
// ADM session and declare no capability, because deciding whether a capability can be
// granted at all is precisely what preflight and create are for.
func adminHandle(g *gin.RouterGroup, method, relPath string, h gin.HandlerFunc) {
	middleware.RegisterDelegatedRoute(method, ginJoinPath(g.BasePath(), relPath))
	g.Handle(method, relPath, h)
}

// credentialHandle mounts a route that reads the credential ITSELF rather than being
// gated on it (GET current / DELETE current / GET current/summary).
//
// These three must NOT use delegatedHandle: that guard fails closed on expiry and
// revocation, so mounting them there means the terminal summary can never be returned
// and the workspace is stuck at "outcome unknown" forever; DELETE would be refused
// with 9274 and latch the workspace closed even after the dialog is dismissed. They
// must not use adminHandle either — they are issued by the www host BFF, which has no
// ADM session, so every 30-second poll would come back 9261.
//
// counts_as_activity for all three is false. GET current is polled every 30 seconds
// plus on focus and reconnect: that is the browser's heartbeat, not the person's.
// Counting it would mean the idle window never closes.
func credentialHandle(g *gin.RouterGroup, method, relPath string, h gin.HandlerFunc) {
	full := ginJoinPath(g.BasePath(), relPath)
	registerAdminNoAuth(method, full)
	middleware.RegisterDelegatedRoute(method, full)
	g.Handle(method, relPath, h)
}

// delegatedHandle mounts one of the 20 delegated resource routes: it registers
// (method, fullPath, cap) in the admin authz registry — declaration and mounting are
// one act — and prepends requireDelegated(cap).
func delegatedHandle(g *gin.RouterGroup, method, relPath string, cap authz.AdminCapability, deps delegationDeps, h gin.HandlerFunc) {
	full := ginJoinPath(g.BasePath(), relPath)
	authz.RegisterAdmin(method, full, cap)
	registerAdminNoAuth(method, full)
	middleware.RegisterDelegatedRoute(method, full)
	g.Handle(method, relPath, requireDelegated(cap, deps), h)
}

// sessionOnlyHandle mounts a route behind requireDelegatedSessionOnly. It stays OUT
// of the admin authz registry on purpose — the registry is the set of routes that
// declared a capability, and the boot assertions read it as exactly that.
func sessionOnlyHandle(g *gin.RouterGroup, method, relPath string, deps delegationDeps, h gin.HandlerFunc) {
	full := ginJoinPath(g.BasePath(), relPath)
	registerAdminNoAuth(method, full)
	middleware.RegisterDelegatedRoute(method, full)
	g.Handle(method, relPath, requireDelegatedSessionOnly(deps), h)
}
