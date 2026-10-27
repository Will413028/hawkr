package handler

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	grpcstatus "google.golang.org/grpc/status"
)

// 56 號 ADM 代行營運的六支工作階段端點（S1–S6）。
//
// 三種身分來源，一支端點只屬於一組（§6.1.0）：
//
//	S1 preflight／S2 create／S5 revoke  ADM session（既有 requireAdmin），失效 → 9261 / 401
//	S3 GET current／S4 DELETE current   operation credential，handler 自己解析
//	S6 handoff redeem                  無身分，授權完全由一次性 handoff code 承擔
//
// 9261 與 9274 永遠不會同時出現在同一支端點上。把 9261 掛到 www host 組的症狀是
// 代行工作區的每一次請求都 401、而 ADM 那邊完全正常。
//
// ⚠️ 內部錯誤一律 424 ＋ 9280。503 與 404 被前端讀成「此環境尚未部署代行營運服務」
// 並讓入口永久消失；裸 500 沒有業務碼，前端分不出可否重試。**也不得用 5xx**：
// Cloudflare 會把 origin 5xx 的 body 換成自己的錯誤頁，前端連 9280 都收不到
// （推導與實測見 internal/response/bizhttp.go 的登錄表註解）。

const (
	// HeaderOperationCredential 是 www host 組的身分本體：那些請求沒有 ADM cookie
	// （host-only，工作區在另一個 origin）也沒有 JWT。
	//
	// Exported 是因為 router 的三支 guard 讀的是同一個 header：兩個 package 各存一份
	// 字串就是「兩份必然漂移」的形狀，而漂移的症狀是工作區每一次請求都 9274，看起來
	// 卻像憑證失效。
	HeaderOperationCredential = "X-HK-Operation-Session"
	headerIdempotencyKey      = "Idempotency-Key"
)

var (
	// §4.4.2：所有 mutation 的冪等鍵格式。不符即「參數格式錯」→ 9260。值域與 gateway
	// 其餘 mutation 共用同一份（`middleware.IdempotencyKeyPattern`，58 號
	// §6.1.0(10) 定版）——這裡不掛 middleware.RequireIdempotencyKey：本路徑的違規
	// 回應是 9260 而非 400，換掉會改變代行的對外契約，但正則只能有一份。
	delegationIdempotencyKeyRE = middleware.IdempotencyKeyPattern
	// sessionId 是 TSID 數字字串。
	delegationSessionIDRE = regexp.MustCompile(`^\d{1,19}$`)
	// handoff code：`dhof_` ＋ 43 字 base64url ＝ 48 字（§8.1.6）。前綴刻意與
	// credential 的 `dops_` 不同，兩者在除錯時才分得出來。
	delegationHandoffCodeRE = regexp.MustCompile(`^dhof_[A-Za-z0-9_-]{43}$`)
)

// ==================== 回應 DTO ====================
//
// 代行 DTO 一律是獨立的 exported struct，不共用 /app/* 的 DTO，也不把 proto struct
// 直接丟進 data：protoc-gen-go 的 tag 是 snake_case ＋ omitempty，直出即欄名全錯、
// 且 false／0／null 整個欄位消失。

// DelegationTargetData 是代行對象店家。
type DelegationTargetData struct {
	StoreID string `json:"storeId"`
	// SellerAccountID 是該店家的登入 principal（accounts PK），不是營運鍵。
	// 名稱由 56 號契約定版，前端據此比對 preflight ↔ create ↔ current 三者。
	SellerAccountID string `json:"sellerAccountId"`
	StoreName       string `json:"storeName"`
}

// DelegationActorData 是執行代行的管理員。
type DelegationActorData struct {
	AdminID     string `json:"adminId"`
	DisplayName string `json:"displayName"`
}

// DelegationReasonData 是本次代行的事由（工作階段快照內的值）。
type DelegationReasonData struct {
	Code                  string `json:"code"`
	Label                 string `json:"label"`
	CaseReferenceRequired bool   `json:"caseReferenceRequired"`
}

// DelegationReasonOptionData 是建立 modal 的事由選項。
type DelegationReasonOptionData struct {
	Code                  string `json:"code"`
	Label                 string `json:"label"`
	CaseReferenceRequired bool   `json:"caseReferenceRequired"`
}

