package usecase

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/testkit"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/stretchr/testify/require"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"
)

// TestEmitActivityEventFuncName_Value 是一支防漂移斷言，不是在測「常數等於它
// 自己」：internal/grpc 的 newActivityEmittingRunner（procurement_stream_test.go）
// 必須從套件外偽造一筆真正會被 runGraphOutcome 認出來的 Activity 事件，而
// emitActivityEventFuncName 是 unexported，那支 fixture 只能逐字複製這個字面值。
//
// 改名時若沒同步改那支 fixture，它 emit 出去的事件會被 runGraphOutcome 的
// `fc.Name != emitActivityEventFuncName` 直接 continue 掉。那三支 grpc 測試
// 確實會紅（已實測），所以這不是在補一個「靜默通過」的漏洞——但它們的失敗訊息
// 只會說「一筆 Activity frame 都沒有」，看不出真正的原因是常數改名。這支測試
// 的價值是：改名的人在**自己這一側**先拿到一句明講「grpc 有一份逐字複本」的
// 失敗訊息，不必反推三支跨套件測試為什麼一起壞掉。
func TestEmitActivityEventFuncName_Value(t *testing.T) {
	if emitActivityEventFuncName != "emit_activity_event" {
		t.Fatalf("internal/grpc/procurement_stream_test.go 的 activityEmitFuncName 是這個值的逐字複本；"+
			"改名會讓該檔三支 Activity 測試一起紅、但訊息不會指出原因是改名。請一併更新那份複本: %q",
			emitActivityEventFuncName)
	}
}

func TestNewExtractSlotsNode_EmitsEntryThenCompletionActivityEvents(t *testing.T) {
	extractor := fakeExtractor{patch: domain.SlotPatch{Item: strPtr("蘋果")}}
	node := NewExtractSlotsNode(extractor, seqIDs(5000))
	ctx := newTestAgentContext(t, map[string]any{
		"current_slots": domain.Slots{},
	})

	var payloads []ActivityEventPayload
	for ev, err := range node.Run(ctx, nil) {
		require.NoError(t, err)
		if ev == nil || ev.Content == nil {
			continue
		}
		for _, p := range ev.Content.Parts {
			if p.FunctionCall != nil && p.FunctionCall.Name == emitActivityEventFuncName {
				payload, ok := p.FunctionCall.Args["payload"].(ActivityEventPayload)
				require.True(t, ok)
				payloads = append(payloads, payload)
			}
		}
	}

	require.Len(t, payloads, 2, "extract_slots must emit an entry snapshot then a completion snapshot")
	entry, completion := payloads[0], payloads[1]

	require.Equal(t, string(domain.StepExtractSlots), entry.Code)
	require.Empty(t, entry.Text, "entry snapshot must carry no text — it fires before the LLM call starts")
	require.NotZero(t, entry.RowID)

	require.Equal(t, string(domain.StepExtractSlots), completion.Code)
	require.NotEmpty(t, completion.Text)
	require.Equal(t, entry.RowID, completion.RowID, "entry and completion must share the same RowID so consumers merge them into one row")
	require.Equal(t, entry.RowID, completion.RetryGroupKey)
	require.Equal(t, 1, completion.Attempt)
	require.Equal(t, 1, completion.MaxAttempts)
}

func TestGuardShortIntent_ValidSentence_PassesThrough(t *testing.T) {
	got := guardShortIntent("正在讀你的需求，抓品項、數量和交期")
	require.Equal(t, "正在讀你的需求，抓品項、數量和交期", got)
}

func TestGuardShortIntent_Empty_FallsBackToStepLabel(t *testing.T) {
	got := guardShortIntent("")
	require.Equal(t, domain.StepLabels[domain.StepExtractSlots], got)
}

func TestGuardShortIntent_TooLong_FallsBackToStepLabel(t *testing.T) {
	got := guardShortIntent(strings.Repeat("字", shortIntentMaxLen+1))
	require.Equal(t, domain.StepLabels[domain.StepExtractSlots], got)
}

