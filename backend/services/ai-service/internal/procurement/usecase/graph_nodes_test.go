// services/ai-service/internal/procurement/usecase/graph_nodes_test.go
package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/pricing"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/testkit"
	"github.com/Will413028/hawkr/go-pkg/identity"
	"github.com/stretchr/testify/require"
	"google.golang.org/genai"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"
)

func TestValidateSlotsNode_MatchesDirectApply_GoldenFixtures(t *testing.T) {
	cases := loadSlotsApplyFixtures(t)

	node := NewValidateSlotsNode(seqIDs(5000))

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wantSlots, wantRejected := tc.before.Apply(tc.patch, tc.now)

			// NewValidateSlotsNode reads CurrentSlots/Now from session
			// state (stateValue keys "current_slots" / "now") and takes
			// the raw Run input as Patch — see graph_nodes.go's
			// NewValidateSlotsNode doc comment. So the per-turn constants
			// have to be seeded into the test session's state up front;
			// only the patch travels as node.Run's input.
			ctx := newTestAgentContext(t, map[string]any{
				"current_slots": tc.before,
				"now":           tc.now,
			})

			got, err := runFunctionNode[ValidateSlotsOutput](t, node, ctx, tc.patch)
			require.NoError(t, err)

			if got.Slots.Hash() != wantSlots.Hash() {
				t.Errorf("Slots mismatch: got %+v, want %+v", got.Slots, wantSlots)
			}
			require.Equal(t, wantRejected, got.Rejected)
		})
	}
}

// slotsApplyFixture is the subset of the T4 golden fixture shape
// (domain/slots_apply_golden_test.go) this test needs: before/patch/now
// as diverse, already-frozen (Slots.Apply) input combinations. after/
// rejected are intentionally not decoded here — this test recomputes
// them by calling Slots.Apply directly (see above) rather than trusting
// the recorded golden values, so it stays a genuine node-vs-direct-call
// comparison instead of a second copy of TestSlots_Apply_GoldenFixtures.
type slotsApplyFixture struct {
	name   string
	before domain.Slots
	patch  domain.SlotPatch
	now    time.Time
}

// loadSlotsApplyFixtures reads every services/ai-service/internal/
// procurement/domain/testdata/slots_apply/*.json fixture, copying the
// same read+unmarshal approach domain/slots_apply_golden_test.go uses
// inline (it exports no loader helper, and this task must not modify
// the domain package to add one).
func loadSlotsApplyFixtures(t *testing.T) []slotsApplyFixture {
	t.Helper()

	paths, err := filepath.Glob(filepath.Join("..", "domain", "testdata", "slots_apply", "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, paths, "no slots_apply golden fixtures found")

	cases := make([]slotsApplyFixture, 0, len(paths))
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		require.NoErrorf(t, err, "reading fixture %s", path)

		var wire struct {
			Before domain.Slots     `json:"before"`
			Patch  domain.SlotPatch `json:"patch"`
			Now    time.Time        `json:"now"`
		}
		require.NoErrorf(t, json.Unmarshal(raw, &wire), "unmarshalling fixture %s", path)

		cases = append(cases, slotsApplyFixture{
			name:   strings.TrimSuffix(filepath.Base(path), ".json"),
			before: wire.Before,
			patch:  wire.Patch,
			now:    wire.Now,
		})
	}
	return cases
}

func TestCropGateNode_MatchesDirectComputeCropGate_GoldenFixtures(t *testing.T) {
	cases := loadCropGateFixtures(t)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node := NewCropGateNode(tc.resolver, seqIDs(6000))

			want := cropidentity.ComputeGate(t.Context(), tc.resolver, tc.before, tc.after)

			// Same state/node-input split as TestValidateSlotsNode above:
			// RequestID/Before are per-turn constants read from session
			// state ("request_id"/"current_slots" via stateValue),
			// Validated (the upstream validate_slots node's output) is the
			// only value that travels as node.Run's raw input.
			ctx := newTestAgentContext(t, map[string]any{
				"request_id":    int64(1),
				"current_slots": tc.before,
			})

			got, err := runFunctionNode[CropGateOutput](t, node, ctx, ValidateSlotsOutput{Slots: tc.after})
			require.NoError(t, err)

			// Full-field comparison against the direct cropidentity.ComputeGate call
			// (not just Pending/len(Candidates)) so a wrong Item or a
			// wrong-content Candidates slice can't hide behind a weaker
			// check.
			require.Equal(t, want.Item, got.Item)
			require.Equal(t, want.Pending, got.Pending)
			require.Equal(t, want.Candidates, got.Candidates)
		})
	}
}

