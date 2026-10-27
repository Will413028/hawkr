package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// degradedReason 是 LLM 抽取降級時回給使用者的固定說法。抽成常數是因為冪等重播
// 也要能重建出同一句話——answer_patch 只存 degraded 布林，不存這段文案。
const degradedReason = "需求解析暫時無法使用，已保留你目前的需求內容"

// RevisionStore 是 turn 需要的持久層能力（由 repository.Repo 實作）。
// 抽成 interface 讓編排邏輯能用記憶體替身單測。
//
// 刻意不含 SetRequestStatus：狀態隨 AppendRevision 同一次交易原子寫入
// （repository.AppendInput.RequestStatus），Handle 正常流程不會、也不應該再另外
// 呼叫一次 SetRequestStatus 補寫（見本檔 Handle 內「狀態要在 AppendRevision 之前
// 就算好」那段註解的完整理由，以及
// TestHandle_SetsRequirementReadyWhenAllRequiredSlotsFilled 的 setStatusCalls
// 斷言）。*repository.Repo 仍保留它自己的 SetRequestStatus 方法（revision.go），
// 供整合測試 setup 直接呼叫；只是不再透過這個 interface 曝露。
type RevisionStore interface {
	CreateRequest(ctx context.Context, buyerID identity.BuyerID) (repository.Request, repository.Revision, error)
	AppendRevision(ctx context.Context, in repository.AppendInput) (repository.Revision, error)
	GetSlots(ctx context.Context, requestID int64, buyerID identity.BuyerID) (domain.Slots, error)
	// GetSnapshot 供 cropidentity.ComputeGate（cropidentity/crop_gate.go）讀取
	// CropDisambiguationOptionsJSON——GetSlots 只回傳 domain.Slots，拿不到
	// procurement_requests 上的品項解析欄位。*repository.Repo 已經實作這個方法
	// （revision.go，AnswerStore／RewindStore 也是用它），這裡只是把既有方法一併
	// 暴露進這個窄介面。
	GetSnapshot(ctx context.Context, requestID int64, buyerID identity.BuyerID) (repository.Request, repository.Revision, error)
	FindByIdempotencyKey(ctx context.Context, buyerID identity.BuyerID, clientRequestID string) (repository.IdempotentResult, error)
	ListBranchRevisions(ctx context.Context, requestID int64, buyerID identity.BuyerID) ([]repository.RevisionWithPatch, error)
}

// turnPayload 是 answer_patch.payload_json 的形狀。寫入與冪等重播讀回共用同一個
// 定義，避免兩端各自寫一份 key 字面值而 drift——重播要能還原出與首次完全同形狀的
// 回應（含 rejected、degraded、reply），否則「重送拿到的東西跟第一次一樣」這個
// 承諾只在部分欄位成立，客戶端仍得分辨自己拿到的是首次還是重播。
// （舊資料的 reasoning key 於 #2447 BE-01A 移除欄位後解碼時自動忽略；歷史清理
// 見 BE-01C。）
type turnPayload struct {
	Message  string                 `json:"message"`
	Patch    domain.SlotPatch       `json:"patch"`
	Rejected []domain.RejectedField `json:"rejected"`
	Degraded bool                   `json:"degraded"`
	Reply    string                 `json:"reply"`
	// Blocks 是 Phase 2 新增的結構化回覆——RenderPlan 現在回傳 []domain.Block，
	// Reply 改由 domain.TextFallback(blocks) 組出，供舊消費端／wire 相容用；additive。
	Blocks []domain.Block `json:"blocks,omitempty"`
	// RenderMode 記這一輪的 blocks 是模型潤飾（"MODEL"）還是 deterministic
	// template（"TEMPLATE"）產生——#2549 Phase 3 新增，additive。冪等重播
	// 需要還原這個值（見 replayResult），否則重播會把 Phase 2 舊列的空值
	// 誤判成某個合法狀態。
	RenderMode string        `json:"renderMode,omitempty"`
	Steps      []domain.Step `json:"steps,omitempty"`
	// RecordedThisTurn 供冪等重播還原——見 replayResult()。
	RecordedThisTurn []domain.RecordedField `json:"recordedThisTurn,omitempty"`
	// Question 是這一輪結束後算出的下一題（含 nil，即 REQUIREMENT_READY 之後
	// pickNextSlot 也沒東西可問的情況）——冪等重播要能還原出跟首次一樣的值。
	Question *domain.ClarifyQuestion `json:"question,omitempty"`
	// Skip 只在 /answers 的 skip:true 路徑非 nil，記錄買家跳過的 slotKey——
	// deriveSkippedSlots 靠這個欄位累積「已被跳過」集合（Task 5）。
	Skip *domain.SlotKey `json:"skip,omitempty"`
	// OptionalStreak 是「這一輪的 question 決定之後」的連續選填提問計數，寫入時
	// 由 computeOptionalStreak 依六規則明確算出（不是從歷史反推）——下一輪呼叫
	// PickNextQuestion 時，previousOptionalStreak 直接讀這個欄位當作
	// previousStreak 輸入。這是本檔案兩輪 bug（under-count／over-count，見
	// computeOptionalStreak doc comment）修好後的設計：streak 是隨每一輪「正向
	// 算出並持久化」的顯式狀態，不再需要在讀取端對歷史做任何猜測性的反向掃描。
	// 0 是合法值也是零值，omitempty 省下的位元組不影響語意（缺欄位與顯式 0
	// 解碼結果相同）。
	OptionalStreak int `json:"optionalStreak,omitempty"`
	// DialoguePlan 是這一輪 PlanDialogue 的完整輸出（Task 3），供冪等重播還原——
	// 跟 Reply／Question／RecordedThisTurn 同一類「這一輪命令產生的內容」，見
	// spec §4 的完整理由（延伸既有機制，不另開持久化路徑）。
	DialoguePlan *domain.DialoguePlanV1 `json:"dialoguePlan,omitempty"`
}

