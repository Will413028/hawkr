package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Will413028/hawkr/gateway/internal/handler/delegation"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

// 本檔是四個 admin_delegated_*.go 的共用底座：請求解析、值域檢查、統一回應形狀。
//
// 值域檢查落在 Gateway 不是為了取代領域服務的驗證——那一層一條都不放寬。它存在的
// 理由只有一個：讓明顯不合法的輸入在**稽核 intent 寫入之前**就失敗。晚一步失敗會
// 留下一列永遠 FAILED 的稽核，配一個看起來像後端壞掉的 500（varchar 超長在
// PostgreSQL 是 22001，不會變成 9260）。

// delegatedIdempotencyKeyRE is the header format (§6.1.0). Rejecting a malformed key
// costs 9260; accepting one would put an unusable value into the uniqueness
// constraint that the entire retry story depends on.
// 值域與 gateway 其餘 mutation 共用同一份（`middleware.IdempotencyKeyPattern`）。
// 這裡不直接掛 middleware.RequireIdempotencyKey：本路徑的違規回應是
// `DelegationInvalidCommand` 而不是 400，換掉會改變代行的對外契約。
// 但**正則只能有一份**——兩份各自維護不會有任何東西轉紅。
var delegatedIdempotencyKeyRE = middleware.IdempotencyKeyPattern

// delegatedVersionRE matches expectedVersion / any numeric id string.
var delegatedVersionRE = regexp.MustCompile(`^\d{1,19}$`)

// delegatedMutationData is the one response shape every delegated mutation returns.
// version is the NEW version after the write — the front end feeds it straight back
// as the next expectedVersion.
type delegatedMutationData struct {
	AuditLogID string `json:"auditLogId"`
	Version    string `json:"version"`
	Resource   any    `json:"resource"`
}

// delegatedRequest is everything the orchestrator needs about the caller that does not
// come from the request body.
type delegatedRequest struct {
	Session        *accountpb.OperationSessionSummary
	StoreID        int64
	IdempotencyKey string
	RequestID      string
	CorrelationID  string
	UserAgent      string
}

// resolveDelegatedRequest reads the session the guard already resolved plus the
// transport-level audit columns. It never re-resolves the credential: a second
// GetCurrentSession would be a second round trip AND a second answer, judged against
// a different now().
//
// requireIdempotency is false for the two GET-shaped delegated actions (the PII
// reveal), which deliberately do not deduplicate — the audit table's unique index is
// partial on idempotency_key IS NOT NULL for exactly that reason.
func resolveDelegatedRequest(c *gin.Context, requireIdempotency bool) (*delegatedRequest, bool) {
	session, ok := middleware.DelegationSession(c)
	if !ok {
		response.BizStatus(c, bizcode.DelegationCredentialInvalid, "代行憑證無效，請重新建立工作階段")
		return nil, false
	}
	storeID, err := strconv.ParseInt(c.Param("store_id"), 10, 64)
	if err != nil || storeID <= 0 {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "店家代號格式不正確")
		return nil, false
	}

	req := &delegatedRequest{
		Session: session,
		StoreID: storeID,
		// ⚠️ 讀 gin context，不是讀 inbound header。TracingMiddleware 已經在每個
		// 請求上決定了這兩個值（有 x-request-id 就沿用、沒有就生一個），並把它們
		// 回寫成 response header。而 inbound 的 `X-Request-Id` 由瀏覽器決定，
		// 實務上永遠不存在——website 的 BFF 刻意只轉送五個 header 名（§4.5 L1040），
		// 這個不在其中。
		//
		// 先前這裡讀 header，結果是 admin_operation_audit_logs.request_id 全表
		// 100%（173/173）為空字串，而同一結構的 correlation_id 因為讀 context 只有
		// 9 列為空 —— 一個必填欄位靜默失去了它唯一的用途（把稽核列接回 gateway
		// 與服務日誌）。§5.1.3 行 1430 定義該欄即 `x-request-id`。
		RequestID:     requestTraceValue(c, "request_id", "X-Request-Id"),
		CorrelationID: requestTraceValue(c, "trace_id", "x-trace-id"),
		UserAgent:     c.GetHeader("User-Agent"),
	}

	key := c.GetHeader("Idempotency-Key")
	if requireIdempotency {
		if !delegatedIdempotencyKeyRE.MatchString(key) {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "Idempotency-Key 格式不正確")
			return nil, false
		}
	}
	req.IdempotencyKey = key
	return req, true
}