// DelegationPreflightData 是 S1 的回應。
//
// 拒絕一律走 200 ＋ allowed:false（不是 403），形狀與允許時完全相同、多 deniedCode
// 與 deniedLabel 兩欄——讓前端的 !ok 分支只承載真正的傳輸／服務錯誤。
//
// ⚠️ allowed:false 時 reasonOptions 與 capabilities 仍必須非空：前端在進入 allowed
// 分支之前就對這兩欄做非空校驗，空陣列會讓整份回應被判契約錯誤（502）並連帶把
// deniedCode 一起丟掉，操作者只看得到「代行不可用」而看不到真正原因。
type DelegationPreflightData struct {
	Allowed       bool                         `json:"allowed"`
	Target        DelegationTargetData         `json:"target"`
	Actor         DelegationActorData          `json:"actor"`
	ReasonOptions []DelegationReasonOptionData `json:"reasonOptions"`
	Capabilities  []string                     `json:"capabilities"`
	// TTLSeconds／IdleSeconds 是 §4「數字對外一律 string」的值域安全例外（§6.1.6）：
	// JSON number，前端以 Number.isSafeInteger 驗證後直接參與時間計算。送 string 即 502。
	TTLSeconds int32 `json:"ttlSeconds"`
	// IdleSeconds 是設定值的下發（DELEGATION_SESSION_IDLE_SECONDS），供建立 modal 渲染
	// 「閒置 N 分鐘後自動結束」。單位是秒不是分；前端不得寫死。
	IdleSeconds int32 `json:"idleSeconds"`
	// CaseReferenceRequired 是 OR：任一 reasonOptions[i].caseReferenceRequired 為 true。
	// 它回答「這張表單有沒有可能需要案件編號」，不是「這一次一定要填」。
	CaseReferenceRequired bool `json:"caseReferenceRequired"`
	// 以下兩欄只在 allowed:false 時出現。
	DeniedCode  int32  `json:"deniedCode,omitempty"`
	DeniedLabel string `json:"deniedLabel,omitempty"`
}

// DelegationSessionData 是工作階段的對外唯一投影（SessionSummary），S2 create 與 S3
// GET current 共用同一份 parser。
//
// ⚠️ 建立流程結束後瀏覽器會再抓一次 current，與 create 的回應做九欄全等比對
// （target.storeId／target.sellerAccountId／actor.adminId／reason.code／caseReference／
// issuedAt／expiresAt／status／revision，外加 capabilities 集合相等）。任一欄漂移即
// 「目前工作階段與剛建立的稽核上下文不相符」，操作者永遠進不了工作區。
//
// ⚠️ operationCredential、任何 seller token／access token／refresh token 絕不得出現
// 在這個型別內。
type DelegationSessionData struct {
	SessionID   string               `json:"sessionId"`
	DisplayCode string               `json:"displayCode"`
	Target      DelegationTargetData `json:"target"`
	Actor       DelegationActorData  `json:"actor"`
	Reason      DelegationReasonData `json:"reason"`
	// CaseReference 未設定時必須輸出 JSON null。**不得加 omitempty**：欄位消失
	// （undefined）即 502，操作者進不了工作區。這是全案最容易被序列化器預設值吃掉
	// 的一欄，contract test 專門守它。
	CaseReference *string  `json:"caseReference"`
	Capabilities  []string `json:"capabilities"`
	IssuedAt      string   `json:"issuedAt"`
	ExpiresAt     string   `json:"expiresAt"`
	// Status 恰三值：active / expired / revoked（小寫）。hard 到期與閒置逾時共用
	// expired，前端不需要也不得區分。
	Status string `json:"status"`
	// Revision 同為 §4 的值域安全例外：JSON number，安全整數 ≥ 0。
	Revision int64 `json:"revision"`
}

// DelegationCreateData 是 S2 的回應：handoffCode 與 session。
//
// handoffCode 不是 session 的欄位（`GET current` 根本不回它），因此不進九欄對帳。
// 未兌換的冪等重放改簽 code；已兌換的重放只回 session。
//
// ⚠️ handoffCode 是明文憑證，不得進 log／Sentry／metric label。
type DelegationCreateData struct {
	HandoffCode string                `json:"handoffCode,omitempty"`
	Session     DelegationSessionData `json:"session"`
}

// DelegationEndData 是 S4 的回應，恰為 {"ended": true}，不得附加任何其他 key。
// 結束摘要的數字另走 GET .../current/summary。
type DelegationEndData struct {
	Ended bool `json:"ended"`
}

// DelegationRevokeData 是 S5 的回應。
type DelegationRevokeData struct {
	SessionID string `json:"sessionId"`
	Status    string `json:"status"`
	Revision  int64  `json:"revision"`
	// RevokedAt 冪等重放時回第一次撤銷的時刻，不是 now()：撤銷時刻是稽核舉證欄，
	// 同一個工作階段不能有兩個。
	RevokedAt string `json:"revokedAt"`
}

// DelegationRedeemData 是 S6 的回應：credential ＋ 與 GET current 完全相同的 summary。
// summary 放在 session 這一個 key 之下，不攤平——攤平會讓 website 端需要第二份 parser。
type DelegationRedeemData struct {
	OperationCredential string                `json:"operationCredential"`
	Session             DelegationSessionData `json:"session"`
}

// ==================== 請求 DTO ====================

// CreateDelegatedOperationRequest 是 S2 的 body。
type CreateDelegatedOperationRequest struct {
	StoreID    string `json:"storeId"`
	ReasonCode string `json:"reasonCode"`
	// CaseReference 用 RawMessage 才分得出「顯式 null」與「整個欄位缺席」：*string
	// 兩者都是 nil。契約要求顯式送 null，缺席即 9260。
	CaseReference json.RawMessage `json:"caseReference"`
	Capabilities  []string        `json:"capabilities"`
	// TTLSeconds 是 JSON number（值域安全例外）。指標型別讓「沒送」與「送 0」可分，
	// 而 0 是非法值要回 9260。
	TTLSeconds *int32 `json:"ttlSeconds"`
}

// RevokeDelegatedOperationRequest 是 S5 的 body。
type RevokeDelegatedOperationRequest struct {
	RevokeReason string `json:"revokeReason"`
}

