package handler

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	"github.com/gin-gonic/gin"
)

// 56 號代行授權列的七支治理端點（#2444）。
//
// 它們與 admin_delegation.go 的 S1–S6 是**不同層**的東西：S1–S6 管「這一次代行」，本檔
// 管「這家店可不可以被代行、範圍多大」。授權的建立與撤回發生在任何工作階段之前，所以
// 七支全部掛在 ADM host 組（JWTAuth + requireAdmin），沒有 operation credential、
// 不宣告 capability、也不寫 admin_operation_audit_logs（那張表的 session_id 是
// NOT NULL REFERENCES delegated_operation_sessions，結構上就填不進去）。留痕走
// account-service 的 delegation_authorization_audit_logs，與授權列同一個交易。
//
// ⚠️ 路徑一律含 `delegated-operations` 這個 segment。hawkr-admin 的 catch-all BFF
// （features/delegated-operations/server/bff-boundary.ts）只認 `delegated-operation`／
// `delegated-operations` 兩個字面值來拒絕代理；換成 `delegation-authorizations` 之類的
// 名字，前端就能用泛用 api.get 繞過整套 parser／safeUpstreamError／no-store 保護，而且
// 不會有任何東西轉紅。

// ==================== 值域 ====================

// delegationAuthorizationSources 是 chk_sda_source 的三值。授權依據的種類是封閉列舉，
// 不是自由文字——「憑什麼授權這家店」必須指得出一份可查的東西。
var delegationAuthorizationSources = map[string]struct{}{
	"contract":        {},
	"ticket":          {},
	"platform_policy": {},
}

// delegationAuthorizationStatuses 是列表篩選的全覆蓋二分。
//
// 刻意不做 {active, revoked, expired} 那種三分：effective_from 落在未來的列（維運 SQL
// 仍可能建）不屬於其中任何一類，篩選時會靜默消失。二分不會漏，畫面要說明「為什麼無效」
// 時讀該列的 revokedAt／effectiveUntil 自己判斷。
var delegationAuthorizationStatuses = map[string]struct{}{
	"effective":   {},
	"ineffective": {},
}

// delegationEffectiveUntilRE 是對外時間格式（ISO 8601 UTC 秒精度）。
//
// 只驗形狀，不驗「是不是已經過去了」：那個判斷要用 now()，而 now() 的權威在
// account-service（回應的 effective 欄就是它算的）。在 Gateway 用自己的時鐘再判一次，
// 兩邊時鐘差幾秒就會出現「Gateway 說不行、服務說可以」的分歧，而分歧的那一側永遠是
// 使用者看到的那一側。
var delegationEffectiveUntilRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`)

// ==================== 回應 DTO ====================

// DelegationAuthorizationData 是授權列對外的唯一投影。
//
// 可空欄位一律 *string 且**不加 omitempty**：欄位消失（undefined）與「值為 null」在
// 前端 parser 上是兩件事，前者被判契約錯誤（502）而整個面板打不開。
type DelegationAuthorizationData struct {
	ID              string `json:"id"`
	StoreID         string `json:"storeId"`
	Source          string `json:"source"`
	SourceReference string `json:"sourceReference"`
	// GrantedCapabilities 恆為陣列，空集合輸出 []（授權列的 cardinality 下界是 1，
	// 所以實務上不會空；但 null 會讓前端在 .map 之前就炸）。
	GrantedCapabilities []string `json:"grantedCapabilities"`
	EffectiveFrom       string   `json:"effectiveFrom"`
	// EffectiveUntil 為 null ＝ 無期限。
	EffectiveUntil      *string `json:"effectiveUntil"`
	ApprovedByAdminID   string  `json:"approvedByAdminId"`
	ApprovedByAdminName string  `json:"approvedByAdminName"`
	ApprovedAt          string  `json:"approvedAt"`
	RevokedAt           *string `json:"revokedAt"`
	RevokedByAdminID    *string `json:"revokedByAdminId"`
	RevokedByAdminName  *string `json:"revokedByAdminName"`
	RevokeReason        *string `json:"revokeReason"`
	// Effective 是伺服端以 now() 逐列評估的結果。前端不得自行從三個時間欄推導——
	// 客戶端重算一次就會出現兩套答案，而其中一套會在邊界時刻說謊。
	Effective bool   `json:"effective"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// DelegationAuthorizationLookupData 是單店查詢的回應。
//
// exists=false 是正常狀態不是錯誤：用 9286 表示「尚未開通」會讓「這家店還沒開通」與
// 「上游掛了」在前端長得一模一樣。
type DelegationAuthorizationLookupData struct {
	Exists        bool                         `json:"exists"`
	Authorization *DelegationAuthorizationData `json:"authorization"`
	// ActiveSessionCount 是該店此刻進行中的代行工作階段數。撤回對話框的二次確認靠它
	// 講出「此操作將同時中止 N 個進行中的代行工作階段」；前端不得另打工作階段列表去數，
	// 那會與撤回看到的不是同一個瞬間。與 exists 正交：已撤回的店仍可能有殘留工作階段。
	ActiveSessionCount string `json:"activeSessionCount"`
}

