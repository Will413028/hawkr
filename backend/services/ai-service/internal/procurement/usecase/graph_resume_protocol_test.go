// services/ai-service/internal/procurement/usecase/graph_resume_protocol_test.go
//
// This is a permanent regression guard, not throwaway code — same
// category as internal/procurement/graphspike/resume_spike_test.go, but
// driving this package's real extract_slots/validate_slots/crop_gate/
// crop_disambiguation_interrupt node constructors instead of a synthetic
// stand-in graph, so a future ADK upgrade or refactor that breaks the
// FunctionResponse.Name resume requirement fails here, not just in
// production. Mirrors graph_dispatch.go's already-merged resumeGraphTurn
// FunctionResponse construction exactly (Task 6, T12 sub-project 3c).
package usecase

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/genai"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/workflowagent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/testkit"
)

// resumeProtocolCountingExtractor/countingCropResolver wrap this package's
// existing fakeExtractor/testkit.FakeResolver test doubles purely to count real
// invocations — proving whether extract_slots/crop_gate actually re-ran
// across a pause+resume pair is the entire point of this test.
//
// Named with a resumeProtocol prefix (unlike countingCropResolver below,
// which has no such collision) because turn_claim_dispatch_test.go used to
// declare an unrelated countingExtractor in this package with a different
// shape (patch/err/mu fields, a callCount() method) for a different
// purpose (dedupe-on-resend assertions) — this type must not collide with
// it. That file (and its countingExtractor, which had no other caller) was
// deleted by Task 6a of the procurement-graph-cutover plan, but the
// resumeProtocol prefix here is kept rather than renamed back, since a
// name-collision risk with a similarly-purposed counting extractor in this
// package is not specific to that one deleted type.
type resumeProtocolCountingExtractor struct {
	inner SlotExtractor
	calls int32
}

func (c *resumeProtocolCountingExtractor) Extract(ctx context.Context, message string, current domain.Slots) (ExtractResult, error) {
	atomic.AddInt32(&c.calls, 1)
	return c.inner.Extract(ctx, message, current)
}

type countingCropResolver struct {
	inner cropidentity.Resolver
	calls int32
}

func (c *countingCropResolver) Resolve(ctx context.Context, itemText string) (cropidentity.DisambiguationResult, error) {
	atomic.AddInt32(&c.calls, 1)
	return c.inner.Resolve(ctx, itemText)
}

// buildResumeProtocolTestChain assembles the smallest real subgraph that
// reproduces the exact pause point graph_dispatch.go's resumeGraphTurn
// resumes in production: the real extract_slots/validate_slots/crop_gate/
// crop_disambiguation_interrupt node constructors, terminated by a local
// pass-through node standing in for NewProcurementGraph's terminal
// (out of scope here — this test is only about the pause/resume
// boundary, not the full graph).
func buildResumeProtocolTestChain(extractor SlotExtractor, resolver cropidentity.Resolver) agent.Agent {
	extractNode := NewExtractSlotsNode(extractor, seqIDs(9000))
	validateNode := NewValidateSlotsNode(seqIDs(9100))
	cropGateNode := NewCropGateNode(resolver, seqIDs(9200))
	interruptNode := NewCropDisambiguationInterruptNode()
	// 跟 NewProcurementGraph 一樣掛在 interrupt 正下游——這條鏈就是為了忠實
	// 重現 production 的 pause/resume 邊界，少一個節點就不是同一條路徑了。
	reconcileNode := NewCropResumeReconcileNode()
	terminal := workflow.NewFunctionNode("resume_protocol_test_terminal",
		func(_ agent.Context, in CropGateOutput) (CropGateOutput, error) { return in, nil },
		workflow.NodeConfig{})

	wfAgent, err := workflowagent.New(workflowagent.Config{
		Name:  "resume_protocol_test",
		Edges: workflow.Chain(workflow.Start, extractNode, validateNode, cropGateNode, interruptNode, reconcileNode, terminal),
	})
	if err != nil {
		panic(err) // construction-time only — a bad chain here is a test-authoring bug
	}
	return wfAgent
}

func resumeProtocolSeedState() map[string]any {
	return map[string]any{
		"current_slots": domain.Slots{},
		"now":           time.Now().UTC(),
		"request_id":    int64(1),
	}
}