func TestGuardShortIntent_WhitespaceOnly_FallsBackToStepLabel(t *testing.T) {
	got := guardShortIntent("   \n\t  ")
	require.Equal(t, domain.StepLabels[domain.StepExtractSlots], got)
}

func TestGuardShortIntent_ExactlyAtLimit_PassesThrough(t *testing.T) {
	sentence := strings.Repeat("字", shortIntentMaxLen)
	got := guardShortIntent(sentence)
	require.Equal(t, sentence, got)
}

func TestNewValidateSlotsNode_EmitsEntryThenCompletionActivityEvent(t *testing.T) {
	node := NewValidateSlotsNode(seqIDs(3000))
	ctx := newTestAgentContext(t, map[string]any{
		"current_slots": domain.Slots{},
		"now":           time.Now().UTC(),
	})

	var payloads []ActivityEventPayload
	for ev, err := range node.Run(ctx, domain.SlotPatch{}) {
		require.NoError(t, err)
		if ev == nil {
			continue
		}
		for k, v := range ev.Actions.StateDelta {
			require.NoError(t, ctx.State().Set(k, v))
		}
		if ev.Content == nil {
			continue
		}
		for _, p := range ev.Content.Parts {
			if p.FunctionCall != nil && p.FunctionCall.Name == emitActivityEventFuncName {
				payload, ok := p.FunctionCall.Args["payload"].(ActivityEventPayload)
				require.True(t, ok)
				payloads = append(payloads, payload)
			}
		}
	}

	require.Len(t, payloads, 2)
	entry, completion := payloads[0], payloads[1]
	require.Equal(t, string(domain.StepValidateSlots), entry.Code)
	require.Empty(t, entry.Text)
	require.Equal(t, string(domain.StepValidateSlots), completion.Code)
	require.Equal(t, domain.StepLabels[domain.StepValidateSlots], completion.Text,
		"node 不再打 narrator，completion 必須是固定短句")
	require.Equal(t, entry.RowID, completion.RowID)
	require.Equal(t, entry.RowID, completion.RetryGroupKey)
	require.Equal(t, 1, completion.Attempt)
	require.Equal(t, 1, completion.MaxAttempts)

	// T23：兩筆事件各自講清楚自己是哪一態，不再靠「收到了幾筆」推論；label
	// 恆為 server 端固定字串，讓 entry（Text 恆空）那一刻也有東西可顯示。
	require.Equal(t, codemeta.ProcurementStepStatusRunning.Value, entry.Status)
	require.Equal(t, codemeta.ProcurementStepStatusSucceeded.Value, completion.Status)
	require.Equal(t, domain.StepLabels[domain.StepValidateSlots], entry.Label)
	require.Equal(t, domain.StepLabels[domain.StepValidateSlots], completion.Label)
	// graph 這三個節點都是 top-level、沒有計數概念。
	require.Zero(t, entry.ParentRowID)
	require.Nil(t, completion.Count)

	progress, err := readGraphTurnProgress(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, progress.NarrationSummaries[domain.StepValidateSlots],
		"摘要必須留給 terminal，不在節點內生成敘述")
}