// DelegationAuthorizationGrantData 是開通／復權的回應。
type DelegationAuthorizationGrantData struct {
	Authorization DelegationAuthorizationData `json:"authorization"`
	// Outcome 恰兩值：created（此前沒有任何授權列）／reinstated（覆寫了一列已撤回的）。
	// 由伺服端依實際走到的 CAS 分支填入，成功訊息據此分流。
	Outcome string `json:"outcome"`
}

// DelegationAuthorizationUpdateData 是調整授權範圍的回應。
//
// 只有一欄仍包成 struct 而不是直接回投影：Grant 與 Revoke 都在 authorization 之外多帶
// 一欄，三支的 data 形狀一致，前端才只需要一個 unwrap。
type DelegationAuthorizationUpdateData struct {
	Authorization DelegationAuthorizationData `json:"authorization"`
}

// DelegationAuthorizationRevokeData 是撤回的回應。
type DelegationAuthorizationRevokeData struct {
	Authorization DelegationAuthorizationData `json:"authorization"`
	// AlreadyRevoked=true ＝ 抵達時該列已是撤回狀態：沒有欄位被改、沒有稽核列、
	// 也沒有工作階段被中止（revokedSessionIds 必為空陣列）。
	AlreadyRevoked bool `json:"alreadyRevoked"`
	// RevokedSessionIDs 恆為陣列。UI 拿它回報「已同時中止 N 個工作階段」，筆數可能與
	// 撤回前查到的 activeSessionCount 不同（中間有人自行結束），以本欄為準。
	RevokedSessionIDs []string `json:"revokedSessionIds"`
}

// DelegationCapabilityDescriptor 是勾選介面需要知道的一個能力的全部事實。
type DelegationCapabilityDescriptor struct {
	Value string `json:"value"`
	// Risk 恰三值：low／medium／high。orders.read 是九值中唯一「唯讀卻是 high」的一項
	// ——它每次揭露都落一列 PII 稽核。
	Risk string `json:"risk"`
	// DependsOn 是**直接**依賴，恆為陣列。前端據此做連動勾選（勾 X 自動補上依賴、
	// 取消依賴時反向取消依賴它的項）。
	DependsOn []string `json:"dependsOn"`
}

// DelegationCapabilityCatalogData 是能力目錄，由 Gateway 直接下發、不打 gRPC。
//
// 它存在的唯一理由是消滅前端的第二份依賴表：依賴閉包在 DB 沒有落點，後端的唯一副本
// 在 gateway/internal/authz。前端若自己維護一份，兩份漂移時的症狀是「畫面允許送出的
// 組合被後端擋掉」或更糟的「畫面擋掉了合法組合」，兩者都沒有任何東西會轉紅。
type DelegationCapabilityCatalogData struct {
	Capabilities []DelegationCapabilityDescriptor `json:"capabilities"`
}

// DelegationAuthorizationHistoryEntry 是授權變更歷程的一列（append-only 稽核）。
type DelegationAuthorizationHistoryEntry struct {
	ID              string `json:"id"`
	AuthorizationID string `json:"authorizationId"`
	StoreID         string `json:"storeId"`
	Action          string `json:"action"`
	// ActionLabel 由後端下發。前端不得自建 action → 標籤對照表：那會是 CodeMeta 的
	// 第二份副本，且改碼時不會有任何東西轉紅。
	ActionLabel  string `json:"actionLabel"`
	ActorAdminID string `json:"actorAdminId"`
	// ActorDisplayName 是寫入當下的快照，不是現況 join。稽核是歷史事實，管理員日後
	// 改了 email 不該回溯修改「當時是誰做的」。
	ActorDisplayName     string   `json:"actorDisplayName"`
	OperationReason      string   `json:"operationReason"`
	Source               *string  `json:"source"`
	SourceReference      *string  `json:"sourceReference"`
	CapabilitiesBefore   []string `json:"capabilitiesBefore"`
	CapabilitiesAfter    []string `json:"capabilitiesAfter"`
	EffectiveUntilBefore *string  `json:"effectiveUntilBefore"`
	EffectiveUntilAfter  *string  `json:"effectiveUntilAfter"`
	CreatedAt            string   `json:"createdAt"`
}

// ==================== 請求 DTO ====================

// GrantDelegationAuthorizationRequest 是開通／復權的 body。
type GrantDelegationAuthorizationRequest struct {
	StoreID             string   `json:"storeId"`
	Source              string   `json:"source"`
	SourceReference     string   `json:"sourceReference"`
	GrantedCapabilities []string `json:"grantedCapabilities"`
	// EffectiveUntil 未送或 null ＝ 無期限。復權時**不沿用舊值**：本 RPC 是一次全新
	// 授權，舊列的期限一律被本欄取代（含取代成無期限）。
	EffectiveUntil  *string `json:"effectiveUntil"`
	OperationReason string  `json:"operationReason"`
}

