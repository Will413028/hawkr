// Package procurement holds the parts of the Hawkr 幫找 (procurement) HTTP wire
// contract that BOTH gateway/internal/handler (the 18 unary/JSON handlers) and
// gateway/internal/proxy (the SSE stream translator) have to produce
// identically.
//
// It exists because of a hard import direction: internal/handler imports
// internal/proxy (for proxy.ServeAIAssistantStream), so internal/proxy can
// never import internal/handler back. Before this package, the TurnResult→JSON
// mapping and the gRPC-code→HTTP-status table both lived in internal/handler,
// which put them out of reach of the stream path — and the stream path needs
// EXACTLY those two things:
//
//   - SubmitTurnStream's terminal AG-UI frames (STATE_SNAPSHOT + RUN_FINISHED)
//     carry the same TurnResult the unary SubmitTurn returns, so the JSON a
//     client sees for one turn must not depend on which transport delivered it.
//   - the stream's AG-UI RUN_ERROR frame carries the same code the unary error
//     envelope carries (an HTTP status, not a raw gRPC code). Note the wire
//     types differ: the unary envelope's `code` is a JSON number, RUN_ERROR's
//     is that same status decimal-formatted into a string (agUIRunError.Code
//     is a Go string) — the value is identical, the JSON type is not.
//
// The alternative — a local copy inside internal/proxy — was rejected
// precisely because "the two transports drifted apart" is the defect class
// this package exists to close; a second copy of the mapping is a second place
// for it to happen. Everything here is pure data + pure functions (no gin, no
// gRPC client), so it can be imported from either side without cycles.
package procurement

