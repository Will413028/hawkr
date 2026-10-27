// Package graphspike is a permanent regression guard, not throwaway code.
// It pins down one ADK v2 behavior this codebase's graph design depends on:
// a static Edge chain (workflow.Chain) does NOT re-run an already-completed
// upstream FunctionNode when a downstream node interrupts and later
// resumes (verified empirically 2026-08-29 against adk/v2 v2.2.0; a future
// ADK upgrade could silently change this). If this test starts failing,
// every guard node upstream of crop_disambiguation_interrupt
// (extract_slots, validate_slots) needs idempotency caching before the
// upgrade can land — see
// docs/superpowers/specs/2026-08-29-adk-workflow-graph-skeleton-design.md §1.3/§5.
package graphspike

import (
	"sync/atomic"
	"testing"

	"google.golang.org/genai"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/workflowagent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"
)

func TestLinearChainResume_DoesUpstreamNodeRerun(t *testing.T) {
	ctx := t.Context()

	var counterA int32
	nodeA := workflow.NewFunctionNode("counter_a",
		func(_ agent.Context, in string) (string, error) {
			atomic.AddInt32(&counterA, 1)
			return in, nil
		},
		workflow.NodeConfig{},
	)

	const interruptID = "b-interrupt"
	nodeB := workflow.NewEmittingFunctionNode("interrupt_b",
		func(ctx agent.Context, in string, emit func(*session.Event) error) (string, error) {
			if response, ok := ctx.ResumedInput(interruptID); ok {
				return response.(string), nil
			}
			if err := emit(workflow.NewRequestInputEvent(ctx, session.RequestInput{
				InterruptID: interruptID,
				Message:     "pause for spike",
				Payload:     in,
			})); err != nil {
				return "", err
			}
			return "", workflow.ErrNodeInterrupted
		},
		workflow.NodeConfig{},
	)

	nodeC := workflow.NewFunctionNode("passthrough_c",
		func(_ agent.Context, in string) (string, error) { return in, nil },
		workflow.NodeConfig{},
	)

	wfAgent, err := workflowagent.New(workflowagent.Config{
		Name:  "resume_spike",
		Edges: workflow.Chain(workflow.Start, nodeA, nodeB, nodeC),
	})
	if err != nil {
		t.Fatalf("workflowagent.New() error = %v", err)
	}

	svc := session.InMemoryService()
	r, err := runner.New(runner.Config{
		AppName: "resume_spike", Agent: wfAgent, SessionService: svc, AutoCreateSession: true,
	})
	if err != nil {
		t.Fatalf("runner.New() error = %v", err)
	}

	const userID, sessionID = "spike-user", "spike-session"

	turn1 := drain(t, r.Run(ctx, userID, sessionID, userText("go"), agent.RunConfig{}))
	callID, callName := findLongRunningInterrupt(turn1)
	if callID == "" {
		t.Fatalf("turn 1 did not pause: no LongRunningToolIDs interrupt found in %d events", len(turn1))
	}
	if got := atomic.LoadInt32(&counterA); got != 1 {
		t.Fatalf("nodeA ran %d times before the pause, want exactly 1", got)
	}

	turn2 := drain(t, r.Run(ctx, userID, sessionID, resumeContent(callID, callName, "resumed-value"), agent.RunConfig{}))
	if id, _ := findLongRunningInterrupt(turn2); id != "" {
		t.Fatalf("turn 2 (resume) unexpectedly paused again: interrupt id %q", id)
	}

	// This is the actual regression guard, not just the t.Logf below: if a
	// future ADK upgrade changes resume semantics so upstream nodes DO
	// re-run, counterA becomes 2 here and this must fail loudly — a silent
	// t.Logf would let the regression ship. See the package doc comment:
	// extract_slots/validate_slots upstream of crop_disambiguation_interrupt
	// would then need idempotency caching (reading session state for an
	// already-computed result before recomputing) before any such upgrade
	// could land.
	if got := atomic.LoadInt32(&counterA); got != 1 {
		t.Fatalf("nodeA ran %d time(s) total across pause+resume, want exactly 1: resume on a "+
			"static workflow.Chain re-ran an already-completed upstream FunctionNode — this is "+
			"the exact regression this permanent guard test exists to catch. extract_slots/"+
			"validate_slots (upstream of crop_disambiguation_interrupt in NewProcurementGraph) "+
			"now need idempotency caching before this ADK behavior change can land.", got)
	}

	t.Logf("EMPIRICAL RESULT: nodeA (upstream FunctionNode) ran %d time(s) total across pause+resume "+
		"on a static workflow.Chain. ==1 means guard nodes upstream of an interrupt do NOT need "+
		"idempotency caching for this graph shape; >1 means they do.", atomic.LoadInt32(&counterA))
}

func userText(text string) *genai.Content {
	return &genai.Content{Role: genai.RoleUser, Parts: []*genai.Part{{Text: text}}}
}

func resumeContent(callID, callName string, payload any) *genai.Content {
	return &genai.Content{
		Role: genai.RoleUser,
		Parts: []*genai.Part{{
			FunctionResponse: &genai.FunctionResponse{
				ID:       callID,
				Name:     callName,
				Response: map[string]any{"payload": payload},
			},
		}},
	}
}

func findLongRunningInterrupt(events []*session.Event) (id, name string) {
	for _, ev := range events {
		if ev == nil || len(ev.LongRunningToolIDs) == 0 || ev.Content == nil {
			continue
		}
		for _, p := range ev.Content.Parts {
			fc := p.FunctionCall
			if fc == nil {
				continue
			}
			for _, lrt := range ev.LongRunningToolIDs {
				if lrt == fc.ID {
					return fc.ID, fc.Name
				}
			}
		}
	}
	return "", ""
}

func drain(t *testing.T, seq func(func(*session.Event, error) bool)) []*session.Event {
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