// UpdateDelegationAuthorizationRequest 是調整授權範圍的 body（全量取代，非 partial）。
// storeId 走 path 不進 body：同一個值出現在兩處，遲早會有一次不一致。
type UpdateDelegationAuthorizationRequest struct {
	Source              string   `json:"source"`
	SourceReference     string   `json:"sourceReference"`
	GrantedCapabilities []string `json:"grantedCapabilities"`
	EffectiveUntil      *string  `json:"effectiveUntil"`
	OperationReason     string   `json:"operationReason"`
}

// RevokeDelegationAuthorizationRequest 是撤回的 body。
type RevokeDelegationAuthorizationRequest struct {
	RevokeReason string `json:"revokeReason"`
}

// ==================== 共用小工具 ====================

// delegationAuthorizationStoreID 讀 path 上的 store_id。
//
// 值域與 S1 preflight 的 storeId 完全相同（isDelegationOpaqueID）：同一個識別碼在同一組
// 端點上不該有兩套值域，否則同一個值在 preflight 過得了、在授權面被擋掉。
func delegationAuthorizationStoreID(c *gin.Context) (string, bool) {
	storeID := c.Param("store_id")
	if !isDelegationOpaqueID(storeID) {
		delegationDeny(c, bizcode.DelegationInvalidCommand, "storeId 格式不正確")
		return "", false
	}
	return storeID, true
}

// validateDelegationAuthorizationScope 驗證三個「授權變成什麼」的欄位，Grant 與 Update
// 共用。回傳已 trim 的值。
//
// 依賴閉包（9287）與其他值域（9260）刻意用不同的碼：前者是「這組能力互相矛盾」，UI 要
// 自動補上缺的那幾項再讓人確認；後者是「這個參數本身就不對」，UI 只能請人重填。混成
// 一個碼之後，前端唯一能做的就是對所有錯誤顯示同一句話。
func validateDelegationAuthorizationScope(c *gin.Context, source, sourceReference string, caps []string, effectiveUntil *string, reason string) (outSource, outRef, outReason string, ok bool) {
	if _, known := delegationAuthorizationSources[source]; !known {
		delegationDeny(c, bizcode.DelegationInvalidCommand, "source 必須為 contract、ticket 或 platform_policy")
		return "", "", "", false
	}

	ref := strings.TrimSpace(sourceReference)
	if n := len([]rune(ref)); n < 4 || n > 200 || hasControlChar(ref) || !strings.ContainsAny(ref, "0123456789") {
		// 「至少含一個數字」是 chk_sda_source_reference 的規則：它要的是依據**編號**
		// （合約號／工單號／政策文號），不是「賣家說要開通」這種說明文字。
		delegationDeny(c, bizcode.DelegationInvalidCommand, "sourceReference 必須為 4–200 字且至少含一個數字")
		return "", "", "", false
	}

	if !validateDelegationCapabilities(caps) {
		delegationDeny(c, bizcode.DelegationInvalidCommand, "grantedCapabilities 必須非空、不重複且為已知能力")
		return "", "", "", false
	}
	typed := make([]authz.AdminCapability, 0, len(caps))
	for _, v := range caps {
		typed = append(typed, authz.AdminCapability(v))
	}
	if missing := authz.MissingAdminCapabilityDependencies(typed); len(missing) > 0 {
		names := make([]string, 0, len(missing))
		for _, m := range missing {
			names = append(names, string(m))
		}
		delegationDeny(c, bizcode.DelegationCapabilityDependencyIncomplete,
			"代行能力組合不完整：選擇編輯或發布時必須同時包含對應的檢視權限（缺少 "+strings.Join(names, "、")+"）")
		return "", "", "", false
	}

	if effectiveUntil != nil && !delegationEffectiveUntilRE.MatchString(*effectiveUntil) {
		delegationDeny(c, bizcode.DelegationInvalidCommand,
			"effectiveUntil 必須為 ISO 8601 UTC 秒精度（YYYY-MM-DDTHH:mm:ssZ），無期限請送 null")
		return "", "", "", false
	}

	r := strings.TrimSpace(reason)
	if n := len([]rune(r)); n < 1 || n > 200 || hasControlChar(r) {
		delegationDeny(c, bizcode.DelegationInvalidCommand, "operationReason 必須為 1–200 字")
		return "", "", "", false
	}

	return source, ref, r, true
}