import (
	"strconv"

	pb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// BusinessErrorFromStatus 取出一個 procurement gRPC error 附帶的 BusinessError
// detail（ai-service internal/grpc/procurement_errors.go 的 procurementStatus／
// procurementQuotaExceededStatus 一律附一個）。沒有 detail 時回 nil——12 支
// AIAssistantService streaming RPC 與純傳輸層失敗（Unavailable／
// DeadlineExceeded）走同一條錯誤路徑，它們的 status 就是這一種。
//
// 放在這個 package 而不是 internal/handler，理由與 GRPCCodeToHTTPStatus 完全相同
// （見 package doc）：unary 錯誤 envelope 與 stream 的 RUN_ERROR frame 必須從同一
// 份 detail 取出同一組欄位，而 internal/proxy 無法 import internal/handler。
// AGENT_MONTHLY_QUOTA_EXCEEDED 的 remaining／reset_at（doc 57 §4.7）就是靠這裡
// 同時餵到兩條路徑上；只有 unary 讀得到 detail 的話，SSE 那半的買家永遠看不到
// 什麼時候重置。
//
// 這仍然是 package doc 說的「pure data + pure functions」：status 只是錯誤型別，
// 不是 gRPC client。
func BusinessErrorFromStatus(st *status.Status) *pb.BusinessError {
	if st == nil {
		return nil
	}
	for _, d := range st.Details() {
		if be, ok := d.(*pb.BusinessError); ok {
			return be
		}
	}
	return nil
}

// GRPCCodeToHTTPStatus 是 procurement 所有 18 支 RPC 共用的固定 bucket 對照表
// （spec §6）——不是 per-RPC 客製，粗粒度分類走 gRPC code，機器碼細節走
// BusinessError.reason。
func GRPCCodeToHTTPStatus(code codes.Code) int {
	switch code {
	case codes.InvalidArgument:
		return 400
	case codes.Unauthenticated:
		return 401
	case codes.PermissionDenied:
		return 403
	case codes.NotFound:
		return 404
	case codes.Aborted:
		return 409
	case codes.FailedPrecondition:
		return 422
	case codes.ResourceExhausted:
		// 429. Gateway 的另一條錯誤路徑早就這樣映射了——AI 額度上限的業務碼 4291
		// 走 envelope_guard_interceptor 轉成 ResourceExhausted，再由
		// response.GrpcDomainErr 交出 429（見 handler/ai.go 的 ProxyAiTransform）。
		// 這張表少了這個 case，等於同一個 gateway 對同一個 gRPC code 依端點給出
		// 兩種 HTTP status：其他 AI 端點回 429，procurement 回 500。
		return 429
	case codes.Unavailable:
		return 503
	case codes.OutOfRange:
		// 410. doc57 §2.5-6／CodeMeta REPLAY_CURSOR_EXPIRED：GET replay 端點的
		// Last-Event-ID 早於這個 run 目前可 replay 的 minAvailableSeq——那段
		// ledger 已經不在保留視窗內，同一個 cursor 重試永遠不會成功（語意上
		// 等同 HTTP 410 Gone，不是可重試的錯誤）。ai-service 端把這個情境映射成
		// codes.OutOfRange（services/ai-service/internal/grpc/
		// procurement_run_events_handler.go replayCursorExpiredStatus），刻意跟
		// REPLAY_CURSOR_AHEAD 用的 codes.Aborted（→ 409，見上面 case）分開——
		// 呼叫端要能單靠 gRPC code 分辨「永遠不會好」跟「重新整理後可能會好」。
		// 這個 case 先前缺席，OutOfRange 會落 default 變成 500，讓 Task 12 的
		// replay 端點把一個明確、可判斷的錯誤誤判成無法分類的系統錯誤。
		return 410
	default:
		return 500
	}
}

// ---- Slots (shared by TurnResult and SnapshotResponse) ----

// SlotsDTO mirrors ai-service's domain.Slots JSON shape exactly: snake_case
// keys, Quantity/BudgetPerUnit as JSON numbers (not the proto wire's string
// representation — api/proto/ai/ai.proto's Slots message uses
// `optional string quantity`/`budget_per_unit` purely as a cross-language wire
// convention; ai-service's own gRPC handler converts them from domain.Slots'
// *int with strconv.Itoa on the way out, see
// services/ai-service/internal/grpc/procurement_handler.go toSlotsProto).
type SlotsDTO struct {
	Item               *string `json:"item,omitempty"`
	Quantity           *int    `json:"quantity,omitempty"`
	Unit               *string `json:"unit,omitempty"`
	Packaging          *string `json:"packaging,omitempty"`
	BudgetPerUnit      *int    `json:"budget_per_unit,omitempty"`
	DeliveryCity       *string `json:"delivery_city,omitempty"`
	DeliveryDate       *string `json:"delivery_date,omitempty"`
	ExactMatchRequired *bool   `json:"exact_match_required,omitempty"`
	// ItemResolution 是品項的解析結果——買家原話被判定成分類體系裡的
	// 哪一層，以及那個判定是誰做的。它跟 item 原話同一個 revision（ADR
	// 2026-09-04-crop-resolution-into-slots），所以住在 slots 而不是另一個頂層欄位。
	ItemResolution *ItemResolutionDTO `json:"item_resolution,omitempty"`
}

// ItemResolutionDTO 沿用本檔 SlotsDTO 的 snake_case（它整組刻意鏡射
// ai-service domain.Slots 的 JSON 形狀，見上方註解），不是本 repo 對外
// camelCase 的例外——它就在那個既有例外的內部。
//
// 相對 domain.ItemResolution 少一個 fallback_group_code：那是 crop 查無行情時
// 退一級重查大類的內部查詢鍵，買家不需要也無從使用（同 proto 端的決定）。
// 多一個 display_value：伺服器組好的顯示字串，見 DisplayValue 欄位註解。
type ItemResolutionDTO struct {
	// CATEGORY_CONFIRMED 或 BUYER_DESCRIBED。
	Kind string `json:"kind"`
	// 分類名稱（「甘藍-初秋」「甘藍」），BUYER_DESCRIBED 時為買家的描述。
	Label string `json:"label,omitempty"`
	// 目前只有 BUYER_CHOICE（買家在消歧卡上親自挑的）。
	Provenance string `json:"provenance"`
	// DisplayValue 是伺服器組好的顯示字串。前端不得自己用 kind＋label 重拼——
	// recordedThisTurn 的 chip 用的就是同一個值，兩邊各拼一次會漂成兩套文案，
	// 而它們在畫面上相鄰。
	DisplayValue string `json:"display_value"`
}

func itemResolutionFromProto(r *pb.ItemResolution) *ItemResolutionDTO {
	if r == nil {
		return nil
	}
	out := &ItemResolutionDTO{
		Kind: r.GetKind(), Label: r.GetLabel(),
		Provenance: r.GetProvenance(), DisplayValue: r.GetDisplayValue(),
	}
	return out
}

// parseSlotInt parses ai-service's itoa'd Quantity/BudgetPerUnit wire values
// back into int. Pulled out to a named function purely so the two call sites
// below read the same way (both "parse the wire string, keep the field nil
// on failure").
func parseSlotInt(s string) (int, error) {
	return strconv.Atoi(s)
}

func SlotsFromProto(s *pb.Slots) SlotsDTO {
	if s == nil {
		return SlotsDTO{}
	}
	out := SlotsDTO{
		Item: s.Item, Unit: s.Unit, Packaging: s.Packaging,
		DeliveryCity: s.DeliveryCity, DeliveryDate: s.DeliveryDate,
		ExactMatchRequired: s.ExactMatchRequired,
		ItemResolution:     itemResolutionFromProto(s.GetItemResolution()),
	}
	// Quantity/BudgetPerUnit come back over the wire as decimal strings
	// (itoa'd by ai-service); convert back to *int so the outward JSON is an
	// unquoted number, matching domain.Slots' *int fields exactly. A parse
	// failure here would mean ai-service sent something that isn't the
	// decimal integer string it always produces — left nil rather than
	// panicking or failing the whole response over a non-critical display field.
	if s.Quantity != nil {
		if v, err := parseSlotInt(*s.Quantity); err == nil {
			out.Quantity = &v
		}
	}
	if s.BudgetPerUnit != nil {
		if v, err := parseSlotInt(*s.BudgetPerUnit); err == nil {
			out.BudgetPerUnit = &v
		}
	}
	return out
}

func MissingRequiredFromProto(keys []string) []string {
	// Never nil: MissingRequired/Rejected/RecordedThisTurn all document the
	// same "always an array, possibly []" invariant on the ai-service side
	// (see usecase.TurnResult's RecordedThisTurn doc comment) — a nil slice
	// here would marshal as JSON null instead of [].
	out := make([]string, 0, len(keys))
	out = append(out, keys...)
	return out
}

// ---- TurnResult ----

type RejectedFieldDTO struct {
	Key    string `json:"key"`
	Reason string `json:"reason"`
}

func RejectedFieldsFromProto(fields []*pb.RejectedField) []RejectedFieldDTO {
	out := make([]RejectedFieldDTO, 0, len(fields))
	for _, f := range fields {
		out = append(out, RejectedFieldDTO{Key: f.GetKey(), Reason: f.GetReason()})
	}
	return out
}

type RecordedFieldDTO struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value string `json:"value"`
}

