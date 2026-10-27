// services/ai-service/internal/procurement/usecase/graph_test.go
package usecase

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/genai"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/testkit"
)

// TestNewProcurementGraph_BuildsWithoutError proves the graph CONSTRUCTS
// cleanly through workflow.New's static validation checks
// (validateStaticSchemas / validateFanIn / validateCycles / validateDefaultRoute
// / validateUniqueEdges / validateConnectivity). It proves nothing about
// whether the graph EXECUTES successfully through a real scheduler — that is
// TestNewProcurementGraph_RealRunner_PausesAtCropDisambiguation below.
// graph_nodes_test.go's own tests call node.Run(ctx, input) directly, which
// bypasses the scheduler entirely and never triggers the scheduler's
// output-validation step — those tests cover each node's own logic but not
// real end-to-end execution either.
func TestNewProcurementGraph_BuildsWithoutError(t *testing.T) {
	extractor := fakeExtractor{patch: domain.SlotPatch{}}
	resolver := cropidentity.Resolver(nil) // 既有慣例：nil 只在單元測試發生，見 cropidentity/crop_gate.go 註解

	g, err := NewProcurementGraph(extractor, resolver, seqIDs(1000), nil, &fakeStepNarrator{text: "narrated"}, nil)
	if err != nil {
		t.Fatalf("NewProcurementGraph() error = %v", err)
	}
	if g == nil {
		t.Fatal("expected non-nil agent.Agent")
	}
}

// TestNewProcurementGraph_RealRunner_PausesAtCropDisambiguation drives the
// FULL, unmodified NewProcurementGraph (extract_slots → validate_slots →
// crop_gate → crop_disambiguation_interrupt) through a real ADK scheduler
// via runner.Run — not node.Run(ctx, input) directly, which bypasses the
// scheduler's output-validation step entirely.
//
// This is the permanent regression guard for the guard node scheduler
// execution bug fix (graph_nodes.go's stateValue / workflow.NewFunctionNode
// rewrite, replacing workflow.NewFunctionNodeFromState): before that fix,
// this test failed on every run at extract_slots — the very first node
// after workflow.Start — with "output validation failed for node
// \"extract_slots\": validating root: cannot validate against a struct"
// (jsonschema-go v0.4.3, https://github.com/google/jsonschema-go/issues/23).
//
// A NeedsDisambiguation resolver result makes crop_gate return Pending=true,
// so crop_disambiguation_interrupt pauses the run before turn_terminal is
// ever reached — no real or fake model call needed anywhere in this graph.
func TestNewProcurementGraph_RealRunner_PausesAtCropDisambiguation(t *testing.T) {
	ctx := t.Context()

	extractor := fakeExtractor{patch: domain.SlotPatch{Item: item("蘋果")}}
	resolver := &testkit.FakeResolver[cropidentity.DisambiguationResult]{ResolveResult: cropidentity.DisambiguationResult{
		NeedsDisambiguation: true,
		Candidates: []domain.CropDisambiguationOption{
			{Label: "富士蘋果"}, {Label: "五爪蘋果"},
		},
	}}

	g, err := NewProcurementGraph(extractor, resolver, seqIDs(1500), nil, &fakeStepNarrator{text: "narrated"}, nil)
	require.NoError(t, err)

	svc := session.InMemoryService()
	r, err := runner.New(runner.Config{
		AppName: "procurement_graph_real_runner_test", Agent: g, SessionService: svc, AutoCreateSession: true,
	})
	require.NoError(t, err)

	events := drainEvents(t, r.Run(ctx, "u", "s",
		&genai.Content{Role: genai.RoleUser, Parts: []*genai.Part{{Text: "我要買蘋果"}}},
		agent.RunConfig{},
		runner.WithStateDelta(map[string]any{
			"current_slots": domain.Slots{},
			"now":           time.Now().UTC(),
			"request_id":    int64(1),
		}),
	))

	var paused bool
	for _, ev := range events {
		if len(ev.LongRunningToolIDs) > 0 {
			paused = true
		}
	}
	require.True(t, paused, "expected the run to pause at crop_disambiguation_interrupt before turn_terminal — extract_slots/validate_slots/crop_gate must all have executed through the real scheduler without hitting output-validation errors")
}