// toDelegationAuthorizationData 投影授權列。proto struct 一律不直出：protoc-gen-go 的
// tag 是 snake_case ＋ omitempty，直出即欄名全錯、且 false／空字串整個欄位消失。
func toDelegationAuthorizationData(a *accountpb.DelegationAuthorization) DelegationAuthorizationData {
	caps := a.GetGrantedCapabilities()
	if caps == nil {
		caps = []string{}
	}
	out := DelegationAuthorizationData{
		ID:                  a.GetId(),
		StoreID:             a.GetTargetStoreId(),
		Source:              a.GetSource(),
		SourceReference:     a.GetSourceReference(),
		GrantedCapabilities: caps,
		EffectiveFrom:       a.GetEffectiveFrom(),
		ApprovedByAdminID:   a.GetApprovedByAdminId(),
		ApprovedByAdminName: a.GetApprovedByAdminDisplayName(),
		ApprovedAt:          a.GetApprovedAt(),
		Effective:           a.GetEffective(),
		CreatedAt:           a.GetCreatedAt(),
		UpdatedAt:           a.GetUpdatedAt(),
	}
	if a.EffectiveUntil != nil {
		v := a.GetEffectiveUntil()
		out.EffectiveUntil = &v
	}
	if a.RevokedAt != nil {
		v := a.GetRevokedAt()
		out.RevokedAt = &v
	}
	if a.RevokedByAdminId != nil {
		v := a.GetRevokedByAdminId()
		out.RevokedByAdminID = &v
	}
	if a.RevokedByAdminDisplayName != nil {
		v := a.GetRevokedByAdminDisplayName()
		out.RevokedByAdminName = &v
	}
	if a.RevokeReason != nil {
		v := a.GetRevokeReason()
		out.RevokeReason = &v
	}
	return out
}

func toDelegationAuthorizationHistoryEntry(e *accountpb.DelegationAuthorizationAuditEntry) DelegationAuthorizationHistoryEntry {
	before := e.GetCapabilitiesBefore()
	if before == nil {
		before = []string{}
	}
	after := e.GetCapabilitiesAfter()
	if after == nil {
		after = []string{}
	}
	out := DelegationAuthorizationHistoryEntry{
		ID:                 e.GetId(),
		AuthorizationID:    e.GetAuthorizationId(),
		StoreID:            e.GetTargetStoreId(),
		Action:             e.GetAction(),
		ActionLabel:        e.GetActionLabel(),
		ActorAdminID:       e.GetActorAdminId(),
		ActorDisplayName:   e.GetActorDisplayName(),
		OperationReason:    e.GetOperationReason(),
		CapabilitiesBefore: before,
		CapabilitiesAfter:  after,
		CreatedAt:          e.GetCreatedAt(),
	}
	if e.Source != nil {
		v := e.GetSource()
		out.Source = &v
	}
	if e.SourceReference != nil {
		v := e.GetSourceReference()
		out.SourceReference = &v
	}
	if e.EffectiveUntilBefore != nil {
		v := e.GetEffectiveUntilBefore()
		out.EffectiveUntilBefore = &v
	}
	if e.EffectiveUntilAfter != nil {
		v := e.GetEffectiveUntilAfter()
		out.EffectiveUntilAfter = &v
	}
	return out
}

// delegationAuthorizationUpstream 檢查 account client 是否可用。Holder 在測試裡可能是
// 零值，而 nil client 會在呼叫的那一行 panic 而不是回一個可讀的錯誤。
func delegationAuthorizationUpstream(h *Holder, c *gin.Context, event string) bool {
	if h.Account == nil {
		delegationUpstreamFailure(c, event, nil)
		return false
	}
	return true
}

// ==================== 1｜能力目錄 ====================

// ListDelegationCapabilities 下發代行能力目錄（值／風險／依賴）
// GET /api/v1/admin/delegated-operations/capabilities
// @Summary      代行能力目錄
// @Description  下發九個代行能力的機器名、風險等級與**直接依賴**，供授權設定介面渲染勾選清單與連動規則。唯讀、無參數、不打領域服務——依賴閉包在 DB 沒有任何落點，Gateway 的 authz package 是它唯一的權威來源，前端不得自行維護第二份對照表。取不到本端點的回應時必須不開啟授權對話框（fail-closed），不得退回本地表。
// @Tags         代行營運 (Delegated Operations)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  object{code=int,message=string,data=handler.DelegationCapabilityCatalogData}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/delegated-operations/capabilities [get]
func (h *Holder) ListDelegationCapabilities(c *gin.Context) {
	if _, ok := delegationAdminID(c); !ok {
		return
	}

	caps := make([]DelegationCapabilityDescriptor, 0, len(authz.AllAdminCapabilities))
	for _, cap := range authz.AllAdminCapabilities {
		deps := authz.AdminCapabilityDependsOn(cap)
		names := make([]string, 0, len(deps))
		for _, d := range deps {
			names = append(names, string(d))
		}
		caps = append(caps, DelegationCapabilityDescriptor{
			Value:     string(cap),
			Risk:      authz.AdminCapabilityRisk(cap),
			DependsOn: names,
		})
	}
	response.OK(c, DelegationCapabilityCatalogData{Capabilities: caps})
}

// ==================== 2｜授權列表 ====================