func RecordedFieldsFromProto(fields []*pb.RecordedField) []RecordedFieldDTO {
	out := make([]RecordedFieldDTO, 0, len(fields))
	for _, f := range fields {
		out = append(out, RecordedFieldDTO{Key: f.GetKey(), Label: f.GetLabel(), Value: f.GetValue()})
	}
	return out
}

type BlockDTO struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

type EvidenceRefDTO struct {
	URL   string `json:"url"`
	Label string `json:"label,omitempty"`
}

// DialoguePlanSummaryDTO 是 issue #2549 §F 的 dialogueActCode/planHash/
// policyVersion/renderMode/templateVersion 五個欄位打包成一個巢狀物件——JSON
// key 用 §F 原文的 "dialogueActCode" 而非巢狀物件裡的 "actCode"，扁平化到
// 頂層以外的欄位，避免跟 wire 上其餘扁平欄位風格不一致。
type DialoguePlanSummaryDTO struct {
	DialogueActCode string `json:"dialogueActCode"`
	PlanHash        string `json:"planHash"`
	PolicyVersion   string `json:"policyVersion"`
	RenderMode      string `json:"renderMode"`
	TemplateVersion string `json:"templateVersion"`
}

func BlocksFromProto(blocks []*pb.Block) []BlockDTO {
	out := make([]BlockDTO, 0, len(blocks))
	for _, b := range blocks {
		out = append(out, BlockDTO{Kind: b.GetKind(), Text: b.GetText()})
	}
	return out
}

func EvidenceRefsFromProto(refs []*pb.EvidenceRef) []EvidenceRefDTO {
	out := make([]EvidenceRefDTO, 0, len(refs))
	for _, r := range refs {
		out = append(out, EvidenceRefDTO{URL: r.GetUrl(), Label: r.GetLabel()})
	}
	return out
}

func DialoguePlanSummaryFromProto(p *pb.DialoguePlanSummary) DialoguePlanSummaryDTO {
	return DialoguePlanSummaryDTO{
		DialogueActCode: p.GetActCode(), PlanHash: p.GetPlanHash(),
		PolicyVersion: p.GetPolicyVersion(), RenderMode: p.GetRenderMode(),
		TemplateVersion: p.GetTemplateVersion(),
	}
}

