// services/ai-service/internal/procurement/usecase/graph_nodes.go
//
// ADK v2 workflow graph nodes (T8 plan): thin FunctionNode wrappers
// around existing, unmodified pure Go functions so a later LLM-driven
// graph controller invokes them as guards the model cannot bypass. This
// file grows across the plan — task 8 still appends graph assembly here
// later. Keep additions here scoped to one node's wrapper + its
// Params/Output types; business logic stays in the wrapped function, not
// in this file.
package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/observability"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/pricing"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"go.uber.org/zap"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"
	"google.golang.org/genai"
)

// stateValue reads a per-turn constant out of the node's ADK session
// state (the same state:"..." keys NewFunctionNodeFromState used to bind
// generically — see the "guard node scheduler execution bug" fix below
// for why this file no longer uses that constructor).
//
// The direct type assertion is the fast path for session.InMemoryService
// (every existing test here, and the graphspike resume test) — Create's
// State map holds the caller's Go value verbatim, no serialization.
// session/database's Postgres-backed session.Service is different: state
// is persisted through a JSON column (gorm_datatypes.go), so a value
// read back after going through that store round-trips as generic JSON
// shapes (map[string]any, float64, RFC3339 string, ...), not the
// original Go type — the same case NewFunctionNodeFromState covered via
// ADK's internal (unexported to this module) typeutil.ConvertToWithJSON-
// Schema. This re-encode-then-decode fallback is the hand-rolled
// equivalent: it doesn't need a schema because these are trusted,
// internal per-turn constants, not LLM-produced values.
func stateValue[T any](ctx agent.Context, key string) (T, error) {
	var zero T
	raw, err := ctx.State().Get(key)
	if err != nil {
		return zero, fmt.Errorf("missing state value for key %q: %w", key, err)
	}
	if v, ok := raw.(T); ok {
		return v, nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return zero, fmt.Errorf("re-encoding state value for key %q: %w", key, err)
	}
	if err := json.Unmarshal(b, &zero); err != nil {
		return zero, fmt.Errorf("decoding state value for key %q into %T: %w", key, zero, err)
	}
	return zero, nil
}

// emitActivityEventFuncName marks a session.Event as carrying an
// ActivityEventPayload — graph_dispatch.go's translation loop matches on
// this to distinguish Activity events from the crop-disambiguation
// interrupt signal and any other event kind. Deliberately not a
// LongRunningToolIDs-based signal: that marks a real pausing tool call
// to the ADK runtime, and Activity events are fire-and-forget, never
// pause anything (spec §2.5).
const emitActivityEventFuncName = "emit_activity_event"

// ActivityEventPayload is the typed value carried in an Activity
// session.Event's FunctionCall.Args["payload"] (spec §2.4/§2.5).
// RetryGroupKey equals RowID for every row this plan produces today —
// extract_slots only ever mints one row per node execution (after any
// internal retry inside SlotExtractor.Extract has already resolved, see
// Task 2's ExtractResult.Attempt), so there is no scenario where a
// second row needs to be grouped with a first. The field stays because
// it's part of doc57 §4.4's general row-identity shape (kept for type
// consistency across future row types, not because it diverges from
// RowID in this implementation).
type ActivityEventPayload struct {
	RowID         int64
	Attempt       int
	MaxAttempts   int
	RetryGroupKey int64
	Code          string
	Text          string
	// Label 是這一列的伺服器固定字串（domain.StepLabels[Code]），恆非空。
	//
	// T23 新增。修前這個 payload 只有 code 與 text，而 text 是 StepNarrator
	// （LLM）產生的敘述、entry 事件上恆為空——consumer 在「這一步剛開始」的那
	// 一刻手上只有一個 code，要嘛自己維護一份 code→中文的對照表（那正是
	// domain.StepLabels 的註解說「label 永遠查這張表」要避免的第二份定義），
	// 要嘛什麼都不顯示。候選搜尋那條 run 更直接：它整條沒有 narrator，text
	// 永遠是空的，沒有 label 就等於整條 rail 沒有文字。
	//
	// Label 與 Text 是兩件事，不可互相代填：Label 是固定文案（「翻平台上的
	// 賣家」），Text 是這一次實際發生了什麼（「甘藍條件有效，無拒絕欄位。」）。
	// Text 為空時 consumer 顯示 Label，不是反過來。
	//
	// validate_slots／crop_gate 的完成事件 Text 恆等於 Label：這兩個 node 不在
	// 事件裡帶模型輸出，那句話在 turn 尾才生成，只進持久化的 steps[]（見
	// graphTurnProgress.NarrationSummaries）。extract_slots 是唯一一個完成事件
	// Text 可能不同於 Label 的 node。
	Label string
	// Status 是這一列此刻的執行狀態，值域是 codemeta.ProcurementStepStatus
	// （PENDING／RUNNING／SUCCEEDED／FAILED…）。T23 新增，恆非空。
	//
	// 修前這條 rail 沒有狀態欄位，「執行中 vs 已完成」是靠 twoPhaseActivity
	// 的 entry／completion 兩筆事件**位置性地**表達的——consumer 只能靠「有沒有
	// 收到第二筆」推論。那個編碼表達不了第三種狀態：PENDING（還沒跑到）與
	// FAILED（跑失敗）都不是「收到了幾筆」能區分的。設計稿 01 的「等待中」與
	// 失敗顯示因此都做不出來。狀態改成顯式欄位之後，entry／completion 仍然照
	// 送（consumer 的 merge-by-rowId 邏輯不變），只是每一筆都自己講清楚它是
	// 哪一態。
	//
	// 這也是 T23「FAILED 放 code 軸還是 status 軸」的落地：失敗是這個欄位的一
	// 個值，不是 domain.StepCode 的一個成員（原 StepFailed 已移除）。
	Status string
	// ParentRowID 非 0 時代表這一列是巢狀子列，掛在該 rowId 底下（設計稿 01
	// 的「平台賣家／外部業者」掛在「搜尋候選供應商」之下）。0 代表 top-level。
	ParentRowID int64
	// Count 非 nil 時是這一列要顯示的計數——設計稿 01 兩條 lane 子列右側的
	// 「12 家」「已整理 7 家」。刻意只給數字不給句子：那兩個前綴是前端的固定
	// 模板，跟 safeArgs 同一種分工（Text 那邊已經是 LLM 自由文字，這裡沒有理由
	// 跟著變成文字）。指標型別區分「這一列沒有計數概念」與「計數是 0」——
	// 兩條 lane 都可能合法地回 0 家。
	Count *int
}