// TestValidateSlotsNode_WritesNextSlotsAndRejectedToProgress proves
// validate_slots' Apply result lands in graph_turn_progress, not just the
// node's own typed output — this is how crop_gate (below) and the terminal
// nodes (Task 5) recover it past the typed node chain.
func TestValidateSlotsNode_WritesNextSlotsAndRejectedToProgress(t *testing.T) {
	node := NewValidateSlotsNode(seqIDs(5100))

	ctx := newTestAgentContext(t, map[string]any{
		"current_slots": domain.Slots{},
		"now":           time.Now().UTC(),
	})

	patch := domain.SlotPatch{Item: item("蘋果")}
	got, err := runFunctionNode[ValidateSlotsOutput](t, node, ctx, patch)
	require.NoError(t, err)

	progress, err := readGraphTurnProgress(ctx)
	require.NoError(t, err)
	require.Equal(t, got.Slots, progress.NextSlots)
	require.Equal(t, got.Rejected, progress.Rejected)
}

// TestCropGateNode_UnavailableItem_MarksProgressPendingWithNoCandidates 守住
// 查無候選這條路徑的正確落點：progress.NextSlots.Item 維持 validate_slots 算出的
// after.Item（買家剛打的字）、progress.Rejected 維持 validate_slots 自己的那一筆，
// crop_gate 一律不碰這兩者；progress.CropPending／CropCandidates 才是這件事真正
// 落地的地方。
//
// Cut 1 之前這裡是硬擋：Unavailable 分支會回一個被拒欄位，crop_gate node 據此把
// Item 退回舊值並疊加 Rejected。Cut 1 改成待處理形狀後那個生產者消失，override
// 分支成為死碼，已隨 cropidentity.GateResult.Rejected 一起移除——本測試因此從「證明
// override 會觸發」翻面成「證明 crop_gate 永遠不碰 NextSlots／Rejected」。
func TestCropGateNode_UnavailableItem_MarksProgressPendingWithNoCandidates(t *testing.T) {
	resolver := &testkit.FakeResolver[cropidentity.DisambiguationResult]{ResolveResult: cropidentity.DisambiguationResult{Unavailable: true}}
	node := NewCropGateNode(resolver, seqIDs(5200))

	ctx := newTestAgentContext(t, map[string]any{
		"request_id":    int64(1),
		"current_slots": domain.Slots{}, // before.Item == nil — required so itemChanged(before, after) is true below
		"graph_turn_progress": graphTurnProgress{
			NextSlots: domain.Slots{Item: item("不存在的東西")},
			Rejected:  []domain.RejectedField{{Key: "quantity", Reason: "看不懂數量"}},
		},
	})

	_, err := runFunctionNode[CropGateOutput](t, node, ctx, ValidateSlotsOutput{Slots: domain.Slots{Item: item("不存在的東西")}})
	require.NoError(t, err)

	progress, err := readGraphTurnProgress(ctx)
	require.NoError(t, err)
	require.NotNil(t, progress.NextSlots.Item)
	require.Equal(t, "不存在的東西", *progress.NextSlots.Item,
		"crop_gate 不再覆寫 Item——查無候選是待處理，不是硬擋")
	require.Len(t, progress.Rejected, 1, "只剩 validate_slots 自己的那一筆，crop_gate 不再疊加")
	require.Equal(t, "quantity", progress.Rejected[0].Key)
	require.True(t, progress.CropPending)
	require.NotNil(t, progress.CropCandidates)
	require.Empty(t, progress.CropCandidates)
}

