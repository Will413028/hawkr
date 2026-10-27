// services/ai-service/internal/procurement/usecase/graph_interrupt_test.go
package usecase

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/genai"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/workflowagent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
)

// TestCropDisambiguationInterruptNode_NotPending_PassesThrough asserts the
// non-pause branch: Pending=false travels through unchanged, byte-for-byte
// — not just the Pending flag — proving the node does not silently drop or
// mutate Item/Candidates on the pass-through path.
func TestCropDisambiguationInterruptNode_NotPending_PassesThrough(t *testing.T) {
	node := NewCropDisambiguationInterruptNode()
	ctx := newTestAgentContext(t, nil)

	in := CropGateOutput{
		Item:       item("蘋果"),
		Candidates: []domain.CropDisambiguationOption{{Label: "富士蘋果"}},
		Pending:    false,
	}

	got, err := runFunctionNode[CropGateOutput](t, node, ctx, in)
	require.NoError(t, err)
	require.Equal(t, in, got)
}

// TestCropDisambiguationInterruptNode_Pending_EmitsRequestInput drives the
// node directly (not through a runner) with a concrete Pending=true
// CropGateOutput — the rigorous test of the pause mechanism per the task
// brief's ruling: it exercises emit + workflow.ErrNodeInterrupted end to
// end and inspects the actual RequestInput event content.
//
// Per FunctionNode.runEmitting (function_node.go:356-382), when the node
// function returns ErrNodeInterrupted the engine swallows it — the pause
// event already went out through emit() — so no iteration of node.Run ever
// yields a non-nil error here; ErrNodeInterrupted never appears on the
// wire. This differs from the task brief's Step 1 sketch (which defensively
// checks `err != workflow.ErrNodeInterrupted` inside the loop); that branch
// is correct-but-unreachable for this node shape, so this test asserts the
// verified real behavior (err is always nil) instead of carrying dead
// defensive code forward.
func TestCropDisambiguationInterruptNode_Pending_EmitsRequestInput(t *testing.T) {
	node := NewCropDisambiguationInterruptNode()
	ctx := newTestAgentContext(t, nil)

	candidates := []domain.CropDisambiguationOption{
		{Label: "紅蘿蔔", CategoryID: "21", CategoryLevel: 3},
		{Label: "白蘿蔔", CategoryID: "22", CategoryLevel: 3},
	}

	var sawRequestInput bool
	var gotPayload CropDisambiguationInterruptPayload
	var eventCount int
	for ev, err := range node.Run(ctx, CropGateOutput{Pending: true, Candidates: candidates}) {
		require.NoError(t, err)
		require.NotNil(t, ev)
		eventCount++

		if len(ev.LongRunningToolIDs) > 0 {
			sawRequestInput = true
			require.NotNil(t, ev.RequestedInput, "RequestInput event must carry a RequestedInput payload")
			require.Equal(t, cropDisambiguationInterruptKind, ev.RequestedInput.InterruptID)
			payload, ok := ev.RequestedInput.Payload.(CropDisambiguationInterruptPayload)
			require.True(t, ok, "RequestedInput.Payload should be a CropDisambiguationInterruptPayload, got %T", ev.RequestedInput.Payload)
			gotPayload = payload
		}
	}

	require.True(t, sawRequestInput, "expected a RequestInput event (LongRunningToolIDs set) when Pending=true")
	require.Equal(t, 1, eventCount, "expected exactly one event: the RequestInput pause event, nothing after ErrNodeInterrupted")
	require.Equal(t, cropDisambiguationInterruptKind, gotPayload.Kind)
	require.Equal(t, candidates, gotPayload.Candidates)
}