// twoPhaseActivity mints one rowID for this node execution and returns two
// emit functions bound to it: emitEntry (call once, before the node's real
// work starts — Text is always "") and emitCompletion (call once, after
// real work finishes, with the final Attempt/MaxAttempts/Text). Same
// RowID/RetryGroupKey on both, so consumers merging by RowID see one
// logical row that updates in place — this is how doc57 §4.4's "heartbeat"
// semantics get realized without any ledger schema change: two append-only
// events, not a DB update (design doc §0.2). Each node's own body still
// owns everything else (state reads, progress bookkeeping,
// emitGraphTurnProgress) — this only replaces the old single mint+emit
// tail with two explicit calls at the right two points in the body.
//
// Both emits are log-only on failure — same discipline as every existing
// emitActivityEvent call site in this file.
//
// The entry snapshot's Attempt/MaxAttempts are always hardcoded 1/1 — a
// known, accepted approximation: ADK gives a node's function body no way
// to observe its own current attempt number, so there is no accurate
// value to put there instead.
func twoPhaseActivity(ctx agent.Context, emit func(*session.Event) error, newID func() int64, code domain.StepCode) (emitEntry func(), emitCompletion func(attempt, maxAttempts int, text string)) {
	rowID := newID()
	emitEntry = func() {
		if err := emitActivityEvent(ctx, emit, ActivityEventPayload{
			RowID: rowID, Attempt: 1, MaxAttempts: 1, RetryGroupKey: rowID,
			Code: string(code), Text: "", Label: domain.StepLabels[code],
			Status: codemeta.ProcurementStepStatusRunning.Value,
		}); err != nil {
			pkglogger.Error(ctx, "procurement: emitting entry activity event failed",
				zap.Error(err), zap.Int64("row_id", rowID), zap.String("code", string(code)))
		}
	}
	emitCompletion = func(attempt, maxAttempts int, text string) {
		if err := emitActivityEvent(ctx, emit, ActivityEventPayload{
			RowID: rowID, Attempt: attempt, MaxAttempts: maxAttempts, RetryGroupKey: rowID,
			Code: string(code), Text: text, Label: domain.StepLabels[code],
			// 這三個節點只在成功走完本體之後才呼叫 emitCompletion——節點失敗
			// 時 ADK 直接讓整個 graph 走錯誤路徑，不會走到這一行，所以這裡恆
			// 為 SUCCEEDED。失敗那一態由 run 層級的終態事件表達，不是留一列
			// 停在 RUNNING 的殭屍列。
			Status: codemeta.ProcurementStepStatusSucceeded.Value,
		}); err != nil {
			pkglogger.Error(ctx, "procurement: emitting completion activity event failed",
				zap.Error(err), zap.Int64("row_id", rowID), zap.String("code", string(code)))
		}
	}
	return emitEntry, emitCompletion
}

// guardShortIntent implements spec §2.2/§2.3's guard: an empty or
// over-length model sentence deterministically falls back to the
// existing StepLabels[StepExtractSlots] fixed string. extract_slots is the
// only node left that guards a model sentence this way — validate_slots/
// crop_gate no longer call the narrator in-node at all (they only compute a
// summary via narrateStepOutcome's later caller, the turn terminal), so they
// have nothing of their own left to guard here.
//
// Trims surrounding whitespace before the emptiness/length checks (and
// returns the trimmed value on the pass-through path) — a pure-whitespace
// sentence (e.g. " " or "\n") would otherwise pass the raw == "" check
// and render as a blank/invisible Activity row.
func guardShortIntent(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || len([]rune(trimmed)) > shortIntentMaxLen {
		return domain.StepLabels[domain.StepExtractSlots]
	}
	// doc 57 §3.1 第 3 條「不承載事實」的執行期強制——在此之前這條只存在於文件裡，
	// 而 §3.2 的紅線明寫這個表面的放寬以四條約束全部在執行期成立為前提。
	// 判準與 step 敘述共用，見 public_text_policy.go。
	if publicTextRejection(trimmed) != "" {
		return domain.StepLabels[domain.StepExtractSlots]
	}
	return trimmed
}