// ListDelegationAuthorizations 列出代行授權（治理總覽）
// GET /api/v1/admin/delegated-operations/authorizations
// @Summary      代行授權列表
// @Description  治理總覽，唯讀。status 只有兩值：effective（此刻有效）與 ineffective（此刻無效，含已撤回、已到期、尚未生效三種成因）。刻意不做 active/revoked/expired 三分——effective_from 落在未來的列不屬於任何一類，篩選時會靜默消失。要說明「為什麼無效」請讀該列的 revokedAt／effectiveUntil。
// @Tags         代行營運 (Delegated Operations)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        status     query  string  false  "狀態篩選"  Enums(effective, ineffective)
// @Param        store_id   query  string  false  "精確比對某一家店的 store_id"
// @Param        page       query  string  false  "頁碼，預設 1"
// @Param        page_size  query  string  false  "每頁筆數，1–50，預設 20（越界回 9260，不靜默夾值）"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]handler.DelegationAuthorizationData,total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Failure      424  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/delegated-operations/authorizations [get]
func (h *Holder) ListDelegationAuthorizations(c *gin.Context) {
	adminID, ok := delegationAdminID(c)
	if !ok {
		return
	}
	page, pageSize, ok := delegatedPaging(c)
	if !ok {
		return
	}
	status, ok := auditFilterValue(c, "status", func(v string) bool {
		_, known := delegationAuthorizationStatuses[v]
		return known
	})
	if !ok {
		return
	}
	storeID := c.Query("store_id")
	if storeID != "" && !isDelegationOpaqueID(storeID) {
		delegationDeny(c, bizcode.DelegationInvalidCommand, "store_id 格式不正確")
		return
	}
	if !delegationAuthorizationUpstream(h, c, "代行授權列表：上游不可用") {
		return
	}

	resp, err := h.Account.ListDelegationAuthorizations(c.Request.Context(), &accountpb.ListDelegationAuthorizationsRequest{
		ActorAdminId:  strconv.FormatInt(adminID, 10),
		Status:        status,
		TargetStoreId: storeID,
		Page:          strconv.FormatInt(int64(page), 10),
		PageSize:      strconv.FormatInt(int64(pageSize), 10),
		RequestId:     c.GetString("request_id"),
		CorrelationId: c.GetString("trace_id"),
	}, h.Delegation.GrpcDeadlineCurrentMs)
	if err != nil {
		delegationGrpcFailure(c, "查詢代行授權列表失敗", err)
		return
	}
	if resp.GetCode() != 0 {
		delegationUpstreamDeny(c, resp.GetCode(), resp.GetMessage())
		return
	}

	records := make([]DelegationAuthorizationData, 0, len(resp.GetRecords()))
	for _, r := range resp.GetRecords() {
		records = append(records, toDelegationAuthorizationData(r))
	}
	response.Page(c, records, resp.GetTotal(), resp.GetPage(), resp.GetPageSize())
}

// ==================== 3｜單店授權詳情 ====================

// GetDelegationAuthorization 查詢單店的代行授權設定
// GET /api/v1/admin/delegated-operations/authorizations/{store_id}
// @Summary      單店代行授權
// @Description  商家詳情頁「代行授權」面板的資料來源，唯讀。查無授權列**不是錯誤**：回 200 ＋ exists=false ＋ authorization=null，用 404 表示「尚未開通」會讓它與「上游掛了」在前端長得一模一樣。只查該店那一列，不回落全域列——全域列若存在會遮蔽所有店，把它當成「這家店的設定」顯示，管理員會對著一列不屬於該店的授權按下撤回。activeSessionCount 是撤回二次確認的唯一資料來源。
// @Tags         代行營運 (Delegated Operations)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        store_id  path  string  true  "目標店家 store_id"
// @Success      200  {object}  object{code=int,message=string,data=handler.DelegationAuthorizationLookupData}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Failure      424  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/delegated-operations/authorizations/{store_id} [get]
func (h *Holder) GetDelegationAuthorization(c *gin.Context) {
	adminID, ok := delegationAdminID(c)
	if !ok {
		return
	}
	storeID, ok := delegationAuthorizationStoreID(c)
	if !ok {
		return
	}
	if !delegationAuthorizationUpstream(h, c, "查詢單店代行授權：上游不可用") {
		return
	}

	resp, err := h.Account.GetDelegationAuthorization(c.Request.Context(), &accountpb.GetDelegationAuthorizationRequest{
		ActorAdminId:  strconv.FormatInt(adminID, 10),
		TargetStoreId: storeID,
		RequestId:     c.GetString("request_id"),
		CorrelationId: c.GetString("trace_id"),
	}, h.Delegation.GrpcDeadlineCurrentMs)
	if err != nil {
		delegationGrpcFailure(c, "查詢單店代行授權失敗", err)
		return
	}
	if resp.GetCode() != 0 {
		delegationUpstreamDeny(c, resp.GetCode(), resp.GetMessage())
		return
	}
	data := resp.GetData()
	if data == nil {
		delegationUpstreamFailure(c, "查詢單店代行授權回應缺少 data", nil)
		return
	}

	out := DelegationAuthorizationLookupData{
		Exists:             data.GetExists(),
		ActiveSessionCount: data.GetActiveSessionCount(),
	}
	// exists=true 卻沒有投影，就是一份自相矛盾的回應：面板會渲染出一個沒有任何欄位的
	// 「已開通」，而管理員據此判斷這家店可以被代行。回可重試的 424 才誠實。
	if data.GetExists() {
		if data.GetAuthorization() == nil {
			delegationUpstreamFailure(c, "查詢單店代行授權：exists 為真但缺少授權列", nil)
			return
		}
		v := toDelegationAuthorizationData(data.GetAuthorization())
		out.Authorization = &v
	}
	if out.ActiveSessionCount == "" {
		// 計數欄缺席時補 "0" 而不是留空字串：前端要拿它做 Number() 與二次確認文案，
		// 空字串會變成 NaN 並把「將中止 N 個工作階段」渲染成空白。
		out.ActiveSessionCount = "0"
	}
	response.OK(c, out)
}