// RedeemDelegationHandoffRequest 是 S6 的 body。
type RedeemDelegationHandoffRequest struct {
	HandoffCode string `json:"handoffCode"`
}

// ==================== 共用小工具 ====================

// delegationLocalMessage 只涵蓋 Gateway 自己產生的拒絕碼。上游帶回來的 message 一律
// 原樣透傳，避免同一個碼在兩個地方各有一份會漂移的文案。
//
// 這些字串使用者永遠看不到（前端把上游 message 換成固定字串，只留 code 與 traceId），
// 它們的讀者是 log 與直接讀原始回應的人——這也正是每個失敗情境都必須有唯一數字碼、
// 不得用 message 區分語意的理由。
func delegationLocalMessage(code int32) string {
	switch code {
	case bizcode.DelegationInvalidCommand:
		return "代行請求參數不正確"
	case bizcode.DelegationAdminSessionRequired:
		return "管理員登入已失效，請重新登入"
	case bizcode.DelegationCredentialInvalid:
		return "代行憑證無效，請重新建立工作階段"
	case bizcode.DelegationSessionNotFound:
		return "找不到指定的代行工作階段"
	case bizcode.DelegationHandoffInvalid:
		return "此代行連結已失效，請回管理後台重新開始"
	case bizcode.DelegationUpstreamUnavailable:
		return "代行營運服務暫時無法使用，請稍後再試"
	default:
		return "代行請求遭拒"
	}
}

// delegationDeny 送出 Gateway 自己判定的拒絕。
func delegationDeny(c *gin.Context, code int32, detail string) {
	msg := detail
	if msg == "" {
		msg = delegationLocalMessage(code)
	}
	response.BizStatus(c, code, msg)
}

// delegationUpstreamDeny 透傳上游 envelope 內的業務碼與訊息；上游沒給訊息才用本地文案。
//
// ⚠️ 實務上這條路徑很少被走到，但它不是死碼：`EnvelopeGuardInterceptor` 會先把
// 「gRPC OK ＋ resp.Code != 0」轉成 gRPC error（見 delegationGrpcFailure），只有列進
// `envelopeCodePreserveData` 的碼才會原樣抵達 handler。9266（版本衝突要帶 diff
// payload，§6.6.4）正是未來必須進那張清單的候選，屆時這裡就是它的出口。
func delegationUpstreamDeny(c *gin.Context, code int32, msg string) {
	if msg == "" {
		msg = delegationLocalMessage(code)
	}
	response.BizStatus(c, code, msg)
}

// delegationBizPrefixRE 解析 "[NNNN] message" —— 業務碼經 gRPC status 傳回時的形狀。
var delegationBizPrefixRE = regexp.MustCompile(`^\[(\d+)\]\s*(.*)$`)

// delegationGrpcFailure 處理一個 gRPC 呼叫失敗，並把夾帶其中的業務碼還原出來。
//
// ⚠️ 這不是防禦性寫法，而是本 repo 的正常路徑：Gateway 的 client 端
// `EnvelopeGuardInterceptor`（grpc/envelope_guard_interceptor.go）把「gRPC OK ＋
// resp.Code != 0」統一轉成 gRPC error，未知業務碼一律 FailedPrecondition ＋ 保留
// "[NNNN] " 前綴 ⇒ **代行的每一個業務拒絕都是從 err 這一側回來的**，handler 裡
// `resp.GetCode() != 0` 的分支只涵蓋 preserveData 名單內的碼。
//
// 漏了這一層的症狀是所有業務拒絕都塌成 424／9280（實測：handoff code 重放本該是
// 410／9282「此連結已失效，返回 ADM」，掉成 9280 之後前端走的是「結果不明」，操作者
// 卡在一個沒有下一步的畫面）。
//
// 為什麼不直接用 response.GrpcErr：它的 FailedPrecondition 分支硬寫 409，而代行需要
// 400／401／403／410／422 分流（§6.5.2 的登錄表就是為此存在）。這裡改走 BizStatus，
// 由同一張登錄表決定 status，未登錄的碼仍塌縮 409 與既有行為一致。
func delegationGrpcFailure(c *gin.Context, event string, err error) {
	if st, ok := grpcstatus.FromError(err); ok {
		if m := delegationBizPrefixRE.FindStringSubmatch(st.Message()); m != nil {
			if code, convErr := strconv.ParseInt(m[1], 10, 32); convErr == nil && code > 0 {
				response.BizStatus(c, int32(code), m[2])
				return
			}
		}
	}
	delegationUpstreamFailure(c, event, err)
}

// delegationUpstreamFailure 是所有「問不到答案」的唯一出口：424 ＋ 9280。
func delegationUpstreamFailure(c *gin.Context, event string, err error) {
	if err != nil {
		pkglogger.Info(c.Request.Context(), event, zap.Error(err))
	} else {
		pkglogger.Info(c.Request.Context(), event)
	}
	response.BizStatus(c, bizcode.DelegationUpstreamUnavailable,
		delegationLocalMessage(bizcode.DelegationUpstreamUnavailable))
}