// emitActivityEvent builds and emits the tagged session.Event carrying
// payload. Shared by extract_slots (this task) and validate_slots/
// crop_gate (Task 4) so all three nodes produce byte-identical event
// shapes, differing only in payload contents.
func emitActivityEvent(ctx agent.Context, emit func(*session.Event) error, payload ActivityEventPayload) error {
	ev := session.NewEvent(ctx, ctx.InvocationID())
	ev.Content = &genai.Content{
		Parts: []*genai.Part{{
			FunctionCall: &genai.FunctionCall{
				Name: emitActivityEventFuncName,
				Args: map[string]any{"payload": payload},
			},
		}},
	}
	return emit(ev)
}

// NewExtractSlotsNode 包裝既有 SlotExtractor.Extract（extract.go:46）。改用
// workflow.NewEmittingFunctionNode（原本是 workflow.NewFunctionNode）：純輸入
// 輸出的 node 不能在回傳終值之前先送出事件，而這個 node 現在要在guard判斷後
// emit 一個 Activity session.Event（spec §2.2/§2.5）。node 對下游
// （validate_slots）的輸出型別不變，仍是 domain.SlotPatch——ShortIntent 只
// 用來構造這個 node 自己 emit 的事件，guard 判斷完就「用完」了，不需要傳給
// 下游任何節點。
//
// newID 是既有 TSID 產生器慣例（同其餘 usecase 建構子
// newID 參數，fx 注入 tsid.NewGenerator）——鑄這一列 Activity row 的 opaque
// row id。
//
// emitActivityEvent 失敗只記 log、不回傳成硬錯誤，跟 NewCropGateNode／
// NewValidateSlotsNode 統一：ADK 的 emit（workflow/dynamic_node.go 的
// makeEmit）只有在 parent context 已取消或事件消費者已經停止迭代時才會回錯，
// 兩種情況都代表這次 run 本來就在收尾，硬錯誤在那個當下保護不了任何健康的
// run，只可能影響一個已經在結束的 run。
func NewExtractSlotsNode(extractor SlotExtractor, newID func() int64) *workflow.FunctionNode {
	return workflow.NewEmittingFunctionNode(
		"extract_slots",
		func(ctx agent.Context, _ any, emit func(*session.Event) error) (domain.SlotPatch, error) {
			currentSlots, err := stateValue[domain.Slots](ctx, "current_slots")
			if err != nil {
				return domain.SlotPatch{}, err
			}
			progress, err := readGraphTurnProgress(ctx)
			if err != nil {
				return domain.SlotPatch{}, err
			}
			message := firstContentText(ctx.UserContent())

			emitEntry, emitCompletion := twoPhaseActivity(ctx, emit, newID, domain.StepExtractSlots)
			emitEntry()

			result, extractErr := extractor.Extract(ctx, message, currentSlots)

			// LLM 抽取失敗不是錯誤而是降級（同 turn.go HandleStream 既有紀律）：
			// 買家仍然拿得到既有需求與缺口清單，之後由固定問法接手，不讓整個
			// graph run 失敗。
			var patch domain.SlotPatch
			var degraded bool
			var shortIntentText string
			attempt, maxAttempts := 1, 1
			if extractErr != nil {
				degraded = true
				shortIntentText = domain.StepLabels[domain.StepExtractSlots]
				requestID, _ := stateValue[int64](ctx, "request_id")
				pkglogger.Warn(ctx, "procurement: graph extract_slots degraded, LLM extraction failed",
					zap.Int64("request_id", requestID),
					zap.Int("message_runes", len([]rune(message))),
					zap.Bool("timeout", errors.Is(extractErr, context.DeadlineExceeded)),
					zap.Error(extractErr),
				)
				observability.RecordProcurementDegraded(ctx)
			} else {
				patch = result.Patch
				attempt, maxAttempts = result.Attempt, result.MaxAttempts
				if maxAttempts == 0 {
					attempt, maxAttempts = 1, 1
				}
				shortIntentText = guardShortIntent(result.ShortIntent)
			}

			progress.Degraded = degraded
			progress.Patch = patch

			progress.Narration = withStepNarration(progress.Narration, domain.StepExtractSlots, shortIntentText)
			emitCompletion(attempt, maxAttempts, shortIntentText)
			if emitErr := emitGraphTurnProgress(ctx, emit, progress); emitErr != nil {
				pkglogger.Error(ctx, "procurement: emitting extract slots progress failed",
					zap.Error(emitErr))
			}

			return patch, nil
		},
		workflow.NodeConfig{},
	)
}

// firstContentText 從 content 抽出第一個非空文字 part，跟官方範例
// examples/workflow/routing/llm/main.go 的 userMessage 同一種模式。
// 既用於 graph_nodes.go 的使用者輸入，也用於 model_llm_extractor.go 的
// 模型回應，改成中性名稱後兩處都能用。
func firstContentText(content *genai.Content) string {
	if content == nil {
		return ""
	}
	for _, p := range content.Parts {
		if p.Text != "" {
			return p.Text
		}
	}
	return ""
}

// ValidateSlotsOutput 是 validate_slots node 的輸出，crop_gate node（任務 3）
// 直接消費這個型別的 .Slots 欄位。
type ValidateSlotsOutput struct {
	Slots    domain.Slots
	Rejected []domain.RejectedField
}