// TestNewValidateSlotsNode_NeverCallsNarrator 證明這個 node 已經不打 narrator：
// 不管輸入是什麼，completion 的 Text 恆為固定 Label，且 progress.Narration（節點
// 內敘述才會寫入的欄位）維持空——node 不再持有任何 StepNarrator 依賴，簽名上
// 已經拿不到一個可以呼叫 Narrate 的東西（見 NewValidateSlotsNode 簽名）。
func TestNewValidateSlotsNode_NeverCallsNarrator(t *testing.T) {
	node := NewValidateSlotsNode(seqIDs(3100))
	ctx := newTestAgentContext(t, map[string]any{
		"current_slots": domain.Slots{},
		"now":           time.Now().UTC(),
	})

	var completionText string
	for ev, err := range node.Run(ctx, domain.SlotPatch{Item: item("蘋果")}) {
		require.NoError(t, err)
		if ev == nil {
			continue
		}
		for k, v := range ev.Actions.StateDelta {
			require.NoError(t, ctx.State().Set(k, v))
		}
		if ev.Content == nil {
			continue
		}
		for _, p := range ev.Content.Parts {
			if p.FunctionCall != nil && p.FunctionCall.Name == emitActivityEventFuncName {
				payload := p.FunctionCall.Args["payload"].(ActivityEventPayload)
				if payload.Text != "" {
					completionText = payload.Text
				}
			}
		}
	}

	require.Equal(t, domain.StepLabels[domain.StepValidateSlots], completionText)

	progress, err := readGraphTurnProgress(ctx)
	require.NoError(t, err)
	require.Empty(t, progress.Narration[domain.StepValidateSlots],
		"node 不得自己填 Narration——那是 narrateStepOutcome 的寫入點，只有 turn 尾會呼叫")
	require.NotEmpty(t, progress.NarrationSummaries[domain.StepValidateSlots],
		"摘要仍要算出來，只是留給 terminal 生成敘述")
}

func TestNewCropGateNode_EmitsEntryThenCompletionActivityEvent(t *testing.T) {
	resolver := &testkit.FakeResolver[cropidentity.DisambiguationResult]{}
	node := NewCropGateNode(resolver, seqIDs(4000))
	ctx := newTestAgentContext(t, map[string]any{
		"request_id":    int64(1),
		"current_slots": domain.Slots{},
	})

	var payloads []ActivityEventPayload
	for ev, err := range node.Run(ctx, ValidateSlotsOutput{Slots: domain.Slots{Item: item("蘋果")}}) {
		require.NoError(t, err)
		if ev == nil {
			continue
		}
		for k, v := range ev.Actions.StateDelta {
			require.NoError(t, ctx.State().Set(k, v))
		}
		if ev.Content == nil {
			continue
		}
		for _, p := range ev.Content.Parts {
			if p.FunctionCall != nil && p.FunctionCall.Name == emitActivityEventFuncName {
				payload, ok := p.FunctionCall.Args["payload"].(ActivityEventPayload)
				require.True(t, ok)
				payloads = append(payloads, payload)
			}
		}
	}

	require.Len(t, payloads, 2)
	entry, completion := payloads[0], payloads[1]
	require.Equal(t, string(domain.StepCropGate), entry.Code)
	require.Empty(t, entry.Text)
	require.Equal(t, string(domain.StepCropGate), completion.Code)
	require.Equal(t, domain.StepLabels[domain.StepCropGate], completion.Text,
		"node 不再打 narrator，completion 必須是固定短句")
	require.Equal(t, entry.RowID, completion.RowID)
	require.Equal(t, entry.RowID, completion.RetryGroupKey)
	require.Equal(t, 1, completion.Attempt)
	require.Equal(t, 1, completion.MaxAttempts)

	progress, err := readGraphTurnProgress(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, progress.NarrationSummaries[domain.StepCropGate],
		"摘要必須留給 terminal，不在節點內生成敘述")
}