type TurnInput struct {
	BuyerID         identity.BuyerID
	RequestID       int64 // 0 表示建立新 request
	Message         string
	ClientRequestID string
}

type TurnResult struct {
	RequestID       int64                  `json:"requestId,string"`
	ConversationID  int64                  `json:"conversationId,string"`
	RevisionID      int64                  `json:"revisionId,string"`
	Slots           domain.Slots           `json:"slots"`
	Rejected        []domain.RejectedField `json:"rejected"`
	MissingRequired []domain.SlotKey       `json:"missingRequired"`
	// RecordedThisTurn 是三段式回覆①確認段（issue #2343 Ask 5）：這一輪新記下
	// 的欄位與內容，伺服器固定 label。不加 omitempty——跟 Rejected／
	// MissingRequired 同款式，永遠回陣列（可能是空的 []），不回 null。
	RecordedThisTurn []domain.RecordedField `json:"recordedThisTurn"`
	Degraded         bool                   `json:"degraded"`
	DegradedReason   string                 `json:"degradedReason,omitempty"`
	// Reply 是給使用者看的自然語言回覆，取代舊版由前端模板組出的「已更新：.../缺：...」
	// 文字——這個欄位有值之後，前端不再需要自己組字。Phase 2 起由
	// domain.TextFallback(blocks) 組出，不再是模板函式直接回傳的字面字串。
	Reply string `json:"reply"`
	// Blocks 是 Phase 2 新增的結構化回覆（PublicAgentMessageV1.blocks[]）——
	// additive，不影響既有只讀 Reply 的消費端。
	Blocks []domain.Block `json:"blocks,omitempty"`
	// RenderMode 對映 turnPayload.RenderMode，toTurnResultProto 讀這個欄位
	// 填進 aipb.DialoguePlanSummary.RenderMode（#2549 Phase 3，取代 Phase 2
	// 寫死的 "TEMPLATE" 常量）。
	RenderMode string `json:"-"`
	// Steps 是這一輪的白名單步驟（doc 55；issue #2343 Ask 2b）。伺服器固定
	// label，永不回填模型原文。
	Steps []domain.Step `json:"steps,omitempty"`
	// Question 是引導式釐清問題卡（issue #2343 Ask 3）。不用 omitempty——缺欄位
	// 時給物件、就緒且 pickNextSlot 沒東西可問時明確回 null，比照前端 mock 契約
	// （hawkr-website tests/e2e/procurement-assistant-clarify.spec.ts）。
	Question *domain.ClarifyQuestion `json:"question"`
	// NextAction 是給前端的下一步提示（issue #2403 Slice 4，Task 11）：additive
	// 新欄位，空字串表示無提示，不影響既有消費端（前端目前尚未讀取這個欄位）。
	// 目前唯一取值是 REQUIREMENT_READY 且尚未有 price preview 記錄時的
	// "REVIEW_PRICE_PREVIEW"，導引買家去呼叫 Task 7 的報價試算端點。刻意不在
	// replayResult() 裡還原這個欄位——冪等重播的 payload（turnPayload）沒有持久化
	// 它，重播永遠回空字串；這個提示是「要不要去看報價試算」的即時狀態，不是這一輪
	// 命令本身產生的內容，用重播的舊值取代目前狀態反而可能過期。
	NextAction string `json:"next_action,omitempty"`
	// DialoguePlan 是這一輪的完整 plan（Task 3）。Phase 2 起 toTurnResultProto
	// （procurement_handler.go）把 ActCode／PlanHash／PolicyVersion／RenderMode／
	// TemplateVersion 五個欄位投影進 aipb.DialoguePlanSummary 對外；其餘欄位
	// （AllowedClaimCodes／ForbiddenClaimCodes／StateVersion 等）仍只給 Go 內部
	// 消費（renderer、regression test），不上 wire。
	DialoguePlan domain.DialoguePlanV1 `json:"-"`
	// Quota 是這一輪扣抵之後的額度餘量（doc 59）。冪等重播路徑不填這個欄位
	// （replayResult 沒有來源可以還原它——turnPayload 沒有持久化額度快照，
	// 而重播本來就沒有產生新的扣抵），維持零值。
	//
	// json:"-" 的理由同 RenderMode／DialoguePlan：對外投影走
	// toTurnResultProto（procurement_handler.go）→ aipb.TurnResult.quota
	// （AgentQuotaSnapshot，api/proto 已有欄位），不走這個 struct 自己的
	// json.Marshal。那條 marshal 路徑今天只剩 internal/grpc 的
	// RetiredDoneFrameJSON 這個「退役 HTTP handler 當年實際寫出什麼位元組」的
	// 歷史錨點在用，退役 handler 當年不可能寫出 quota 欄位，讓這個新欄位進去
	// 會把那個錨點的宣稱變成假的（也會連帶弄髒 gateway 端逐字複製的那份常數）。
	Quota QuotaSnapshot `json:"-"`
}