// summarizeValidateSlots builds narrateStepOutcome's input summary from
// validate_slots' own result — plain Chinese sentence fragments, not JSON:
// the model only needs to paraphrase this into one sentence, not parse
// structured data.
//
// 🔴 recorded（這一輪真的被記下的欄位，domain.RecordedThisTurn 的輸出）是這段
// 摘要裡唯一逐輪會變的部分，必須帶上：少了它，摘要的內容只剩「目前的品項」與
// 「有沒有被拒絕的欄位」，而這兩者在一段多輪對話裡通常從頭到尾不變——LLM 拿到
// 逐字相同的輸入就會產出逐字相同的句子，買家於是在每一輪都看到同一句「X 條件
// 皆有效，無拒絕欄位」。這不是模型不夠好，是我們沒給它任何差異可講。
func summarizeValidateSlots(next domain.Slots, rejected []domain.RejectedField, recorded []domain.RecordedField) string {
	var b strings.Builder
	if len(recorded) > 0 {
		b.WriteString("這一輪新記下：")
		for i, r := range recorded {
			if i > 0 {
				b.WriteString("、")
			}
			fmt.Fprintf(&b, "%s＝%s", r.Label, r.Value)
		}
		b.WriteString("；")
	} else {
		b.WriteString("這一輪沒有新記下任何欄位；")
	}
	if next.Item != nil {
		fmt.Fprintf(&b, "品項：%s；", *next.Item)
	}
	if len(rejected) == 0 {
		b.WriteString("目前條件皆有效，沒有被拒絕的欄位。")
		return b.String()
	}
	b.WriteString("被拒絕的欄位：")
	for i, r := range rejected {
		if i > 0 {
			b.WriteString("、")
		}
		fmt.Fprintf(&b, "%s（%s）", r.Key, r.Reason)
	}
	b.WriteString("。")
	return b.String()
}

// NewValidateSlotsNode 包裝既有 domain.Slots.Apply（domain/slots.go:103），函式
// 本身一行不改。純函式、無 I/O，故 NodeConfig 不設 RetryConfig（doc57 §1.5：
// guard 是模型無 handle 的既有 Go 函式，這裡只是型別包裝）。改用 workflow.
// NewEmittingFunctionNode（原本是 workflow.NewFunctionNode）：這個 node 現在要
// 在 Apply 完成後 emit 一個 Activity session.Event（spec §2.5，跟 extract_slots
// 共用同一組 ActivityEventPayload／emitActivityEvent helper，見本檔頂部文件）
// ——這個 node 不在自己身上打 narrator：Apply 結果只算成一段摘要留進
// progress.NarrationSummaries，真正的一句話敘述留給 turn 尾的平行群組生成
// （避免每輪多一次序列 LLM 呼叫）。這裡的 Activity 事件 Text 因此恆為
// domain.StepLabels[StepValidateSlots] 固定字串，不是 fallback，是唯一路徑。
//
// ⚠️ 已修復的 guard node scheduler 執行 bug（原本用 workflow.
// NewFunctionNodeFromState 建這個 node）：那個建構子一律會自動從節點的 OUT
// 泛型型別產生一個非 nil 的 struct output JSON schema，真正經由 scheduler
// 執行時 engine 會拿這個 schema 驗證節點回傳的 struct 值，而 jsonschema-go
// v0.4.3 對任何 reflect.Struct 值一律回錯（已確認的上游 bug，
// https://github.com/google/jsonschema-go/issues/23，
// jsonschema-go@v0.4.3/jsonschema/validate.go:449-451 硬編碼
// `if instance.Kind() == reflect.Struct { return errors.New(...) }`，不論
// schema 內容為何都會命中）。NewFunctionNode／NewEmittingFunctionNode 兩者的
// inputSchema／outputSchema 都一律傳 nil（見 function_node.go:61-63、91-93）
// 徹底避開這個路徑——這三個 node 是純 Go 函式包裝、輸出永遠型別正確，schema
// 驗證對它們本來就是多餘的一層。Patch（上游 extract_slots 的輸出）直接當
// node 的 Run 輸入接收，跟 NewFunctionNodeFromState 的 state:"node_input"
// 語意完全相同——這正是一般 FunctionNode 天生的行為（predecessor 輸出即
// Run 的 input），不需要任何轉接。
//
// newID 同 NewExtractSlotsNode 的參數慣例（鑄這一列 Activity row 的 opaque
// row id）。
//
// emitActivityEvent 失敗只記 log、不回傳成硬錯誤——理由同 NewExtractSlotsNode
// 文件上方那段：ADK 的 emit 只在 run 本來就在收尾時才會回錯，跟這個 node
// 有沒有 RetryConfig 無關。
func NewValidateSlotsNode(newID func() int64) *workflow.FunctionNode {
	return workflow.NewEmittingFunctionNode(
		"validate_slots",
		func(ctx agent.Context, patch domain.SlotPatch, emit func(*session.Event) error) (ValidateSlotsOutput, error) {
			currentSlots, err := stateValue[domain.Slots](ctx, "current_slots")
			if err != nil {
				return ValidateSlotsOutput{}, err
			}
			now, err := stateValue[time.Time](ctx, "now")
			if err != nil {
				return ValidateSlotsOutput{}, err
			}
			progress, err := readGraphTurnProgress(ctx)
			if err != nil {
				return ValidateSlotsOutput{}, err
			}

			emitEntry, emitCompletion := twoPhaseActivity(ctx, emit, newID, domain.StepValidateSlots)
			emitEntry()

			next, rejected := currentSlots.Apply(patch, now)
			progress.NextSlots = next
			progress.Rejected = rejected

			// recorded 在這裡重算一次，而不是從 progress 讀：terminal node 的
			// domain.RecordedThisTurn(progress.Patch, current, next) 排在整張圖的
			// 最後，這個 node 執行時還沒有那個值。三個輸入（patch／currentSlots／
			// next）都在手上，RecordedThisTurn 是純函式。
			//
			// 這份 recorded 與 terminal 那份現在恆等：唯一會讓兩者分歧的那條路徑
			// （crop_gate 硬擋時把 NextSlots.Item 退回舊值）已隨 cropidentity.GateResult.
			// Rejected 一起移除。除 validate_slots 自己以外，全 repo 只剩
			// crop_resume_reconcile 會寫 progress.NextSlots.Item，而它寫的是 gate
			// 這一輪判定的新值、不是回退，不會讓 terminal 算不到品項。
			recorded := domain.RecordedThisTurn(patch, currentSlots, next)
			progress.NarrationSummaries = withNarrationSummary(progress.NarrationSummaries,
				domain.StepValidateSlots, summarizeValidateSlots(next, rejected, recorded))
			emitCompletion(1, 1, domain.StepLabels[domain.StepValidateSlots])
			if emitErr := emitGraphTurnProgress(ctx, emit, progress); emitErr != nil {
				pkglogger.Error(ctx, "procurement: emitting validate slots progress failed",
					zap.Error(emitErr))
			}

			return ValidateSlotsOutput{Slots: next, Rejected: rejected}, nil
		},
		workflow.NodeConfig{},
	)
}