// ==================== 4｜開通／復權 ====================

// GrantDelegationAuthorization 開通或重新開通一家店的代行授權
// POST /api/v1/admin/delegated-operations/authorizations
// @Summary      開通代行授權
// @Description  首建與復權共用同一支：uq_sda_store 的述詞不含 revoked_at ⇒ 每店終生一列，復權只能覆寫同一列。實際走了哪一條由回應的 outcome 回報（created／reinstated），不由呼叫方指定。**同一把 Idempotency-Key 重放回傳與首次完全相同的 authorization 與 outcome**，且不補寫第二列稽核；同一把鍵改送不同內容回 9267／409（請換一把新的鍵，否則你的新內容會被當成重放而永遠不生效）。**另一把**鍵撞上已有的**有效**授權才是 9285／409（請改用調整範圍）。grantedCapabilities 必須是依賴閉包完整的組合，不齊回 9287／400。effectiveUntil 送 null ＝ 無期限；復權時舊列的期限一律被本欄取代。
// @Tags         代行營運 (Delegated Operations)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string                                       true  "冪等鍵，格式 ^[A-Za-z0-9._:-]{8,128}$"
// @Param        request          body    handler.GrantDelegationAuthorizationRequest  true  "授權內容"
// @Success      200  {object}  object{code=int,message=string,data=handler.DelegationAuthorizationGrantData}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Failure      409  {object}  object{code=int,message=string,traceId=string}
// @Failure      424  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/delegated-operations/authorizations [post]
func (h *Holder) GrantDelegationAuthorization(c *gin.Context) {
	adminID, ok := delegationAdminID(c)
	if !ok {
		return
	}
	idempotencyKey, ok := delegationIdempotencyKey(c)
	if !ok {
		return
	}

	var req GrantDelegationAuthorizationRequest
	if _, bound := bindDelegatedBody(c, &req); !bound {
		return
	}
	if !isDelegationOpaqueID(req.StoreID) {
		delegationDeny(c, bizcode.DelegationInvalidCommand, "storeId 格式不正確")
		return
	}
	source, ref, reason, ok := validateDelegationAuthorizationScope(c,
		req.Source, req.SourceReference, req.GrantedCapabilities, req.EffectiveUntil, req.OperationReason)
	if !ok {
		return
	}
	if !delegationAuthorizationUpstream(h, c, "開通代行授權：上游不可用") {
		return
	}

	grpcReq := &accountpb.GrantDelegationAuthorizationRequest{
		ActorAdminId:        strconv.FormatInt(adminID, 10),
		TargetStoreId:       req.StoreID,
		Source:              source,
		SourceReference:     ref,
		GrantedCapabilities: req.GrantedCapabilities,
		OperationReason:     reason,
		IdempotencyKey:      idempotencyKey,
		RequestId:           c.GetString("request_id"),
		CorrelationId:       c.GetString("trace_id"),
		UserAgent:           c.Request.UserAgent(),
	}
	grpcReq.EffectiveUntil = req.EffectiveUntil

	resp, err := h.Account.GrantDelegationAuthorization(c.Request.Context(), grpcReq, h.Delegation.GrpcDeadlineCreateMs)
	if err != nil {
		delegationGrpcFailure(c, "開通代行授權失敗", err)
		return
	}
	if resp.GetCode() != 0 {
		delegationUpstreamDeny(c, resp.GetCode(), resp.GetMessage())
		return
	}
	data := resp.GetData()
	if data == nil || data.GetAuthorization() == nil {
		delegationUpstreamFailure(c, "開通代行授權回應不完整", nil)
		return
	}

	response.OK(c, DelegationAuthorizationGrantData{
		Authorization: toDelegationAuthorizationData(data.GetAuthorization()),
		Outcome:       data.GetOutcome(),
	})
}

// ==================== 5｜調整授權範圍 ====================