// TurnEventKind 是 HandleStream 產生的事件種類。
type TurnEventKind string

const (
	TurnEventStep   TurnEventKind = "step"
	TurnEventResult TurnEventKind = "result" // 終態事件，一定是序列裡最後一個
)

// TurnEvent 是 HandleStream 的單一輸出。Kind==TurnEventStep 時 Step 有效，
// Kind==TurnEventResult 時 Result 有效——兩者互斥，呼叫端用 Kind 判斷讀哪一個。
type TurnEvent struct {
	Kind   TurnEventKind
	Step   domain.Step // Status 是 ProcurementStepStatus 的 RUNNING 或 SUCCEEDED
	Result TurnResult
}

// PricePreviewChecker 是 turn 需要的「這個 request 有沒有 price preview 記錄」
// 查詢能力（由 *repository.Repo 的 HasPricePreview 實作，Task 11）。抽成介面的理由
// 是讓既有的 turn 單元測試能傳 nil，不必為了一個提示旗標替
// 每個既有測試準備替身（見本檔 nil 的處理與 resolveCropIfItemChanged 的 nil 慣例）。
type PricePreviewChecker interface {
	HasPricePreview(ctx context.Context, requestID int64) (bool, error)
}

// RfqConfirmationReader 是 PlanDialogue 需要的 RFQ 確認新鮮度判別式來源——
// 由 *repository.Repo 的 FindLatestConfirmedRfqSnapshotByRequestID（Task 2）
// 實作。抽成介面的理由跟 PricePreviewChecker 相同：讓既有的
// turn 單元測試能傳 nil。
type RfqConfirmationReader interface {
	FindLatestConfirmedRfqSnapshotByRequestID(ctx context.Context, requestID int64, buyerID identity.BuyerID) (repository.ConfirmedRfqSnapshot, error)
}