// CropGateOutput 只保留 cropidentity.GateResult 裡對下游（controller、interrupt node）
// 有意義且全部 exported 的欄位——刻意不直接用 cropidentity.GateResult 本身當 node 輸出型別，
// 它有一個 unexported 的 persist 欄位，若被序列化跨 node 邊界會被 encoding/json
// 靜默丟掉。
type CropGateOutput struct {
	Item *string
	// Resolution 走這條路的理由跟 Item 完全相同：resume 時買家的答案是以
	// CropGateOutput 的形式送進 interrupt node 下游的（handoff 模式），
	// crop_resume_reconcile 是唯一能把它寫回 progress 的地方。少了這個欄位，
	// 買家挑中的候選在 resume 路徑上會被整個丟掉——resumeGraphTurn 只取 Item，
	// patch 裡的解析結果沒有人接。
	Resolution *domain.ItemResolution
	Pending    bool
	Candidates []domain.CropDisambiguationOption
	Selection  *domain.CropSelection
	// PendingReason／FactsAsOf 走這條路的理由跟 Candidates 相同：它們是
	// crop_disambiguation_interrupt node 組 interrupt payload 的輸入，而那個
	// node 只看得到上游 node 的輸出。少了它們，落庫的 payload 會退回「靠候選
	// 清單長度推斷待處理原因」——正是 Cut 1 移除的那個推斷。
	PendingReason domain.CropPendingReason
	FactsAsOf     time.Time
	// ResolveFailed 走這條路的理由跟 Candidates 相同：它是下游唯一能知道
	// 「這一輪分類查詢是失敗，不是查到了沒有」的地方。少了它，
	// composeGraphTurnResult 會把 required slots 齊全當成 Ready，然後催買家去
	// 做價格預覽——而沒有 crop code 的 PRICEBOOK_GUIDED 必定回 422
	// MAPPING_MISSING（required slots 不含 item_resolution，見
	// domain.Slots.MissingRequired）。
	ResolveFailed bool
}

// summarizeSlotsRejected builds narrateStepOutcome's input summary for
// SLOTS_REJECTED — 這一碼的固定文案是「有幾個條件先擱著」，完全沒說擱了什麼、
// 為什麼，而這偏偏是買家最需要知道的一輪（他剛打的東西沒被接受）。被拒欄位與
// 理由本來就在手上，跟 summarizeValidateSlots 讀的是同一份 rejected。
//
// Key 走 domain.SlotLabels 轉成中文欄位名——那張表是伺服器固定字串表，跟
// RecordedThisTurn 用的是同一張，不會讓模型看到 slot key 這種內部識別值。表裡
// 查不到的 key 直接用原值，不猜也不丟棄（漏顯示一個被拒欄位比顯示一個醜名字糟）。
func summarizeSlotsRejected(rejected []domain.RejectedField) string {
	var b strings.Builder
	b.WriteString("這一輪沒有被採用的欄位：")
	for i, r := range rejected {
		if i > 0 {
			b.WriteString("、")
		}
		label := r.Key
		if l, ok := domain.SlotLabels[domain.SlotKey(r.Key)]; ok {
			label = l
		}
		fmt.Fprintf(&b, "%s（%s）", label, r.Reason)
	}
	b.WriteString("。")
	return b.String()
}

// cropGateChoiceNoun 讓敘述的名詞跟卡片上實際列的東西一致：Pricebook 候選確實
// 是作物，taxonomy 兩階段列的是商品分類（豬肉／牛肉）。判準用 Selection，與
// BuildCropDisambiguationQuestion 挑問句同源。
func cropGateChoiceNoun(selection *domain.CropSelection) string {
	if selection == nil {
		return "候選作物"
	}
	switch selection.Stage {
	case domain.CropSelectionGroup:
		return "大品項分類"
	case domain.CropSelectionItem:
		return "小品項分類"
	}
	return "候選作物"
}