// TestCropGateNode_Resolved_LeavesItemRejectedAndResolutionUntouched 釘住 crop_gate
// 對 progress 的寫入面：它只准寫 CropPending／CropCandidates／CropSelection 這組待處理
// 狀態，不准碰 NextSlots.Item、Rejected 與 ItemResolution。
//
// Item 那一半是既有不變量的延續——Cut 1 之前 crop_gate 硬擋時會把 Item 退回舊值，
// 那個 override 隨 cropidentity.GateResult.Rejected 一起移除了；沒有這支測試，日後有人把
// override 加回來會通過本檔其他每一支測試（golden fixture 根本不讀
// graph_turn_progress），卻悄悄破壞下游組裝。
func TestCropGateNode_Resolved_LeavesItemRejectedAndResolutionUntouched(t *testing.T) {
	resolver := &testkit.FakeResolver[cropidentity.DisambiguationResult]{ResolveResult: cropidentity.DisambiguationResult{
		Unavailable: true,
		Selection:   &domain.CropSelection{Stage: domain.CropSelectionPrice, CategoryID: "17", Label: "高麗菜"},
	}}
	node := NewCropGateNode(resolver, seqIDs(5300))

	seedRejected := []domain.RejectedField{{Key: "quantity", Reason: "看不懂數量"}}
	ctx := newTestAgentContext(t, map[string]any{
		"request_id":    int64(1),
		"current_slots": domain.Slots{}, // before.Item == nil — item changes below so cropidentity.ComputeGate actually resolves instead of taking the item-unchanged shortcut
		"graph_turn_progress": graphTurnProgress{
			NextSlots: domain.Slots{Item: item("蘋果")},
			Rejected:  seedRejected,
		},
	})

	got, err := runFunctionNode[CropGateOutput](t, node, ctx, ValidateSlotsOutput{Slots: domain.Slots{Item: item("芭樂")}})
	require.NoError(t, err)

	progress, err := readGraphTurnProgress(ctx)
	require.NoError(t, err)
	require.Equal(t, item("蘋果"), progress.NextSlots.Item,
		"crop_gate 不得覆寫 Item——validate_slots 算出來的值原樣通過")
	require.Equal(t, seedRejected, progress.Rejected, "crop_gate 不得疊加 Rejected")
	require.Nil(t, progress.NextSlots.ItemResolution, "分類確認後待買家填單價，解析結果由 resume 寫入")
	require.Nil(t, progress.Patch.ItemResolution)
	require.True(t, progress.CropPending)
	require.Empty(t, progress.CropCandidates)
	require.Equal(t, domain.CropPendingReasonUnavailable, got.PendingReason)
	require.Equal(t, "17", got.Selection.CategoryID)
}