// TestNewCropGateNode_NeverCallsNarrator 證明這個 node 已經不打 narrator：
// 不管輸入是什麼，completion 的 Text 恆為固定 Label，且 progress.Narration（節點
// 內敘述才會寫入的欄位）維持空——node 不再持有任何 StepNarrator 依賴，簽名上
// 已經拿不到一個可以呼叫 Narrate 的東西（見 NewCropGateNode 簽名）。
func TestNewCropGateNode_NeverCallsNarrator(t *testing.T) {
	resolver := &testkit.FakeResolver[cropidentity.DisambiguationResult]{}
	node := NewCropGateNode(resolver, seqIDs(4200))
	ctx := newTestAgentContext(t, map[string]any{
		"request_id":    int64(1),
		"current_slots": domain.Slots{},
	})

	var completionText string
	for ev, err := range node.Run(ctx, ValidateSlotsOutput{Slots: domain.Slots{Item: item("蘋果")}}) {
		require.NoError(t, err)
		if ev == nil {
			continue
		}
		for k, v := range ev.Actions.StateDelta {
			require.NoError(t, ctx.State().Set(k, v))
		}
		if ev.Content == nil {
			continue
		}
		for _, p := range ev.Content.Parts {
			if p.FunctionCall != nil && p.FunctionCall.Name == emitActivityEventFuncName {
				payload := p.FunctionCall.Args["payload"].(ActivityEventPayload)
				if payload.Text != "" {
					completionText = payload.Text
				}
			}
		}
	}

	require.Equal(t, domain.StepLabels[domain.StepCropGate], completionText)

	progress, err := readGraphTurnProgress(ctx)
	require.NoError(t, err)
	require.Empty(t, progress.Narration[domain.StepCropGate],
		"node 不得自己填 Narration——那是 narrateStepOutcome 的寫入點，只有 turn 尾會呼叫")
	require.NotEmpty(t, progress.NarrationSummaries[domain.StepCropGate],
		"摘要仍要算出來，只是留給 terminal 生成敘述")
}

// TestNewCropGateNode_EmitFailure_IsNonBlocking pins the fix for the
// review finding on commit 541937502: emitActivityEvent's error must not
// propagate as this node's own error. NewCropGateNode's NodeConfig carries
// RetryConfig, and emit is the LAST fallible thing in the function body —
// a hard error there would make the ADK scheduler retry the entire node
// body (duplicate persistCropGate call, plus a freshly-minted rowID／
// RetryGroupKey each retry while Attempt／MaxAttempts stay hardcoded 1/1),
// breaking the RetryGroupKey grouping doc57 §4.4 promises. See
// graph_nodes.go's NewCropGateNode doc comment for the full reasoning.
//
// Forces emit() to fail by running the node with an already-canceled
// context: ADK's own workflow.makeEmit (function_node.go's runEmitting)
// checks parentCtx.Err() before ever calling the consumer's yield, so a
// canceled ctx deterministically fails emit() without needing to drive the
// real ADK scheduler's retry loop (this package's node.Run-based tests
// never exercise that scheduler — see runFunctionNode's doc comment).
// stateValue's session.State.Get and testkit.FakeResolver's Resolve/Persist
// don't take a context argument at all, so canceling ctx up front doesn't
// perturb anything except the emit call this test targets.
func TestNewCropGateNode_EmitFailure_IsNonBlocking(t *testing.T) {
	resolver := &testkit.FakeResolver[cropidentity.DisambiguationResult]{}
	node := NewCropGateNode(resolver, seqIDs(4100))

	svc := session.InMemoryService()
	resp, err := svc.Create(t.Context(), &session.CreateRequest{
		AppName:   "procurement_graph_nodes_test",
		UserID:    "test-user",
		SessionID: "test-session",
		State: map[string]any{
			"request_id":    int64(1),
			"current_slots": domain.Slots{},
		},
	})
	require.NoError(t, err)

	canceledCtx, cancel := context.WithCancel(t.Context())
	cancel()
	ctx := agent.NewContext(&testInvocationContext{Context: canceledCtx, sess: resp.Session})

	var gotOutput CropGateOutput
	var gotOutputSeen bool
	var sawActivityEvent bool
	for ev, err := range node.Run(ctx, ValidateSlotsOutput{Slots: domain.Slots{Item: item("蘋果")}}) {
		require.NoError(t, err, "emit failure must not surface as a node error")
		if ev == nil {
			continue
		}
		if ev.Output != nil {
			out, ok := ev.Output.(CropGateOutput)
			require.True(t, ok)
			gotOutput = out
			gotOutputSeen = true
		}
		if ev.Content == nil {
			continue
		}
		for _, p := range ev.Content.Parts {
			if p.FunctionCall != nil && p.FunctionCall.Name == emitActivityEventFuncName {
				sawActivityEvent = true
			}
		}
	}

	require.True(t, gotOutputSeen, "crop_gate must still yield its terminal output despite the emit failure")
	require.NotNil(t, gotOutput.Item)
	require.Equal(t, "蘋果", *gotOutput.Item)
	require.False(t, sawActivityEvent, "the canceled-context emit must never reach the consumer as a yielded event")
}