// resolveRfqState derives the three-way RFQ confirmation freshness judgment
// (Sub-project 3b, Task 4 dedup) — extracted verbatim from HandleStream's own
// derivation (turn.go, formerly lines 577-589) and graph_dispatch.go's
// byte-for-byte duplicate (formerly resolveGraphTurnState's inline block) —
// both call sites now call this one function. tipRevisionID is the turn's
// starting active revision (history's last entry — ListBranchRevisions
// guarantees oldest-to-newest ordering); rfqConfirmationReader nil or a
// query error both degrade to RfqNone rather than blocking the turn.
func resolveRfqState(ctx context.Context, rfqConfirmationReader RfqConfirmationReader, requestID int64, buyerID identity.BuyerID, history []repository.RevisionWithPatch) domain.RfqConfirmationState {
	if rfqConfirmationReader == nil || len(history) == 0 {
		return domain.RfqNone
	}
	tipRevisionID := history[len(history)-1].ID
	snap, err := rfqConfirmationReader.FindLatestConfirmedRfqSnapshotByRequestID(ctx, requestID, buyerID)
	switch {
	case err == nil && snap.RequestRevisionID == tipRevisionID:
		return domain.RfqConfirmed
	case err == nil:
		return domain.RfqStale
	default:
		return domain.RfqNone
	}
}

// Entitlement 是 buyer-service 解析出來的買家目前方案（對應 buyer.Entitlement）。
// 在這裡重新宣告一份而不是 import client package，是本檔既有的窄介面慣例
// （同 PricePreviewChecker）：usecase 只依賴自己宣告的形狀，
// 綁定發生在組合根（procurement/module.go）。
type Entitlement struct {
	PlanCode           string
	MonthlyAnswerLimit int
	PeriodStart        time.Time
	PeriodEnd          time.Time
}

// EntitlementClient 解析買家目前的 Agent 額度方案。
type EntitlementClient interface {
	GetBuyerEntitlement(ctx context.Context, buyerID identity.BuyerID) (Entitlement, error)
}

// QuotaReserver 是「以買家為軸的月額度」預約／釋放能力（由 *repository.Repo
// 實作，見 internal/procurement/repository/quota.go）。
//
// 刻意沒有 Confirm：確認必須跟 revision／answer_patch 的持久化在同一個 DB 交易裡
// （doc 57 §7.5 point 3），所以它不是這一層可以獨立呼叫的動作，而是
// RevisionStore.AppendRevision 透過 repository.AppendInput 的 Quota* 欄位在自己的
// 交易內完成的。usecase 唯一要做的是把 reservationID／periodStart 送進去。
type QuotaReserver interface {
	ReserveQuota(ctx context.Context, buyerID identity.BuyerID, periodStart time.Time, limit int) (reservationID int64, reserved bool, confirmedBefore int, err error)
	ReleaseQuota(ctx context.Context, reservationID int64, buyerID identity.BuyerID)
}

// QuotaExceededError 表示買家本月的 Agent 額度已用完。用具名型別而不是 sentinel
// error：呼叫端（gateway handler）要把 Limit／ResetAt 映射到對外的錯誤 payload。
type QuotaExceededError struct {
	Limit   int
	ResetAt time.Time
}

func (e *QuotaExceededError) Error() string {
	return fmt.Sprintf("本月 Agent 額度已用完（0/%d），%s 重置", e.Limit, e.ResetAt.Format("2006-01-02"))
}

// QuotaSnapshot 是這一輪結束後回給前端的額度餘量快照。
type QuotaSnapshot struct {
	PlanCode  string    `json:"planCode"`
	Remaining int       `json:"remaining"`
	Limit     int       `json:"limit"`
	ResetAt   time.Time `json:"resetAt"`
}

// ErrRunResultUnavailable 表示 claim 命中了既有冪等鍵（同一個命令重送），但那次
// 執行還沒有留下可重播的結果——最可能的情況是前一次呼叫的背景執行仍在進行中，
// 也可能是它以 FAILED 收尾、從未寫出 revision。
//
// 刻意不在這裡「重跑一次 pipeline 當作沒事」：那正是 ledger 要消滅的行為（同一
// 個 clientRequestID 打兩次模型、兩次寫入）。也刻意不回一個空的 TurnResult：呼叫
// 端分辨不出「這是重播的空結果」還是「這一輪真的沒產生內容」。誠實的回應是告訴
// 呼叫端「這個命令已經被接受了，但結果還沒好，請稍後再取」——Task 12 的
// WatchRunEvents／GET replay 落地之後，這個情境會有真正的訂閱端點可以等；在那之前
// 呼叫端唯一能做的就是重試。
var ErrRunResultUnavailable = errors.New("procurement: run accepted but its result is not available yet")