// TestCropGateNode_ItemUnchanged_DoesNotOverwriteCarriedResolution 是上面那條的
// 反面：item 沒變動時 gate 不產生新解析（Resolution 為 nil），既有的解析結果由
// Slots 自己帶著走，node 不得用 nil 把它洗掉。少了這條，把寫入改成無條件覆寫會
// 讓買家在消歧之後隨便說一句話就把剛選好的品項分類弄丟。
func TestCropGateNode_ItemUnchanged_DoesNotOverwriteCarriedResolution(t *testing.T) {
	carried := domain.NewSelfPricedItemResolution(75, "甘藍-初秋", domain.CategoryAbsent("測試"))
	resolver := &testkit.FakeResolver[cropidentity.DisambiguationResult]{ResolveErr: errors.New("must not be called")}
	node := NewCropGateNode(resolver, seqIDs(5400))

	ctx := newTestAgentContext(t, map[string]any{
		"request_id":    int64(1),
		"current_slots": domain.Slots{Item: item("高麗菜"), ItemResolution: carried},
		"graph_turn_progress": graphTurnProgress{
			NextSlots: domain.Slots{Item: item("高麗菜"), ItemResolution: carried},
		},
	})

	_, err := runFunctionNode[CropGateOutput](t, node, ctx,
		ValidateSlotsOutput{Slots: domain.Slots{Item: item("高麗菜"), ItemResolution: carried}})
	require.NoError(t, err)

	progress, err := readGraphTurnProgress(ctx)
	require.NoError(t, err)
	require.Equal(t, carried, progress.NextSlots.ItemResolution, "既有解析不得被 nil 洗掉")
	require.Nil(t, progress.Patch.ItemResolution,
		"沒有新判定就不該進 patch，否則消歧期間每一輪都重複冒出同一個 chip")
}

// TestCropGateNode_CarriesPendingReasonAndFactsAsOfAcrossTheNodeBoundary pins
// what replaced the old "computeCropGate and persistCropGate must run in the
// same call" landmine: the gate no longer writes anything itself, so the two
// values the interrupt row needs (pendingReason and factsAsOf) have to survive
// the node boundary on CropGateOutput. If either is dropped here, the
// interrupt lands with a payload that cannot say why it is pending — exactly
// the "infer it from the candidate count" regression Cut 1 removed.
func TestCropGateNode_CarriesPendingReasonAndFactsAsOfAcrossTheNodeBoundary(t *testing.T) {
	resolver := &testkit.FakeResolver[cropidentity.DisambiguationResult]{ResolveResult: cropidentity.DisambiguationResult{
		NeedsDisambiguation: true,
		Candidates:          []domain.CropDisambiguationOption{{Label: "蘋果-富士"}},
	}}

	node := NewCropGateNode(resolver, seqIDs(7000))

	ctx := newTestAgentContext(t, map[string]any{
		"request_id":    int64(1),
		"current_slots": domain.Slots{},
	})
	// An input that triggers an item change (before has no item, after
	// does) so cropidentity.ComputeGate takes a resolve branch.
	got, err := runFunctionNode[CropGateOutput](t, node, ctx, ValidateSlotsOutput{Slots: domain.Slots{Item: item("蘋果")}})
	require.NoError(t, err)

	require.True(t, got.Pending)
	require.Equal(t, domain.CropPendingReasonAmbiguous, got.PendingReason,
		"待處理原因必須跨過 node 邊界，interrupt payload 才說得出「為什麼在問」")
	require.False(t, got.FactsAsOf.IsZero(),
		"查詢時刻必須跨過 node 邊界，才寫得進 procurement_agent_run_interrupts.facts_as_of")
}