// delegationAdminID 取 ADM session 解析出的管理員 id。
//
// requireAdmin 已經擋在前面，這裡取不到值只可能是 session 中途失效或本機
// SkipRoleCheck 模式——兩者都回 9261，讓 ADM 前端回登入頁而不是顯示一個泛用錯誤。
func delegationAdminID(c *gin.Context) (int64, bool) {
	id, ok := getAccountID(c)
	if !ok || id <= 0 {
		delegationDeny(c, bizcode.DelegationAdminSessionRequired, "")
		return 0, false
	}
	return id, true
}

// delegationCredential 讀出 operation credential 並從請求上移除。
//
// 移除是為了讓它不可能被任何下游轉發沿用：它是「單獨持有即可操作」的憑證，暴露面
// 必須壓到最小（樣板 router/seller_capability.go 的同款 Header.Del）。
func delegationCredential(c *gin.Context) string {
	v := c.GetHeader(HeaderOperationCredential)
	c.Request.Header.Del(HeaderOperationCredential)
	return v
}

// delegationIdempotencyKey 驗證冪等鍵。header 也是參數的一種，格式不符即 9260，
// 不另開新碼。
func delegationIdempotencyKey(c *gin.Context) (string, bool) {
	key := c.GetHeader(headerIdempotencyKey)
	if !delegationIdempotencyKeyRE.MatchString(key) {
		delegationDeny(c, bizcode.DelegationInvalidCommand, "Idempotency-Key 格式不正確")
		return "", false
	}
	return key, true
}