// TestCropDisambiguationInterruptNode_PauseResume_ViaRunner is Step 5's
// runner-wiring smoke test — per the task brief's ruling, this is a lighter
// check that the runner.New / workflowagent.New / Runner.Run call pattern
// (the one Task 9 will actually use) compiles and runs, not a second proof
// of the pause mechanism itself (that's TestCropDisambiguationInterruptNode_
// Pending_EmitsRequestInput above, which drives node.Run directly).
//
// It deliberately does not chain workflow.Start straight to this node.
// workflow.Start's seed input is always the plain string built from the
// inbound user message's text parts (workflow.userInput, workflow.go:327-
// 339); this node's IN type is CropGateOutput, a struct. Feeding a bare
// string into a struct-typed FunctionNode forces ADK's any->IN JSON-schema
// conversion path (function_node.go's newEmittingFunctionNodeWithResolved
// Schemas), which json.Marshals the string to a JSON string literal and
// then tries to json.Unmarshal that literal into the struct — that always
// fails ("cannot unmarshal string into Go value of type ..."), regardless
// of Pending. So a text message can never reach this node directly after
// Start; in the real Task 8 graph this node instead sits right after
// crop_gate, which hands it a real Go CropGateOutput value in-process
// (scheduler.go:921-936: the successor's input is the predecessor's raw
// output, never round-tripped through JSON). A tiny local passthrough node
// (string -> CropGateOutput{Pending:false}) stands in for that upstream
// hand-off here, so this test still exercises a genuine in-process Go value
// hand-off into the interrupt node — the same shape Task 8's real graph
// uses — rather than a contrived struct-shaped user message.
//
// The pause branch is intentionally not exercised through the runner here:
// this fixture always produces Pending=false, so this test only proves the
// non-pending pass-through case works end-to-end through the runner. Doing
// so via a real Pending=true value would require the full upstream chain
// (extract_slots -> validate_slots -> crop_gate) that Task 8 assembles;
// forcing it here would duplicate Task 9's end-to-end test for no added
// rigor over the direct node.Run test above.
func TestCropDisambiguationInterruptNode_PauseResume_ViaRunner(t *testing.T) {
	ctx := t.Context()

	toNotPendingCropGateOutput := workflow.NewFunctionNode(
		"test_seed_crop_gate_output",
		func(_ agent.Context, _ string) (CropGateOutput, error) {
			return CropGateOutput{Pending: false}, nil
		},
		workflow.NodeConfig{},
	)

	node := NewCropDisambiguationInterruptNode()

	wfAgent, err := workflowagent.New(workflowagent.Config{
		Name:  "interrupt_test",
		Edges: workflow.Chain(workflow.Start, toNotPendingCropGateOutput, node),
	})
	require.NoError(t, err)

	svc := session.InMemoryService()
	r, err := runner.New(runner.Config{
		AppName: "interrupt_test", Agent: wfAgent, SessionService: svc, AutoCreateSession: true,
	})
	require.NoError(t, err)

	events := drainEvents(t, r.Run(ctx, "u", "s",
		&genai.Content{Role: genai.RoleUser, Parts: []*genai.Part{{Text: "go"}}},
		agent.RunConfig{},
		runner.WithStateDelta(map[string]any{}),
	))

	var gotOutput CropGateOutput
	var sawOutput bool
	for _, ev := range events {
		require.Empty(t, ev.LongRunningToolIDs, "did not expect a pause: this fixture always yields Pending=false")
		if out, ok := ev.Output.(CropGateOutput); ok {
			gotOutput = out
			sawOutput = true
		}
	}

	require.True(t, sawOutput, "expected the interrupt node's pass-through output to reach a runner event")
	require.False(t, gotOutput.Pending)
}

// drainEvents collects a runner.Run event stream, failing the test
// immediately on any yielded error — mirrors graphspike's drain
// (internal/procurement/graphspike/resume_spike_test.go), duplicated here
// per the task brief's own note that it may be copied into this file
// rather than shared as a package-level utility for a single caller.
func drainEvents(t *testing.T, seq func(func(*session.Event, error) bool)) []*session.Event {
	t.Helper()
	var out []*session.Event
	seq(func(ev *session.Event, err error) bool {
		if err != nil {
			t.Fatalf("runner yielded error: %v", err)
		}
		out = append(out, ev)
		return true
	})
	return out
}

// TestSyncResumedCropProgress_ClearsCropGateSummary 證明消歧掛起時 CROP_GATE 的
// 摘要要跟 Narration 一起清掉，否則 terminal 會為一個被撤回的判定生成敘述。
func TestSyncResumedCropProgress_ClearsCropGateSummary(t *testing.T) {
	seed := graphTurnProgress{
		Narration:          domain.StepNarration{domain.StepCropGate: "舊敘述"},
		NarrationSummaries: map[domain.StepCode]string{domain.StepCropGate: "舊摘要"},
		CropPending:        true,
	}
	ctx := newTestAgentContext(t, map[string]any{"graph_turn_progress": seed})

	emit := func(ev *session.Event) error {
		for k, v := range ev.Actions.StateDelta {
			require.NoError(t, ctx.State().Set(k, v))
		}
		return nil
	}
	require.NoError(t, syncResumedCropProgress(ctx, emit, CropGateOutput{}))

	progress, err := readGraphTurnProgress(ctx)
	require.NoError(t, err)
	require.Empty(t, progress.Narration[domain.StepCropGate])
	require.Empty(t, progress.NarrationSummaries[domain.StepCropGate])
}