// summarizeCropGate builds narrateStepOutcome's input summary from
// cropidentity.ComputeGate's result — mirrors summarizeValidateSlots' plain-
// sentence-fragment shape.
func summarizeCropGate(result cropidentity.GateResult) string {
	switch {
	case result.ResolveFailed:
		// 必須排在 Pending／Rejected 之前判斷嗎？不必——降級路徑回傳的 result 兩者
		// 皆為零值。排在最前面是為了讓「這條路徑有被獨立處理」在讀碼時一眼看得到：
		// 少了這一支，Pricebook 查詢失敗會落進 default 分支，對買家說「分類判斷已
		// 完成」——而實際上這一輪什麼都沒判定成功。
		//
		// 措辭刻意不宣稱「已清除」：清空是 persistCropGate 的事，而那支失敗時只
		// 記 log、不回報，所以敘述無從得知寫入成功與否。只講這一輪確定成立的事實。
		return "品項分類服務這一輪查詢失敗，沒有取得新的分類結果，下一輪會再試一次。"
	case result.Pending && len(result.Candidates) == 0:
		// Cut 1：檢索用盡仍查無候選（PendingReason=UNAVAILABLE）。跟下面
		// 「N 個候選」共用 result.Pending，必須排在它前面，否則 N=0 會落進那句
		// 讀成「品項尚待買家從 0 個候選中選擇」——候選是空的，不是還沒展示
		// 完，兩者要對買家講的話不同。
		//
		// 不點名逃生艙按鈕：BuildCropDisambiguationQuestion 帶 Selection 時會把它
		// 改寫成「沒有符合的品項，自行填寫單價」，而這段摘要是 StepNarrator 的
		// 輸入，寫死的按鈕名會被忠實複誦給買家。
		return "查無吻合的品項分類，買家可以自行描述品項並填入單價。"
	case result.Pending:
		return fmt.Sprintf("品項尚待買家從 %d 個%s中選擇。", len(result.Candidates), cropGateChoiceNoun(result.Selection))
	default:
		itemText := "（未指定）"
		if result.Item != nil {
			itemText = *result.Item
		}
		return fmt.Sprintf("品項「%s」的分類判斷已完成。", itemText)
	}
}