func TestGraphResumeProtocol_MissingFunctionResponseName_TreatedAsFreshRestart(t *testing.T) {
	ctx := t.Context()
	extractor := &resumeProtocolCountingExtractor{inner: fakeExtractor{patch: domain.SlotPatch{Item: item("小白菜")}}}
	resolver := &countingCropResolver{inner: &testkit.FakeResolver[cropidentity.DisambiguationResult]{ResolveResult: cropidentity.DisambiguationResult{
		NeedsDisambiguation: true,
		Candidates:          []domain.CropDisambiguationOption{{Label: "有機小白菜"}},
	}}}

	svc := session.InMemoryService()
	r, err := runner.New(runner.Config{
		AppName: "resume_protocol_test", Agent: buildResumeProtocolTestChain(extractor, resolver),
		SessionService: svc, AutoCreateSession: true,
	})
	require.NoError(t, err)

	const userID, sessionID = "resume-test-user", "resume-test-session"
	drainEvents(t, r.Run(ctx, userID, sessionID,
		&genai.Content{Role: genai.RoleUser, Parts: []*genai.Part{{Text: "我要小白菜"}}},
		agent.RunConfig{}, runner.WithStateDelta(resumeProtocolSeedState())))

	require.EqualValues(t, 1, atomic.LoadInt32(&extractor.calls), "turn 1 must pause at crop_disambiguation_interrupt")
	require.EqualValues(t, 1, atomic.LoadInt32(&resolver.calls))

	// Deliberately malformed resume: ID matches, Name is omitted — the
	// exact bug graph_dispatch.go's resumeGraphTurn fix prevents (see
	// adk-v2-resume-requires-functionresponse-name in the
	// private archive). detectResume's filter
	// (agent/workflowagent/workflow.go: `if fr.Name !=
	// workflow.WorkflowInputFunctionCallName { continue }`) drops this
	// response entirely, so the engine treats this call as a brand new
	// invocation instead of a resume.
	drainEvents(t, r.Run(ctx, userID, sessionID,
		&genai.Content{Role: genai.RoleUser, Parts: []*genai.Part{{
			FunctionResponse: &genai.FunctionResponse{
				ID:       cropDisambiguationInterruptKind,
				Response: map[string]any{"payload": CropGateOutput{Item: item("小白菜"), Pending: false}},
			},
		}}},
		agent.RunConfig{}))

	require.EqualValues(t, 2, atomic.LoadInt32(&extractor.calls),
		"missing FunctionResponse.Name must be treated as a fresh restart: extract_slots re-ran")
	require.EqualValues(t, 2, atomic.LoadInt32(&resolver.calls),
		"missing FunctionResponse.Name must be treated as a fresh restart: crop_gate's resolver re-ran")
}

func TestGraphResumeProtocol_CorrectFunctionResponseName_DoesNotRerunUpstreamNodes(t *testing.T) {
	ctx := t.Context()
	extractor := &resumeProtocolCountingExtractor{inner: fakeExtractor{patch: domain.SlotPatch{Item: item("小白菜")}}}
	resolver := &countingCropResolver{inner: &testkit.FakeResolver[cropidentity.DisambiguationResult]{ResolveResult: cropidentity.DisambiguationResult{
		NeedsDisambiguation: true,
		Candidates:          []domain.CropDisambiguationOption{{Label: "有機小白菜"}},
	}}}

	svc := session.InMemoryService()
	r, err := runner.New(runner.Config{
		AppName: "resume_protocol_test", Agent: buildResumeProtocolTestChain(extractor, resolver),
		SessionService: svc, AutoCreateSession: true,
	})
	require.NoError(t, err)

	const userID, sessionID = "resume-test-user", "resume-test-session"
	drainEvents(t, r.Run(ctx, userID, sessionID,
		&genai.Content{Role: genai.RoleUser, Parts: []*genai.Part{{Text: "我要小白菜"}}},
		agent.RunConfig{}, runner.WithStateDelta(resumeProtocolSeedState())))

	require.EqualValues(t, 1, atomic.LoadInt32(&extractor.calls))
	require.EqualValues(t, 1, atomic.LoadInt32(&resolver.calls))

	// Correctly formed resume — mirrors graph_dispatch.go's resumeGraphTurn
	// exactly (ID: cropDisambiguationInterruptKind, Name:
	// workflow.WorkflowInputFunctionCallName, Response["payload"] a
	// resolved CropGateOutput).
	events := drainEvents(t, r.Run(ctx, userID, sessionID,
		&genai.Content{Role: genai.RoleUser, Parts: []*genai.Part{{
			FunctionResponse: &genai.FunctionResponse{
				ID: cropDisambiguationInterruptKind, Name: workflow.WorkflowInputFunctionCallName,
				Response: map[string]any{"payload": CropGateOutput{Item: item("小白菜"), Pending: false}},
			},
		}}},
		agent.RunConfig{}))

	require.EqualValues(t, 1, atomic.LoadInt32(&extractor.calls),
		"correct FunctionResponse.Name must resume, not restart: extract_slots must not re-run")
	require.EqualValues(t, 1, atomic.LoadInt32(&resolver.calls),
		"correct FunctionResponse.Name must resume, not restart: crop_gate's resolver must not re-run")

	var gotOutput CropGateOutput
	var sawOutput bool
	for _, ev := range events {
		if out, ok := ev.Output.(CropGateOutput); ok {
			gotOutput = out
			sawOutput = true
		}
	}
	require.True(t, sawOutput, "resumed run must reach the terminal node with the resumed CropGateOutput")
	require.False(t, gotOutput.Pending, "resumed value must be the resolved (non-pending) CropGateOutput passed in the FunctionResponse")
}

