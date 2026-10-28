package handler

import (
	"regexp"
	"strconv"
	"strings"

	gwcodemeta "github.com/Will413028/hawkr/gateway/internal/codemeta"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ==================== DTO ====================

// DelegatedAuditActor / Target are repeated on every audit row rather than derived
// from the session summary. Two independent reasons: ① the names on an audit row are
// the snapshot AT INTENT TIME while the session's are from creation time — a rename
// mid-session makes them differ, and what the audit has to evidence is the wording at
// the moment the action happened; ② the future cross-session audit endpoint reuses
// this exact field set, and there every row's actor/target genuinely differs.
type DelegatedAuditActor struct {
	AdminID     string `json:"adminId"`
	DisplayName string `json:"displayName"`
}

type DelegatedAuditTarget struct {
	StoreID         string `json:"storeId"`
	SellerAccountID string `json:"sellerAccountId"`
	StoreName       string `json:"storeName"`
}

// DelegatedChangedField is the structured field difference the three-column UI reads
// directly. label comes from the backend — admin repo must not build its own
// field→label table, or adding one field means editing two repos and the one that
// gets missed renders an English field name.
type DelegatedChangedField struct {
	Field  string `json:"field"`
	Label  string `json:"label"`
	Before string `json:"before"`
	After  string `json:"after"`
}

// DelegatedAuditLogEntry is one row of GET .../{sessionId}/audit-logs.
//
// ⚠️ beforeSnapshot / afterSnapshot 刻意**沒有 API 出口**。三個理由各自獨立：
// ① 照定版渲染出來的面板不可讀（快照存投影後的原始值：狀態是數字碼、skus／images 是
// 數量、hashtags 是 join 後截斷字串）；② 體積：單列 64 KB × 兩欄 × pageSize 50 ⇒
// 單次回應最壞 6.4 MB，而這支是每次開「代行紀錄」頁就打的；③「可以寫進稽核表」不蘊含
// 「可以回給前端」。爭議舉證走維運 SQL。
type DelegatedAuditLogEntry struct {
	AuditLogID  string  `json:"auditLogId"`
	CreatedAt   string  `json:"createdAt"`
	ConfirmedAt *string `json:"confirmedAt"`

	Actor  DelegatedAuditActor  `json:"actor"`
	Target DelegatedAuditTarget `json:"target"`

	Action      string `json:"action"`
	ActionLabel string `json:"actionLabel"`
	RiskLevel   string `json:"riskLevel"`
	Capability  string `json:"capability"`

	ResourceType     string `json:"resourceType"`
	ResourceID       string `json:"resourceId"`
	ResourceName     string `json:"resourceName"`
	ResourceImageKey string `json:"resourceImageKey"`

	OperationReason string  `json:"operationReason"`
	CaseReference   *string `json:"caseReference"`

	// PENDING 是合法終態之一，如實顯示「結果未確認」、不計入成功數。把它併進成功
	// 樣式後，操作者這一側就再也看不到「結果未確認」，於是 DelegationAuditPending
	// 告警響起時唯一能自助核對的畫面上一列都沒有。
	Result  string `json:"result"`
	BizCode int32  `json:"bizCode"`
	// bizCodeLabel 是 bizCode 的中文說明，查 CodeMeta 錯誤碼表得來。稽核列只存了
	// 碼、沒有存當時的訊息，所以少了這一欄，面板上就只剩「業務碼 9266」——對操作者
	// 與客服等於沒有資訊。查表放在後端而不是前端：前端自建一份碼→文案對照就是第二
	// 份 CodeMeta 鏡像，Markdown 改了文案它不會轉紅，只會從那天起顯示舊字串。
	//
	// 未登錄的碼回空字串（不是「未知錯誤」），由前端 fallback 顯示原碼——顯示原碼
	// 還追得到，一句假的說明則會蓋掉唯一的線索。result 非 FAILED 時 bizCode 恆為 0，
	// 本欄也恆為空。
	BizCodeLabel string `json:"bizCodeLabel"`

	ResourceVersionBefore string                  `json:"resourceVersionBefore"`
	ResourceVersionAfter  string                  `json:"resourceVersionAfter"`
	ChangedFields         []DelegatedChangedField `json:"changedFields"`

	CorrelationID  string `json:"correlationId"`
	RequestID      string `json:"requestId"`
	IdempotencyKey string `json:"idempotencyKey"`
	Origin         string `json:"origin"`
	// userAgent 回 user_agent 欄的原文，不解析。後端代為解析等於在稽核表存一份會隨
	// UA 慣例過時的衍生值；要顯示成「Chrome 126 (macOS)」是前端的事。
	UserAgent string `json:"userAgent"`
	// 語意是「代行完成事件已發布至 MQ」，不是「賣家已收到」——回一個永遠 DELIVERED
	// 的假值會讓稽核紀錄在爭議時反過來咬人。notifySeller=false 時為 null。
	NotifiedAt *string `json:"notifiedAt"`
}

// DelegatedSessionSummary backs BOTH summary endpoints. One shape on purpose: the
// front end parses it once.
type DelegatedSessionSummary struct {
	Session     DelegationSessionData `json:"session"`
	SessionID   string                `json:"sessionId"`
	DisplayCode string                `json:"displayCode"`
	Target      DelegatedAuditTarget  `json:"target"`
	Actor       DelegatedAuditActor   `json:"actor"`
	IssuedAt    string                `json:"issuedAt"`
	EndedAt     *string               `json:"endedAt"`
	// durationSeconds 取四個候選的最小值 − issued_at：ended_at、閒置逾時點、
	// expires_at、now。
	//
	// 閒置那一項取 last_active_at 本身、不是 last_active_at + idle：授權窗口確實延續
	// 到加了寬限的那一刻，但這一欄回答的是稽核問題「這次代行實際持續多久」，而
	// last_active_at 之後什麼都沒發生。且它**只在真的閒置逾時後才是候選** —— 寫成
	// 無條件的話 end 恆等於 last_active_at，「操作到最後一刻才被撤銷」的工作階段
	// 時長會歸零。
	//
	// now 那一項不可省：三個候選同時落在未來只發生在一種列上（還在進行中），少了它
	// 那一類列會取到未來的 expires_at ⇒ 剛簽發 30 秒的工作階段回報整個 TTL，把上界
	// 當成值。
	//
	// 取「四者最小」而不是「挑第一個命中」：後者在「閒置逾時之後才被撤銷」的列上會
	// 答撤銷時刻而不是最後活動時刻。SSOT 是 docs/modules/delegation.md §3 的規則，實作在
	// 單體的 internal/modules/identity/delegation_query.go（DelegationSessionSummary）。
	DurationSeconds string `json:"durationSeconds"`
	Status          string `json:"status"`

	OperationCount     string `json:"operationCount"`
	RiskOperationCount string `json:"riskOperationCount"`
	// pendingAuditCount 決定結束 modal 的「稽核狀態」那一列，不是固定文案：只有 "0"
	// 才可顯示「均已寫入稽核紀錄」。寫死成全綠，操作者就永遠不會在他最該去對帳的
	// 那一刻知道自己這次代行有寫入結果未確認。
	PendingAuditCount string `json:"pendingAuditCount"`
	NotifiedSeller    bool   `json:"notifiedSeller"`

	// 這兩個結構上恆為 "0"：代行 capability 封閉列舉裡根本沒有履約／財務能力。它們
	// 正是 UI「未執行任何出貨或財務操作」那句宣稱的資料來源，而且**與有沒有 PENDING
	// 無關**——綁在一起會讓一次 PENDING 殘留把一句永遠為真的安全宣稱變成紅色。
	// 一旦非零，代表三層鎖定被打穿，屬 P0 事故訊號。
	FulfillmentActionCount string `json:"fulfillmentActionCount"`
	FinanceActionCount     string `json:"financeActionCount"`

	// 設定值下發（單位：天），不是每列存一份。前端不得把「2 年」寫死——保留期是
	// k8s values 改值即生效的設定，硬寫的那一刻它就成了第三個各自為政的數字。
	AuditRetentionDays string `json:"auditRetentionDays"`

	// 撤銷四件套（#2483 A9）。未撤銷時四欄皆為 JSON null，**不得加 omitempty**——
	// 欄位消失（undefined）與「值為 null」在前端是兩種不同的判斷。
	//
	// 撤銷理由在 UI 是必填的，卻從來沒有一條路徑把它讀回來。強制收集一個永遠不會被
	// 顯示的欄位，只會訓練操作者亂填，而它正是爭議時唯一能舉證「為什麼中止」的字串。
	RevokedAt *string `json:"revokedAt"`
	// revokedByAdminDisplayName 填的是 admin_accounts.email——該表沒有顯示名欄位，
	// 與 actor.displayName 同一個來源。
	RevokedByAdminID          *string `json:"revokedByAdminId"`
	RevokedByAdminDisplayName *string `json:"revokedByAdminDisplayName"`
	RevokeReason              *string `json:"revokeReason"`
}

// DelegatedOperationSessionRecord 是跨工作階段列表的一列（§11.3 P1）。
//
// 欄位集合＝DelegationSessionData 的子集 ＋ 撤銷四件套。刻意**沒有 revision**：樂觀鎖
// 是工作區那一側的東西，治理列表拿到它只會被讀成版本號。也絕不含 operationCredential
// 或 handoffCode——它們連 session 的投影都不是。
type DelegatedOperationSessionRecord struct {
	SessionID   string               `json:"sessionId"`
	DisplayCode string               `json:"displayCode"`
	Target      DelegatedAuditTarget `json:"target"`
	Actor       DelegatedAuditActor  `json:"actor"`
	Reason      DelegationReasonData `json:"reason"`
	// 與 DelegationSessionData.caseReference 同一條規則：未設定時輸出 JSON null，
	// 不得 omitempty。
	CaseReference *string  `json:"caseReference"`
	Capabilities  []string `json:"capabilities"`
	IssuedAt      string   `json:"issuedAt"`
	ExpiresAt     string   `json:"expiresAt"`
	EndedAt       *string  `json:"endedAt"`
	// Status 恰三值：active / expired / revoked。由 delegation 模組以單一 now() 對
	// 整頁逐列推導——同一頁的兩列若各自取時間，邊界上會出現兩個不同的答案。
	Status                    string  `json:"status"`
	RevokedAt                 *string `json:"revokedAt"`
	RevokedByAdminID          *string `json:"revokedByAdminId"`
	RevokedByAdminDisplayName *string `json:"revokedByAdminDisplayName"`
	RevokeReason              *string `json:"revokeReason"`
}

// ==================== A｜audit-logs ====================

// ListDelegatedAuditLogs handles GET /api/v1/admin/delegated-operations/:session_id/audit-logs.
// Mounted with requireAdmin(): this is a governance query over any session, and the
// ADM session is its identity (9261 on failure, never 9274).
func (h *Holder) ListDelegatedAuditLogs(c *gin.Context) {
	sessionID := c.Param("session_id")
	if !delegatedVersionRE.MatchString(sessionID) {
		response.BizStatus(c, bizcode.DelegationSessionNotFound, "找不到指定的代行工作階段")
		return
	}
	page, pageSize, ok := delegatedPaging(c)
	if !ok {
		return
	}
	action, ok := auditFilterValue(c, "action", gwcodemeta.IsKnownDelegationAuditAction)
	if !ok {
		return
	}
	resourceType, ok := auditFilterValue(c, "resource_type", func(v string) bool {
		_, known := gwcodemeta.DelegationResourceTypes[v]
		return known
	})
	if !ok {
		return
	}
	result, ok := auditFilterValue(c, "result", func(v string) bool {
		_, known := gwcodemeta.DelegationAuditResults[v]
		return known
	})
	if !ok {
		return
	}
	dateFrom, dateTo, ok := delegatedDateRange(c)
	if !ok {
		return
	}
	if h.Account == nil {
		// 9289 而不是 9280（#2744）：未接線是永久失敗，重試不會好。見 admin_delegated_media.go
		// 同形分支的完整理由。
		zap.L().Error("Delegation.dependency.unwired", zap.String("dep", "Account"), zap.String("path", c.FullPath()))
		response.BizStatus(c, bizcode.DelegationInternalContractMismatch, "代行營運服務設定異常，重試無效")
		return
	}

	resp, err := h.Account.ListDelegationAuditLogs(c.Request.Context(), &accountpb.ListAuditLogsRequest{
		SessionId:          sessionID,
		ActionFilter:       action,
		ResourceTypeFilter: resourceType,
		ResultFilter:       result,
		DateFrom:           dateFrom,
		DateTo:             dateTo,
		Page:               strconv.FormatInt(int64(page), 10),
		PageSize:           strconv.FormatInt(int64(pageSize), 10),
	}, h.Delegation.GrpcDeadlineAuditIntentMs)
	if err != nil {
		delegationGrpcFailure(c, "查詢代行稽核紀錄失敗", err)
		return
	}
	if resp.GetCode() != 0 {
		delegationUpstreamDeny(c, resp.GetCode(), resp.GetMessage())
		return
	}

	records := make([]DelegatedAuditLogEntry, 0, len(resp.GetRecords()))
	for _, entry := range resp.GetRecords() {
		records = append(records, delegatedAuditEntryDTO(entry))
	}
	response.Page(c, records, resp.GetTotal(), resp.GetPage(), resp.GetPageSize())
}

func delegatedAuditEntryDTO(e *accountpb.AuditLogEntry) DelegatedAuditLogEntry {
	changed := make([]DelegatedChangedField, 0, len(e.GetChangedFields()))
	for _, cf := range e.GetChangedFields() {
		changed = append(changed, DelegatedChangedField{
			Field:  cf.GetField(),
			Label:  cf.GetLabel(),
			Before: cf.GetBefore(),
			After:  cf.GetAfter(),
		})
	}
	out := DelegatedAuditLogEntry{
		AuditLogID: e.GetAuditLogId(),
		CreatedAt:  e.GetCreatedAt(),
		Actor: DelegatedAuditActor{
			AdminID:     e.GetActor().GetAdminId(),
			DisplayName: e.GetActor().GetDisplayName(),
		},
		Target: DelegatedAuditTarget{
			StoreID:         e.GetTarget().GetStoreId(),
			SellerAccountID: e.GetTarget().GetSellerAccountId(),
			StoreName:       e.GetTarget().GetStoreName(),
		},
		Action:                e.GetAction(),
		ActionLabel:           e.GetActionLabel(),
		RiskLevel:             e.GetRiskLevel(),
		Capability:            e.GetCapability(),
		ResourceType:          e.GetResourceType(),
		ResourceID:            e.GetResourceId(),
		ResourceName:          e.GetResourceName(),
		ResourceImageKey:      e.GetResourceImageKey(),
		OperationReason:       e.GetOperationReason(),
		Result:                e.GetResult(),
		BizCode:               e.GetBizCode(),
		BizCodeLabel:          bizcode.Message(e.GetBizCode()),
		ResourceVersionBefore: e.GetResourceVersionBefore(),
		ResourceVersionAfter:  e.GetResourceVersionAfter(),
		ChangedFields:         changed,
		CorrelationID:         e.GetCorrelationId(),
		RequestID:             e.GetRequestId(),
		IdempotencyKey:        e.GetIdempotencyKey(),
		Origin:                e.GetOrigin(),
		UserAgent:             e.GetUserAgent(),
	}
	if e.ConfirmedAt != nil {
		v := e.GetConfirmedAt()
		out.ConfirmedAt = &v
	}
	if e.CaseReference != nil {
		v := e.GetCaseReference()
		out.CaseReference = &v
	}
	if e.NotifiedAt != nil {
		v := e.GetNotifiedAt()
		out.NotifiedAt = &v
	}
	return out
}

// ==================== B｜{sessionId}/summary ====================

// GetDelegatedSessionSummary handles GET .../{sessionId}/summary (requireAdmin).
// It stays a governance query over ANY session and must NOT be widened to also accept
// a credential: one endpoint belongs to one identity group, and merging them would
// hand credential holders cross-session read access — trading an authorization
// boundary for a UI convenience.
func (h *Holder) GetDelegatedSessionSummary(c *gin.Context) {
	sessionID := c.Param("session_id")
	if !delegatedVersionRE.MatchString(sessionID) {
		response.BizStatus(c, bizcode.DelegationSessionNotFound, "找不到指定的代行工作階段")
		return
	}
	h.respondSessionSummary(c, sessionID)
}

// ==================== C｜current/summary ====================

// GetCurrentDelegatedSessionSummary handles GET .../current/summary.
//
// 這一支是結束摘要綠卡的唯一資料來源。工作區在 www host 上，那裡沒有 ADM session ⇒
// B 那一支打不到；少了本支，綠卡的每一個數字都取不到，而前端唯一還能做的事就是把它
// 寫死——包括「未執行任何出貨或財務操作」那一句，那句話一旦與兩個計數脫鉤，就從 P0
// 事故訊號退化成永遠顯示綠色的裝飾文案。
//
// sessionId **不從 URL 來**，由 credential 反查：credential 持有者若能自行指定
// sessionId，就讀得到別人的代行紀錄，而跨工作階段回看是治理權限、不是代行權限。
//
// counts_as_activity = false：它在工作階段即將結束或已結束時才被呼叫，算成活動只會
// 把 last_active_at 往後推，而那之後不會再有任何操作。
//
// 終態必須可讀：ended／expired／revoked 一律回 200 ＋ 完整摘要。改成 fail-closed 的
// 9270／9271 等於綠卡永遠是空的。
func (h *Holder) GetCurrentDelegatedSessionSummary(c *gin.Context) {
	credential := c.GetHeader(headerDelegationOperationCredential)
	if credential == "" {
		response.BizStatus(c, bizcode.DelegationCredentialInvalid, "代行憑證無效，請重新建立工作階段")
		return
	}
	if h.Account == nil {
		// 9289 而不是 9280（#2744）：未接線是永久失敗，重試不會好。見 admin_delegated_media.go
		// 同形分支的完整理由。
		zap.L().Error("Delegation.dependency.unwired", zap.String("dep", "Account"), zap.String("path", c.FullPath()))
		response.BizStatus(c, bizcode.DelegationInternalContractMismatch, "代行營運服務設定異常，重試無效")
		return
	}

	current, err := h.Account.GetCurrentDelegationSession(c.Request.Context(), credential, false, h.Delegation.GrpcDeadlineCurrentMs)
	if err != nil {
		delegationGrpcFailure(c, "查詢當前代行工作階段失敗", err)
		return
	}
	if current.GetCode() != 0 {
		// cookie 保留、不宣稱已結束：憑證解不開才是 9274，工作階段的終態不是。
		response.BizStatus(c, current.GetCode(), current.GetMessage())
		return
	}
	sessionID := current.GetData().GetSessionId()
	if sessionID == "" {
		response.BizStatus(c, bizcode.DelegationCredentialInvalid, "代行憑證無效，請重新建立工作階段")
		return
	}
	h.respondSessionSummary(c, sessionID)
}

// headerDelegationOperationCredential mirrors the router's header constant. The two
// are separate declarations because handler must not import router; the value is the
// contract, and the contract test asserts they agree.
const headerDelegationOperationCredential = "X-HK-Operation-Session"

func (h *Holder) respondSessionSummary(c *gin.Context, sessionID string) {
	if h.Account == nil {
		// 9289 而不是 9280（#2744）：未接線是永久失敗，重試不會好。見 admin_delegated_media.go
		// 同形分支的完整理由。
		zap.L().Error("Delegation.dependency.unwired", zap.String("dep", "Account"), zap.String("path", c.FullPath()))
		response.BizStatus(c, bizcode.DelegationInternalContractMismatch, "代行營運服務設定異常，重試無效")
		return
	}
	resp, err := h.Account.GetDelegationSessionSummary(c.Request.Context(), sessionID, h.Delegation.GrpcDeadlineCurrentMs)
	if err != nil {
		delegationGrpcFailure(c, "查詢代行工作階段摘要失敗", err)
		return
	}
	if resp.GetCode() != 0 {
		delegationUpstreamDeny(c, resp.GetCode(), resp.GetMessage())
		return
	}
	d := resp.GetData()
	if d == nil || d.GetSession() == nil {
		delegationUpstreamFailure(c, "查詢代行工作階段摘要回應不完整", nil)
		return
	}
	out := DelegatedSessionSummary{
		Session:     toDelegationSessionData(d.GetSession()),
		SessionID:   d.GetSessionId(),
		DisplayCode: d.GetDisplayCode(),
		Target: DelegatedAuditTarget{
			StoreID:         d.GetTarget().GetStoreId(),
			SellerAccountID: d.GetTarget().GetSellerAccountId(),
			StoreName:       d.GetTarget().GetStoreName(),
		},
		Actor: DelegatedAuditActor{
			AdminID:     d.GetActor().GetAdminId(),
			DisplayName: d.GetActor().GetDisplayName(),
		},
		IssuedAt:               d.GetIssuedAt(),
		DurationSeconds:        d.GetDurationSeconds(),
		Status:                 d.GetStatus(),
		OperationCount:         d.GetOperationCount(),
		RiskOperationCount:     d.GetRiskOperationCount(),
		PendingAuditCount:      d.GetPendingAuditCount(),
		NotifiedSeller:         d.GetNotifiedSeller(),
		FulfillmentActionCount: d.GetFulfillmentActionCount(),
		FinanceActionCount:     d.GetFinanceActionCount(),
		AuditRetentionDays:     d.GetAuditRetentionDays(),
	}
	if d.EndedAt != nil {
		v := d.GetEndedAt()
		out.EndedAt = &v
	}
	if d.RevokedAt != nil {
		v := d.GetRevokedAt()
		out.RevokedAt = &v
	}
	if d.RevokedByAdminId != nil {
		v := d.GetRevokedByAdminId()
		out.RevokedByAdminID = &v
	}
	if d.RevokedByAdminDisplayName != nil {
		v := d.GetRevokedByAdminDisplayName()
		out.RevokedByAdminDisplayName = &v
	}
	if d.RevokeReason != nil {
		v := d.GetRevokeReason()
		out.RevokeReason = &v
	}
	response.OK(c, out)
}

// ==================== D｜跨工作階段治理列表（§11.3 P1） ====================

// ListDelegatedOperations 列出代行工作階段（跨工作階段治理查詢）
// GET /api/v1/admin/delegated-operations
// @Summary      代行工作階段列表
// @Description  治理回看，唯讀。回答「這段期間誰代行了哪些店」，取代原本只能由後端工程師代跑的維運 SQL。刻意**不提供 status 篩選**：DB 只存 active／revoked，expired 是讀取當下依到期與閒置推導的，把那條推導再寫一次進查詢就有了兩份實作而沒有任何東西守得住；要看已到期的列請讀回應的 status。時間區間比對 issuedAt。
// @Tags         代行營運 (Delegated Operations)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        actor_admin_id  query  string  false  "精確比對某一位操作者的 admin id"
// @Param        store_id        query  string  false  "精確比對某一家店的 store_id"
// @Param        display_code    query  string  false  "精確比對工作階段代號，形如 OPS-0000000ABCDEF"
// @Param        date_from       query  string  false  "起始時間，ISO 8601 UTC（比對 issuedAt）"
// @Param        date_to         query  string  false  "結束時間，ISO 8601 UTC（比對 issuedAt）"
// @Param        page            query  string  false  "頁碼，預設 1"
// @Param        page_size       query  string  false  "每頁筆數，1–50，預設 20（越界回 9260，不靜默夾值）"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]handler.DelegatedOperationSessionRecord,total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Failure      424  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/delegated-operations [get]
func (h *Holder) ListDelegatedOperations(c *gin.Context) {
	page, pageSize, ok := delegatedPaging(c)
	if !ok {
		return
	}
	actorAdminID, ok := delegationIDFilter(c, "actor_admin_id")
	if !ok {
		return
	}
	storeID, ok := delegationIDFilter(c, "store_id")
	if !ok {
		return
	}
	displayCode := c.Query("display_code")
	if displayCode != "" && !delegatedDisplayCodeRE.MatchString(displayCode) {
		delegationDeny(c, bizcode.DelegationInvalidCommand, "display_code 格式不正確")
		return
	}
	dateFrom, dateTo, ok := delegatedDateRange(c)
	if !ok {
		return
	}
	if !delegationAuthorizationUpstream(h, c, "代行工作階段列表：上游不可用") {
		return
	}

	resp, err := h.Account.ListDelegationSessions(c.Request.Context(), &accountpb.ListSessionsRequest{
		ActorAdminId:  actorAdminID,
		TargetStoreId: storeID,
		DisplayCode:   displayCode,
		DateFrom:      dateFrom,
		DateTo:        dateTo,
		Page:          strconv.FormatInt(int64(page), 10),
		PageSize:      strconv.FormatInt(int64(pageSize), 10),
	}, h.Delegation.GrpcDeadlineCurrentMs)
	if err != nil {
		delegationGrpcFailure(c, "查詢代行工作階段列表失敗", err)
		return
	}
	if resp.GetCode() != 0 {
		delegationUpstreamDeny(c, resp.GetCode(), resp.GetMessage())
		return
	}

	records := make([]DelegatedOperationSessionRecord, 0, len(resp.GetRecords()))
	for _, r := range resp.GetRecords() {
		records = append(records, delegatedSessionRecordDTO(r))
	}
	response.Page(c, records, resp.GetTotal(), resp.GetPage(), resp.GetPageSize())
}

// ListAllDelegatedAuditLogs 跨工作階段的代行稽核紀錄
// GET /api/v1/admin/delegated-operations/audit-logs
// @Summary      跨工作階段代行稽核紀錄
// @Description  治理回看，唯讀。依店家或操作者跨工作階段查稽核列，列的形狀與 `/{sessionId}/audit-logs` 完全相同（同一份 parser）。beforeSnapshot／afterSnapshot 一律不出現在回應，爭議舉證走維運 SQL。時間區間比對 createdAt。
// @Tags         代行營運 (Delegated Operations)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        actor_admin_id  query  string  false  "精確比對某一位操作者的 admin id"
// @Param        store_id        query  string  false  "精確比對某一家店的 store_id"
// @Param        action          query  string  false  "動作篩選，單值；值域為 DelegationAuditAction"
// @Param        resource_type   query  string  false  "資源類型篩選，單值"  Enums(storefront, product, post, order)
// @Param        result          query  string  false  "結果篩選，單值"  Enums(PENDING, SUCCEEDED, FAILED)
// @Param        date_from       query  string  false  "起始時間，ISO 8601 UTC（比對 createdAt）"
// @Param        date_to         query  string  false  "結束時間，ISO 8601 UTC（比對 createdAt）"
// @Param        page            query  string  false  "頁碼，預設 1"
// @Param        page_size       query  string  false  "每頁筆數，1–50，預設 20（越界回 9260，不靜默夾值）"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]handler.DelegatedAuditLogEntry,total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Failure      424  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/delegated-operations/audit-logs [get]
func (h *Holder) ListAllDelegatedAuditLogs(c *gin.Context) {
	page, pageSize, ok := delegatedPaging(c)
	if !ok {
		return
	}
	actorAdminID, ok := delegationIDFilter(c, "actor_admin_id")
	if !ok {
		return
	}
	storeID, ok := delegationIDFilter(c, "store_id")
	if !ok {
		return
	}
	action, ok := auditFilterValue(c, "action", gwcodemeta.IsKnownDelegationAuditAction)
	if !ok {
		return
	}
	resourceType, ok := auditFilterValue(c, "resource_type", func(v string) bool {
		_, known := gwcodemeta.DelegationResourceTypes[v]
		return known
	})
	if !ok {
		return
	}
	result, ok := auditFilterValue(c, "result", func(v string) bool {
		_, known := gwcodemeta.DelegationAuditResults[v]
		return known
	})
	if !ok {
		return
	}
	dateFrom, dateTo, ok := delegatedDateRange(c)
	if !ok {
		return
	}
	if !delegationAuthorizationUpstream(h, c, "跨工作階段代行稽核紀錄：上游不可用") {
		return
	}

	resp, err := h.Account.ListAllDelegationAuditLogs(c.Request.Context(), &accountpb.ListAllAuditLogsRequest{
		ActorAdminId:       actorAdminID,
		TargetStoreId:      storeID,
		ActionFilter:       action,
		ResourceTypeFilter: resourceType,
		ResultFilter:       result,
		DateFrom:           dateFrom,
		DateTo:             dateTo,
		Page:               strconv.FormatInt(int64(page), 10),
		PageSize:           strconv.FormatInt(int64(pageSize), 10),
	}, h.Delegation.GrpcDeadlineAuditIntentMs)
	if err != nil {
		delegationGrpcFailure(c, "查詢跨工作階段代行稽核紀錄失敗", err)
		return
	}
	if resp.GetCode() != 0 {
		delegationUpstreamDeny(c, resp.GetCode(), resp.GetMessage())
		return
	}

	records := make([]DelegatedAuditLogEntry, 0, len(resp.GetRecords()))
	for _, entry := range resp.GetRecords() {
		records = append(records, delegatedAuditEntryDTO(entry))
	}
	response.Page(c, records, resp.GetTotal(), resp.GetPage(), resp.GetPageSize())
}

// delegatedDisplayCodeRE mirrors chk_dos_display_code_format. Rejecting a malformed
// code with 9260 rather than passing it through matters: an unvalidated typo comes
// back as an empty page, which reads as "no such session" instead of "you typed it
// wrong".
var delegatedDisplayCodeRE = regexp.MustCompile(`^OPS-[0-9A-Z]{13}$`)

// delegationIDFilter reads one optional numeric-id filter, empty string meaning no
// filter. Malformed is 9260, never silently dropped — a dropped filter hands back
// every session on the platform while the operator believes he narrowed to one store.
func delegationIDFilter(c *gin.Context, name string) (string, bool) {
	raw := c.Query(name)
	if raw == "" {
		return "", true
	}
	if !delegatedVersionRE.MatchString(raw) {
		delegationDeny(c, bizcode.DelegationInvalidCommand, name+" 格式不正確")
		return "", false
	}
	return raw, true
}

func delegatedSessionRecordDTO(r *accountpb.DelegatedOperationSessionRecord) DelegatedOperationSessionRecord {
	out := DelegatedOperationSessionRecord{
		SessionID:   r.GetSessionId(),
		DisplayCode: r.GetDisplayCode(),
		Target: DelegatedAuditTarget{
			StoreID:         r.GetTarget().GetStoreId(),
			SellerAccountID: r.GetTarget().GetSellerAccountId(),
			StoreName:       r.GetTarget().GetStoreName(),
		},
		Actor: DelegatedAuditActor{
			AdminID:     r.GetActor().GetAdminId(),
			DisplayName: r.GetActor().GetDisplayName(),
		},
		Reason: DelegationReasonData{
			Code:                  r.GetReason().GetCode(),
			Label:                 r.GetReason().GetLabel(),
			CaseReferenceRequired: r.GetReason().GetCaseReferenceRequired(),
		},
		Capabilities: append([]string{}, r.GetCapabilities()...),
		IssuedAt:     r.GetIssuedAt(),
		ExpiresAt:    r.GetExpiresAt(),
		Status:       r.GetStatus(),
	}
	if r.CaseReference != nil {
		v := r.GetCaseReference()
		out.CaseReference = &v
	}
	if r.EndedAt != nil {
		v := r.GetEndedAt()
		out.EndedAt = &v
	}
	if r.RevokedAt != nil {
		v := r.GetRevokedAt()
		out.RevokedAt = &v
	}
	if r.RevokedByAdminId != nil {
		v := r.GetRevokedByAdminId()
		out.RevokedByAdminID = &v
	}
	if r.RevokedByAdminDisplayName != nil {
		v := r.GetRevokedByAdminDisplayName()
		out.RevokedByAdminDisplayName = &v
	}
	if r.RevokeReason != nil {
		v := r.GetRevokeReason()
		out.RevokeReason = &v
	}
	return out
}

// ==================== query 值域 ====================

// auditFilterValue reads one single-valued filter.
//
// 單值，不支援逗號分隔：proto 的三個 filter 都是單一 string 且註解明寫「空字串 =
// 不篩選」，要支援多值就得改成 repeated，那是契約變更而非實作細節。出現逗號一律
// 9260，理由與「值不在值域內回 9260」相同——靜默忽略會讓前端以為篩選生效而實際回全集。
func auditFilterValue(c *gin.Context, name string, valid func(string) bool) (string, bool) {
	raw := c.Query(name)
	if raw == "" {
		return "", true
	}
	if strings.Contains(raw, ",") {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, name+" 不支援多值")
		return "", false
	}
	if !valid(raw) {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, name+" 不在合法值域內")
		return "", false
	}
	return raw, true
}

// date_from／date_to 的解析在 admin_delegated_common.go 的 delegatedDateRange——它與
// 訂單列表（§6.1.10）共用同一份契約。
