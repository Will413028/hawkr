// services/ai-service/internal/procurement/usecase/graph_terminal.go
package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"go.uber.org/zap"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"

	"github.com/Will413028/hawkr/ai-service/internal/observability"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/syncutil"
)

// graphTurnProgress accumulates the values extract_slots/validate_slots/
// crop_gate each compute, threaded across the graph's typed-chain gap
// (the terminal is the only emitting node after crop_resume_reconcile —
// so nothing downstream of it can read these off a typed node input).
// Each producing node reads the current value, updates the one field it
// owns, and writes it back via emitGraphTurnProgress — see
// graph_nodes.go's NewExtractSlotsNode/NewValidateSlotsNode/NewCropGateNode
// for the three writers, and graph_terminal.go's runGraphTerminalNode
// (Task 5) for the one reader.
type graphTurnProgress struct {
	Degraded bool
	// Patch is this turn's accumulated patch — extract_slots' raw output (or a
	// zero value on a degraded turn), plus crop_gate's own ItemResolution, the
	// one slot the server fills rather than the buyer (see NewCropGateNode).
	// domain.RecordedThisTurn needs the raw patch, not just the slots it
	// produced, to know what was newly recorded this turn. It also lands in the
	// answer_patch audit row, where carrying the resolution is correct: it is
	// part of what this turn changed.
	Patch domain.SlotPatch
	// NextSlots is validate_slots' Apply result, then crop_gate's ItemResolution
	// written on top — the final "next" slots this turn produces. crop_gate no
	// longer overrides Item itself (spec §4.3's override went with
	// cropidentity.GateResult.Rejected, which had no producer after Cut 1).
	NextSlots      domain.Slots
	Rejected       []domain.RejectedField
	CropPending    bool
	CropCandidates []domain.CropDisambiguationOption
	CropSelection  *domain.CropSelection
	// CropGateEvaluated 是 crop_gate node 那一輪 cropidentity.GateResult.Evaluated() 的值
	// ——gate 有沒有做出新的判定。composeGraphTurnResult 用它決定 steps[] 要不要
	// 有 CROP_GATE 這一格。不能在 terminal 自己從 CropPending／CropCandidates 反推：
	// 「品項沒變動、只是把上一輪的待選狀態帶著走」那條路徑的 CropPending 同樣是
	// true，反推會讓消歧期間每一輪都多出一格內容相同的步驟。
	CropGateEvaluated bool

	// Narration 是 extract_slots 已經算出來的 LLM 敘述，key 是 StepCode。這句話
	// 本來只送進 Activity 事件、落地就丟掉（2026-09-03 前 graphTurnProgress 沒有
	// 任何欄位留它）；留在這裡是為了讓 composeGraphTurnResult 把它填進持久化的
	// steps[]，**不需要多打任何一次 LLM**——同一份敘述同時走即時串流與對話紀錄
	// 兩條路。validate_slots／crop_gate 改走 NarrationSummaries，不再寫這個欄位。
	Narration domain.StepNarration
	// NarrationSummaries 是 validate_slots／crop_gate 算好、留給 terminal 生成的
	// 敘述輸入。這兩個節點不自己打 narrator：那會讓每輪多兩次序列呼叫，而 turn
	// 尾本來就有一個平行群組在做同一件事。
	NarrationSummaries map[domain.StepCode]string
}

// readGraphTurnProgress reads "graph_turn_progress" out of session state,
// tolerating an absent key as a zero-value progress (extract_slots is the
// first node to touch this key — resolveGraphTurnState deliberately does
// not seed it, unlike current_slots/request_id/history,
// which are genuine per-turn constants known before the run starts).
func readGraphTurnProgress(ctx agent.Context) (graphTurnProgress, error) {
	progress, err := stateValue[graphTurnProgress](ctx, "graph_turn_progress")
	if err == nil {
		return progress, nil
	}
	if errors.Is(err, session.ErrStateKeyNotExist) {
		return graphTurnProgress{}, nil
	}
	return graphTurnProgress{}, err
}

// decodeGraphTurnProgress mirrors stateValue's dual-path decode (direct type
// assertion for the in-memory-session fast path, JSON round-trip fallback for
// a Postgres-backed session) but operates on a raw event StateDelta value
// rather than an agent.Context — graphTurnExecutor (graph_dispatch.go) reads
// graph_turn_progress directly off yielded session.Events, not from inside a
// node (Sub-project 3b, Task 6).
func decodeGraphTurnProgress(raw any) (graphTurnProgress, bool) {
	if p, ok := raw.(graphTurnProgress); ok {
		return p, true
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return graphTurnProgress{}, false
	}
	var p graphTurnProgress
	if err := json.Unmarshal(b, &p); err != nil {
		return graphTurnProgress{}, false
	}
	return p, true
}