// TestNewExtractSlotsNode_EmitFailure_IsNonBlocking pins this review-fix
// wave's unification: extract_slots now handles an emitActivityEvent
// failure the same way crop_gate already does (log-and-continue, see
// TestNewCropGateNode_EmitFailure_IsNonBlocking above and graph_nodes.go's
// NewExtractSlotsNode doc comment) instead of the previous hard error. Same
// canceled-context trick as the crop_gate test to force emit() to fail
// deterministically without driving the real ADK scheduler.
func TestNewExtractSlotsNode_EmitFailure_IsNonBlocking(t *testing.T) {
	extractor := fakeExtractor{patch: domain.SlotPatch{Item: strPtr("蘋果")}}
	node := NewExtractSlotsNode(extractor, seqIDs(5100))

	svc := session.InMemoryService()
	resp, err := svc.Create(t.Context(), &session.CreateRequest{
		AppName:   "procurement_graph_nodes_test",
		UserID:    "test-user",
		SessionID: "test-session",
		State: map[string]any{
			"current_slots": domain.Slots{},
		},
	})
	require.NoError(t, err)

	canceledCtx, cancel := context.WithCancel(t.Context())
	cancel()
	ctx := agent.NewContext(&testInvocationContext{Context: canceledCtx, sess: resp.Session})

	var gotOutput domain.SlotPatch
	var gotOutputSeen bool
	var sawActivityEvent bool
	for ev, err := range node.Run(ctx, nil) {
		require.NoError(t, err, "emit failure must not surface as a node error")
		if ev == nil {
			continue
		}
		if ev.Output != nil {
			out, ok := ev.Output.(domain.SlotPatch)
			require.True(t, ok)
			gotOutput = out
			gotOutputSeen = true
		}
		if ev.Content == nil {
			continue
		}
		for _, p := range ev.Content.Parts {
			if p.FunctionCall != nil && p.FunctionCall.Name == emitActivityEventFuncName {
				sawActivityEvent = true
			}
		}
	}

	require.True(t, gotOutputSeen, "extract_slots must still yield its terminal output despite the emit failure")
	require.NotNil(t, gotOutput.Item)
	require.Equal(t, "蘋果", *gotOutput.Item)
	require.False(t, sawActivityEvent, "the canceled-context emit must never reach the consumer as a yielded event")
}

// TestNewValidateSlotsNode_EmitFailure_IsNonBlocking is validate_slots's
// counterpart to TestNewExtractSlotsNode_EmitFailure_IsNonBlocking above —
// same review-fix unification, same canceled-context trick.
func TestNewValidateSlotsNode_EmitFailure_IsNonBlocking(t *testing.T) {
	node := NewValidateSlotsNode(seqIDs(5200))

	svc := session.InMemoryService()
	resp, err := svc.Create(t.Context(), &session.CreateRequest{
		AppName:   "procurement_graph_nodes_test",
		UserID:    "test-user",
		SessionID: "test-session",
		State: map[string]any{
			"current_slots": domain.Slots{},
			"now":           time.Now().UTC(),
		},
	})
	require.NoError(t, err)

	canceledCtx, cancel := context.WithCancel(t.Context())
	cancel()
	ctx := agent.NewContext(&testInvocationContext{Context: canceledCtx, sess: resp.Session})

	var gotOutput ValidateSlotsOutput
	var gotOutputSeen bool
	var sawActivityEvent bool
	for ev, err := range node.Run(ctx, domain.SlotPatch{Item: item("蘋果")}) {
		require.NoError(t, err, "emit failure must not surface as a node error")
		if ev == nil {
			continue
		}
		if ev.Output != nil {
			out, ok := ev.Output.(ValidateSlotsOutput)
			require.True(t, ok)
			gotOutput = out
			gotOutputSeen = true
		}
		if ev.Content == nil {
			continue
		}
		for _, p := range ev.Content.Parts {
			if p.FunctionCall != nil && p.FunctionCall.Name == emitActivityEventFuncName {
				sawActivityEvent = true
			}
		}
	}

	require.True(t, gotOutputSeen, "validate_slots must still yield its terminal output despite the emit failure")
	require.NotNil(t, gotOutput.Slots.Item)
	require.Equal(t, "蘋果", *gotOutput.Slots.Item)
	require.False(t, sawActivityEvent, "the canceled-context emit must never reach the consumer as a yielded event")
}