// requestTraceValue resolves one of the two audit trace columns.
//
// The gin context is the primary source and the header is only a fallback,
// because TracingMiddleware has already decided the value for this request —
// it reuses an inbound x-request-id when there is one and mints an ID when
// there is not, then echoes both back as response headers. Reading the inbound
// header instead therefore returns "" for every request that did not carry one,
// which on the delegated paths is all of them (§4.5 L1040 fixes the website BFF
// to exactly five forwarded header names, and neither trace header is among
// them). The fallback stays for direct callers that do set the header.
func requestTraceValue(c *gin.Context, contextKey, headerName string) string {
	if v := c.GetString(contextKey); v != "" {
		return v
	}
	return c.GetHeader(headerName)
}

// orchestrator wires the three-step audit sequence for this request.
func (h *Holder) orchestrator() *delegation.Orchestrator {
	return &delegation.Orchestrator{Audit: h.Account, Cfg: h.Delegation}
}

// readDenial builds the Q4 (9275) denial row from this request.
//
// readDenial 建立唯讀拒絕的稽核輸入，保留 request、correlation ID 與 user agent。
func (r *delegatedRequest) readDenial(resourceType, resourceID string, code int32) delegation.ReadDenial {
	return delegation.ReadDenial{
		Session:       r.Session,
		ResourceType:  resourceType,
		ResourceID:    resourceID,
		BizCode:       code,
		RequestID:     r.RequestID,
		CorrelationID: r.CorrelationID,
		UserAgent:     r.UserAgent,
	}
}

// bindDelegatedBody decodes a delegated mutation body with unknown keys REJECTED.
// The allow-list is deny-by-default: silently ignoring an unregistered key would let
// a front end believe it sent something the backend never read.
//
// It also returns the payload hash, which is the only way "same key, different
// payload" (9267) can be detected at all.
func bindDelegatedBody(c *gin.Context, dst any) (payloadHash string, ok bool) {
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "請求內容無法讀取")
		return "", false
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "請求內容格式不正確")
		return "", false
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), true
}

// validateOperationReason enforces the 1–100 character rule after trimming.
//
// Control characters are rejected rather than stripped: the reason is the operator's
// own statement of why they did this, and silently rewriting it would make the audit
// row quote something nobody typed.
func validateOperationReason(c *gin.Context, reason string) (string, bool) {
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "請填寫本次操作的原因")
		return "", false
	}
	if len([]rune(trimmed)) > 100 {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "操作原因不得超過 100 字")
		return "", false
	}
	for _, r := range trimmed {
		if unicode.IsControl(r) {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "操作原因不得包含控制字元")
			return "", false
		}
	}
	return trimmed, true
}

// validateExpectedVersion checks the optimistic-lock token's shape only; whether it
// still matches is the domain service's call (9266).
func validateExpectedVersion(c *gin.Context, version string) bool {
	if !delegatedVersionRE.MatchString(version) {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "expectedVersion 格式不正確")
		return false
	}
	return true
}

// notifySellerOrDefault resolves the tri-state body flag into the explicit boolean the
// audit intent must carry.
//
// The default is true, and it has to be materialised HERE. Leaving the field at its
// proto3 zero value means false, which builds no outbox row, sends the seller nothing,
// and leaves an audit table where every row is present and nothing looks wrong.
func notifySellerOrDefault(v *bool) bool {
	if v == nil {
		return true
	}
	return *v
}