// lastEmittedProgress returns the graph_turn_progress carried by the last
// event whose StateDelta contains it — emitGraphTurnProgress is the only
// writer of that key, so this is exactly "what the terminal node would read
// next".
func lastEmittedProgress(t *testing.T, events []*session.Event) (graphTurnProgress, bool) {
	t.Helper()
	var out graphTurnProgress
	var seen bool
	for _, ev := range events {
		raw, ok := ev.Actions.StateDelta["graph_turn_progress"]
		if !ok {
			continue
		}
		p, decoded := decodeGraphTurnProgress(raw)
		if !decoded {
			t.Fatalf("graph_turn_progress present but undecodable: %#v", raw)
		}
		out, seen = p, true
	}
	return out, seen
}

// TestGraphResumeProtocol_ResumeClearsCropPendingProgress guards the exact
// production defect: crop_gate is the only writer of
// progress.CropPending/CropCandidates, and it does not re-run across a
// pause+resume (the test right above proves that). Without
// crop_resume_reconcile the parked turn's CropPending=true survives into the
// resumed activation, and graph_terminal.go's unconditional
// `if progress.CropPending { question = BuildCropDisambiguationQuestion(...) }`
// re-asks the very question the buyer just answered.
func TestGraphResumeProtocol_ResumeClearsCropPendingProgress(t *testing.T) {
	ctx := t.Context()
	extractor := &resumeProtocolCountingExtractor{inner: fakeExtractor{patch: domain.SlotPatch{Item: item("小白菜")}}}
	resolver := &countingCropResolver{inner: &testkit.FakeResolver[cropidentity.DisambiguationResult]{ResolveResult: cropidentity.DisambiguationResult{
		NeedsDisambiguation: true,
		Candidates:          []domain.CropDisambiguationOption{{Label: "有機小白菜"}},
	}}}

	svc := session.InMemoryService()
	r, err := runner.New(runner.Config{
		AppName: "resume_protocol_test", Agent: buildResumeProtocolTestChain(extractor, resolver),
		SessionService: svc, AutoCreateSession: true,
	})
	require.NoError(t, err)

	const userID, sessionID = "reconcile-user", "reconcile-session"
	parked := drainEvents(t, r.Run(ctx, userID, sessionID,
		&genai.Content{Role: genai.RoleUser, Parts: []*genai.Part{{Text: "我要小白菜"}}},
		agent.RunConfig{}, runner.WithStateDelta(resumeProtocolSeedState())))

	before, seen := lastEmittedProgress(t, parked)
	require.True(t, seen, "crop_gate must have emitted graph_turn_progress on the parked turn")
	require.True(t, before.CropPending, "fixture must actually park on crop disambiguation")
	require.NotEmpty(t, before.CropCandidates)

	chosen := &domain.ItemResolution{
		Kind: domain.ItemResolvedCrop, Code: testkit.Ptr("BC-ORG"), Label: "有機小白菜",
		Provenance: domain.ItemResolutionByBuyerChoice,
	}
	resumed := drainEvents(t, r.Run(ctx, userID, sessionID,
		&genai.Content{Role: genai.RoleUser, Parts: []*genai.Part{{
			FunctionResponse: &genai.FunctionResponse{
				ID: cropDisambiguationInterruptKind, Name: workflow.WorkflowInputFunctionCallName,
				Response: map[string]any{"payload": CropGateOutput{
					Item: item("有機小白菜"), Resolution: chosen, Pending: false,
				}},
			},
		}}},
		agent.RunConfig{}))

	after, seen := lastEmittedProgress(t, resumed)
	require.True(t, seen, "the resumed activation must re-emit graph_turn_progress")
	require.False(t, after.CropPending,
		"resumed activation must clear CropPending, else the terminal re-asks the answered question")
	require.Empty(t, after.CropCandidates)
	require.NotNil(t, after.NextSlots.Item)
	require.Equal(t, "有機小白菜", *after.NextSlots.Item,
		"buyer's disambiguation answer must land on the turn's next slots")
	// 買家挑中的解析結果要同時落在 slots（跟著 revision 走）與 patch（terminal 的
	// RecordedThisTurn 只看 patch，少了它①確認段就不會出現「品項分類」那一格，
	// 買家挑完候選畫面依舊零回饋——這條線一開始要修的就是這件事）。
	require.Equal(t, chosen, after.NextSlots.ItemResolution)
	require.Equal(t, chosen, after.Patch.ItemResolution)
	// 上一輪累積的其他欄位不能被這個節點洗掉——它只負責消歧那幾個值。
	wantPatch := before.Patch
	wantPatch.ItemResolution = chosen
	require.Equal(t, wantPatch, after.Patch)
	require.Equal(t, before.Degraded, after.Degraded)
}

