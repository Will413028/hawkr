package usecase

import (
	"context"
	"encoding/json"
	"sync/atomic"
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
	"github.com/Will413028/hawkr/ai-service/internal/procurement/testkit"
)

// Persist events through JSON, just as the Postgres session boundary does. A
// recreated runner must hydrate progress and all previous resume responses from
// those events; no Go-only CropGateOutput value can survive in session history.
// This is a durable-shape contract test, not a live Postgres integration test.
type jsonHydratedStageSessionService struct{ session.Service }

func (s jsonHydratedStageSessionService) AppendEvent(ctx context.Context, sess session.Session, event *session.Event) error {
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	var hydrated session.Event
	if err := json.Unmarshal(raw, &hydrated); err != nil {
		return err
	}
	return s.Service.AppendEvent(ctx, sess, &hydrated)
}

func TestGraphStageResumeProtocol_GroupItemPriceComplete(t *testing.T) {
	for _, durableJSON := range []bool{false, true} {
		name := "generated_ids"
		if durableJSON {
			name = "json_hydrated_legacy_ids_runner_restart"
		}
		t.Run(name, func(t *testing.T) {
			quantity, budget := 50, 220
			extractor := &resumeProtocolCountingExtractor{inner: fakeExtractor{patch: domain.SlotPatch{
				Item: item("臺灣豬五花"), Quantity: &quantity, Unit: item("KG"),
				BudgetPerUnit: &budget, DeliveryCity: item("台北市"),
			}}}
			groups := []domain.CropDisambiguationOption{{CategoryID: "10", CategoryLevel: 1, Label: "肉類"}}
			resolver := &countingCropResolver{inner: &testkit.FakeResolver[cropidentity.DisambiguationResult]{ResolveResult: cropidentity.DisambiguationResult{
				NeedsDisambiguation: true, Candidates: groups,
				Selection: &domain.CropSelection{Stage: domain.CropSelectionGroup},
			}}}
			var terminalCalls atomic.Int32
			svc := session.InMemoryService()
			if durableJSON {
				svc = jsonHydratedStageSessionService{Service: svc}
			}
			ids := seqIDs(7000)
			newRunner := func() *runner.Runner {
				interrupt := NewCropDisambiguationInterruptNode(ids)
				if durableJSON {
					interrupt = NewCropDisambiguationInterruptNode()
				}
				terminal := workflow.NewFunctionNode("stage_protocol_controller_tail", func(_ agent.Context, in CropGateOutput) (CropGateOutput, error) {
					terminalCalls.Add(1)
					return in, nil
				}, workflow.NodeConfig{})
				graph, err := workflowagent.New(workflowagent.Config{
					Name: "stage_protocol",
					Edges: workflow.Chain(workflow.Start, NewExtractSlotsNode(extractor, ids),
						NewValidateSlotsNode(ids), NewCropGateNode(resolver, ids),
						interrupt, NewCropResumeReconcileNode(), terminal),
				})
				require.NoError(t, err)
				r, err := runner.New(runner.Config{AppName: "stage_protocol", Agent: graph, SessionService: svc, AutoCreateSession: true})
				require.NoError(t, err)
				return r
			}

			const userID, sessionID = "stage-user", "stage-session"
			firstEvents := drainEvents(t, newRunner().Run(t.Context(), userID, sessionID,
				&genai.Content{Role: genai.RoleUser, Parts: []*genai.Part{{Text: "我要臺灣豬五花 50 公斤"}}},
				agent.RunConfig{}, runner.WithStateDelta(resumeProtocolSeedState())))
			first := stageProtocolInterrupt(t, firstEvents)
			require.Equal(t, domain.CropSelectionGroup, first.Selection.Stage)
			require.Equal(t, groups, first.Candidates)
			require.NotEmpty(t, first.WorkflowInterruptID)
			require.Zero(t, terminalCalls.Load())

			resume := func(interruptID string, out CropGateOutput) []*session.Event {
				// Force the inbound answer itself through JSON, independently of the
				// storage variant, to exercise the production map-shaped payload.
				raw, err := json.Marshal(out)
				require.NoError(t, err)
				var hydrated map[string]any
				require.NoError(t, json.Unmarshal(raw, &hydrated))
				return drainEvents(t, newRunner().Run(t.Context(), userID, sessionID,
					&genai.Content{Role: genai.RoleUser, Parts: []*genai.Part{{FunctionResponse: &genai.FunctionResponse{
						ID: interruptID, Name: workflow.WorkflowInputFunctionCallName,
						Response: map[string]any{"payload": hydrated},
					}}}}, agent.RunConfig{}))
			}
			items := []domain.CropDisambiguationOption{{CategoryID: "11", CategoryLevel: 2, Label: "豬肉"}}
			secondEvents := resume(first.WorkflowInterruptID, CropGateOutput{
				Item: item("臺灣豬五花"), Pending: true, Candidates: items,
				Selection: &domain.CropSelection{Stage: domain.CropSelectionItem, ParentCategoryID: "10", ParentLabel: "肉類"},
			})
			second := stageProtocolInterrupt(t, secondEvents)
			require.Equal(t, domain.CropSelectionItem, second.Selection.Stage)
			require.Equal(t, items, second.Candidates)
			require.NotEqual(t, first.WorkflowInterruptID, second.WorkflowInterruptID)
			secondProgress, seen := lastEmittedProgress(t, secondEvents)
			require.True(t, seen)
			require.True(t, secondProgress.CropPending)
			require.Equal(t, second.Selection, secondProgress.CropSelection)
			require.Equal(t, &quantity, secondProgress.NextSlots.Quantity)
			require.Equal(t, &budget, secondProgress.NextSlots.BudgetPerUnit)
			require.Equal(t, item("台北市"), secondProgress.NextSlots.DeliveryCity)
			require.Zero(t, terminalCalls.Load(), "a second stage must park before the controller")

			thirdEvents := resume(second.WorkflowInterruptID, CropGateOutput{
				Item: item("臺灣豬五花"), Pending: true,
				Selection: &domain.CropSelection{Stage: domain.CropSelectionDescribe, CategoryID: "11", Label: "豬肉"},
			})
			third := stageProtocolInterrupt(t, thirdEvents)
			require.Equal(t, domain.CropSelectionDescribe, third.Selection.Stage)
			require.Empty(t, third.Candidates)
			require.NotEqual(t, second.WorkflowInterruptID, third.WorkflowInterruptID)
			require.NotEqual(t, first.WorkflowInterruptID, third.WorkflowInterruptID)
			require.Zero(t, terminalCalls.Load(), "manual price must also park before the controller")
			thirdProgress, seen := lastEmittedProgress(t, thirdEvents)
			require.True(t, seen)
			require.Equal(t, third.Selection, thirdProgress.CropSelection)
			require.Empty(t, thirdProgress.CropCandidates, "old item choices must not leak into the price card")

			chosen := testkit.ConfirmedResolution("")
			completed := resume(third.WorkflowInterruptID, CropGateOutput{Item: item("臺灣豬五花"), Resolution: chosen})
			for _, ev := range completed {
				require.Nil(t, ev.RequestedInput, "completed selection must not ask an earlier stage again")
			}
			finalProgress, seen := lastEmittedProgress(t, completed)
			require.True(t, seen)
			require.False(t, finalProgress.CropPending)
			require.Nil(t, finalProgress.CropSelection)
			require.Empty(t, finalProgress.CropCandidates)
			require.Equal(t, chosen, finalProgress.NextSlots.ItemResolution)
			require.Equal(t, chosen, finalProgress.Patch.ItemResolution)
			require.Equal(t, item("臺灣豬五花"), finalProgress.NextSlots.Item)
			require.Equal(t, &quantity, finalProgress.NextSlots.Quantity)
			require.Equal(t, item("KG"), finalProgress.NextSlots.Unit)
			require.Equal(t, &budget, finalProgress.NextSlots.BudgetPerUnit)
			require.Equal(t, item("台北市"), finalProgress.NextSlots.DeliveryCity)
			require.EqualValues(t, 1, terminalCalls.Load())
			require.EqualValues(t, 1, atomic.LoadInt32(&extractor.calls), "all resumes must avoid slot extraction")
			require.EqualValues(t, 1, atomic.LoadInt32(&resolver.calls), "all resumes must avoid initial retrieval")
		})
	}
}

func stageProtocolInterrupt(t *testing.T, events []*session.Event) CropDisambiguationInterruptPayload {
	t.Helper()
	var result *CropDisambiguationInterruptPayload
	for _, ev := range events {
		if ev.RequestedInput == nil {
			continue
		}
		require.Nil(t, result, "one resumed stage must create exactly one new interrupt")
		raw, err := json.Marshal(ev.RequestedInput.Payload)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(raw, &result))
		require.Equal(t, result.WorkflowInterruptID, ev.RequestedInput.InterruptID)
		require.Equal(t, result.WorkflowInterruptID, ev.Actions.StateDelta[cropWorkflowInterruptStateKey])
	}
	require.NotNil(t, result, "stage must be durably parked")
	return *result
}