// UpdateDelegationAuthorization 調整既有有效授權的範圍／依據／期限
// PUT /api/v1/admin/delegated-operations/authorizations/{store_id}
// @Summary      調整代行授權範圍
// @Description  **全量取代，不是 partial update**：grantedCapabilities 送什麼就是什麼，未送的欄位不會沿用舊值。只對**未撤回**的列成立——這支與開通刻意分開，合成一支 upsert 會讓管理員對著一張過時分頁按下「調整範圍」時，把一個因故撤回的店靜默復權且沒有任何錯誤訊號。該列不存在回 9286／404（請改用開通），已撤回回 9264／403（請改用開通以復權）。**同一把 Idempotency-Key 重放不重跑調整**，回傳與首次相同的 authorization 且不補寫第二列稽核；同一把鍵改送不同內容回 9267／409（請換一把新的鍵）。
// @Tags         代行營運 (Delegated Operations)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        store_id         path    string                                        true  "目標店家 store_id"
// @Param        Idempotency-Key  header  string                                        true  "冪等鍵，格式 ^[A-Za-z0-9._:-]{8,128}$"
// @Param        request          body    handler.UpdateDelegationAuthorizationRequest  true  "授權變成什麼（全量）"
// @Success      200  {object}  object{code=int,message=string,data=handler.DelegationAuthorizationUpdateData}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,message=string,traceId=string}
// @Failure      409  {object}  object{code=int,message=string,traceId=string}
// @Failure      424  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/delegated-operations/authorizations/{store_id} [put]
func (h *Holder) UpdateDelegationAuthorization(c *gin.Context) {
	adminID, ok := delegationAdminID(c)
	if !ok {
		return
	}
	storeID, ok := delegationAuthorizationStoreID(c)
	if !ok {
		return
	}
	idempotencyKey, ok := delegationIdempotencyKey(c)
	if !ok {
		return
	}

	var req UpdateDelegationAuthorizationRequest
	if _, bound := bindDelegatedBody(c, &req); !bound {
		return
	}
	source, ref, reason, ok := validateDelegationAuthorizationScope(c,
		req.Source, req.SourceReference, req.GrantedCapabilities, req.EffectiveUntil, req.OperationReason)
	if !ok {
		return
	}
	if !delegationAuthorizationUpstream(h, c, "調整代行授權：上游不可用") {
		return
	}

	grpcReq := &accountpb.UpdateDelegationAuthorizationRequest{
		ActorAdminId:        strconv.FormatInt(adminID, 10),
		TargetStoreId:       storeID,
		Source:              source,
		SourceReference:     ref,
		GrantedCapabilities: req.GrantedCapabilities,
		OperationReason:     reason,
		IdempotencyKey:      idempotencyKey,
		RequestId:           c.GetString("request_id"),
		CorrelationId:       c.GetString("trace_id"),
		UserAgent:           c.Request.UserAgent(),
	}
	grpcReq.EffectiveUntil = req.EffectiveUntil

	resp, err := h.Account.UpdateDelegationAuthorization(c.Request.Context(), grpcReq, h.Delegation.GrpcDeadlineCreateMs)
	if err != nil {
		delegationGrpcFailure(c, "調整代行授權失敗", err)
		return
	}
	if resp.GetCode() != 0 {
		delegationUpstreamDeny(c, resp.GetCode(), resp.GetMessage())
		return
	}
	data := resp.GetData()
	if data == nil || data.GetAuthorization() == nil {
		delegationUpstreamFailure(c, "調整代行授權回應不完整", nil)
		return
	}

	response.OK(c, DelegationAuthorizationUpdateData{
		Authorization: toDelegationAuthorizationData(data.GetAuthorization()),
	})
}

// ==================== 6｜撤回 ====================