// emitGraphTurnProgress persists progress into session state via the
// emitted event's StateDelta — session.NewEvent always initializes
// Actions.StateDelta as a non-nil map (session.go's NewEvent), so no nil
// check is needed before assigning into it. A bare state-only event (no
// Content) is intentionally distinct from the Activity events
// emitActivityEvent produces: graphTurnExecutor's event-inspection loop
// only branches on ev.Content.Parts[].FunctionCall, so this event is
// invisible to (and does not perturb) that existing ledger-translation
// logic — it exists purely to get merged into session state via the
// runner's own AppendEvent call (which happens for every yielded event
// before the caller ever sees it).
func emitGraphTurnProgress(ctx agent.Context, emit func(*session.Event) error, progress graphTurnProgress) error {
	ev := session.NewEvent(ctx, ctx.InvocationID())
	ev.Actions.StateDelta["graph_turn_progress"] = progress
	return emit(ev)
}

// composeGraphTurnResult builds this turn's persisted payload and final
// slots from the graph's accumulated progress — the graph-path counterpart
// of HandleStream's tail (turn.go's PlanDialogue/RenderPlan/ComputeTurnSteps
// sequence), reusing the exact same pure domain/usecase helpers so the two
// composition paths can never silently drift apart (spec §3 approach A).
// Quota has no parameter here and never will (spec §4.3): it lives entirely
// in graphTurnExecutor's Go-land (reserve/confirm/release around this call),
// deliberately never threaded into this function. RFQ (Sub-project 3a Task 1)
// and price-preview hasPreview/nextAction (Sub-project 3a Task 2) are both
// real now, mirroring HandleStream's own turn.go derivations. SurfaceRealizer
// (#2549 Phase 3, Task 8) is now consulted via realizeOrFallback — blocks/
// reply/RenderMode carry whatever it returns; a nil realizer (or any
// failure inside realizeOrFallback) degrades to the exact same
// domain.RenderPlan template this comment used to describe as the only
// path.
//
// status ("CLARIFYING"/"REQUIREMENT_READY") is returned alongside slots/
// payload, not recomputed a second time by graphTurnExecutor, because
// computeOptionalStreak needs it internally and it is the same value
// AppendInput.RequestStatus needs (spec §4.5).
func composeGraphTurnResult(ctx context.Context, requestID int64, current domain.Slots, history []repository.RevisionWithPatch, progress graphTurnProgress, message string, rfqState domain.RfqConfirmationState, pricePreviewChecker PricePreviewChecker, narrator StepNarrator, realizer SurfaceRealizer) (domain.Slots, string, turnPayload, string) {
	next := progress.NextSlots
	rejected := progress.Rejected
	if rejected == nil {
		rejected = []domain.RejectedField{}
	}
	missing := next.MissingRequired()
	ready := len(missing) == 0
	status := "CLARIFYING"
	if ready {
		status = "REQUIREMENT_READY"
	}

	// Price-preview nudge — verbatim port of HandleStream's own logic
	// (turn.go:520-528): gated on ready (a query is wasted, and a
	// misleading nextAction could fire, before the requirement is even
	// complete), fail-safe split between hasPreview/previewKnown so a
	// query error or nil checker never gets mistaken for "confirmed no
	// preview exists yet."
	hasPreview, previewKnown := false, false
	if ready && pricePreviewChecker != nil {
		if hp, err := pricePreviewChecker.HasPricePreview(ctx, requestID); err == nil {
			hasPreview, previewKnown = hp, true
		}
	}
	nextAction := ""
	// 報價預覽只看供應商報價，不需要品項分類，所以分類查詢失敗
	// 也照樣催。
	if previewKnown && !hasPreview {
		nextAction = "REVIEW_PRICE_PREVIEW"
	}

	skipped := deriveSkippedSlots(history)
	prevStreak := previousOptionalStreak(history)
	question, capExhausted := domain.PickNextQuestion(next, missing, skipped, prevStreak)
	if progress.CropPending {
		question = domain.BuildCropDisambiguationQuestion(progress.CropCandidates, progress.CropSelection)
		capExhausted = false
	}
	optionalStreak := computeOptionalStreak(status, question, capExhausted, prevStreak)

	recorded := domain.RecordedThisTurn(progress.Patch, current, next)

	var tipRevisionID int64
	if len(history) > 0 {
		tipRevisionID = history[len(history)-1].ID
	}

	plan := domain.PlanDialogue(domain.DialoguePlanInput{
		Current: current, Next: next, Missing: missing, Ready: ready,
		Rejected: rejected, HasRejected: len(rejected) > 0, Degraded: progress.Degraded,
		Recorded: recorded, Question: question,
		CropDisambiguationPending:    progress.CropPending,
		CropDisambiguationCandidates: progress.CropCandidates,
		PricePreviewAvailable:        hasPreview,
		RfqConfirmationState:         rfqState,
	})
	plan.StateVersion = strconv.FormatInt(requestID, 10) + "-rev" + strconv.FormatInt(tipRevisionID, 10)
	plan.PlanHash = hashDialoguePlan(plan)

	renderArgs := domain.SafeRenderArgs{
		MissingRequiredLabels: missingRequiredLabels(missing),
		SalientFactLabels:     salientFactLabels(plan, recorded),
		NextQuestionPrompt:    questionPrompt(question),
		RejectedLabels:        rejectedLabels(rejected),
	}

	// blocks 與 steps 敘述併在同一個平行群組：realize 與 narrate 之間沒有資料
	// 相依，序列執行等於把 RealizeTimeout 疊在 narrationTimeout 後面。
	//
	// blocks／renderMode 宣告在這裡（g.Go 閉包外層）是刻意的：它們由下面的
	// goroutine 寫、g.Wait() 之後才讀，宣告在閉包內層的話賦值會落在區域變數
	// 上，這裡讀到的永遠是這行的初始值。
	blocks, renderMode := domain.RenderPlan(plan, renderArgs), "TEMPLATE"
	narration := progress.Narration

	g := syncutil.NewGroup(ctx)
	g.Go(func() error {
		blocks, renderMode = realizeOrFallback(ctx, realizer, plan, renderArgs, next, buildRecentNarratives(history))
		return nil
	})

	// UNDERSTAND_INPUT／PICK_NEXT_QUESTION 兩碼沒有對應的 emitting node（前者是
	// 「收到輸入」這件事本身，後者的決策就發生在這個函式裡），所以敘述在這裡補。
	//
	// 兩個平行跑：narrateStepOutcome 各自有 narrationTimeout（3s）的上限，序列
	// 執行等於把最壞情況疊成 6s 加在每一輪的尾巴上。它們之間沒有資料相依，
	// 沒有理由排隊。narrateStepOutcome 本身永不回錯（逾時或被 guard 擋下就退回
	// fallback），所以這裡不需要錯誤匯集。
	//
	// jobs／texts 宣告在 if 外層，讓下面 g.Wait() 之後的 withStepNarration 迴圈
	// 不管 narrator 是否為 nil 都能跑（nil 時兩者都是零值，迴圈是 no-op）——
	// group 的建立與 Wait 不能被這個條件包住，否則 realize 那個 job 也會被
	// 一起跳過。
	var jobs []struct {
		code    domain.StepCode
		summary string
	}
	var texts []string
	if narrator != nil {
		finalCode := domain.StepPickNextQuestion
		if ready {
			finalCode = domain.StepRequirementReady
		}
		// SLOTS_REJECTED 併進同一個平行群組，不另外排隊：多一個序列的
		// narrateStepOutcome 就是在有被拒欄位的那一輪多疊一個 narrationTimeout
		// （3s）。它的條件與 ComputeTurnSteps 產不產生那一格完全相同，兩者必須
		// 同步——否則不是白燒一次呼叫，就是有格子卻是空的。
		jobs = []struct {
			code    domain.StepCode
			summary string
		}{
			{domain.StepUnderstandInput, summarizeUnderstandInput(message)},
			{finalCode, summarizeNextQuestion(question, missing, ready)},
		}
		// validate_slots／crop_gate 留下的摘要在這裡才生成敘述——與 UNDERSTAND_INPUT／
		// 下一題那兩筆併在同一個平行群組，整條 tail 是 max 不是 sum。
		//
		// CROP_GATE 額外要求 cropGateEvaluated：判準必須與 ComputeTurnSteps 產不
		// 產生那一格的判準是同一個值，否則敘述生成出來會被 applyStepNarration 丟掉。
		// 光看摘要非空不夠——progress 跨輪次存活，上一輪的摘要在這一輪還讀得到。
		for _, code := range []domain.StepCode{domain.StepValidateSlots, domain.StepCropGate} {
			if code == domain.StepCropGate && !progress.CropGateEvaluated {
				continue
			}
			if summary := progress.NarrationSummaries[code]; summary != "" {
				jobs = append(jobs, struct {
					code    domain.StepCode
					summary string
				}{code, summary})
			}
		}
		if len(rejected) > 0 {
			jobs = append(jobs, struct {
				code    domain.StepCode
				summary string
			}{domain.StepSlotsRejected, summarizeSlotsRejected(rejected)})
		}

		// 用 syncutil.Group 而不是裸 goroutine + sync.WaitGroup：裸 goroutine 裡的
		// panic 沒有任何攔截點，會直接帶走整個 process——請求層的 recover
		// interceptor 攔不到別的 goroutine。narrateStepOutcome 會呼叫外部模型
		// provider，正是最不該讓它有能力終結 process 的那種相依。
		texts = make([]string, len(jobs))
		for i, j := range jobs {
			g.Go(func() error {
				texts[i] = narrateStepOutcome(ctx, narrator, j.code, j.summary,
					domain.StepLabels[j.code])
				return nil
			})
		}
	}
	if err := g.Wait(); err != nil {
		// 只可能是被 recover 的 panic（narrateStepOutcome 自己永不回錯，
		// realizeOrFallback 也永不回錯——任何失敗都在內部退回模板）。
		pkglogger.Error(ctx, "procurement: turn tail composition panicked", zap.Error(err))
	}
	for i, j := range jobs {
		narration = withStepNarration(narration, j.code, texts[i])
	}
	reply := domain.TextFallback(blocks)
	steps := domain.ComputeTurnSteps(domain.TurnStepTimings{}, next, len(rejected) > 0, ready, progress.Degraded, progress.CropGateEvaluated, narration)

	payload := turnPayload{
		Message: message, Patch: progress.Patch, Rejected: rejected, Degraded: progress.Degraded,
		Reply: reply, Blocks: blocks, RenderMode: renderMode, Steps: steps,
		RecordedThisTurn: recorded, Question: question, OptionalStreak: optionalStreak,
		DialoguePlan: &plan,
	}
	return next, status, payload, nextAction
}