// TestNewProcurementGraph_ExtractSlots_EmitsActivityEvent proves
// extract_slots (graph_nodes.go's NewExtractSlotsNode, rewritten in Task 3
// to workflow.NewEmittingFunctionNode) actually emits a correctly-shaped
// ActivityEventPayload event when driven through a real scheduler — not
// just that node.Run(ctx, input) returns the right terminal SlotPatch (that
// narrower claim is graph_nodes_test.go's
// TestExtractSlotsNode_DelegatesUnchanged). Modeled directly on
// TestNewProcurementGraph_RealRunner_PausesAtCropDisambiguation above,
// reusing its exact fixture shape: a resolver that needs disambiguation, so
// the run pauses at crop_disambiguation_interrupt before turn_terminal is
// ever reached — extract_slots still executes and emits its event
// regardless of what the graph does afterward.
func TestNewProcurementGraph_ExtractSlots_EmitsActivityEvent(t *testing.T) {
	ctx := t.Context()

	extractor := fakeExtractor{patch: domain.SlotPatch{Item: item("蘋果")}, attempt: 1, shortIntent: "正在讀你的需求，抓品項、數量和交期"}
	resolver := &testkit.FakeResolver[cropidentity.DisambiguationResult]{ResolveResult: cropidentity.DisambiguationResult{
		NeedsDisambiguation: true,
		Candidates: []domain.CropDisambiguationOption{
			{Label: "富士蘋果"}, {Label: "五爪蘋果"},
		},
	}}

	g, err := NewProcurementGraph(extractor, resolver, seqIDs(2000), nil, &fakeStepNarrator{text: "narrated"}, nil)
	require.NoError(t, err)

	svc := session.InMemoryService()
	r, err := runner.New(runner.Config{
		AppName: "procurement_graph_activity_event_test", Agent: g, SessionService: svc, AutoCreateSession: true,
	})
	require.NoError(t, err)

	events := drainEvents(t, r.Run(ctx, "u", "s",
		&genai.Content{Role: genai.RoleUser, Parts: []*genai.Part{{Text: "我要買蘋果"}}},
		agent.RunConfig{},
		runner.WithStateDelta(map[string]any{
			"current_slots": domain.Slots{},
			"now":           time.Now().UTC(),
			"request_id":    int64(1),
		}),
	))

	// The run continues past extract_slots into validate_slots and
	// crop_gate before pausing at crop_disambiguation_interrupt (this
	// resolver needs disambiguation) — each of those nodes also emits its
	// own Activity events via the same emitActivityEventFuncName tag, so
	// activityPayloads legitimately holds 6 entries (extract_slots,
	// validate_slots, and crop_gate each emit an entry + a completion).
	// Filter down to extract_slots's own Code before pinning the count.
	var activityPayloads []ActivityEventPayload
	var extractSlotsPayloads []ActivityEventPayload
	for _, ev := range events {
		if ev.Content == nil {
			continue
		}
		for _, p := range ev.Content.Parts {
			if p.FunctionCall != nil && p.FunctionCall.Name == emitActivityEventFuncName {
				payload, ok := p.FunctionCall.Args["payload"].(ActivityEventPayload)
				require.True(t, ok)
				activityPayloads = append(activityPayloads, payload)
				if payload.Code == string(domain.StepExtractSlots) {
					extractSlotsPayloads = append(extractSlotsPayloads, payload)
				}
			}
		}
	}

	require.NotEmpty(t, activityPayloads, "the graph must emit at least one Activity event")
	require.Len(t, extractSlotsPayloads, 2, "extract_slots must emit entry and completion Activity events")
	entry, completion := extractSlotsPayloads[0], extractSlotsPayloads[1]
	require.Equal(t, string(domain.StepExtractSlots), entry.Code)
	require.Empty(t, entry.Text, "entry must carry no text")
	require.Equal(t, 1, entry.Attempt)
	require.NotZero(t, entry.RowID)
	require.Equal(t, entry.RowID, entry.RetryGroupKey)
	require.Equal(t, string(domain.StepExtractSlots), completion.Code)
	require.Equal(t, entry.RowID, completion.RowID, "entry and completion must share the same RowID")
	require.Equal(t, completion.RowID, completion.RetryGroupKey)
	require.Equal(t, 1, completion.Attempt)
	require.Equal(t, "正在讀你的需求，抓品項、數量和交期", completion.Text,
		"completion Text must carry the model's ShortIntent through guardShortIntent, not a hardcoded StepLabels fallback")
}

// 圖內不再有任何 llmagent：SubAgents 為空是「模型無決策權」的結構性證明——
// runner 不會為 controller 解析 author，也沒有 callback 掛點可以掛 loop budget。
func TestNewProcurementGraph_HasNoLLMAgent(t *testing.T) {
	g, err := NewProcurementGraph(fakeExtractor{}, &testkit.FakeResolver[cropidentity.DisambiguationResult]{}, seqIDs(3000), nil, &fakeStepNarrator{text: "narrated"}, nil)
	require.NoError(t, err)
	require.Empty(t, g.SubAgents(), "procurement_graph must not carry a controller sub-agent")
}
