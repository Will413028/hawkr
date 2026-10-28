// services/ai-service/internal/procurement/usecase/graph_terminal_test.go
package usecase

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/workflowagent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"
	"google.golang.org/genai"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/testkit"
)

// TestReadGraphTurnProgress_AbsentKey_ReturnsZeroValue proves the very
// first node to touch "graph_turn_progress" (extract_slots, which runs
// before anything ever writes this key) gets a usable zero value instead
// of stateValue's usual hard error on a missing key — this key is
// deliberately never seeded by resolveGraphTurnState, unlike current_slots/
// request_snapshot/request_id/history.
func TestReadGraphTurnProgress_AbsentKey_ReturnsZeroValue(t *testing.T) {
	ctx := newTestAgentContext(t, nil)

	got, err := readGraphTurnProgress(ctx)
	require.NoError(t, err)
	require.Equal(t, graphTurnProgress{}, got)
}

// TestReadGraphTurnProgress_PresentKey_ReturnsStoredValue proves the
// normal case: a value seeded into state is read back unchanged.
func TestReadGraphTurnProgress_PresentKey_ReturnsStoredValue(t *testing.T) {
	want := graphTurnProgress{Degraded: true, NextSlots: domain.Slots{Item: item("蘋果")}}
	ctx := newTestAgentContext(t, map[string]any{"graph_turn_progress": want})

	got, err := readGraphTurnProgress(ctx)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

// TestEmitGraphTurnProgress_WritesStateDeltaOnEmittedEvent proves
// emitGraphTurnProgress builds a real session.Event carrying the given
// progress in Actions.StateDelta, calling the injected emit callback
// exactly once — the mechanism validate_slots/crop_gate (Task 3) and the
// terminal nodes (Task 5) rely on to thread progress across the graph's
// typed-chain gap.
func TestEmitGraphTurnProgress_WritesStateDeltaOnEmittedEvent(t *testing.T) {
	ctx := newTestAgentContext(t, nil)
	progress := graphTurnProgress{Degraded: true}

	var got *session.Event
	err := emitGraphTurnProgress(ctx, func(ev *session.Event) error {
		got = ev
		return nil
	}, progress)
	require.NoError(t, err)

	require.NotNil(t, got)
	require.Equal(t, progress, got.Actions.StateDelta["graph_turn_progress"])
}

// TestComposeGraphTurnResult_MissingRequired_AsksNextQuestion mirrors
// HandleStream's own "not yet ready" scenario: a request with required
// slots still missing must produce a non-nil Question and status ==
// "CLARIFYING", via the exact same domain.PickNextQuestion the legacy path
// calls.
func TestComposeGraphTurnResult_MissingRequired_AsksNextQuestion(t *testing.T) {
	progress := graphTurnProgress{
		NextSlots: domain.Slots{Item: item("蘋果")}, // DeliveryCity/Unit (domain.Slots' other two required fields) still missing
	}

	slots, status, payload, _ := composeGraphTurnResult(t.Context(), 1, domain.Slots{}, nil, progress, "我要買蘋果", domain.RfqNone, nil, nil, nil)

	require.Equal(t, progress.NextSlots, slots)
	require.Equal(t, "CLARIFYING", status)
	require.NotNil(t, payload.Question, "expected a follow-up question while required slots are still missing")
	require.NotEmpty(t, payload.Reply)
	require.Equal(t, "TEMPLATE", payload.RenderMode, "nil realizer here — realizeOrFallback degrades to TEMPLATE")
	require.False(t, payload.Degraded)
}

// TestComposeGraphTurnResult_Degraded_SetsDegradedTrue proves
// progress.Degraded flows through into the composed turnPayload.
func TestComposeGraphTurnResult_Degraded_SetsDegradedTrue(t *testing.T) {
	progress := graphTurnProgress{Degraded: true}

	_, _, payload, _ := composeGraphTurnResult(t.Context(), 1, domain.Slots{}, nil, progress, "嗯嗯嗯", domain.RfqNone, nil, nil, nil)
	require.True(t, payload.Degraded)
}

// TestComposeGraphTurnResult_CropPending_AsksDisambiguationQuestion proves
// a pending crop-disambiguation overrides the usual PickNextQuestion
// result — mirrors HandleStream's turn.go handling of cropGate.Pending.
func TestComposeGraphTurnResult_CropPending_AsksDisambiguationQuestion(t *testing.T) {
	candidates := []domain.CropDisambiguationOption{{Label: "富士蘋果"}, {Label: "五爪蘋果"}}
	progress := graphTurnProgress{
		NextSlots:      domain.Slots{Item: item("蘋果")},
		CropPending:    true,
		CropCandidates: candidates,
	}

	want := domain.BuildCropDisambiguationQuestion(candidates)
	_, _, payload, _ := composeGraphTurnResult(t.Context(), 1, domain.Slots{}, nil, progress, "我要買蘋果", domain.RfqNone, nil, nil, nil)

	require.Equal(t, want, payload.Question)
}

// TestComposeGraphTurnResult_AllRequiredFilled_ReadyToFinalize proves the
// REQUIREMENT_READY branch: a fully-filled Slots produces status ==
// "REQUIREMENT_READY" and an empty MissingRequired (checked indirectly via
// the returned slots — composeGraphTurnResult itself doesn't return
// MissingRequired; graphTurnExecutor, Task 6, recomputes it from slots).
func TestComposeGraphTurnResult_AllRequiredFilled_ReadyToFinalize(t *testing.T) {
	// domain.Slots' required set is exactly Item/DeliveryCity/Unit
	// (domain/slots.go's requiredSlots — Quantity/BudgetPerUnit/DeliveryDate
	// are tracked but not required).
	full := domain.Slots{Item: item("蘋果"), DeliveryCity: strPtr("台北市"), Unit: strPtr("箱")}
	require.Empty(t, full.MissingRequired(), "sanity check: this fixture must be genuinely complete")

	progress := graphTurnProgress{NextSlots: full}
	_, status, payload, _ := composeGraphTurnResult(t.Context(), 1, domain.Slots{}, nil, progress, "都給你了", domain.RfqNone, nil, nil, nil)

	require.Equal(t, "REQUIREMENT_READY", status)
	require.NotEmpty(t, payload.Reply)
}

// TestComposeGraphTurnResult_RfqConfirmed_ThreadsIntoDialoguePlan proves
// rfqState (Task 1's new 6th parameter) actually reaches
// domain.PlanDialogue's RfqConfirmationState input, not just get accepted
// and dropped — asserted via the resulting DialoguePlan.ActCode differing
// from the RfqNone case, the same way HandleStream's own RFQ-state tests
// distinguish behavior (dialogue_planner.go:81-105's three-way branch).
func TestComposeGraphTurnResult_RfqConfirmed_ThreadsIntoDialoguePlan(t *testing.T) {
	full := domain.Slots{Item: item("蘋果"), DeliveryCity: strPtr("台北市"), Unit: strPtr("箱")}
	require.Empty(t, full.MissingRequired(), "sanity check: this fixture must be genuinely complete (Ready)")
	progress := graphTurnProgress{NextSlots: full}

	_, _, noneResult, _ := composeGraphTurnResult(t.Context(), 1, domain.Slots{}, nil, progress, "都給你了", domain.RfqNone, nil, nil, nil)
	_, _, confirmedResult, _ := composeGraphTurnResult(t.Context(), 1, domain.Slots{}, nil, progress, "都給你了", domain.RfqConfirmed, nil, nil, nil)

	require.NotEqual(t, noneResult.DialoguePlan.ActCode, confirmedResult.DialoguePlan.ActCode,
		"RfqConfirmationState must change which dialogue act gets selected when Ready — otherwise composeGraphTurnResult accepted rfqState and silently ignored it")
}

// TestComposeGraphTurnResult_ReadyWithoutPricePreview_SetsNextAction proves
// the headline case: Ready + no price preview yet → the buyer gets nudged
// to go create one (turn.go:526-528's exact condition, previewKnown && !hasPreview).
func TestComposeGraphTurnResult_ReadyWithoutPricePreview_SetsNextAction(t *testing.T) {
	// All required AND optional slots filled — not just the three required
	// ones — so PickNextQuestion returns a nil Question. Otherwise
	// selectActCode's `case in.Question != nil` (dialogue_planner.go) fires
	// before the `case in.PricePreviewAvailable` branch this sanity check
	// needs to actually reach.
	full := domain.Slots{
		Item: item("蘋果"), DeliveryCity: strPtr("台北市"), Unit: strPtr("箱"),
		Quantity: intPtr(10), DeliveryDate: strPtr("2026-09-10"), Packaging: strPtr("紙箱"), BudgetPerUnit: intPtr(50),
	}
	require.Empty(t, full.MissingRequired(), "sanity check: this fixture must be genuinely complete (Ready)")
	progress := graphTurnProgress{NextSlots: full}

	_, _, payload, nextAction := composeGraphTurnResult(t.Context(), 1, domain.Slots{}, nil, progress, "都給你了", domain.RfqNone, fakePricePreviewChecker{hasPreview: false}, nil, nil)

	require.Equal(t, "REVIEW_PRICE_PREVIEW", nextAction)
	// DialoguePlanV1 has no PricePreviewAvailable field of its own (that's a
	// DialoguePlanInput field) — ActCode is the observable proxy:
	// selectActCode only reaches ActRequestBasicsReady, instead of
	// ActPricePreviewAvailable, when PricePreviewAvailable is false. This
	// proves hasPreview really got threaded into domain.PlanDialogue's
	// input, not just used to compute nextAction in isolation.
	require.Equal(t, domain.ActRequestBasicsReady, payload.DialoguePlan.ActCode,
		"PricePreviewAvailable=false must not select ActPricePreviewAvailable")
}

// TestComposeGraphTurnResult_ReadyWithPricePreview_NoNextAction proves the
// opposite: a preview already exists — no nudge needed. It also proves the
// "true" direction of the hasPreview -> DialoguePlanInput.PricePreviewAvailable
// threading (the sibling ..._SetsNextAction test above only proves the
// "false" direction via ActRequestBasicsReady) — asserted the same way, via
// ActCode, since DialoguePlanV1 has no PricePreviewAvailable field of its
// own. Like the sibling test, this needs ALL optional slots filled (not just
// the three required ones), otherwise PickNextQuestion returns a non-nil
// Question and selectActCode's `case in.Question != nil` branch
// (dialogue_planner.go) fires before `case in.PricePreviewAvailable` is ever
// reached — confirmed by hand-tracing selectActCode's branch order together
// with a throwaway probe test against this exact fixture before writing this
// assertion.
func TestComposeGraphTurnResult_ReadyWithPricePreview_NoNextAction(t *testing.T) {
	full := domain.Slots{
		Item: item("蘋果"), DeliveryCity: strPtr("台北市"), Unit: strPtr("箱"),
		Quantity: intPtr(10), DeliveryDate: strPtr("2026-09-10"), Packaging: strPtr("紙箱"), BudgetPerUnit: intPtr(50),
	}
	require.Empty(t, full.MissingRequired(), "sanity check: this fixture must be genuinely complete (Ready)")
	progress := graphTurnProgress{NextSlots: full}

	_, _, payload, nextAction := composeGraphTurnResult(t.Context(), 1, domain.Slots{}, nil, progress, "都給你了", domain.RfqNone, fakePricePreviewChecker{hasPreview: true}, nil, nil)

	require.Empty(t, nextAction)
	require.Equal(t, domain.ActPricePreviewAvailable, payload.DialoguePlan.ActCode,
		"hasPreview=true must thread into DialoguePlanInput.PricePreviewAvailable, not just gate nextAction in isolation")
}

// TestComposeGraphTurnResult_NotReady_NoNextActionRegardlessOfPreview proves
// the ready-gate itself: HasPricePreview must not even be consulted (turn.go:521's
// `if ready && ...`) when required slots are still missing — asserted here via
// outcome (empty nextAction) rather than a call-count spy, matching this file's
// existing black-box testing style for composeGraphTurnResult.
func TestComposeGraphTurnResult_NotReady_NoNextActionRegardlessOfPreview(t *testing.T) {
	progress := graphTurnProgress{NextSlots: domain.Slots{Item: item("蘋果")}} // DeliveryCity/Unit still missing

	_, _, _, nextAction := composeGraphTurnResult(t.Context(), 1, domain.Slots{}, nil, progress, "我要買蘋果", domain.RfqNone, fakePricePreviewChecker{hasPreview: false}, nil, nil)

	require.Empty(t, nextAction, "not Ready — the price-preview nudge must not fire even though hasPreview=false")
}

// TestComposeGraphTurnResult_PricePreviewQueryError_NoNextAction proves the
// fail-safe split (hasPreview/previewKnown, turn.go:520): a query error must
// leave nextAction empty, not be misread as "no preview exists, nag the buyer".
func TestComposeGraphTurnResult_PricePreviewQueryError_NoNextAction(t *testing.T) {
	full := domain.Slots{Item: item("蘋果"), DeliveryCity: strPtr("台北市"), Unit: strPtr("箱")}
	progress := graphTurnProgress{NextSlots: full}

	_, _, _, nextAction := composeGraphTurnResult(t.Context(), 1, domain.Slots{}, nil, progress, "都給你了", domain.RfqNone, fakePricePreviewChecker{err: errors.New("boom")}, nil, nil)

	require.Empty(t, nextAction, "a query error must not be mistaken for 'confirmed no preview exists'")
}

// TestComposeGraphTurnResult_NilPricePreviewChecker_NoNextAction proves the
// nil-checker fail-safe used elsewhere in this package (crop resolver,
// rfqConfirmationReader) applies here too — never panics, degrades cleanly.
func TestComposeGraphTurnResult_NilPricePreviewChecker_NoNextAction(t *testing.T) {
	full := domain.Slots{Item: item("蘋果"), DeliveryCity: strPtr("台北市"), Unit: strPtr("箱")}
	progress := graphTurnProgress{NextSlots: full}

	_, _, _, nextAction := composeGraphTurnResult(t.Context(), 1, domain.Slots{}, nil, progress, "都給你了", domain.RfqNone, nil, nil, nil)

	require.Empty(t, nextAction)
}

// readySlots returns a domain.Slots with the three required fields
// (Item/DeliveryCity/Unit) AND Quantity all filled, but DeliveryDate/
// Packaging/BudgetPerUnit still empty. Ready is therefore true, but
// domain.PickNextQuestion still has an eligible optional slot to ask about
// (DeliveryDate is next in optionalSlotOrder once Quantity is filled) —
// selectActCode's `case in.Question != nil` branch (dialogue_planner.go)
// fires before `case in.PricePreviewAvailable`, landing on
// ActAckAndAskOptional. This is deliberately NOT the same fixture as the
// "full" literal used by TestComposeGraphTurnResult_AllRequiredFilled_
// ReadyToFinalize/_RfqConfirmed_ThreadsIntoDialoguePlan (Item/DeliveryCity/
// Unit only, no Quantity) — those two only need Ready, not a specific act.
func readySlots(t *testing.T) domain.Slots {
	t.Helper()
	slots := domain.Slots{
		Item: item("蘋果"), DeliveryCity: strPtr("台北市"), Unit: strPtr("箱"),
		Quantity: intPtr(50),
	}
	require.Empty(t, slots.MissingRequired(), "sanity check: readySlots must be genuinely Ready")
	return slots
}

// TestComposeGraphTurnResult_ModelBlocksSetRenderModeModel proves the /turns
// wiring itself (#2549 Phase 3, Task 8): a SurfaceRealizer that returns
// blocks makes it all the way into payload.Blocks/Reply, and RenderMode
// honestly reports "MODEL" instead of the old hardcoded "TEMPLATE" literal.
func TestComposeGraphTurnResult_ModelBlocksSetRenderModeModel(t *testing.T) {
	realizer := &fakeSurfaceRealizer{blocks: []RealizedBlock{{
		Kind: domain.BlockNextStep, Text: "想問一下大概什麼時候要用？",
		ClaimCodes: []string{string(domain.ClaimNextQuestionOptional)},
	}}}

	_, _, payload, _ := composeGraphTurnResult(
		context.Background(), 1, domain.Slots{}, nil,
		graphTurnProgress{NextSlots: readySlots(t)}, "50 公斤",
		domain.RfqNone, nil, nil, realizer,
	)

	if payload.RenderMode != "MODEL" {
		t.Fatalf("RenderMode = %q, want MODEL", payload.RenderMode)
	}
	if payload.Reply != "想問一下大概什麼時候要用？" {
		t.Fatalf("Reply = %q，textFallback 必須是模型 blocks 的 join", payload.Reply)
	}
}

// TestComposeGraphTurnResult_NilRealizerKeepsTemplate proves the fail-safe
// side of the same wiring: DI not wired up (or a local run with no API key)
// must not panic and must keep RenderMode == "TEMPLATE".
func TestComposeGraphTurnResult_NilRealizerKeepsTemplate(t *testing.T) {
	_, _, payload, _ := composeGraphTurnResult(
		context.Background(), 1, domain.Slots{}, nil,
		graphTurnProgress{NextSlots: readySlots(t)}, "50 公斤",
		domain.RfqNone, nil, nil, nil,
	)
	if payload.RenderMode != "TEMPLATE" {
		t.Fatalf("RenderMode = %q, want TEMPLATE", payload.RenderMode)
	}
}

// newTerminalMarkerRunner builds a *runner.Runner over a minimal graph —
// Start -> a seed node that writes an arbitrary graph_turn_progress into
// state -> runGraphTerminalNode itself (the function under test, wrapped
// exactly the way graph.go's three real terminal nodes wrap it) — instead
// of the full NewProcurementGraph, following this package's own established
// test-isolation precedent (graph_dispatch_test.go's newPendingInterruptRunner/
// newNotPendingRunner/newActivityEmittingRunner): this file's job is
// proving runGraphTerminalNode's own composition+emission logic, not
// re-proving extract_slots/validate_slots/crop_gate/the controller, which
// are already covered elsewhere and would require a real or fake LLM to
// reach a terminal node through the genuine graph.
func newTerminalMarkerRunner(t *testing.T, progress graphTurnProgress) *runner.Runner {
	t.Helper()

	seedNode := workflow.NewEmittingFunctionNode(
		"test_seed_progress",
		func(ctx agent.Context, _ any, emit func(*session.Event) error) (any, error) {
			return nil, emitGraphTurnProgress(ctx, emit, progress)
		},
		workflow.NodeConfig{},
	)
	terminalNode := workflow.NewEmittingFunctionNode(
		"test_terminal",
		func(ctx agent.Context, _ any, emit func(*session.Event) error) (any, error) {
			return runGraphTerminalNode(ctx, emit, nil, nil, nil)
		},
		workflow.NodeConfig{},
	)

	wfAgent, err := workflowagent.New(workflowagent.Config{
		Name:  "graph_terminal_test_chain",
		Edges: workflow.Chain(workflow.Start, seedNode, terminalNode),
	})
	require.NoError(t, err)

	r, err := runner.New(runner.Config{
		AppName: string(ProcurementAppName), Agent: wfAgent,
		SessionService: session.InMemoryService(), AutoCreateSession: true,
	})
	require.NoError(t, err)
	return r
}

// TestRunGraphTerminalNode_EmitsMarkerEventWithComposedResult drives a
// real ADK scheduler (not node.Run(ctx, input) directly — this test needs
// the seed node's StateDelta to actually merge into session state before
// the terminal node reads it back, which only happens via AppendEvent
// inside a real runner.Run() call) and asserts the yielded marker event
// carries composeGraphTurnResult's real output.
func TestRunGraphTerminalNode_EmitsMarkerEventWithComposedResult(t *testing.T) {
	progress := graphTurnProgress{NextSlots: domain.Slots{Item: item("蘋果")}}
	r := newTerminalMarkerRunner(t, progress)

	svc := session.InMemoryService()
	_ = svc // seeded state comes from runner.WithStateDelta below, not a pre-created session

	var got *graphTurnResultPayload
	for ev, err := range r.Run(t.Context(), "u1", "s1",
		&genai.Content{Role: genai.RoleUser, Parts: []*genai.Part{{Text: "我要買蘋果"}}},
		agent.RunConfig{},
		runner.WithStateDelta(map[string]any{
			"request_id": int64(1), "current_slots": domain.Slots{}, "history": []repository.RevisionWithPatch(nil),
			"rfq_state": domain.RfqNone,
		}),
	) {
		require.NoError(t, err)
		if ev == nil || ev.Content == nil {
			continue
		}
		for _, p := range ev.Content.Parts {
			fc := p.FunctionCall
			if fc == nil || fc.Name != emitTurnResultFuncName {
				continue
			}
			payload, ok := fc.Args["payload"].(graphTurnResultPayload)
			require.True(t, ok, "expected fc.Args[\"payload\"] to be a graphTurnResultPayload, got %T", fc.Args["payload"])
			got = &payload
		}
	}

	require.NotNil(t, got, "expected exactly one emit_turn_result marker event")
	require.Equal(t, progress.NextSlots, got.Slots)
	require.NotEmpty(t, got.Payload.Reply)
	require.NotNil(t, got.Payload.Question, "still missing required slots — expect a follow-up question")
}

// codeNarrator 依 StepCode 回不同句子，讓測試能斷言「每一碼拿到的是自己的敘述」
// 而不是全部共用同一個字串——共用字串會讓「敘述有沒有填錯格」這件事驗不出來。
type codeNarrator struct{ byCode map[domain.StepCode]string }

func (c *codeNarrator) Narrate(_ context.Context, code domain.StepCode, _ string) (string, error) {
	if t, ok := c.byCode[code]; ok {
		return t, nil
	}
	return "", nil
}

// TestComposeGraphTurnResult_NarratesUnderstandInputAndNextQuestion 守住這兩碼
// 的敘述。它們沒有對應的 emitting node（UNDERSTAND_INPUT 是「收到輸入」這件事
// 本身，PICK_NEXT_QUESTION 的決策就發生在 composeGraphTurnResult 裡），所以敘述
// 在那裡補；漏掉的話畫面上這兩列會退回固定文案。
func TestComposeGraphTurnResult_NarratesUnderstandInputAndNextQuestion(t *testing.T) {
	narrator := &codeNarrator{byCode: map[domain.StepCode]string{
		domain.StepUnderstandInput:  "你想找甘藍",
		domain.StepPickNextQuestion: "還要知道送到哪個城市",
	}}
	progress := graphTurnProgress{NextSlots: domain.Slots{Item: item("甘藍")}}

	_, _, payload, _ := composeGraphTurnResult(t.Context(), 1, domain.Slots{}, nil,
		progress, "我要買甘藍", domain.RfqNone, nil, narrator, nil)

	byCode := map[domain.StepCode]domain.Step{}
	for _, s := range payload.Steps {
		byCode[s.Code] = s
	}
	if got := byCode[domain.StepUnderstandInput].Text; got != "你想找甘藍" {
		t.Fatalf("UNDERSTAND_INPUT.Text = %q，敘述沒填進去", got)
	}
	if got := byCode[domain.StepPickNextQuestion].Text; got != "還要知道送到哪個城市" {
		t.Fatalf("PICK_NEXT_QUESTION.Text = %q", got)
	}
	// 兩碼各拿自己的句子，不得互相串位。
	if byCode[domain.StepUnderstandInput].Text == byCode[domain.StepPickNextQuestion].Text {
		t.Fatal("兩碼拿到同一句敘述——填錯格了")
	}
}

// TestComposeGraphTurnResult_NilNarrator_LeavesTextEmpty 確認沒接 narrator 的
// 呼叫端（graph_dispatch 的暫停分支在 deps.Narrator 未注入時）行為不變。
func TestComposeGraphTurnResult_NilNarrator_LeavesTextEmpty(t *testing.T) {
	progress := graphTurnProgress{NextSlots: domain.Slots{Item: item("甘藍")}}
	_, _, payload, _ := composeGraphTurnResult(t.Context(), 1, domain.Slots{}, nil,
		progress, "我要買甘藍", domain.RfqNone, nil, nil, nil)
	for _, s := range payload.Steps {
		if s.Text != "" {
			t.Fatalf("%s.Text = %q，nil narrator 不該產生任何文字", s.Code, s.Text)
		}
	}
}

// recordingNarrator 記下被要求敘述的每一碼與摘要，供「該不該打這通 LLM」的
// 斷言使用。回傳固定字串，內容不是這組測試關心的。
type recordingNarrator struct {
	mu       sync.Mutex
	askedFor map[domain.StepCode]string
}

func newRecordingNarrator() *recordingNarrator {
	return &recordingNarrator{askedFor: map[domain.StepCode]string{}}
}

func (n *recordingNarrator) Narrate(_ context.Context, code domain.StepCode, summary string) (string, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.askedFor[code] = summary
	return "敘述：" + string(code), nil
}

func (n *recordingNarrator) asked(code domain.StepCode) (string, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	s, ok := n.askedFor[code]
	return s, ok
}

func stepText(payload turnPayload, code domain.StepCode) (string, bool) {
	for _, s := range payload.Steps {
		if s.Code == code {
			return s.Text, true
		}
	}
	return "", false
}

// TestComposeGraphTurnResult_SlotsRejectedNarration 釘住 SLOTS_REJECTED 的
// 敘述：有被拒欄位時才問 LLM，且句子真的落到那一格；沒有被拒欄位時完全不問
// （多問一次就是白燒——applyStepNarration 找不到格子會把句子丟掉）。
func TestComposeGraphTurnResult_SlotsRejectedNarration(t *testing.T) {
	t.Run("有被拒欄位：問了 LLM，摘要帶上欄位與理由，句子落到 SLOTS_REJECTED", func(t *testing.T) {
		narrator := newRecordingNarrator()
		progress := graphTurnProgress{
			NextSlots: domain.Slots{Item: item("蘋果")},
			Rejected:  []domain.RejectedField{{Key: string(domain.SlotDeliveryDate), Reason: "缺少西元年份，請補上年份"}},
		}

		_, _, payload, _ := composeGraphTurnResult(t.Context(), 1, domain.Slots{}, nil, progress,
			"8/20 送到", domain.RfqNone, nil, narrator, nil)

		summary, asked := narrator.asked(domain.StepSlotsRejected)
		require.True(t, asked, "有被拒欄位卻沒有敘述這一碼")
		require.Contains(t, summary, "到貨日期")
		require.Contains(t, summary, "缺少西元年份，請補上年份")

		text, present := stepText(payload, domain.StepSlotsRejected)
		require.True(t, present, "steps[] 裡必須有 SLOTS_REJECTED 這一格")
		require.NotEmpty(t, text, "敘述必須落到那一格，不能算了又被丟掉")
	})

	t.Run("沒有被拒欄位：完全不問 LLM", func(t *testing.T) {
		narrator := newRecordingNarrator()
		progress := graphTurnProgress{NextSlots: domain.Slots{Item: item("蘋果")}}

		_, _, payload, _ := composeGraphTurnResult(t.Context(), 1, domain.Slots{}, nil, progress,
			"我要買蘋果", domain.RfqNone, nil, narrator, nil)

		_, asked := narrator.asked(domain.StepSlotsRejected)
		require.False(t, asked, "沒有被拒欄位就不該打這通 LLM——沒有格子可填，句子會被丟掉")
		_, present := stepText(payload, domain.StepSlotsRejected)
		require.False(t, present)
	})
}

// TestComposeGraphTurnResult_ConsumesDeferredNarrationSummaries 證明兩個節點
// 留下的摘要必須進 turn 尾的平行群組，持久化 steps 的 Text 因此與節點內生成
// 時完全相同。
func TestComposeGraphTurnResult_ConsumesDeferredNarrationSummaries(t *testing.T) {
	narrator := newRecordingNarrator()
	progress := graphTurnProgress{
		NextSlots: domain.Slots{Item: item("蘋果"), DeliveryCity: strPtr("台北市"), Unit: strPtr("箱")},
		NarrationSummaries: map[domain.StepCode]string{
			domain.StepValidateSlots: "validate 摘要",
			domain.StepCropGate:      "crop gate 摘要",
		},
		CropGateEvaluated: true,
	}

	_, _, payload, _ := composeGraphTurnResult(t.Context(), 1, domain.Slots{}, nil, progress,
		"我要蘋果", domain.RfqNone, nil, narrator, nil)

	validateText, present := stepText(payload, domain.StepValidateSlots)
	require.True(t, present, "steps[] 裡必須有 VALIDATE_SLOTS 這一格")
	require.Equal(t, "敘述："+string(domain.StepValidateSlots), validateText)

	cropText, present := stepText(payload, domain.StepCropGate)
	require.True(t, present, "steps[] 裡必須有 CROP_GATE 這一格")
	require.Equal(t, "敘述："+string(domain.StepCropGate), cropText)

	// 每一碼拿到的必須是自己那份摘要：固定回覆的 narrator 對「兩碼摘要互換」
	// 這類錯配零鑑別力，而錯配會讓買家在某一格讀到另一格的事。
	gotValidate, asked := narrator.asked(domain.StepValidateSlots)
	require.True(t, asked)
	require.Equal(t, "validate 摘要", gotValidate)

	gotCrop, asked := narrator.asked(domain.StepCropGate)
	require.True(t, asked)
	require.Equal(t, "crop gate 摘要", gotCrop)
}

// TestComposeGraphTurnResult_StaleCropSummaryNotNarrated 釘住跨輪次殘留：
// graph_turn_progress 活在以 requestID 為 key 的 ADK session 裡，會跨輪次存活，
// 所以上一輪留下的 CROP_GATE 摘要在這一輪 gate 沒做新判定時仍然讀得到。這種
// 時候 ComputeTurnSteps 不產生 CROP_GATE 那一格，敘述生成出來也會被
// applyStepNarration 丟掉——排這筆 job 就是白燒一次 LLM 呼叫。
func TestComposeGraphTurnResult_StaleCropSummaryNotNarrated(t *testing.T) {
	narrator := newRecordingNarrator()
	progress := graphTurnProgress{
		NextSlots: domain.Slots{Item: item("蘋果"), DeliveryCity: strPtr("台北市"), Unit: strPtr("箱")},
		NarrationSummaries: map[domain.StepCode]string{
			domain.StepValidateSlots: "validate 摘要",
			domain.StepCropGate:      "上一輪留下的 crop gate 摘要",
		},
		CropGateEvaluated: false,
	}

	_, _, payload, _ := composeGraphTurnResult(t.Context(), 1, domain.Slots{}, nil, progress,
		"再加兩箱", domain.RfqNone, nil, narrator, nil)

	_, asked := narrator.asked(domain.StepCropGate)
	require.False(t, asked, "gate 沒做新判定時不得為 CROP_GATE 生成敘述")

	_, present := stepText(payload, domain.StepCropGate)
	require.False(t, present, "steps[] 不該有 CROP_GATE 這一格")
}

// TestComposeGraphTurnResult_NoSummaryNoJob 沒有摘要的步驟碼不得憑空長出一筆
// job——多問一次就是白燒一次 LLM 呼叫，跟 SLOTS_REJECTED 那組既有測試同一個
// 判準（見 TestComposeGraphTurnResult_SlotsRejectedNarration）。
func TestComposeGraphTurnResult_NoSummaryNoJob(t *testing.T) {
	narrator := newRecordingNarrator()
	progress := graphTurnProgress{
		NextSlots:          domain.Slots{Item: item("蘋果"), DeliveryCity: strPtr("台北市"), Unit: strPtr("箱")},
		NarrationSummaries: map[domain.StepCode]string{},
	}

	_, _, _, _ = composeGraphTurnResult(t.Context(), 1, domain.Slots{}, nil, progress,
		"我要蘋果", domain.RfqNone, nil, narrator, nil)

	_, asked := narrator.asked(domain.StepValidateSlots)
	require.False(t, asked, "沒有摘要就不該打這通 LLM")
	_, asked = narrator.asked(domain.StepCropGate)
	require.False(t, asked, "沒有摘要就不該打這通 LLM")
}

// TestComposeGraphTurnResult_CategoryLookupFailed_StillNudgesToPricePreview 守住：
// 分類查詢出錯（resolver 回 error，不是查無結果）那一輪，品項沒有 ItemResolution，
// 其他條件齊了照樣進 REVIEW_PRICE_PREVIEW。報價預覽不依賴品項分類。
func TestComposeGraphTurnResult_CategoryLookupFailed_StillNudgesToPricePreview(t *testing.T) {
	full := domain.Slots{
		Item: item("芭樂"), DeliveryCity: strPtr("桃園市"), Unit: strPtr("KG"),
		Quantity: intPtr(10), DeliveryDate: strPtr("2026-09-30"), Packaging: strPtr("紙箱"), BudgetPerUnit: intPtr(50),
	}
	require.Empty(t, full.MissingRequired(), "sanity check：fixture 的 required slots 是完整的")

	resolver := &testkit.FakeResolver[cropidentity.DisambiguationResult]{ResolveErr: errors.New("category lookup down")}
	ctx := newTestAgentContext(t, map[string]any{
		"request_id":          int64(1),
		"current_slots":       domain.Slots{},
		"graph_turn_progress": graphTurnProgress{NextSlots: full},
	})
	got, err := runFunctionNode[CropGateOutput](t, NewCropGateNode(resolver, seqIDs(9100)), ctx, ValidateSlotsOutput{Slots: full})
	require.NoError(t, err)
	require.True(t, got.ResolveFailed, "sanity check：這一輪的分類查詢確實失敗")

	progress, err := readGraphTurnProgress(ctx)
	require.NoError(t, err)
	require.Nil(t, progress.NextSlots.ItemResolution, "查詢失敗時品項沒有解析結果")

	slots, _, _, nextAction := composeGraphTurnResult(t.Context(), 1, domain.Slots{}, nil, progress,
		"都給你了", domain.RfqNone, fakePricePreviewChecker{hasPreview: false}, nil, nil)

	require.Nil(t, slots.ItemResolution)
	require.Equal(t, "REVIEW_PRICE_PREVIEW", nextAction,
		"分類查詢失敗時也能進報價預覽")
}

// TestComposeGraphTurnResult_UnresolvedItem_StillNudgesToPricePreview 守住：報價預覽
// 不需要品項分類，item_resolution 沒有值（品項沒辨識出來，例如分類查詢失敗）的那一輪，
// 其他條件齊了照樣進 REVIEW_PRICE_PREVIEW。required slots 不含 item_resolution。
func TestComposeGraphTurnResult_UnresolvedItem_StillNudgesToPricePreview(t *testing.T) {
	full := domain.Slots{
		Item: item("芭樂"), DeliveryCity: strPtr("桃園市"), Unit: strPtr("KG"),
		Quantity: intPtr(10), DeliveryDate: strPtr("2026-09-30"), Packaging: strPtr("紙箱"), BudgetPerUnit: intPtr(50),
	}
	require.Empty(t, full.MissingRequired(), "sanity check：fixture 的 required slots 是完整的")
	require.Nil(t, full.ItemResolution, "sanity check：fixture 還沒有品項解析結果")

	progress := graphTurnProgress{NextSlots: full}

	_, _, _, nextAction := composeGraphTurnResult(t.Context(), 1, domain.Slots{}, nil, progress,
		"都給你了", domain.RfqNone, fakePricePreviewChecker{hasPreview: false}, nil, nil)

	require.Equal(t, "REVIEW_PRICE_PREVIEW", nextAction,
		"品項沒辨識出來時也能進報價預覽")
}