// TestCropResumeReconcile_ResolutionSurvivesJSONRoundTrip 針對 production 與這個
// 檔案其餘測試的一個真實差異：這裡用的是 session.InMemoryService()，值有可能原封
// 不動地傳進節點；production 的 ADK session 是 Postgres-backed，resume payload 會
// 經過一次 JSON round-trip（graph_interrupt.go 的 doc comment 已經記過這個差異，
// 而且記的正是「型別斷言在 round-trip 之後會失敗」）。
//
// 所以「在記憶體 session 下解析結果傳得到」證明不了 production 也傳得到。這支測試
// 把 payload 真的序列化成 JSON 再反序列化成 map[string]any（＝Postgres session 還原
// 出來的形狀）餵給節點，斷言 ItemResolution 仍然完整——包含它自己的 json tag
// （kind/code/label/provenance）有沒有對上。
func TestCropResumeReconcile_ResolutionSurvivesJSONRoundTrip(t *testing.T) {
	chosen := &domain.ItemResolution{
		Kind: domain.ItemResolvedCrop, Code: testkit.Ptr("LA1"), Label: "甘藍-初秋",
		FallbackGroupCode: testkit.Ptr("LA"), Provenance: domain.ItemResolutionByBuyerChoice,
	}
	raw, err := json.Marshal(CropGateOutput{Item: item("高麗菜"), Resolution: chosen, Pending: false})
	require.NoError(t, err)
	var roundTripped map[string]any
	require.NoError(t, json.Unmarshal(raw, &roundTripped))

	node := NewCropResumeReconcileNode()
	ctx := newTestAgentContext(t, map[string]any{
		"graph_turn_progress": graphTurnProgress{
			CropPending:    true,
			CropCandidates: []domain.CropDisambiguationOption{{Label: "甘藍-初秋"}},
			NextSlots:      domain.Slots{Item: item("高麗菜")},
		},
	})

	_, err = runFunctionNode[CropGateOutput](t, node, ctx, roundTripped)
	require.NoError(t, err)

	progress, err := readGraphTurnProgress(ctx)
	require.NoError(t, err)
	require.Equal(t, chosen, progress.NextSlots.ItemResolution,
		"JSON round-trip 之後解析結果必須完整——production 的 session 就是這個形狀")
	require.Equal(t, chosen, progress.Patch.ItemResolution)
	require.False(t, progress.CropPending)
}

// TestCropResumeReconcile_ClearsParkedTurnGateNarration 釘住一個跨 interrupt
// 的狀態殘留：被暫停那一輪的 progress（ADK session state 跨 interrupt 持久化）
// 帶著 CropGateEvaluated=true 與 Narration[CROP_GATE]「品項尚待買家從 N 個候選
// 作物中選擇」。買家 resume 時剛選完，那句話已經不成立——但 terminal 只看這兩個
// 欄位，會照樣在 resume 這一輪生出一格 CROP_GATE 並填上那句過期的話。
//
// reconcile 節點負責把 gate 的狀態一次收乾淨，不只是 CropPending/CropCandidates。
func TestCropResumeReconcile_ClearsParkedTurnGateNarration(t *testing.T) {
	node := NewCropResumeReconcileNode()
	chosen := "甘藍-初秋"

	ctx := newTestAgentContext(t, map[string]any{
		"graph_turn_progress": graphTurnProgress{
			CropPending:       true,
			CropCandidates:    []domain.CropDisambiguationOption{{Label: "甘藍"}, {Label: "球莖甘藍"}},
			CropGateEvaluated: true,
			Narration: domain.StepNarration{
				domain.StepCropGate:      "品項尚待買家從 2 個候選作物中選擇。",
				domain.StepValidateSlots: "高麗菜條件皆有效。",
			},
		},
	})

	_, err := runFunctionNode[CropGateOutput](t, node, ctx, CropGateOutput{Item: &chosen, Pending: false})
	require.NoError(t, err)

	progress, err := readGraphTurnProgress(ctx)
	require.NoError(t, err)
	require.False(t, progress.CropPending)
	require.Nil(t, progress.CropCandidates)
	require.False(t, progress.CropGateEvaluated,
		"留著的話 resume 這一輪會多出一格 CROP_GATE，內容是上一輪的狀態")
	require.NotContains(t, progress.Narration, domain.StepCropGate,
		"「尚待選擇」在買家剛選完的這一輪已經是假的")
	require.Contains(t, progress.Narration, domain.StepValidateSlots,
		"只清 gate 自己那一碼，不得連坐清掉其他節點的敘述")
}