// delegatedPaging parses the shared page / page_size query pair.
//
// Out-of-range is 9260, never a silent clamp: after clamping, the echoed pageSize
// disagrees with the request, and the front end computes its page count from the
// echo — so the paging is wrong with no error anywhere. Default page size is 20 (this
// contract), not the Gateway-wide default of 10.
func delegatedPaging(c *gin.Context) (page, pageSize int32, ok bool) {
	page, pageSize = 1, delegatedDefaultPageSize

	if raw := c.Query("page"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "page 必須是大於 0 的整數")
			return 0, 0, false
		}
		page = int32(n)
	}
	if raw := c.Query("page_size"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < delegatedMinPageSize || n > maxPageSize {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "page_size 必須介於 1 與 50 之間")
			return 0, 0, false
		}
		pageSize = int32(n)
	}
	return page, pageSize, true
}

// delegatedDefaultPageSize is the delegated contract's default. The 1–50 bounds are
// the Gateway-wide minPageSize / maxPageSize constants — the batch items cap of 50
// must come from that same maxPageSize rather than a second literal.
// delegatedMinPageSize 是代行分頁的下界。
//
// 刻意不跟隨 6f9e40403 把全 repo 統一成 clamp 語義：那次改的是「超上限就 reset 成
// default」的九個呼叫點（page_size=9999 回得比 page_size=50 更少，非單調），代行面
// 是第三種語義——明確拒絕並回 9260。ADM 是內部工具，讓越界輸入當場現形，比靜默夾
// 值後回一份「數量對不上請求」的資料更容易發現前端的組參數 bug。
//
// 常數留在本檔而非 constants.go：handler 那份已隨該次重構移除下界，補回去會讓下一個
// 讀 constants.go 的人以為全域仍有下界檢查。
const delegatedMinPageSize = 1

const delegatedDefaultPageSize = 20

// delegatedKeywordMaxRunes bounds the shared fuzzy-search term.
//
// 這不是拍出來的數字：keyword 能比對到的最寬欄位是 `order_items.product_name`／
// `sku_name` 與 `product.name`，三者都是 `VARCHAR(255)`，而 PostgreSQL 的 varchar 長度
// 以字元計 ⇒ 超過 255 個字元的關鍵字對 `ILIKE '%q%'` 恆為 false，永遠比對不到任何一列。
// 上界訂在「還可能有答案」的最後一點，而不是一個憑感覺的小數字——後者會擋掉合法查詢。
const delegatedKeywordMaxRunes = 255

// delegatedKeyword reads the shared keyword filter. 回傳值已去頭尾空白，空字串＝不篩選。
//
// 超長一律 9260，**不截斷**：截斷後送下去的是另一個搜尋條件，回應卻長得像操作者送的那
// 個查完的結果——他會據此以為「查無此單」，而事實是他的查詢從沒被執行過。這與
// delegatedPaging 拒絕靜默夾值是同一條理由。
//
// 控制字元同樣擋在這裡：NUL 在 PostgreSQL 的 text 值裡不合法，讓它走到下游會炸成 500
// 而不是 9260 —— 一個看起來像後端壞掉的錯誤，實際上是輸入問題。
func delegatedKeyword(c *gin.Context) (string, bool) {
	trimmed := strings.TrimSpace(c.Query("keyword"))
	if len([]rune(trimmed)) > delegatedKeywordMaxRunes {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "keyword 不得超過 255 字")
		return "", false
	}
	for _, r := range trimmed {
		if unicode.IsControl(r) {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "keyword 不得包含控制字元")
			return "", false
		}
	}
	return trimmed, true
}