// TestCropGateNode_NoNewDecision_ClearsStaleSummary 釘住摘要的生命週期：
// graph_turn_progress 活在以 requestID 為 key 的 session 裡、跨輪次存活，所以
// gate 沒做新判定的那一輪必須清掉上一輪的摘要，否則 terminal 會為一個
// ComputeTurnSteps 不產生的格子生成敘述。
func TestCropGateNode_NoNewDecision_ClearsStaleSummary(t *testing.T) {
	cropGateNode := NewCropGateNode(&testkit.FakeResolver[cropidentity.DisambiguationResult]{}, seqIDs(7300))
	ctx := newTestAgentContext(t, map[string]any{
		// 已解析＝這一輪 gate 真的沒有新判定；少了 resolution 代表上一次失敗過，
		// 那一輪 gate 會重試，就不再是這條測試要驗的情境。
		"current_slots": domain.Slots{Item: item("蘋果"),
			ItemResolution: domain.NewSystemItemResolution(
				nil, strPtr("N04-001"), nil, nil, "蘋果", domain.CategoryFromEntity("category-apple"))},
		"now":        time.Now().UTC(),
		"request_id": int64(1),
		"graph_turn_progress": graphTurnProgress{
			NextSlots: domain.Slots{Item: item("蘋果"),
				ItemResolution: domain.NewSystemItemResolution(
					nil, strPtr("N04-001"), nil, nil, "蘋果", domain.CategoryFromEntity("category-apple"))},
			CropGateEvaluated:  true,
			NarrationSummaries: map[domain.StepCode]string{domain.StepCropGate: "上一輪留下的摘要"},
		},
	})

	for ev, err := range cropGateNode.Run(ctx, ValidateSlotsOutput{Slots: domain.Slots{Item: item("蘋果"),
		ItemResolution: domain.NewSystemItemResolution(
			nil, strPtr("N04-001"), nil, nil, "蘋果", domain.CategoryFromEntity("category-apple"))}}) {
		require.NoError(t, err)
		if ev == nil {
			continue
		}
		for k, v := range ev.Actions.StateDelta {
			require.NoError(t, ctx.State().Set(k, v))
		}
	}

	progress, err := readGraphTurnProgress(ctx)
	require.NoError(t, err)
	require.False(t, progress.CropGateEvaluated, "品項未變動這一輪 gate 不做新判定")
	require.Empty(t, progress.NarrationSummaries[domain.StepCropGate],
		"沒做新判定就必須清掉上一輪的摘要，兩者的生命週期綁在同一個 Evaluated()")
}