// TestGraphResumeProtocol_MissingCheckpoint_FailsInsteadOfFreshTurn 釘住 24 小時
// 清理之後的行為：checkpoint 真的不在了時，resume 必須回錯誤，**不得**靜默開一個
// 空 session 當成新的一輪跑。
//
// 靜默重跑的代價不是「慢一點」：上游 node 會重新抽取、重新解析、再開一個新的
// interrupt，而買家手上那個 interruptId 指向的掛起點已經不存在——他按下的答案
// 落在一個他從沒看過的問題上。AutoCreateSession=true 就是這條路徑。
//
// ⛔ 判準必須是 atomic 的：先做一次存在檢查、後面仍讓 runner AutoCreate，中間那個
// 窗口還是會開出空 session（TOCTOU）。這裡驗的是 runner 自己拒絕。
func TestGraphResumeProtocol_MissingCheckpoint_FailsInsteadOfFreshTurn(t *testing.T) {
	ctx := t.Context()
	extractor := &resumeProtocolCountingExtractor{inner: fakeExtractor{patch: domain.SlotPatch{Item: item("小白菜")}}}
	resolver := &countingCropResolver{inner: &testkit.FakeResolver[cropidentity.DisambiguationResult]{ResolveResult: cropidentity.DisambiguationResult{
		NeedsDisambiguation: true,
		Candidates:          []domain.CropDisambiguationOption{{Label: "有機小白菜"}},
	}}}

	svc := session.InMemoryService()
	chain := buildResumeProtocolTestChain(extractor, resolver)
	const appName, userID, sessionID = "resume_protocol_test", "resume-test-user", "resume-test-session"

	turnRunner, err := runner.New(runner.Config{
		AppName: appName, Agent: chain, SessionService: svc, AutoCreateSession: true,
	})
	require.NoError(t, err)
	// resume 專用：關掉 AutoCreateSession，缺 checkpoint 直接回錯。
	resumeRunner, err := runner.New(runner.Config{
		AppName: appName, Agent: chain, SessionService: svc, AutoCreateSession: false,
	})
	require.NoError(t, err)

	drainEvents(t, turnRunner.Run(ctx, userID, sessionID,
		&genai.Content{Role: genai.RoleUser, Parts: []*genai.Part{{Text: "我要小白菜"}}},
		agent.RunConfig{}, runner.WithStateDelta(resumeProtocolSeedState())))
	require.EqualValues(t, 1, atomic.LoadInt32(&extractor.calls))
	require.EqualValues(t, 1, atomic.LoadInt32(&resolver.calls))

	// 模擬 24 小時清理把 checkpoint 掃掉。
	require.NoError(t, svc.Delete(ctx, &session.DeleteRequest{
		AppName: appName, UserID: userID, SessionID: sessionID,
	}))

	var resumeErr error
	for _, err := range resumeRunner.Run(ctx, userID, sessionID,
		&genai.Content{Role: genai.RoleUser, Parts: []*genai.Part{{
			FunctionResponse: &genai.FunctionResponse{
				ID:       cropDisambiguationInterruptKind,
				Name:     workflow.WorkflowInputFunctionCallName,
				Response: map[string]any{"payload": CropGateOutput{Item: item("小白菜"), Pending: false}},
			},
		}}},
		agent.RunConfig{}) {
		if err != nil {
			resumeErr = err
		}
	}

	require.Error(t, resumeErr, "checkpoint 不在了必須回錯，不能當成新的一輪跑")
	require.EqualValues(t, 1, atomic.LoadInt32(&extractor.calls),
		"不得重新抽取：那會在買家看不到的地方產生新狀態")
	require.EqualValues(t, 1, atomic.LoadInt32(&resolver.calls),
		"不得重新解析品項，也就不會再開一個新的 interrupt")
}