// errTurnPipelinePanicked 是背景執行 panic 時送回呼叫端的哨兵錯誤——panic 本身
// 會被重新拋出交給 syncutil.SafeGo 記錄（見 ClaimAndDispatchTurn 內的 defer），
// 呼叫端拿到的是這個具名 error，而不是永遠等不到的結果。
var errTurnPipelinePanicked = errors.New("procurement: turn pipeline panicked")

// SubmitTurnResult 是 claim+dispatch 包裝後，SubmitTurn／SubmitTurnStream 兩個
// gRPC 入口共用的回傳形狀。
type SubmitTurnResult struct {
	// RunID 是 ledger（procurement_agent_runs）產生的 run 身分，供 gateway 組
	// AG-UI RUN_STARTED／RUN_FINISHED 的 runId，取代目前充當該值的 trace_id
	// （spec §6 落差 2）。
	RunID int64
	// Result 是這次命令的終態結果，與 flag 關閉時 uc.Handle 回傳的東西同形狀、
	// 同內容——unary 入口直接把它轉成 proto 回去。
	Result TurnResult
	// Events 是這次執行 HandleStream 時收集到的完整事件序列（每個 step 事件
	// ＋最後的 result 事件），順序與 HandleStream 原本 yield 的順序逐一相同。
	// 串流入口靠它把既有的 StepEvent／MessageEvent／TurnResult 三種 frame 照樣
	// 送出去；unary 入口不讀這個欄位。
	//
	// ⚠️ 本輪（Task 7）串流的即時性因此暫時降級：這些事件會等整條 pipeline 跑完
	// 才一次送出，不像 flag 關閉時那樣邊跑邊送。Task 12 接上 WatchRunEvents 訂閱
	// 之後會改回真正的即時轉發。
	Events []TurnEvent
	// InterruptID／InterruptKind 非空時，這次呼叫的終態是 WAITING_FOR_INPUT
	// （品項消歧待選，見 interruptOutcomeFor），不是一般完成。兩個 gRPC 入口都把
	// 它們放進 TurnResult proto 的 interrupt_id／interrupt_kind 欄位交給 gateway。
	InterruptID   string
	InterruptKind string
}

// turnDispatchResult 是背景 goroutine 回送給呼叫端的完整結果（含錯誤）——用一個
// 具名型別而不是匿名 struct，讓 defer 裡的「保證送出一個結果」寫得出來。
type turnDispatchResult struct {
	outcome turnPipelineOutcome
	err     error
}

// canonicalTurnHash 是 SUBMIT_TURN 這個命令的冪等 payload hash（doc 57 §6.4-2：
// 同 key 同 hash 回原 run，同 key 不同 hash 回 409）。
//
// 刻意只取 BuyerID＋Message，不含 RequestID：客戶端沒收到回應而重送時，第一次
// 可能不帶 requestId（新建語意）、重送時帶著剛拿到的 requestId，這在既有語意裡是
// 同一個命令的重送（見 HandleStream 開頭的冪等檢查：`in.RequestID != 0 &&
// in.RequestID != prev.RequestID` 才算衝突）。把 RequestID 放進 hash 會讓這種
// 完全合法的重送變成 409。「同 key 但打到別的 request」這個真正的衝突仍然被擋
// 下——由 replayExistingRun 沿用 HandleStream 既有的那一行判斷處理。
//
// Message 則必須在 hash 裡：同一個 clientRequestID 配不同訊息是真正的鍵重用，
// 靜默回放舊訊息的結果等於讓買家這句話憑空消失。這比既有行為嚴格（既有路徑會
// 直接回放），是 ledger 刻意收緊的一點。
func canonicalTurnHash(in TurnInput) string {
	return canonicalHash(struct {
		BuyerID identity.BuyerID
		Message string
	}{in.BuyerID, in.Message})
}