// delegatedDateRange parses the shared date_from / date_to pair (ISO 8601 UTC，
// 空字串＝不篩選)，回傳原字串交由領域服務解析。
//
// 兩個面共用同一份實作：稽核查詢（§6.1.11）與訂單列表（§6.1.10）的契約逐字相同，
// 各寫一份就會在下一次調整值域時只改到一邊，而那種不一致沒有任何東西守得住。
func delegatedDateRange(c *gin.Context) (from, to string, ok bool) {
	from = c.Query("date_from")
	to = c.Query("date_to")
	var fromTime, toTime time.Time
	if from != "" {
		t, err := time.Parse(time.RFC3339, from)
		if err != nil {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "date_from 格式不正確")
			return "", "", false
		}
		fromTime = t
	}
	if to != "" {
		t, err := time.Parse(time.RFC3339, to)
		if err != nil {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "date_to 格式不正確")
			return "", "", false
		}
		toTime = t
	}
	if from != "" && to != "" && fromTime.After(toTime) {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "date_from 不得晚於 date_to")
		return "", "", false
	}
	return from, to, true
}

// denyDelegated renders a Denial from the orchestrator.
func denyDelegated(c *gin.Context, d *delegation.Denial) {
	response.BizStatus(c, d.Code, d.Message)
}

// respondDelegatedMutation renders the unified mutation envelope.
func respondDelegatedMutation(c *gin.Context, result *delegation.Result) {
	response.OK(c, delegatedMutationData{
		AuditLogID: result.AuditLogID,
		Version:    result.Version,
		Resource:   result.Resource,
	})
}

// delegatedProductMutationData 是代行商品建立／更新專屬的回應 data：共用三欄之外多一個
// certificationWarnings（19 §9.5 的適用端點含代行建立／更新）。
//
// 不把欄位加進共用的 delegatedMutationData：那份形狀是每一種代行變更都吐的（訂單、貼文、
// 店面、商品狀態…），加上去等於對每一支端點宣告一個它永遠不會有的欄位。
type delegatedProductMutationData struct {
	AuditLogID string `json:"auditLogId"`
	Version    string `json:"version"`
	Resource   any    `json:"resource"`
	// 本次未被採用的認證輸入；無警告時是空陣列不是省略（19 §9.5）。
	CertificationWarnings []CertificationWarningOutput `json:"certificationWarnings"`
}

// respondDelegatedProductMutation 是 respondDelegatedMutation 的商品版：多帶
// certificationWarnings，其餘三欄逐位元相同（前端既有解析不受影響）。
func respondDelegatedProductMutation(c *gin.Context, result *delegation.Result, warnings []CertificationWarningOutput) {
	if warnings == nil {
		warnings = []CertificationWarningOutput{}
	}
	response.OK(c, delegatedProductMutationData{
		AuditLogID:            result.AuditLogID,
		Version:               result.Version,
		Resource:              result.Resource,
		CertificationWarnings: warnings,
	})
}

// delegationDenialFromCode turns a domain-service business code into a Denial. Codes
// the delegated contract owns pass through unchanged; anything else is surfaced as
// itself so the §6.5.2 mapping keeps working (an unregistered code collapses to 409,
// which is the existing Gateway behaviour).
func delegationDenialFromCode(code int32, message string) *delegation.Denial {
	if code == 0 {
		return nil
	}
	return delegation.Deny(code, message)
}

// upstreamDenial is what a transport failure becomes: 9280 / 424, never 500 / 503 /
// 404. The front end reads 503 and 404 as "此環境尚未部署代行營運服務" and hides the
// entry point permanently (C12).
func upstreamDenial() *delegation.Denial {
	return delegation.Deny(bizcode.DelegationUpstreamUnavailable, "代行服務暫時不可用，請稍後再試")
}

// delegatedBizMsgRE recovers the numeric business code EnvelopeGuardInterceptor
// prefixed onto the gRPC status message.
var delegatedBizMsgRE = regexp.MustCompile(`^\[(\d+)\]\s*(.*)$`)