// NewCropGateNode 包裝既有 cropidentity.ComputeGate + persistCropGate（crop_gate.go），
// 兩者必須在同一個函式呼叫內完成——cropidentity.GateResult.persist 是 unexported 欄位，
// 一旦 cropidentity.GateResult 本身跨出這個函式（例如被拆成兩個 node），persist 欄位在
// node 邊界的 JSON 序列化會被靜默丟棄，落庫會悄悄失效。RetryConfig 用 ADK 預設
// （5 次、1s 起跳、60s 上限）：這個 guard 有真實 I/O（品項分類服務），有暫時性
// 失敗的可能，不同於 validate_slots 的純函式。
//
// 用 workflow.NewEmittingFunctionNode，不是 workflow.NewFunctionNodeFromState
// ——理由見 NewValidateSlotsNode 文件的 guard node scheduler 執行 bug 說明，
// 同一個 root cause；改用 Emitting 變體（不是單純 NewFunctionNode）則是因為
// 這個 node 現在要在 persistCropGate 完成後 emit 一個 Activity session.Event
// （跟 validate_slots 共用同一組 helper）——這個 node 不在自己身上打
// narrator：判定結果只算成一段摘要留進 progress.NarrationSummaries，一句話
// 敘述留給 turn 尾的平行群組生成（同一份判準見下方 Evaluated() 的說明）。這裡
// 的 Activity 事件 Text 因此恆為 domain.StepLabels[StepCropGate] 固定字串。
// Validated（上游 validate_slots 的完整輸出）直接當 node 的 Run 輸入接收，跟
// 原本 state:"node_input" 讀整個 ValidateSlotsOutput 語意相同。
//
// emitActivityEvent 失敗只記 log、不回傳成硬錯誤——這個 node 的 NodeConfig
// 帶 RetryConfig（見下方），emit 又是函式本體裡最後一個可能失敗的動作：若
// emit 失敗回傳 error 會讓 ADK scheduler 重跑整個節點本體，導致
// persistCropGate 被重複呼叫、newID() 在重跑時鑄出全新的 rowID／
// RetryGroupKey（Attempt／MaxAttempts 卻仍硬編碼 1/1），讓同一次邏輯上的
// crop_gate 判定在 Activity Rail 上看起來像兩筆互不相關的「attempt 1 of
// 1」（doc57 §4.4 RetryGroupKey 語意因此失效）。改成 log-only，跟
// persistCropGate 自己既有的「失敗只記 log、不讓整輪對話失敗」降級語意
// 一致（見 crop_gate.go persistCropGate 文件），也結構性地讓 emit 失敗不再
// 可能觸發這個節點的 retry。extract_slots／validate_slots 現在也統一用同一種
// log-only 處理（見 NewExtractSlotsNode 文件那段 ADK emit 語意說明）——即使
// 那兩個 node 沒有 RetryConfig，統一處理方式對讀者更一致，也跟 ledger-append
// 那側（graph_dispatch.go）一貫的 non-blocking 降級紀律同調。
//
// newID 同 NewValidateSlotsNode 的參數慣例。
func NewCropGateNode(resolver cropidentity.Resolver, newID func() int64) *workflow.FunctionNode {
	return workflow.NewEmittingFunctionNode(
		"crop_gate",
		func(ctx agent.Context, validated ValidateSlotsOutput, emit func(*session.Event) error) (CropGateOutput, error) {
			requestID, err := stateValue[int64](ctx, "request_id")
			if err != nil {
				return CropGateOutput{}, err
			}
			before, err := stateValue[domain.Slots](ctx, "current_slots")
			if err != nil {
				return CropGateOutput{}, err
			}
			progress, err := readGraphTurnProgress(ctx)
			if err != nil {
				return CropGateOutput{}, err
			}

			emitEntry, emitCompletion := twoPhaseActivity(ctx, emit, newID, domain.StepCropGate)
			emitEntry()

			// 買家正在回答上一輪的消歧提問時不得重查——重查會換掉他眼前那份候選。
			//
			// turn／answer 兩個進入點的 GuardNoOpenInterrupt 已經擋掉大部分情況，但
			// 那個 guard 自己的註解承認它有一個不持鎖的毫秒級 race，並且論證「這條
			// race 的壞結果集合是空的」。跨輪重試一旦上線，那個集合就不再是空的
			// （放行的那一輪會重查、可能開出第二個 interrupt），所以這裡補一道在
			// graph 內、讀同一份 session state 的判準，讓那句話繼續成立。
			var gateOpts []cropidentity.GateOption
			pendingInterruptID, err := stateValue[string](ctx, cropWorkflowInterruptStateKey)
			if err != nil && !errors.Is(err, session.ErrStateKeyNotExist) {
				return CropGateOutput{}, err
			}
			if pendingInterruptID != "" {
				gateOpts = append(gateOpts, cropidentity.AwaitingBuyerAnswer())
			}

			result := cropidentity.ComputeGate(ctx, resolver, before, validated.Slots, gateOpts...)

			// 解析結果併進這一輪的 slots 與 patch——**在 AppendRevision 之前**，
			// 所以它跟 item 原話落在同一個 revision、同一次寫入（ADR
			// 2026-09-04-crop-resolution-into-slots）。寫進 Patch 是為了讓
			// terminal 的 domain.RecordedThisTurn 看得到它：買家挑完候選要在
			// ①確認段看到「品項分類＝甘藍-初秋」，而那個函式的判準是「patch 有
			// 帶到且 Apply 後真的改變」。
			//
			// 只在非 nil 時寫、不做「用 nil 覆寫」：舊解析的清除是 validate_slots
			// 那一步 Slots.Apply 在 item 變動時做掉的（見該函式內的說明），這裡
			// 無條件覆寫會把「item 未變動、解析原樣帶著走」那條路徑的既有解析
			// 洗掉。
			result.ApplyTo(&progress.NextSlots, &progress.Patch)
			progress.CropPending = result.Pending
			progress.CropCandidates = result.Candidates
			progress.CropSelection = result.Selection
			progress.CropGateEvaluated = result.Evaluated()
			progress.CropResolveFailed = result.ResolveFailed

			// 摘要的生命週期與上一行的 CropGateEvaluated 綁死，判準是同一個
			// Evaluated()——也就是 ComputeTurnSteps 產不產生 CROP_GATE 那一格的
			// 判準。progress 活在以 requestID 為 key 的 session 裡、跨輪次存活，
			// 所以沒做判定的那一輪必須清掉上一輪的摘要：留著會讓 terminal 為一個
			// 不存在的格子生成敘述，產物被 applyStepNarration 靜默丟棄。
			if result.Evaluated() {
				progress.NarrationSummaries = withNarrationSummary(progress.NarrationSummaries,
					domain.StepCropGate, summarizeCropGate(result))
			} else {
				progress.NarrationSummaries = withoutNarrationSummary(progress.NarrationSummaries,
					domain.StepCropGate)
			}
			emitCompletion(1, 1, domain.StepLabels[domain.StepCropGate])
			if emitErr := emitGraphTurnProgress(ctx, emit, progress); emitErr != nil {
				pkglogger.Error(ctx, "procurement: emitting crop gate progress failed",
					zap.Error(emitErr), zap.Int64("request_id", requestID))
			}

			return CropGateOutput{
				Item: result.Item, Resolution: result.Resolution,
				Pending: result.Pending, Candidates: result.Candidates, Selection: result.Selection,
				PendingReason: result.PendingReason, FactsAsOf: result.FactsAsOf,
				ResolveFailed: result.ResolveFailed,
			}, nil
		},
		workflow.NodeConfig{RetryConfig: workflow.DefaultRetryConfig()},
	)
}