// RevokeDelegationAuthorization 撤回一家店的代行授權
// POST /api/v1/admin/delegated-operations/authorizations/{store_id}/revoke
// @Summary      撤回代行授權
// @Description  撤回三欄（revokedAt／revokedByAdminId／revokeReason）同生同滅，理由必填——少一個就是一筆解釋不了的撤回。**同一個交易內連帶中止該店所有進行中的代行工作階段**，被中止的 id 回在 revokedSessionIds；把中止留成人工第二步等於交付一個半殘的治理動作。已撤回為冪等成功（alreadyRevoked=true，回第一次撤回的值，不補寫第二列稽核、不中止任何工作階段）；該店從未開通才是 9286／404。Idempotency-Key 不得沿用該店其他治理動作用過的鍵，沿用回 9267／409。
// @Tags         代行營運 (Delegated Operations)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        store_id         path    string                                        true  "目標店家 store_id"
// @Param        Idempotency-Key  header  string                                        true  "冪等鍵，格式 ^[A-Za-z0-9._:-]{8,128}$"
// @Param        request          body    handler.RevokeDelegationAuthorizationRequest  true  "撤回理由"
// @Success      200  {object}  object{code=int,message=string,data=handler.DelegationAuthorizationRevokeData}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,message=string,traceId=string}
// @Failure      409  {object}  object{code=int,message=string,traceId=string}
// @Failure      424  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/delegated-operations/authorizations/{store_id}/revoke [post]
func (h *Holder) RevokeDelegationAuthorization(c *gin.Context) {
	adminID, ok := delegationAdminID(c)
	if !ok {
		return
	}
	storeID, ok := delegationAuthorizationStoreID(c)
	if !ok {
		return
	}
	idempotencyKey, ok := delegationIdempotencyKey(c)
	if !ok {
		return
	}

	var req RevokeDelegationAuthorizationRequest
	if _, bound := bindDelegatedBody(c, &req); !bound {
		return
	}
	reason := strings.TrimSpace(req.RevokeReason)
	if n := len([]rune(reason)); n < 1 || n > 200 || hasControlChar(reason) {
		delegationDeny(c, bizcode.DelegationInvalidCommand, "revokeReason 必須為 1–200 字")
		return
	}
	if !delegationAuthorizationUpstream(h, c, "撤回代行授權：上游不可用") {
		return
	}

	resp, err := h.Account.RevokeDelegationAuthorization(c.Request.Context(), &accountpb.RevokeDelegationAuthorizationRequest{
		ActorAdminId:   strconv.FormatInt(adminID, 10),
		TargetStoreId:  storeID,
		RevokeReason:   reason,
		IdempotencyKey: idempotencyKey,
		RequestId:      c.GetString("request_id"),
		CorrelationId:  c.GetString("trace_id"),
		UserAgent:      c.Request.UserAgent(),
	}, h.Delegation.GrpcDeadlineCreateMs)
	if err != nil {
		delegationGrpcFailure(c, "撤回代行授權失敗", err)
		return
	}
	if resp.GetCode() != 0 {
		delegationUpstreamDeny(c, resp.GetCode(), resp.GetMessage())
		return
	}
	data := resp.GetData()
	if data == nil || data.GetAuthorization() == nil {
		delegationUpstreamFailure(c, "撤回代行授權回應不完整", nil)
		return
	}
	sessions := data.GetRevokedSessionIds()
	if sessions == nil {
		sessions = []string{}
	}

	response.OK(c, DelegationAuthorizationRevokeData{
		Authorization:     toDelegationAuthorizationData(data.GetAuthorization()),
		AlreadyRevoked:    data.GetAlreadyRevoked(),
		RevokedSessionIDs: sessions,
	})
}

// ==================== 7｜授權變更歷程 ====================

// ListDelegationAuthorizationHistory 列出單店的授權變更歷程
// GET /api/v1/admin/delegated-operations/authorizations/{store_id}/history
// @Summary      代行授權變更歷程
// @Description  append-only 稽核，唯讀，依時間由新到舊。這張表存在的理由：每店終生一列 ⇒ 復權會就地覆寫前一次撤回的 revokedAt／revokedByAdminId／revokeReason，「這家店曾經被撤回過」在授權列上不留任何痕跡。從未開通過的店回 9286／404 而不是空清單——「查無此店的授權」與「有授權但沒改過」是兩件事，而後者不可能發生（開通本身就會留下第一列 GRANT）。
// @Tags         代行營運 (Delegated Operations)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        store_id   path   string  true   "目標店家 store_id"
// @Param        page       query  string  false  "頁碼，預設 1"
// @Param        page_size  query  string  false  "每頁筆數，1–50，預設 20（越界回 9260，不靜默夾值）"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]handler.DelegationAuthorizationHistoryEntry,total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,message=string,traceId=string}
// @Failure      424  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/delegated-operations/authorizations/{store_id}/history [get]
func (h *Holder) ListDelegationAuthorizationHistory(c *gin.Context) {
	adminID, ok := delegationAdminID(c)
	if !ok {
		return
	}
	storeID, ok := delegationAuthorizationStoreID(c)
	if !ok {
		return
	}
	page, pageSize, ok := delegatedPaging(c)
	if !ok {
		return
	}
	if !delegationAuthorizationUpstream(h, c, "查詢代行授權歷程：上游不可用") {
		return
	}

	resp, err := h.Account.ListDelegationAuthorizationHistory(c.Request.Context(), &accountpb.ListDelegationAuthorizationHistoryRequest{
		ActorAdminId:  strconv.FormatInt(adminID, 10),
		TargetStoreId: storeID,
		Page:          strconv.FormatInt(int64(page), 10),
		PageSize:      strconv.FormatInt(int64(pageSize), 10),
		RequestId:     c.GetString("request_id"),
		CorrelationId: c.GetString("trace_id"),
	}, h.Delegation.GrpcDeadlineCurrentMs)
	if err != nil {
		delegationGrpcFailure(c, "查詢代行授權歷程失敗", err)
		return
	}
	if resp.GetCode() != 0 {
		delegationUpstreamDeny(c, resp.GetCode(), resp.GetMessage())
		return
	}

	records := make([]DelegationAuthorizationHistoryEntry, 0, len(resp.GetRecords()))
	for _, r := range resp.GetRecords() {
		records = append(records, toDelegationAuthorizationHistoryEntry(r))
	}
	response.Page(c, records, resp.GetTotal(), resp.GetPage(), resp.GetPageSize())
}