// AgentQuotaSnapshotDTO 是買家會員分級的月配額快照（proto AgentQuotaSnapshot，
// Task 10）——PlanCode/Remaining/Limit 沿用 proto 的字串型別（避免 JS 數字精度
// 問題），ResetAt 是 RFC3339 UTC 字串，原樣帶出。
type AgentQuotaSnapshotDTO struct {
	PlanCode  string `json:"planCode"`
	Remaining string `json:"remaining"`
	Limit     string `json:"limit"`
	ResetAt   string `json:"resetAt"`
}

// AgentQuotaSnapshotFromProto 跟 DialoguePlanSummaryFromProto 同一套慣例：不做
// 顯式 nil 檢查——protoc-gen-go 產生的 GetXxx() 在 receiver 為 nil 時本來就回傳零值
// （見 pb.AgentQuotaSnapshot 的 GetPlanCode 等方法），所以 q 為 nil（Task 13：
// idempotent replay 時 ai-service 不 populate Quota）時這裡自然拿到零值
// AgentQuotaSnapshotDTO，不會 panic。
func AgentQuotaSnapshotFromProto(q *pb.AgentQuotaSnapshot) AgentQuotaSnapshotDTO {
	return AgentQuotaSnapshotDTO{
		PlanCode: q.GetPlanCode(), Remaining: q.GetRemaining(), Limit: q.GetLimit(), ResetAt: q.GetResetAt(),
	}
}

type StepDTO struct {
	Code   string `json:"code"`
	Status string `json:"status"`
	Label  string `json:"label"`
	// Text 是這一步的 LLM 敘述（例如「甘藍條件有效，無拒絕欄位。」），跟固定的
	// Label 是兩回事。narrator 逾時或被 guard 擋下時為空——消費端必須能退回
	// Label，不能假設一定有值。
	Text       string            `json:"text,omitempty"`
	DurationMs int64             `json:"durationMs"`
	SafeArgs   map[string]string `json:"safeArgs,omitempty"`
}

// StepsFromProto's result is assigned into a `,omitempty` field — len==0 is
// omitted by encoding/json regardless of nil-ness, so returning a non-nil
// empty slice here is only for internal consistency with the other From-proto
// helpers, not required for the omitempty behavior itself.
func StepsFromProto(steps []*pb.Step) []StepDTO {
	out := make([]StepDTO, 0, len(steps))
	for _, s := range steps {
		out = append(out, StepDTO{Code: s.GetCode(), Status: s.GetStatus(), Label: s.GetLabel(), Text: s.GetText(), DurationMs: s.GetDurationMs(), SafeArgs: s.GetSafeArgs()})
	}
	return out
}

type QuestionOptionDTO struct {
	OptionID string `json:"optionId"`
	Label    string `json:"label"`
	Value    string `json:"value"`
}

func QuestionOptionsFromProto(opts []*pb.QuestionOption) []QuestionOptionDTO {
	out := make([]QuestionOptionDTO, 0, len(opts))
	for _, o := range opts {
		out = append(out, QuestionOptionDTO{OptionID: o.GetOptionId(), Label: o.GetLabel(), Value: o.GetValue()})
	}
	return out
}

// ClarifyQuestionDTO's Question field in TurnResultDTO is a
// *ClarifyQuestionDTO with NO omitempty (see below) — the old TurnResult.Question
// field is deliberately not omitempty either, so a nil question must serialize
// as JSON null, not be absent.
type ClarifyQuestionDTO struct {
	SlotKey       string              `json:"slotKey"`
	Prompt        string              `json:"prompt"`
	Options       []QuestionOptionDTO `json:"options"`
	UnitOptions   []QuestionOptionDTO `json:"unitOptions,omitempty"`
	AllowFreeText bool                `json:"allowFreeText"`
	Source        string              `json:"source"`
	CanSkip       bool                `json:"canSkip"`
}

func ClarifyQuestionFromProto(q *pb.ClarifyQuestion) *ClarifyQuestionDTO {
	if q == nil {
		return nil
	}
	return &ClarifyQuestionDTO{
		SlotKey: q.GetSlotKey(), Prompt: q.GetPrompt(),
		Options:       QuestionOptionsFromProto(q.GetOptions()),
		UnitOptions:   QuestionOptionsFromProto(q.GetUnitOptions()),
		AllowFreeText: q.GetAllowFreeText(), Source: q.GetSource(), CanSkip: q.GetCanSkip(),
	}
}