// NewPricePreviewNode 包裝既有 PreviewRfqPriceUseCase.PreviewRfqPrice
// （price_preview.go:135），函式本身一行不改——它已經是一個高度收斂的原子
// 單元（idempotency 短路、CAS 版本檢查、依 PricingMode 分兩條互斥分支、
// Pricebook I/O、持久化都耦合在一起），沒有可以乾淨拆出的「pricing mode
// 判定」子函式，見設計書 §3。
//
// 跟 validate_slots／crop_gate 不同，這個 node 直接沿用既有的
// PreviewRfqPriceInput／PricePreviewResult 當泛型參數（用 NewFunctionNode，
// 不是 NewFunctionNodeFromState + 自訂 Params 型別）：PreviewRfqPriceInput
// 全欄位皆 exported、本身就是一個自足的請求 DTO（既有 gRPC handler 呼叫
// PreviewRfqPrice 的方式就是整包傳入，不是「per-turn 常數＋上游 node 輸出」
// 的組合），PricePreviewResult 同樣全欄位皆 exported，不像 cropidentity.GateResult
// 需要窄化剔除 unexported 的 persist 欄位，所以不必另建 Params/Output
// 型別，直接讓 PreviewRfqPriceInput 整包走 node.Run 的原始 input 參數。這個
// node 從一開始就用 NewFunctionNode（nil schema），不受上面三個 node 修復的
// scheduler 執行 bug 影響。
//
// 這個 node 本次不會被任何邊引用（見任務 8 T8 plan）——PricingMode／
// BuyerRequestedRange 目前是買家明確選擇的結構化輸入，對話中怎麼收集這兩個
// 欄位需要獨立設計，超出這次骨架範圍。本任務只交付「node 存在、獨立測試
// 通過」，不交付「controller 能路由到它」。RetryConfig 用 ADK 預設，跟
// crop_gate 同理：這個 guard 有真實 I/O（Pricebook gRPC client、DB
// 持久化），有暫時性失敗的可能，不同於 validate_slots 的純函式。
func NewPricePreviewNode(uc *pricing.PreviewRfqPriceUseCase) *workflow.FunctionNode {
	return workflow.NewFunctionNode(
		"price_preview",
		func(ctx agent.Context, in pricing.PreviewRfqPriceInput) (pricing.PricePreviewResult, error) {
			return uc.PreviewRfqPrice(ctx, in)
		},
		workflow.NodeConfig{RetryConfig: workflow.DefaultRetryConfig()},
	)
}

// withStepNarration 把一句敘述記進 progress.Narration，map 為 nil 時就地建立。
// extract_slots 專用——算出來的句子本來只進 Activity 事件，這裡多留一份給
// 持久化的 steps[]（見 graphTurnProgress.Narration 的說明）。validate_slots／
// crop_gate 改走 withNarrationSummary，不再呼叫這支。
func withStepNarration(n domain.StepNarration, code domain.StepCode, text string) domain.StepNarration {
	if text == "" {
		return n
	}
	if n == nil {
		n = domain.StepNarration{}
	}
	n[code] = text
	return n
}

// withoutStepNarration 作廢某一碼已經算好的敘述——用在「上游節點算敘述時講的
// 是真話，但下游節點推翻了它所根據的狀態」這種情況。刪掉之後那一格退回固定
// Label（消費端本來就要能處理 Text 為空，見 domain.Step.Text 的不變量），比
// 留著一句已經不成立的話好。
func withoutStepNarration(n domain.StepNarration, code domain.StepCode) domain.StepNarration {
	if n == nil {
		return nil
	}
	delete(n, code)
	return n
}

// withNarrationSummary 把一段摘要記進 progress.NarrationSummaries，map 為 nil
// 時就地建立。validate_slots／crop_gate 共用——它們不再自己打 narrator，只把算
// 好的摘要留給 turn 尾的平行群組（見 graphTurnProgress.NarrationSummaries 的
// 說明）。
func withNarrationSummary(m map[domain.StepCode]string, code domain.StepCode, summary string) map[domain.StepCode]string {
	if summary == "" {
		return m
	}
	if m == nil {
		m = map[domain.StepCode]string{}
	}
	m[code] = summary
	return m
}

// withoutNarrationSummary 作廢某一碼已經算好的摘要——用在中斷／恢復把上游判定
// 推翻的情況，理由同 withoutStepNarration：留著一份已經不成立的摘要，比讓那一
// 格在 terminal 退回固定 Label 更糟。
func withoutNarrationSummary(m map[domain.StepCode]string, code domain.StepCode) map[domain.StepCode]string {
	if m == nil {
		return nil
	}
	delete(m, code)
	return m
}

// summarizeUnderstandInput 是 UNDERSTAND_INPUT 這一碼的敘述輸入。
//
// 刻意摘要「買家要的是什麼」而不是「這一步做了什麼」：EXTRACT_SLOTS 的敘述
// 已經是「正在讀你的需求，抓品項」那個角度，兩碼若都講「讀需求」會在畫面上
// 變成兩行講同一件事。這一碼負責把買家原話濃縮成一句，讓對話紀錄第一行就
// 說得出「你要的是什麼」。
//
// 只餵買家自己打的字，不摻任何抽取結果——抽取還沒發生（這一碼在管線最前面），
// 摻進去會讓敘述宣稱一件當下還不成立的事。
func summarizeUnderstandInput(message string) string {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return "買家這一輪沒有輸入文字。"
	}
	return fmt.Sprintf("買家的原話：「%s」。請用一句話說出他想找什麼。", trimmed)
}

// summarizeNextQuestion 是 PICK_NEXT_QUESTION／REQUIREMENT_READY 這兩碼的敘述
// 輸入——同一個函式涵蓋兩碼，因為它們是同一個決策的兩種結果（還缺東西 vs 齊了）。
func summarizeNextQuestion(question *domain.ClarifyQuestion, missing []domain.SlotKey, ready bool) string {
	if ready {
		return "必填欄位都齊了，可以進入候選比對。"
	}
	var b strings.Builder
	if len(missing) > 0 {
		b.WriteString("還缺的必填欄位：")
		for i, m := range missing {
			if i > 0 {
				b.WriteString("、")
			}
			b.WriteString(domain.SlotLabels[m])
		}
		b.WriteString("。")
	}
	if question != nil {
		fmt.Fprintf(&b, "這一輪要問的是「%s」。", domain.SlotLabels[question.SlotKey])
	}
	if b.Len() == 0 {
		return "這一輪沒有要再問的欄位。"
	}
	return b.String()
}