// hashDialoguePlan 對 plan 的 canonical JSON 序列化結果算 sha256——跟
// domain 層 slots_hash 的既有算法一致（見 spec §2 的既有先例，這裡沿用同一種
// 「canonical JSON + sha256」做法，不是新演算法）。放在 usecase 層而非 domain
// 層：domain.PlanDialogue 產生 plan 內容本身，但 hash 需要在 StateVersion 等
// 欄位填完之後才算「這次完整的 plan」，順序上更適合放在呼叫端。
func hashDialoguePlan(plan domain.DialoguePlanV1) string {
	b, _ := json.Marshal(plan) // plan 是已知欄位的具體型別，Marshal 不會失敗
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// missingRequiredLabels 把 SlotKey 清單轉成 domain.SlotLabels 的中文顯示字串，
// 供 RenderPlan 的 SafeRenderArgs 用——跟舊版 reply_composer.go
// missingRequiredSuffix 同樣的資料來源（domain.SlotLabels），但這裡回傳
// []string 而非組好的字串，串接邏輯交給 RenderPlan 自己的模板（保持
// SafeRenderArgs 只帶資料、不帶格式化決策）。
func missingRequiredLabels(missing []domain.SlotKey) []string {
	labels := make([]string, 0, len(missing))
	for _, k := range missing {
		labels = append(labels, domain.SlotLabels[k])
	}
	return labels
}

// salientFactLabels 把 plan.SalientFactRefs 對應的 recorded 欄位轉成
// "標籤：值" 顯示字串，最多 2 個（SalientFactRefs 本身已經限制在 2 個，
// 見 dialogue_planner.go salientFactRefsFor）。recorded 的順序與
// SalientFactRefs 一致（同一個來源，見 turn.go/answer.go 呼叫順序）。
func salientFactLabels(plan domain.DialoguePlanV1, recorded []domain.RecordedField) []string {
	n := len(plan.SalientFactRefs)
	if n > len(recorded) {
		n = len(recorded)
	}
	labels := make([]string, 0, n)
	for i := 0; i < n; i++ {
		labels = append(labels, recorded[i].Label+"："+recorded[i].Value)
	}
	return labels
}

// rejectedLabels 把 domain.RejectedField 轉成 "標籤：原因" 顯示字串。
func rejectedLabels(rejected []domain.RejectedField) []string {
	labels := make([]string, 0, len(rejected))
	for _, r := range rejected {
		label := domain.SlotLabels[domain.SlotKey(r.Key)]
		if label == "" {
			label = r.Key
		}
		labels = append(labels, label+"："+r.Reason)
	}
	return labels
}

// questionPrompt 取出這一輪下一題的問句本文，q 為 nil（沒有下一題）時回空字串
// ——RenderPlan 的 ActAckAndAskOptional 模板在空字串時會退回自己的預設文案
// （dialogue_templates.go），不需要呼叫端另外判斷。
func questionPrompt(q *domain.ClarifyQuestion) string {
	if q == nil {
		return ""
	}
	return q.Prompt
}

// replayResult 把持久化的既有結果還原成與正常路徑同形狀的 TurnResult。
// slots 來自 revision（權威內容），rejected/degraded 來自當初那次的 payload——
// 兩者都是同一次命令留下的，所以重播出來的回應與首次逐欄位一致。
func replayResult(prev repository.IdempotentResult) (TurnResult, error) {
	var payload turnPayload
	if len(prev.PayloadJSON) > 0 {
		if err := json.Unmarshal(prev.PayloadJSON, &payload); err != nil {
			return TurnResult{}, err
		}
	}
	if payload.Rejected == nil {
		payload.Rejected = []domain.RejectedField{}
	}
	if payload.RecordedThisTurn == nil {
		payload.RecordedThisTurn = []domain.RecordedField{}
	}
	blocks := payload.Blocks
	if len(blocks) == 0 && payload.Reply != "" {
		// Phase 2 上線前寫入的舊列：只有 reply，沒有 blocks。比照前端
		// legacyReplyToBlocks 的邏輯，包成單一 legacy block，不臆測拆分。
		blocks = []domain.Block{{Kind: domain.BlockUnderstood, Text: payload.Reply}}
	}
	dialoguePlan := domain.DialoguePlanV1{}
	if payload.DialoguePlan != nil {
		dialoguePlan = *payload.DialoguePlan
	}
	// renderMode 預設回 "TEMPLATE"（final review finding 3）：Phase 2 上線前
	// 寫入的舊列沒有 renderMode key，unmarshal 出來是空字串——toTurnResultProto
	// 直接讀 TurnResult.RenderMode 上 wire（不再寫死常量），空字串會變成一個
	// 不在 {"MODEL","TEMPLATE"} 合法值域內的第三種狀態。這一行就是本檔案這段
	// doc comment 承諾的「重播需要還原這個值，否則會把 Phase 2 舊列的空值
	// 誤判成某個合法狀態」——舊列在 Phase 2 只有 deterministic template 一種輸出，
	// 所以空值唯一誠實的還原結果就是 "TEMPLATE"。
	renderMode := payload.RenderMode
	if renderMode == "" {
		renderMode = "TEMPLATE"
	}
	return TurnResult{
		RequestID: prev.RequestID, ConversationID: prev.ConversationID, RevisionID: prev.RevisionID, Slots: prev.Slots,
		Rejected: payload.Rejected, MissingRequired: prev.Slots.MissingRequired(),
		RecordedThisTurn: payload.RecordedThisTurn,
		Degraded:         payload.Degraded, DegradedReason: reasonFor(payload.Degraded),
		Reply: payload.Reply, Blocks: blocks, RenderMode: renderMode, Steps: payload.Steps, Question: payload.Question,
		DialoguePlan: dialoguePlan,
	}, nil
}

func reasonFor(degraded bool) string {
	if degraded {
		return degradedReason
	}
	return ""
}

// deriveSkippedSlots 從版本鏈衍生「哪些選填欄位已被買家明確跳過」——正向掃描
// 全部歷史，收集每一筆 source=="ANSWER" 記錄的 Skip 欄位。domain.PickNextQuestion
// 保持純函式、不認識 repository.RevisionWithPatch，衍生邏輯留在 usecase 層做。
// Answer usecase 也會呼叫這個函式，因此放在同套件的 turn.go——procurement/usecase
// 目前的慣例是同套件內共用的小函式不特別拆檔（見 reasonFor）。
//
// 這段邏輯不是本輪要修的 bug，維持原樣：full-chain 正向掃描，不像
// previousOptionalStreak 只讀 tip——「哪些欄位被跳過」是集合語意，天生需要看
// 整條歷史；跳過一次即永久跳過（除非之後被 rewind 重新打開，那時鏈上會多一筆
// REWIND 記錄，但沒有對應的 Skip=nil 記錄可以撤銷舊的 skipped 標記——這是既有
// 行為，不在本次修復範圍內）。
func deriveSkippedSlots(history []repository.RevisionWithPatch) map[domain.SlotKey]bool {
	skipped := map[domain.SlotKey]bool{}
	for _, h := range history {
		if h.PatchSource != "ANSWER" || len(h.PatchPayloadJSON) == 0 {
			continue
		}
		var p turnPayload
		if err := json.Unmarshal(h.PatchPayloadJSON, &p); err != nil {
			continue // 壞資料不致命：skip 集合是盡力而為的優化，略過即可
		}
		if p.Skip != nil {
			skipped[*p.Skip] = true
		}
	}
	return skipped
}

// previousOptionalStreak 讀出版本鏈「目前 tip」（最近一筆改動）persisted 的
// optionalStreak，作為這一輪 PickNextQuestion 呼叫的 previousStreak 輸入。
//
// 這是本檔案 streak 衍生邏輯的重新設計：舊版（deriveQuestionHistory）靠反向
// 掃描整條歷史、從每一筆的 Question/Skip 欄位「猜」streak 目前多少，而
// Question==nil 這個訊號本身就有歧義（cap 打滿／真的沒東西可問／其他原因都會
// 留下同樣的 nil），猜測式的反向掃描不管怎麼調規則都會在某個情境下猜錯——第一輪
// 修復把「cap 打滿那一輪的 nil 該不該打斷往前數」猜對了，卻讓「REQUIREMENT_READY
// 之後所有非必填打斷的紀錄」全部無差別地折進同一個 streak，連 /rewinds
// 重新打開一個舊欄位這種明確的「這是新的開始」訊號都會被誤吞成延續舊 streak
// （over-count regression，見 computeOptionalStreak）。
//
// 修法是結構性的：streak 不再是讀取端猜出來的衍生值，而是寫入端在
// computeOptionalStreak 就依明確規則算好、存進這一輪自己的 payload
// （turnPayload.OptionalStreak）。讀取端因此只需要讀「最近一筆」，O(1)，不必
// 也不該再檢視更早的歷史——每一筆記錄都已經是「它自己那個時間點的正確 streak」，
// 不需要後來的讀取者重新推理。
//
// 沒有歷史（全新 request，鏈只有沒有 patch 的 root）或 tip 的 patch 解不出來，
// 視為 0（全新開始，跟「一個新對話」的直覺一致）。
func previousOptionalStreak(history []repository.RevisionWithPatch) int {
	if len(history) == 0 {
		return 0
	}
	tip := history[len(history)-1]
	if len(tip.PatchPayloadJSON) == 0 {
		return 0
	}
	var p turnPayload
	if err := json.Unmarshal(tip.PatchPayloadJSON, &p); err != nil {
		return 0
	}
	return p.OptionalStreak
}

// computeOptionalStreak 依這一輪的 status／question／capExhausted 決定要持久化
// 進這一輪 payload 的 optionalStreak 新值，供「下一輪」的 previousOptionalStreak
// 讀回當作 PickNextQuestion 的 previousStreak 輸入。只給 turn.go／answer.go 用
// ——rewind.go 不呼叫這個函式，它的 payload 永遠明確寫 OptionalStreak: 0（理由見
// rewind.go 該處註解：rewind 是買家主動重新打開一個欄位，語意上就是「重新開始」，
// 不該延續 rewind 之前任何一輪的選填連續計數，不需要再跑一次規則判斷）。
//
// 依序判斷，符合第一條就回傳：
//  1. status == "CLARIFYING"（必填還沒填完）→ 0：選填提問階段根本還沒開始，
//     不會有任何選填連續計數需要保護。
//  2. question 指向消歧問句（domain.SlotCropDisambiguation）→ previousStreak
//     原樣延續：final review finding I2 要修的行為——消歧問句是持久性 gate，不是
//     一般選填問題（跟 !Ready 同一類，turn.go／answer.go 呼叫端在 cropGate.Pending
//     時已經把 capExhausted 明確設回 false），但若不在這裡攔下，規則 3（原規則 2）
//     會因為消歧問句的 SlotKey 不在 domain.requiredSlots 裡而誤判成「這輪問了一題
//     選填」並遞增。買家卡在消歧卡片上的每一輪都會這樣把 streak 往上疊，疊到 cap
//     之後，即使消歧最終解開，quantity／delivery_date 這兩個「選填但報價必填」的
//     欄位（QuoteRequiredOptionalSlots）也會因為 streak 已經打滿而永久問不到——
//     買家從未真的被問過這兩題。必須排在規則 3 之前，否則永遠不會被走到。
//  3. question 指向選填欄位（PickNextQuestion 決定要問一題選填）→
//     previousStreak + 1：這輪確實問了一題選填，延續累加。
//  4. question == nil 且 capExhausted（cap 剛打滿：本來有符合資格的選填欄位可
//     問，但被 cap 擋下）→ previousStreak 原樣延續：這正是原本回報的
//     under-count bug 要修的行為——cap 觸發那一輪自己的 nil 不能讓 streak 歸零，
//     否則下一輪 PickNextQuestion 會看到 streak 掉回 0、cap 形同虛設。
//  5. 其餘情況（question == nil 且非 cap 造成，即真的沒東西可問；或防禦性地
//     question 指向必填欄位——理論上 status 已經 READY 時不會發生，因為
//     PickNextQuestion 在 missingRequired 為空時只會回選填問題或 nil，這裡列出
//     只是不依賴這個不變式）→ 0：沒有需要保護的連續計數，即使之後有欄位被
//     /rewinds 重新打開，也該讓它從乾淨的 0 開始被公平地考慮要不要問。
func computeOptionalStreak(status string, question *domain.ClarifyQuestion, capExhausted bool, previousStreak int) int {
	if status == "CLARIFYING" {
		return 0
	}
	if question != nil && question.SlotKey == domain.SlotCropDisambiguation {
		return previousStreak
	}
	if question != nil && !domain.IsRequiredSlot(question.SlotKey) {
		return previousStreak + 1
	}
	if question == nil && capExhausted {
		return previousStreak
	}
	return 0
}