// TestPricePreviewNode_MatchesDirectCall proves
// price_preview's FunctionNode wrapper doesn't corrupt input/output at the
// node boundary, exercising the BUYER_ENTERED_NO_REFERENCE branch of
// PreviewRfqPrice.
//
// Both calls share the same uc (and its fakePricePreviewRepo). The first,
// direct call runs the real BUYER_ENTERED_NO_REFERENCE logic and inserts a
// row keyed by ClientRequestID. The second, through-the-node call with the
// identical input hits PreviewRfqPrice's own idempotency short-circuit
// (pricing.HashJSON(in) matches the stored PreviewInputHash) and replays that exact
// stored row — so want and got are expected to be byte-identical, including
// PricePreviewID/Version/ExpiresAt, without this test needing to freeze a
// clock to dodge time.Now() nondeterminism. That replay is a genuine branch
// of PreviewRfqPrice, so this remains a real node-vs-direct-call comparison,
// not a tautology.
func TestPricePreviewNode_MatchesDirectCall(t *testing.T) {
	repo := &testkit.FakePricePreviewRepo{}
	revisions := testkit.FakeRevisionReader{RevisionID: 1}
	uc := pricing.NewPreviewRfqPriceUseCase(repo, revisions)

	in := pricing.PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "test-1",
		ExpectedRevisionID: 1,
		BuyerRequestedRange: &pricing.BuyerRequestedRange{
			Currency: "TWD", LowerBoundAmount: "100", UpperBoundAmount: "200",
		},
	}

	want, err := uc.PreviewRfqPrice(t.Context(), in)
	require.NoError(t, err)
	require.Equal(t, "100", want.BuyerRequestedRange.LowerBoundAmount, "sanity check: the fixture must reach the persisted preview, not fall through to an error path")

	node := NewPricePreviewNode(uc)

	ctx := newTestAgentContext(t, nil)
	got, err := runFunctionNode[pricing.PricePreviewResult](t, node, ctx, in)
	require.NoError(t, err)

	require.Equal(t, want, got)
}

// cropGateFixture is the subset of testdata/crop_gate's golden-fixture
// wire format (cropidentity/crop_gate_golden_test.go's
// TestComputeCropGate_GoldenFixtures) this test needs to reconstruct a
// cropidentity.ComputeGate call: resolverNil/resolveResult/resolveErr
// rebuild the same testkit.FakeResolver test double the golden test uses,
// before/after are cropidentity.ComputeGate's other three inputs. The
// fixture's recorded "result" is intentionally not decoded — this test
// recomputes it via a direct cropidentity.ComputeGate call (see above)
// rather than trusting the recorded golden values, same reasoning as
// loadSlotsApplyFixtures's doc comment.
type cropGateFixture struct {
	name     string
	resolver cropidentity.Resolver
	before   domain.Slots
	after    domain.Slots
}