// hasControlChar 判斷字串是否含 ASCII 控制字元（含 DEL）。
func hasControlChar(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

// isDelegationOpaqueID 是 storeId 這類對外 id 的共通值域：1–128 字、已 trim、無控制
// 字元、不得含 `/` 或 `\`。本專案的實際值是 TSID 數字字串，但契約刻意只約束到這裡
// ——前端不得對它做前綴解析或格式驗證（產品需求畫的 `S9001` 那種短碼是 mockup）。
func isDelegationOpaqueID(v string) bool {
	if v == "" || len([]rune(v)) > 128 {
		return false
	}
	if strings.TrimSpace(v) != v || hasControlChar(v) {
		return false
	}
	return !strings.ContainsAny(v, `/\`)
}

// parseDelegationCaseReference 解出 caseReference 三態：顯式 null、合法字串、或非法。
//
// 值域：4–200 字、已 trim、無控制字元、**至少含一個數字**。刻意不做形狀驗證
// （例如 HK-YYYY-NNNN）：格式驗證只能證明它長得像、不能證明它存在，反而製造假的
// 確定性，而 content_governance 要填的本來就是純數字 TSID。
func parseDelegationCaseReference(raw json.RawMessage) (*string, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		// 欄位整個缺席。契約要求顯式送 null，因為「沒填」與「不需要填」是兩件事，
		// 而 BFF 會把 create body 逐欄與 preflight 的選擇比對。
		return nil, false
	}
	if string(trimmed) == "null" {
		return nil, true
	}
	var v string
	if err := json.Unmarshal(trimmed, &v); err != nil {
		return nil, false
	}
	n := len([]rune(v))
	if n < 4 || n > 200 || strings.TrimSpace(v) != v || hasControlChar(v) {
		return nil, false
	}
	if !strings.ContainsAny(v, "0123456789") {
		return nil, false
	}
	return &v, true
}

// validateDelegationCapabilities 驗證請求的能力集合：非空、不重複、全部落在九個封閉
// 列舉內。
//
// 「已知但這個工作階段沒被核發」不在這裡判——那是伺服端當下重新評估的結果，由
// account-service 回 9262（fail-closed，禁止靜默剔除）。Gateway 只擋根本不存在的
// token，那是格式錯（9260）。
func validateDelegationCapabilities(caps []string) bool {
	if len(caps) == 0 {
		return false
	}
	seen := make(map[string]struct{}, len(caps))
	for _, cap := range caps {
		if _, dup := seen[cap]; dup {
			return false
		}
		seen[cap] = struct{}{}
		if !authz.IsKnownAdmin(authz.AdminCapability(cap)) {
			return false
		}
	}
	return true
}

// toDelegationSessionData 把工作階段快照投影成對外 DTO。
func toDelegationSessionData(s *accountpb.OperationSessionSummary) DelegationSessionData {
	caps := s.GetCapabilities()
	if caps == nil {
		// 契約要求非空陣列；即使上游給了 nil 也不能讓 JSON 變成 null，那會讓前端在
		// 進入 status 分支之前就判契約錯誤。
		caps = []string{}
	}
	out := DelegationSessionData{
		SessionID:   s.GetSessionId(),
		DisplayCode: s.GetDisplayCode(),
		Target: DelegationTargetData{
			StoreID:         s.GetTarget().GetStoreId(),
			SellerAccountID: s.GetTarget().GetSellerAccountId(),
			StoreName:       s.GetTarget().GetStoreName(),
		},
		Actor: DelegationActorData{
			AdminID:     s.GetActor().GetAdminId(),
			DisplayName: s.GetActor().GetDisplayName(),
		},
		Reason: DelegationReasonData{
			Code:                  s.GetReason().GetCode(),
			Label:                 s.GetReason().GetLabel(),
			CaseReferenceRequired: s.GetReason().GetCaseReferenceRequired(),
		},
		Capabilities: caps,
		IssuedAt:     s.GetIssuedAt(),
		ExpiresAt:    s.GetExpiresAt(),
		Status:       s.GetStatus(),
		Revision:     s.GetRevision(),
	}
	// proto3 optional：未設定即 JSON null。**契約上仍必須輸出 null 而不是省略欄位**
	// （前端以欄位存在與否區分「沒有值」與「壞掉的回應」）。⚠️ 自 #2749 起三個 reason
	// 一律必填，所以新建的工作階段不會有 null；此分支保留是為了 #2749 之前建立的既有列。
	if s.CaseReference != nil {
		v := s.GetCaseReference()
		out.CaseReference = &v
	}
	return out
}

// ==================== S1 preflight ====================

// GetDelegatedOperationPreflight 判定此刻能否對該店家開始代行
// GET /api/v1/admin/sellers/{store_id}/delegated-operation/preflight
// @Summary      代行營運 preflight
// @Description  判定目前登入的管理員此刻能否對指定店家開始代行，並回傳可用能力、事由選項與 TTL／閒置窗口。唯讀且可安全重複呼叫：入口每 30 秒重抓、視窗 focus 重抓、每次建立前再抓一次，因此不寫入任何資料、不產生稽核列、不消耗冪等額度。被拒絕時仍回 200 並帶 allowed=false ＋ deniedCode。
// @Tags         代行營運 (Delegated Operations)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        store_id  path  string  true  "目標店家 store_id"
// @Success      200  {object}  object{code=int,message=string,data=handler.DelegationPreflightData}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      424  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/sellers/{store_id}/delegated-operation/preflight [get]
func (h *Holder) GetDelegatedOperationPreflight(c *gin.Context) {
	adminID, ok := delegationAdminID(c)
	if !ok {
		return
	}
	storeID := c.Param("store_id")
	if !isDelegationOpaqueID(storeID) {
		delegationDeny(c, bizcode.DelegationInvalidCommand, "storeId 格式不正確")
		return
	}

	resp, err := h.Account.DelegationPreflight(c.Request.Context(), &accountpb.PreflightRequest{
		ActorAdminId:  strconv.FormatInt(adminID, 10),
		TargetStoreId: storeID,
		UserAgent:     c.Request.UserAgent(),
	}, h.Delegation.GrpcDeadlinePreflightMs)
	if err != nil {
		delegationGrpcFailure(c, "代行 preflight 失敗", err)
		return
	}
	if resp.GetCode() != 0 {
		delegationUpstreamDeny(c, resp.GetCode(), resp.GetMessage())
		return
	}
	data := resp.GetData()
	if data == nil {
		delegationUpstreamFailure(c, "代行 preflight 回應缺少 data", nil)
		return
	}
	// 拒絕卻沒有拒絕碼，前端會在讀到 allowed 之前就把整份回應判成契約錯誤，操作者
	// 只看得到「代行不可用」。與其送出一份注定被丟掉的 200，不如誠實回 424 讓它可重試。
	if !data.GetAllowed() && data.GetDeniedCode() == 0 {
		delegationUpstreamFailure(c, "代行 preflight 拒絕但未帶 deniedCode", nil)
		return
	}

	opts := make([]DelegationReasonOptionData, 0, len(data.GetReasonOptions()))
	for _, o := range data.GetReasonOptions() {
		opts = append(opts, DelegationReasonOptionData{
			Code:                  o.GetCode(),
			Label:                 o.GetLabel(),
			CaseReferenceRequired: o.GetCaseReferenceRequired(),
		})
	}
	caps := data.GetCapabilities()
	if caps == nil {
		caps = []string{}
	}

	response.OK(c, DelegationPreflightData{
		Allowed: data.GetAllowed(),
		Target: DelegationTargetData{
			StoreID:         data.GetTarget().GetStoreId(),
			SellerAccountID: data.GetTarget().GetSellerAccountId(),
			StoreName:       data.GetTarget().GetStoreName(),
		},
		Actor: DelegationActorData{
			AdminID:     data.GetActor().GetAdminId(),
			DisplayName: data.GetActor().GetDisplayName(),
		},
		ReasonOptions:         opts,
		Capabilities:          caps,
		TTLSeconds:            data.GetTtlSeconds(),
		IdleSeconds:           data.GetIdleSeconds(),
		CaseReferenceRequired: data.GetCaseReferenceRequired(),
		DeniedCode:            data.GetDeniedCode(),
		DeniedLabel:           data.GetDeniedLabel(),
	})
}

// ==================== S2 建立工作階段 ====================

// CreateDelegatedOperation 建立代行工作階段
// POST /api/v1/admin/delegated-operations
// @Summary      建立代行營運工作階段
// @Description  建立短效代行工作階段，回傳一次性 handoff code 與工作階段摘要；operation credential 只在兌換時核發。capabilities 由伺服端重新評估（任一項未被核發即 9262，不靜默剔除）；ttlSeconds 由伺服端封頂。同一把 Idempotency-Key 重放回同一個工作階段；兌換前改簽 handoffCode，兌換後不再回 code。
// @Tags         代行營運 (Delegated Operations)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string                                  true  "冪等鍵，格式 ^[A-Za-z0-9._:-]{8,128}$"
// @Param        request          body    handler.CreateDelegatedOperationRequest  true  "建立參數（caseReference 必須顯式提供，無案件編號時送 null）"
// @Success      200  {object}  object{code=int,message=string,data=handler.DelegationCreateData}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,message=string,traceId=string}
// @Failure      409  {object}  object{code=int,message=string,traceId=string}
// @Failure      422  {object}  object{code=int,message=string,traceId=string}
// @Failure      424  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/delegated-operations [post]
func (h *Holder) CreateDelegatedOperation(c *gin.Context) {
	adminID, ok := delegationAdminID(c)
	if !ok {
		return
	}
	idempotencyKey, ok := delegationIdempotencyKey(c)
	if !ok {
		return
	}

	var req CreateDelegatedOperationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		delegationDeny(c, bizcode.DelegationInvalidCommand, "request body 格式不正確")
		return
	}
	if !isDelegationOpaqueID(req.StoreID) {
		delegationDeny(c, bizcode.DelegationInvalidCommand, "storeId 格式不正確")
		return
	}
	if req.ReasonCode == "" {
		delegationDeny(c, bizcode.DelegationInvalidCommand, "reasonCode 必填")
		return
	}
	caseReference, valid := parseDelegationCaseReference(req.CaseReference)
	if !valid {
		delegationDeny(c, bizcode.DelegationInvalidCommand,
			"caseReference 必須顯式提供：4–200 字且至少含一個數字，無案件編號時送 null")
		return
	}
	if !validateDelegationCapabilities(req.Capabilities) {
		delegationDeny(c, bizcode.DelegationInvalidCommand, "capabilities 必須非空、不重複且為已知能力")
		return
	}
	// 依賴閉包（#2744）。授權列端（admin_delegation_authorization.go）一直有這道檢查，
	// 工作階段端沒有 —— 而它只驗「已知／不重複／是授權集合的子集」，子集本身可以不自洽。
	//
	// 後果不是「少一道檢查」而是**把失敗延後並換掉主體**：只勾 products.write 的工作階段
	// 建得起來，操作者進到編輯畫面時才被 orchestrator 的 ReadBefore（products.read）以
	// 9262「此代行工作階段未取得該項操作權限」擋下 —— 憑證已經發出去了，而訊息看起來
	// 像是他的授權有問題。9287 的文案（「選擇編輯或發布時必須同時包含對應的檢視權限」）
	// 逐字就是這件事，只是先前沒有人在這條路徑上呼叫它。
	//
	// 同一個錯誤在授權列是「送出前擋下並告訴你缺哪一項」，在工作階段是「先發憑證、
	// 操作到一半才失敗且不說原因」。
	{
		typed := make([]authz.AdminCapability, 0, len(req.Capabilities))
		for _, v := range req.Capabilities {
			typed = append(typed, authz.AdminCapability(v))
		}
		if missing := authz.MissingAdminCapabilityDependencies(typed); len(missing) > 0 {
			names := make([]string, 0, len(missing))
			for _, m := range missing {
				names = append(names, string(m))
			}
			delegationDeny(c, bizcode.DelegationCapabilityDependencyIncomplete,
				"代行能力組合不完整：選擇編輯或發布時必須同時包含對應的檢視權限（缺少 "+strings.Join(names, "、")+"）")
			return
		}
	}
	// clamp 是伺服端的義務，前端夾一次不會錯，但一旦被當成唯一防線，curl 直打就繞
	// 過去了。這裡擋值域，真正的封頂（min(requested, MAX)）在 account-service。
	if req.TTLSeconds == nil || *req.TTLSeconds < 1 || *req.TTLSeconds > 1800 {
		delegationDeny(c, bizcode.DelegationInvalidCommand, "ttlSeconds 必須為 1–1800 的整數")
		return
	}

	grpcReq := &accountpb.CreateSessionRequest{
		ActorAdminId:  strconv.FormatInt(adminID, 10),
		TargetStoreId: req.StoreID,
		ReasonCode:    req.ReasonCode,
		// ActorDisplayName 刻意留空：Gateway 手上只有 ADM session 的 account id，
		// 管理員的顯示名由 account-service 從自己的權威資料補（preflight 同一支
		// 判定的 actor）。在這裡塞一個猜來的值只會讓稽核列記錯人。
		RequestedCapabilities: req.Capabilities,
		RequestedTtlSeconds:   *req.TTLSeconds,
		IdempotencyKey:        idempotencyKey,
		RequestId:             c.GetString("request_id"),
		CorrelationId:         c.GetString("trace_id"),
		UserAgent:             c.Request.UserAgent(),
	}
	if caseReference != nil {
		grpcReq.CaseReference = caseReference
	}

	resp, err := h.Account.CreateDelegationSession(c.Request.Context(), grpcReq, h.Delegation.GrpcDeadlineCreateMs)
	if err != nil {
		delegationGrpcFailure(c, "建立代行工作階段失敗", err)
		return
	}
	if resp.GetCode() != 0 {
		delegationUpstreamDeny(c, resp.GetCode(), resp.GetMessage())
		return
	}
	data := resp.GetData()
	// session 缺失時不能對帳；handoffCode 在已兌換的重放中可以缺席。
	if data == nil || data.GetSession() == nil {
		delegationUpstreamFailure(c, "建立代行工作階段回應不完整", nil)
		return
	}

	response.OK(c, DelegationCreateData{
		HandoffCode: data.GetHandoffCode(),
		Session:     toDelegationSessionData(data.GetSession()),
	})
}

// ==================== S3 查詢當前工作階段 ====================

// GetCurrentDelegatedOperation 由 credential 反查當前工作階段
// GET /api/v1/admin/delegated-operations/current
// @Summary      查詢當前代行工作階段
// @Description  由 operation credential 反查工作階段摘要，含終態（status 為 active／expired／revoked 三值之一，hard 到期與閒置逾時共用 expired）。完全唯讀：不 bump revision、不延長 expiresAt、不順延 last_active_at——工作區每 30 秒輪詢一次，把輪詢算成活動閒置逾時就永遠不會觸發。無 query、無 body：目標店家只能由工作階段快照決定。
// @Tags         代行營運 (Delegated Operations)
// @Accept       json
// @Produce      json
// @Param        X-HK-Operation-Session  header  string  true  "operation credential"
// @Success      200  {object}  object{code=int,message=string,data=handler.DelegationSessionData}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      424  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/delegated-operations/current [get]
func (h *Holder) GetCurrentDelegatedOperation(c *gin.Context) {
	credential := delegationCredential(c)
	if credential == "" {
		delegationDeny(c, bizcode.DelegationCredentialInvalid, "")
		return
	}

	// counts_as_activity 恆為 false：這是六支 session 端點之一，而本支被輪詢。
	resp, err := h.Account.GetCurrentDelegationSession(c.Request.Context(), credential, false,
		h.Delegation.GrpcDeadlineCurrentMs)
	if err != nil {
		delegationGrpcFailure(c, "查詢代行工作階段失敗", err)
		return
	}
	if resp.GetCode() != 0 {
		delegationUpstreamDeny(c, resp.GetCode(), resp.GetMessage())
		return
	}
	data := resp.GetData()
	if data == nil {
		// 9274 表達的只有「不存在」。前端對它保留 cookie、不宣稱已結束，所以回 401
		// 是安全的；反過來不得用 404，那會與「endpoint 沒部署」混淆。
		delegationDeny(c, bizcode.DelegationCredentialInvalid, "")
		return
	}

	response.OK(c, toDelegationSessionData(data))
}

// ==================== S4 結束工作階段 ====================

// EndCurrentDelegatedOperation 結束當前工作階段
// DELETE /api/v1/admin/delegated-operations/current
// @Summary      結束當前代行工作階段
// @Description  結束＝立刻到期（expires_at 收到 now），語意與 expired 一致，不需要任何排程器。冪等：已結束／已到期／已撤銷仍回同樣的 200；credential 無法解析時同樣回 200 {"ended":true}——此時不存在任何以該 credential 進行的代行，而回 401／404 會讓前端判定「結果不明」並把工作區寫入永久封鎖。
// @Tags         代行營運 (Delegated Operations)
// @Accept       json
// @Produce      json
// @Param        X-HK-Operation-Session  header  string  false  "operation credential"
// @Success      200  {object}  object{code=int,message=string,data=handler.DelegationEndData}
// @Failure      424  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/delegated-operations/current [delete]
func (h *Holder) EndCurrentDelegatedOperation(c *gin.Context) {
	credential := delegationCredential(c)
	if credential == "" {
		// 沒有 credential 就沒有要結束的東西，ended 在客戶端關心的層次上為真。
		// 回應不因 credential 是否存在而不同，因此也不洩漏任何資訊。
		response.OK(c, DelegationEndData{Ended: true})
		return
	}

	// EndSession／RevokeSession 沿用 create 的 deadline（§7.3.1 未另立參數）。
	resp, err := h.Account.EndDelegationSession(c.Request.Context(), credential, c.GetString("request_id"),
		h.Delegation.GrpcDeadlineCreateMs)
	if err != nil {
		// 這一支刻意不走 delegationGrpcFailure：它的失敗情境表只有 9280 一列，把上游
		// 的業務碼原樣分流出去會讓「credential 無法解析」變成 401／410，而那正是本端點
		// 明令要回 200 的情境（回 401/404 會把工作區寫入永久封鎖）。
		delegationUpstreamFailure(c, "結束代行工作階段失敗", err)
		return
	}
	if resp.GetCode() != 0 {
		// 契約上這支沒有業務拒絕。真的收到一個，代表工作階段沒有被終止，此時宣稱
		// ended:true 就是一份假的終止證明——回可重試的 424 才誠實。
		delegationUpstreamFailure(c, "結束代行工作階段回應非零業務碼", nil)
		return
	}

	response.OK(c, DelegationEndData{Ended: true})
}

// ==================== S5 撤銷工作階段 ====================

// RevokeDelegatedOperation 撤銷指定工作階段
// POST /api/v1/admin/delegated-operations/{session_id}/revoke
// @Summary      撤銷代行工作階段
// @Description  ADM 治理動作（不是代行動作）：由主控台對任一工作階段執行，只需要 ADM session，不接受也不需要 operation credential。撤銷即時生效，下一次查詢與任何代行寫入立即 fail-closed（9271／410）。已到期的工作階段仍可撤銷以留下治理痕跡；冪等重放回第一次撤銷的 revokedAt 與 revision，不再 bump。
// @Tags         代行營運 (Delegated Operations)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        session_id       path    string                                   true  "工作階段 sessionId"
// @Param        Idempotency-Key  header  string                                   true  "冪等鍵，格式 ^[A-Za-z0-9._:-]{8,128}$"
// @Param        request          body    handler.RevokeDelegatedOperationRequest  true  "撤銷理由"
// @Success      200  {object}  object{code=int,message=string,data=handler.DelegationRevokeData}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      409  {object}  object{code=int,message=string,traceId=string}
// @Failure      410  {object}  object{code=int,message=string,traceId=string}
// @Failure      424  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/delegated-operations/{session_id}/revoke [post]
func (h *Holder) RevokeDelegatedOperation(c *gin.Context) {
	adminID, ok := delegationAdminID(c)
	if !ok {
		return
	}
	sessionID := c.Param("session_id")
	if !delegationSessionIDRE.MatchString(sessionID) {
		// 形狀不對的 sessionId 與查無此列是同一件事：那個工作階段不存在。
		delegationDeny(c, bizcode.DelegationSessionNotFound, "")
		return
	}
	idempotencyKey, ok := delegationIdempotencyKey(c)
	if !ok {
		return
	}

	var req RevokeDelegatedOperationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		delegationDeny(c, bizcode.DelegationInvalidCommand, "request body 格式不正確")
		return
	}
	reason := strings.TrimSpace(req.RevokeReason)
	if n := len([]rune(reason)); n < 1 || n > 200 || hasControlChar(reason) {
		delegationDeny(c, bizcode.DelegationInvalidCommand, "revokeReason 必須為 1–200 字")
		return
	}

	resp, err := h.Account.RevokeDelegationSession(c.Request.Context(), &accountpb.RevokeSessionRequest{
		SessionId:      sessionID,
		ActorAdminId:   strconv.FormatInt(adminID, 10),
		RevokeReason:   reason,
		IdempotencyKey: idempotencyKey,
		RequestId:      c.GetString("request_id"),
	}, h.Delegation.GrpcDeadlineCreateMs)
	if err != nil {
		delegationGrpcFailure(c, "撤銷代行工作階段失敗", err)
		return
	}
	if resp.GetCode() != 0 {
		delegationUpstreamDeny(c, resp.GetCode(), resp.GetMessage())
		return
	}
	data := resp.GetData()
	if data == nil {
		delegationUpstreamFailure(c, "撤銷代行工作階段回應缺少 data", nil)
		return
	}

	response.OK(c, DelegationRevokeData{
		SessionID: data.GetSessionId(),
		Status:    data.GetStatus(),
		Revision:  data.GetRevision(),
		RevokedAt: data.GetRevokedAt(),
	})
}

// ==================== S6 handoff 兌換 ====================

// RedeemDelegationHandoff 兌換一次性 handoff code
// POST /api/v1/admin/delegated-operations/handoff/redeem
// @Summary      兌換代行交接碼
// @Description  ADM 與代行工作區在兩個不同 host，兩邊的憑證都是 host-only cookie 彼此讀不到，因此跨 host 傳遞的是一把單次、60 秒、與工作階段綁定的兌換碼，credential 本身從不離開伺服器之間的連線。本端點不要 ADM session、也不要 operation credential，授權完全由 code 自己的四道防護承擔。失敗一律 9282／410：不存在、已兌換、逾時、工作階段已不可用四種成因刻意共用同一個碼——分開等於送給攻擊者一支「這把 code 存在嗎」的探測器。
// @Tags         代行營運 (Delegated Operations)
// @Accept       json
// @Produce      json
// @Param        request  body  handler.RedeemDelegationHandoffRequest  true  "handoff code"
// @Success      200  {object}  object{code=int,message=string,data=handler.DelegationRedeemData}
// @Failure      410  {object}  object{code=int,message=string,traceId=string}
// @Failure      424  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/delegated-operations/handoff/redeem [post]
func (h *Holder) RedeemDelegationHandoff(c *gin.Context) {
	var req RedeemDelegationHandoffRequest
	// body 壞掉或 code 形狀不對，同樣回 9282 而不是 9260：任何能區分「這把 code 格式
	// 對但不存在」與「格式就不對」的回應，都是一支可用來收窄猜測空間的探測器。
	if err := c.ShouldBindJSON(&req); err != nil || !delegationHandoffCodeRE.MatchString(req.HandoffCode) {
		delegationDeny(c, bizcode.DelegationHandoffInvalid, "")
		return
	}

	resp, err := h.Account.RedeemDelegationHandoff(c.Request.Context(), req.HandoffCode,
		c.GetString("request_id"), h.Delegation.GrpcDeadlineCreateMs)
	if err != nil {
		delegationGrpcFailure(c, "兌換代行交接碼失敗", err)
		return
	}
	if resp.GetCode() != 0 {
		delegationUpstreamDeny(c, resp.GetCode(), resp.GetMessage())
		return
	}
	data := resp.GetData()
	if data == nil || data.GetOperationCredential() == "" || data.GetSession() == nil {
		delegationUpstreamFailure(c, "兌換代行交接碼回應不完整", nil)
		return
	}

	response.OK(c, DelegationRedeemData{
		OperationCredential: data.GetOperationCredential(),
		Session:             toDelegationSessionData(data.GetSession()),
	})
}