// TestValidateSlotsAndCropGateNodes_DeferNarrationToTerminal 證明兩個節點只留
// 摘要給 terminal，不在自己身上打 narrator：不管輸入是什麼，completion 事件的
// Text 恆是固定 StepLabels，一句話敘述改到 turn 尾的平行群組才生成（見
// graph_terminal.go composeGraphTurnResult 的 jobs 組裝段）。
func TestValidateSlotsAndCropGateNodes_DeferNarrationToTerminal(t *testing.T) {
	resolver := &testkit.FakeResolver[cropidentity.DisambiguationResult]{}
	validateNode := NewValidateSlotsNode(seqIDs(7100)) // 建構子無 narrator：節點拿不到它
	cropGateNode := NewCropGateNode(resolver, seqIDs(7200))
	ctx := newTestAgentContext(t, map[string]any{
		"current_slots": domain.Slots{},
		"now":           time.Now().UTC(),
		"request_id":    int64(1),
	})

	var payloads []ActivityEventPayload
	drain := func(node *workflow.FunctionNode, input any) {
		for ev, err := range node.Run(ctx, input) {
			require.NoError(t, err)
			if ev == nil {
				continue
			}
			for k, v := range ev.Actions.StateDelta {
				require.NoError(t, ctx.State().Set(k, v))
			}
			if ev.Content == nil {
				continue
			}
			for _, p := range ev.Content.Parts {
				if p.FunctionCall != nil && p.FunctionCall.Name == emitActivityEventFuncName {
					payload, ok := p.FunctionCall.Args["payload"].(ActivityEventPayload)
					require.True(t, ok)
					payloads = append(payloads, payload)
				}
			}
		}
	}

	drain(validateNode, domain.SlotPatch{Item: item("蘋果")})
	drain(cropGateNode, ValidateSlotsOutput{Slots: domain.Slots{Item: item("蘋果")}})

	require.Len(t, payloads, 4, "each node emits an entry + completion pair")
	require.Equal(t, domain.StepLabels[domain.StepValidateSlots], payloads[1].Text,
		"completion 必須是固定短句，narrator 的輸出不得出現在這裡")
	require.Equal(t, domain.StepLabels[domain.StepCropGate], payloads[3].Text,
		"completion 必須是固定短句，narrator 的輸出不得出現在這裡")

	progress, err := readGraphTurnProgress(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, progress.NarrationSummaries[domain.StepValidateSlots],
		"摘要必須留給 terminal")
	require.NotEmpty(t, progress.NarrationSummaries[domain.StepCropGate],
		"摘要必須留給 terminal")
	require.Empty(t, progress.Narration[domain.StepValidateSlots],
		"節點不得自己填 Narration")
	require.Empty(t, progress.Narration[domain.StepCropGate],
		"節點不得自己填 Narration")
}

// TestCropGateNode_PendingInterrupt_DoesNotReResolve 釘住 node 這一層的 guard：
// session state 還留著未解決的 workflow interrupt id 時，即使品項沒有解析結果也
// 不得重查——重查會產生一份新候選並蓋掉買家眼前那一份。
//
// turn／answer 的 GuardNoOpenInterrupt 擋掉大部分情況，但它自己的註解承認有一個
// 不持鎖的毫秒級 race。跨輪重試上線後，那條 race 放行的一輪就會重查，所以這道
// in-graph 判準是讓「那條 race 的壞結果集合是空的」繼續成立的東西。
func TestCropGateNode_PendingInterrupt_DoesNotReResolve(t *testing.T) {
	resolver := &countingCropResolver{inner: &testkit.FakeResolver[cropidentity.DisambiguationResult]{
		ResolveResult: cropidentity.DisambiguationResult{
			NeedsDisambiguation: true,
			Candidates:          []domain.CropDisambiguationOption{{Label: "另一份候選"}},
		},
	}}
	cropGateNode := NewCropGateNode(resolver, seqIDs(7400))
	ctx := newTestAgentContext(t, map[string]any{
		"current_slots":               domain.Slots{Item: item("蘋果")},
		"now":                         time.Now().UTC(),
		"request_id":                  int64(1),
		cropWorkflowInterruptStateKey: "int-pending-1",
		"graph_turn_progress":         graphTurnProgress{NextSlots: domain.Slots{Item: item("蘋果")}},
	})

	for ev, err := range cropGateNode.Run(ctx, ValidateSlotsOutput{Slots: domain.Slots{Item: item("蘋果")}}) {
		require.NoError(t, err)
		if ev == nil {
			continue
		}
		for k, v := range ev.Actions.StateDelta {
			require.NoError(t, ctx.State().Set(k, v))
		}
	}

	require.Zero(t, atomic.LoadInt32(&resolver.calls), "買家還在回答時不得重查品項")
}