// emitTurnResultFuncName marks a session.Event as carrying this turn's
// fully composed result — the one-shot terminal counterpart to
// emitActivityEventFuncName (graph_nodes.go, turn-scoped narration) and
// cropDisambiguationInterruptKind's LongRunningToolIDs signal
// (graph_interrupt.go, a pause signal): this marks "the graph is done
// deciding what to say," for graphTurnExecutor (Task 6) to capture after
// its r.Run() loop ends.
const emitTurnResultFuncName = "emit_turn_result"

// graphTurnResultPayload is what a terminal node's marker event carries —
// composeGraphTurnResult's four return values bundled into one struct so
// a single FunctionCall.Args["payload"] carries everything Task 6's
// persistence step needs (AppendRevision's Slots/RequestStatus/PayloadJSON
// inputs), plus NextAction (Sub-project 3a Task 2) for graphTurnExecutor's
// TurnResult.
type graphTurnResultPayload struct {
	Slots      domain.Slots
	Status     string
	Payload    turnPayload
	NextAction string
}

// runGraphTerminalNode is the body graph.go's single turn_terminal node
// calls — there is a single terminal since F2; the reply is composed purely
// from slot completeness (`ready` is still computed deterministically from
// missing, exactly as HandleStream does). Reads the per-turn constants
// resolveGraphTurnState seeded before r.Run() plus the graph_turn_progress
// accumulated by extract_slots/validate_slots/crop_gate, composes via
// composeGraphTurnResult, and emits the result as a marker event for
// graphTurnExecutor to capture.
//
// A failure to emit this event is NOT treated as log-only (unlike
// emitActivityEvent/emitGraphTurnProgress's discipline elsewhere in this
// file) — this event carries this turn's only copy of its composed
// content; if the caller can never be told what happened, the run
// genuinely failed, so the error propagates to the node's own return value
// and becomes runErr in graphTurnExecutor's loop.
func runGraphTerminalNode(ctx agent.Context, emit func(*session.Event) error, pricePreviewChecker PricePreviewChecker, narrator StepNarrator, realizer SurfaceRealizer) (any, error) {
	requestID, err := stateValue[int64](ctx, "request_id")
	if err != nil {
		return nil, err
	}
	current, err := stateValue[domain.Slots](ctx, "current_slots")
	if err != nil {
		return nil, err
	}
	history, err := stateValue[[]repository.RevisionWithPatch](ctx, "history")
	if err != nil {
		return nil, err
	}
	rfqState, err := stateValue[domain.RfqConfirmationState](ctx, "rfq_state")
	if err != nil {
		return nil, err
	}
	progress, err := readGraphTurnProgress(ctx)
	if err != nil {
		return nil, err
	}
	message := firstContentText(ctx.UserContent())

	slots, status, payload, nextAction := composeGraphTurnResult(ctx, requestID, current, history, progress, message, rfqState, pricePreviewChecker, narrator, realizer)
	observability.RecordProcurementDialoguePlan(ctx, string(payload.DialoguePlan.ActCode))

	ev := session.NewEvent(ctx, ctx.InvocationID())
	ev.Content = &genai.Content{
		Parts: []*genai.Part{{
			FunctionCall: &genai.FunctionCall{
				Name: emitTurnResultFuncName,
				Args: map[string]any{"payload": graphTurnResultPayload{Slots: slots, Status: status, Payload: payload, NextAction: nextAction}},
			},
		}},
	}
	if emitErr := emit(ev); emitErr != nil {
		return nil, emitErr
	}
	return nil, nil
}