// loadCropGateFixtures reads every services/ai-service/internal/
// procurement/cropidentity/testdata/crop_gate/*.json fixture (written by
// crop_gate_golden_test.go's -update-crop-gate-golden flag, which now lives
// in the cropidentity package — this file reaches across to its sibling
// package's testdata since this node-wrapper test still needs the same
// fixture set) and decodes the input side of each into a cropGateFixture.
func loadCropGateFixtures(t *testing.T) []cropGateFixture {
	t.Helper()

	paths, err := filepath.Glob(filepath.Join("..", "cropidentity", "testdata", "crop_gate", "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, paths, "no crop_gate golden fixtures found")

	cases := make([]cropGateFixture, 0, len(paths))
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		require.NoErrorf(t, err, "reading fixture %s", path)

		var wire struct {
			ResolverNil   bool                              `json:"resolverNil"`
			ResolveResult cropidentity.DisambiguationResult `json:"resolveResult"`
			ResolveErr    string                            `json:"resolveErr"`
			Before        domain.Slots                      `json:"before"`
			After         domain.Slots                      `json:"after"`
		}
		require.NoErrorf(t, json.Unmarshal(raw, &wire), "unmarshalling fixture %s", path)

		var resolver cropidentity.Resolver
		if !wire.ResolverNil {
			fake := &testkit.FakeResolver[cropidentity.DisambiguationResult]{ResolveResult: wire.ResolveResult}
			if wire.ResolveErr != "" {
				fake.ResolveErr = errors.New(wire.ResolveErr)
			}
			resolver = fake
		}

		cases = append(cases, cropGateFixture{
			name:     strings.TrimSuffix(filepath.Base(path), ".json"),
			resolver: resolver,
			before:   wire.Before,
			after:    wire.After,
		})
	}
	return cases
}

// TestExtractSlotsNode_DelegatesUnchanged proves NewExtractSlotsNode's
// SlotPatch output is exactly what the extractor returns, and CurrentSlots
// still travels through session state ("current_slots" via stateValue) the
// same way NewValidateSlotsNode/NewCropGateNode do. Since Task 3, the node
// also emits an Activity session.Event and mints a row id via newID — that
// behavior is covered separately by graph_test.go's
// TestNewProcurementGraph_ExtractSlots_EmitsActivityEvent (driven through a
// real scheduler, where the emitted event is actually observable); this
// test's own runFunctionNode helper only surfaces the node's *terminal*
// Output, so it stays focused on the one thing it already proved: the
// returned SlotPatch is unchanged. Reuses the existing fakeExtractor test
// double from turn_test.go (same package) rather than introducing a second,
// parallel fake — it already has the (patch, err) shape this test needs.
func TestExtractSlotsNode_DelegatesUnchanged(t *testing.T) {
	extractor := fakeExtractor{patch: domain.SlotPatch{Item: strPtr("蘋果")}}

	node := NewExtractSlotsNode(extractor, seqIDs(1))

	ctx := newTestAgentContext(t, map[string]any{
		"current_slots": domain.Slots{},
	})

	// extract_slots ignores node.Run's raw input entirely — nil is a
	// valid placeholder here (see graph_nodes.go's NewExtractSlotsNode
	// doc comment: only current_slots binds from state; the message text
	// comes from ctx.UserContent(), which testInvocationContext stubs to
	// nil).
	got, err := runFunctionNode[domain.SlotPatch](t, node, ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, got.Item)
	require.Equal(t, "蘋果", *got.Item)
}

// TestExtractSlotsNode_ExtractorFails_DegradesInsteadOfErroring proves the
// fail-open fix (spec §4.4): an extractor error must not fail the whole
// node — it must return an empty SlotPatch with a nil error, and record
// Degraded:true in graph_turn_progress, matching HandleStream's own
// "LLM 抽取失敗不是錯誤而是降級" discipline (turn.go).
func TestExtractSlotsNode_ExtractorFails_DegradesInsteadOfErroring(t *testing.T) {
	extractor := fakeExtractor{err: errBoom}

	node := NewExtractSlotsNode(extractor, seqIDs(2))

	ctx := newTestAgentContext(t, map[string]any{
		"current_slots": domain.Slots{},
		"request_id":    int64(1),
	})

	got, err := runFunctionNode[domain.SlotPatch](t, node, ctx, nil)
	require.NoError(t, err, "extractor failure must degrade, not fail the node")
	require.Equal(t, domain.SlotPatch{}, got)

	progress, err := readGraphTurnProgress(ctx)
	require.NoError(t, err)
	require.True(t, progress.Degraded)
	require.Equal(t, domain.SlotPatch{}, progress.Patch)
}

// runFunctionNode drives node.Run(ctx, input) to completion and
// collapses the resulting event stream into a single (OUT, error): ADK's
// Node.Run returns iter.Seq2[*session.Event, error], but callers here
// only care about the terminal event's Output. Shared by this task and
// (per the T8 plan) task 4's guard-node tests.
//
// For every yielded event, it also merges Actions.StateDelta into the
// context's session state — mirroring what the real *runner.Runner does
// for every event before a caller ever sees it (runner.go's AppendEvent
// call; see graph_dispatch_test.go's
// TestClaimAndDispatchTurnViaGraph_PayloadRuntimeType_IsStructNotMap doc
// comment for the exact call site). Without this, a node's state-only
// progress event (graph_terminal.go's emitGraphTurnProgress, added in
// Task 2) would never become visible to a subsequent readGraphTurnProgress
// call in a test built on this helper — this package has no smaller,
// already-existing seam for that merge than reimplementing it here.
// Harmless for every pre-existing caller: NewExtractSlotsNode/
// NewValidateSlotsNode/NewCropGateNode's Activity events, and every
// FunctionNode's own terminal event, are all built via session.NewEvent,
// which always initializes Actions.StateDelta as an empty (non-nil) map
// (session.go's NewEvent) — merging an empty map is a no-op.
func runFunctionNode[OUT any](t *testing.T, node workflow.Node, ctx agent.Context, input any) (OUT, error) {
	t.Helper()
	var out OUT
	var runErr error
	for ev, err := range node.Run(ctx, input) {
		if err != nil {
			runErr = err
			continue
		}
		if ev == nil {
			continue
		}
		for k, v := range ev.Actions.StateDelta {
			require.NoError(t, ctx.State().Set(k, v))
		}
		if ev.Output != nil {
			if v, ok := ev.Output.(OUT); ok {
				out = v
			}
		}
	}
	return out, runErr
}

// newTestAgentContext builds a minimal agent.Context suitable for
// driving a workflow.FunctionNode directly in a unit test, backed by a
// real in-memory ADK session (session.InMemoryService, the same
// constructor internal/procurement/graphspike/resume_spike_test.go
// drives a full runner with) pre-loaded with initialState.
//
// google.golang.org/adk/v2/workflow's own tests build their mock
// context the same way (agent.NewContext wrapping a hand-written
// agent.InvocationContext — see workflow_test.go's MockInvocationContext
// in the ADK module source), but that type is defined in workflow's
// _test.go files, so it does not exist in the compiled, importable
// package — only mirroring its construction here, not importing it.
func newTestAgentContext(t *testing.T, initialState map[string]any) agent.Context {
	t.Helper()

	svc := session.InMemoryService()
	resp, err := svc.Create(t.Context(), &session.CreateRequest{
		AppName:   "procurement_graph_nodes_test",
		UserID:    "test-user",
		SessionID: "test-session",
		State:     initialState,
	})
	require.NoError(t, err)

	return agent.NewContext(&testInvocationContext{Context: t.Context(), sess: resp.Session})
}

// testInvocationContext is a minimal agent.InvocationContext test
// double. FunctionNode.Run (via NewFunctionNodeFromState's wrapped fn)
// only exercises Session() and InvocationID(); the remaining methods
// are harmless zero-value stubs, mirroring the shape of ADK's own
// workflow-package-internal MockInvocationContext.
type testInvocationContext struct {
	context.Context
	sess session.Session
}

func (c *testInvocationContext) Agent() agent.Agent          { return nil }
func (c *testInvocationContext) Artifacts() agent.Artifacts  { return nil }
func (c *testInvocationContext) Memory() agent.Memory        { return nil }
func (c *testInvocationContext) Session() session.Session    { return c.sess }
func (c *testInvocationContext) InvocationID() string        { return "test-invocation-id" }
func (c *testInvocationContext) Branch() string              { return "" }
func (c *testInvocationContext) IsolationScope() string      { return "" }
func (c *testInvocationContext) UserContent() *genai.Content { return nil }
func (c *testInvocationContext) RunConfig() *agent.RunConfig { return nil }
func (c *testInvocationContext) EndInvocation()              {}
func (c *testInvocationContext) Ended() bool                 { return false }
func (c *testInvocationContext) ResumedInput(string) (any, bool) {
	return nil, false
}

func (c *testInvocationContext) WithContext(ctx context.Context) agent.InvocationContext {
	cp := *c
	cp.Context = ctx
	return &cp
}

func (c *testInvocationContext) WithICDelta(_ *agent.InvocationContextDelta) agent.InvocationContext {
	return c
}

// TestCropGateNode_UnavailableItem_DoesNotInvalidateValidateSlotsNarration 是
// Cut 1（品項別名檢索死路解鎖）對前一支修法的直接推翻，取代原本的
// TestCropGateNode_HardBlock_InvalidatesStaleValidateSlotsNarration：那支測試
// 釘住「crop_gate 硬擋時把 Item 退回舊值，讓 validate_slots 算敘述時看到的
// NextSlots.Item（買家這輪打的新值）失真，所以必須作廢那句敘述」——這個前提在
// Cut 1 之後不再成立，查無候選（Unavailable）現在是待處理，Item 維持
// after.Item 不變，validate_slots 算的「這一輪新記下：品項＝X」仍然是真話，
// 不該被作廢。
//
// withoutStepNarration 這個函式本身仍然活著（crop_resume_reconcile 節點還在用
// 它清 resume 後過期的 CROP_GATE 敘述），只是 graph_nodes.go 裡「crop_gate 因
// Rejected 非空而作廢 VALIDATE_SLOTS」這個特定呼叫點，隨 cropidentity.ComputeGate 唯一
// 會產生 Rejected 的分支（原本的 Unavailable 硬擋）一起變成死碼，已於本刀移除。
func TestCropGateNode_UnavailableItem_DoesNotInvalidateValidateSlotsNarration(t *testing.T) {
	resolver := &testkit.FakeResolver[cropidentity.DisambiguationResult]{ResolveResult: cropidentity.DisambiguationResult{Unavailable: true}}
	node := NewCropGateNode(resolver, seqIDs(5900))

	ctx := newTestAgentContext(t, map[string]any{
		"request_id":    int64(1),
		"current_slots": domain.Slots{},
		"graph_turn_progress": graphTurnProgress{
			NextSlots: domain.Slots{Item: item("紫蘇")},
			Narration: domain.StepNarration{
				domain.StepExtractSlots: "正在讀你的需求，抓品項",
			},
			NarrationSummaries: map[domain.StepCode]string{
				domain.StepValidateSlots: "這一輪新記下：品項＝紫蘇；目前條件皆有效。",
			},
		},
	})

	_, err := runFunctionNode[CropGateOutput](t, node, ctx, ValidateSlotsOutput{Slots: domain.Slots{Item: item("紫蘇")}})
	require.NoError(t, err)

	progress, err := readGraphTurnProgress(ctx)
	require.NoError(t, err)
	require.NotNil(t, progress.NextSlots.Item, "前提：查無候選不再退回舊值")
	require.Equal(t, "紫蘇", *progress.NextSlots.Item)
	require.Equal(t, "這一輪新記下：品項＝紫蘇；目前條件皆有效。", progress.NarrationSummaries[domain.StepValidateSlots],
		"Item 沒有被推翻，這份摘要仍然是真的，不該被作廢")
	require.Contains(t, progress.Narration, domain.StepExtractSlots)
}

// TestCropGateNode_NoHardBlock_KeepsValidateSlotsNarration 是上面那支的對照組：
// 沒有硬擋就沒有推翻，敘述必須原樣留著（否則等於每一輪都把 VALIDATE_SLOTS 打回
// 固定文案，正是本分支要修掉的東西）。
func TestCropGateNode_NoHardBlock_KeepsValidateSlotsNarration(t *testing.T) {
	resolver := &testkit.FakeResolver[cropidentity.DisambiguationResult]{ResolveResult: cropidentity.DisambiguationResult{Unavailable: true}}
	node := NewCropGateNode(resolver, seqIDs(5950))

	ctx := newTestAgentContext(t, map[string]any{
		"request_id":    int64(1),
		"current_slots": domain.Slots{},
		"graph_turn_progress": graphTurnProgress{
			NextSlots:          domain.Slots{Item: item("高麗菜")},
			NarrationSummaries: map[domain.StepCode]string{domain.StepValidateSlots: "這一輪新記下：品項＝高麗菜。"},
		},
	})

	_, err := runFunctionNode[CropGateOutput](t, node, ctx, ValidateSlotsOutput{Slots: domain.Slots{Item: item("高麗菜")}})
	require.NoError(t, err)

	progress, err := readGraphTurnProgress(ctx)
	require.NoError(t, err)
	require.Equal(t, "這一輪新記下：品項＝高麗菜。", progress.NarrationSummaries[domain.StepValidateSlots])
}