// denialFromGrpcErr turns a gRPC error from a domain service into a Denial.
//
// 這一步不能省。EnvelopeGuardInterceptor 把「gRPC OK ＋ 非零 envelope code」轉成
// gRPC status error 並在 message 前面加上 "[code] "，所以領域服務的業務碼**永遠**是
// 從 err 這一側回來的，`resp.GetCode() != 0` 那條分支只有 preserveData 的三個碼走得到。
// 把 err 一律翻譯成 9280，會讓「商品需先下架後再編輯（3019）」這種可恢復的業務拒絕
// 變成「代行服務暫時不可用」——操作者看到的是一個叫他稍後再試的錯誤，而稍後再試永遠
// 不會成功。
func denialFromGrpcErr(err error) *delegation.Denial {
	st, ok := grpcstatus.FromError(err)
	if !ok {
		return upstreamDenial()
	}
	switch st.Code() {
	case grpccodes.Unavailable, grpccodes.DeadlineExceeded, grpccodes.Internal, grpccodes.Unknown:
		// 真正的傳輸層失敗：結果未知，回 9280 讓呼叫端以同一把 Idempotency-Key 重試。
		return upstreamDenial()
	}
	if m := delegatedBizMsgRE.FindStringSubmatch(st.Message()); m != nil {
		if n, convErr := strconv.ParseInt(m[1], 10, 32); convErr == nil {
			return delegation.Deny(int32(n), m[2])
		}
	}
	// 沒有數字碼的拒絕：EnvelopeGuardInterceptor 對 HTTP 鏡射碼（400/403/404…）
	// 刻意不加前綴，所以領域服務的通用 400 到這裡只剩一個 gRPC status。翻譯成 9280
	// 會把「參數不合法」講成「服務暫時不可用，請稍後再試」——那是一個對應不到任何
	// 恢復動作的錯誤，而稍後再試永遠不會成功。
	switch st.Code() {
	case grpccodes.InvalidArgument:
		return delegation.Deny(bizcode.DelegationInvalidCommand, st.Message())
	case grpccodes.PermissionDenied, grpccodes.NotFound:
		// 「不屬於這家店」與「不存在」刻意不區分：區分開來就是一個可以拿來枚舉他人
		// 資源的訊號，而代行者本來就不該知道別家店有沒有這個 id。
		return delegation.Deny(bizcode.DelegationResourceNotOwned, st.Message())
	}
	// 9280 的語意是「結果未知」。上游到底回了什麼必須留在 log 裡——message 不透傳給
	// 前端，所以沒有這一行的話，一個落到這裡的碼在任何地方都查不到。
	zap.L().Warn("Delegation.upstream.unmapped",
		zap.String("grpcCode", st.Code().String()), zap.String("grpcMessage", st.Message()))
	return upstreamDenial()
}

// mapDomainItemCode normalises a per-element business code from a batch response.
//
// 批次的逐項 code 不經 gRPC status（整支 RPC 是成功的），所以領域服務的 HTTP 鏡射碼
// （400／403／404）會原樣浮到前端。它們不在 §6.5.1 的封閉碼表內，前端也不透傳
// message ⇒ 操作者拿到一個對應不到任何恢復動作的數字。單筆路徑已由 denialFromGrpcErr
// 收斂，逐項這一半必須做同一件事，否則同一個失敗在兩條路徑上是兩個碼。
func mapDomainItemCode(code int32) int32 {
	switch code {
	case 0:
		return 0
	case 400:
		return bizcode.DelegationInvalidCommand
	case 403, 404:
		return bizcode.DelegationResourceNotOwned
	default:
		return code
	}
}

// joinForDisplay renders a string slice into the projection's scalar form. The
// projection layer is what makes seven formatters enough: everything reaching a
// formatter is already a scalar.
func joinForDisplay(values []string) string {
	return strings.Join(values, "、")
}