// TurnResultDTO mirrors usecase.TurnResult's JSON tags field for field
// (services/ai-service/internal/procurement/usecase/turn.go) — camelCase at
// this level (unlike the nested Slots), quoted-string IDs, and the same
// omitempty set (DegradedReason/Steps/NextAction omitted when empty; Question
// never omitted, explicit null when there is none). The proto's reasoning
// field no longer exists to map: #2447 BE-01A stopped populating it and then
// reserved the field number and name in ai.proto, so re-adding it is a protoc
// error. wire_test.go's DropsReasoning sentinel stays as the output-shape
// guard — that a `reasoning` key never appears here from any other source.
//
// This is the shape BOTH the unary SubmitTurn/SubmitAnswer/Rewind responses'
// `data` and SubmitTurnStream's terminal AG-UI frames carry — the latter emits
// it twice per turn, once as STATE_SNAPSHOT's `snapshot` and once as
// RUN_FINISHED's `result` (proxy/ai_assistant_stream.go, StreamEvent_TurnResult
// case). Before the AG-UI migration the retired ai-service HTTP handler
// produced the terminal frame as json.Marshal(usecase.TurnResult)
// (procurement/handler/stream.go), i.e. the exact same bytes as its own unary
// response body; keeping that byte-for-byte equality is why this type lives
// here rather than in internal/handler.
//
// NextAction (issue #2403 Slice 4, Task 11/13): the buyer-facing next-step
// hint. Additive, omitempty — empty string (no hint) does not add the key,
// matching the ai-service proto/usecase field's own omitempty semantics.
type TurnResultDTO struct {
	RequestID        string              `json:"requestId"`
	ConversationID   string              `json:"conversationId"`
	RevisionID       string              `json:"revisionId"`
	Slots            SlotsDTO            `json:"slots"`
	Rejected         []RejectedFieldDTO  `json:"rejected"`
	MissingRequired  []string            `json:"missingRequired"`
	RecordedThisTurn []RecordedFieldDTO  `json:"recordedThisTurn"`
	Degraded         bool                `json:"degraded"`
	DegradedReason   string              `json:"degradedReason,omitempty"`
	Reply            string              `json:"reply"`
	Steps            []StepDTO           `json:"steps,omitempty"`
	Question         *ClarifyQuestionDTO `json:"question"`
	NextAction       string              `json:"nextAction,omitempty"`
	// ---- #2549 Phase 2：PublicAgentMessageV1 wire 化，additive ----
	Blocks       []BlockDTO             `json:"blocks,omitempty"`
	TextFallback string                 `json:"textFallback,omitempty"`
	EvidenceRefs []EvidenceRefDTO       `json:"evidenceRefs,omitempty"`
	DialoguePlan DialoguePlanSummaryDTO `json:"dialoguePlan"`
	MessageID    string                 `json:"messageId,omitempty"`
	CreatedAt    string                 `json:"createdAt,omitempty"`
	// Quota（Task 15）：買家會員分級的月配額快照。ai-service 只在真的有 snapshot
	// 時 populate proto 的 Quota（idempotent replay 時是 nil，見 Task 13）——
	// AgentQuotaSnapshotFromProto 對 nil 是安全的，這裡拿到零值 DTO 而非 panic。
	Quota AgentQuotaSnapshotDTO `json:"quota"`
}

func TurnResultFromProto(r *pb.TurnResult) TurnResultDTO {
	return TurnResultDTO{
		RequestID: r.GetRequestId(), ConversationID: r.GetConversationId(), RevisionID: r.GetRevisionId(),
		Slots:            SlotsFromProto(r.GetSlots()),
		Rejected:         RejectedFieldsFromProto(r.GetRejected()),
		MissingRequired:  MissingRequiredFromProto(r.GetMissingRequired()),
		RecordedThisTurn: RecordedFieldsFromProto(r.GetRecordedThisTurn()),
		Degraded:         r.GetDegraded(), DegradedReason: r.GetDegradedReason(), Reply: r.GetReply(),
		Steps:        StepsFromProto(r.GetSteps()),
		Question:     ClarifyQuestionFromProto(r.GetQuestion()),
		NextAction:   r.GetNextAction(),
		Blocks:       BlocksFromProto(r.GetBlocks()),
		TextFallback: r.GetTextFallback(),
		EvidenceRefs: EvidenceRefsFromProto(r.GetEvidenceRefs()),
		DialoguePlan: DialoguePlanSummaryFromProto(r.GetDialoguePlan()),
		MessageID:    r.GetMessageId(),
		CreatedAt:    r.GetCreatedAt(),
		Quota:        AgentQuotaSnapshotFromProto(r.GetQuota()),
	}
}
