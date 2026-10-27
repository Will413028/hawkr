package usecase

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/adk/v2/session"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/goldenset"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/testkit"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// 多輪情境的離線 replay。
//
// ⛔ 走的是**真的** graph：extract→validate→crop gate→terminal，真的 Slots.Apply、
// 真的 domain 規則。只有模型與外部依賴換成替身。把 production 的組裝複製一份到
// 測試裡，驗到的就只是那份複製品。
//
// ⛔ 離線綠只證明接線：stub_patch 把抽取結果寫死了，所以這裡完全沒有驗到「模型看
// 得懂買家的話」。那是 real-model eval 的事。

// scenarioFixture 是具名的 replay 前提。
// ⛔ 不以今天日期動態推算期待值——corpus 的期望要能在任何一天重跑都一樣。
type scenarioFixture struct {
	resolver cropidentity.Resolver
}

var scenarioFixtures = map[string]func() scenarioFixture{
	// guava-resolved：品項解析固定成芭樂的分類（待買家填單價），沒有既有試算、沒有任何發布。
	"guava-resolved": func() scenarioFixture {
		return scenarioFixture{
			resolver: &testkit.FakeResolver[cropidentity.DisambiguationResult]{
				ResolveResult: cropidentity.DisambiguationResult{
					Unavailable: true,
					Selection:   &domain.CropSelection{Stage: domain.CropSelectionPrice, CategoryID: "guava", Label: "芭樂"},
				},
			},
		}
	},
}

// scenarioExtractor 是模型的替身：每一輪回傳該輪 stub_patch 指定的抽取結果。
type scenarioExtractor struct {
	patches []domain.SlotPatch
	calls   int
}

func (s *scenarioExtractor) Extract(_ context.Context, _ string, _ domain.Slots) (ExtractResult, error) {
	i := s.calls
	s.calls++
	if i >= len(s.patches) {
		return ExtractResult{}, nil
	}
	// ShortIntent 固定：這一層不驗買家可見文案，那是 surface 那條路的事。
	return ExtractResult{Patch: s.patches[i], ShortIntent: "正在整理你的需求"}, nil
}

// slotPatchFromStub 把 YAML 的 stub_patch 轉成 production 的 SlotPatch 型別。
// ⛔ 經過真的 Apply，不直接當成期望值——expect.slots 比的是 Apply **之後**的結果。
func slotPatchFromStub(t *testing.T, stub map[string]any) domain.SlotPatch {
	t.Helper()
	var p domain.SlotPatch
	for k, v := range stub {
		s, _ := v.(string)
		switch k {
		case "item":
			p.Item = strPtr(s)
		case "unit":
			p.Unit = strPtr(s)
		case "delivery_city":
			p.DeliveryCity = strPtr(s)
		case "delivery_date":
			p.DeliveryDate = strPtr(s)
		case "packaging":
			p.Packaging = strPtr(s)
		case "quantity":
			n, ok := v.(int)
			require.True(t, ok, "quantity 必須是整數：%v", v)
			p.Quantity = &n
		case "budget_per_unit":
			n, ok := v.(int)
			require.True(t, ok, "budget_per_unit 必須是整數：%v", v)
			p.BudgetPerUnit = &n
		default:
			t.Fatalf("stub_patch 不認得的欄位 %q——corpus 打錯字不得被靜默忽略", k)
		}
	}
	return p
}

// slotsToMap 把 production 的 Slots 攤成比對用的 map。
// ⛔ ID 與代碼一律字串；數量維持數值由 comparator 正規化。
func slotsToMap(s domain.Slots) map[string]any {
	out := map[string]any{}
	if s.Item != nil {
		out["item"] = *s.Item
	}
	if s.Unit != nil {
		out["unit"] = *s.Unit
	}
	if s.DeliveryCity != nil {
		out["delivery_city"] = *s.DeliveryCity
	}
	if s.DeliveryDate != nil {
		out["delivery_date"] = *s.DeliveryDate
	}
	if s.Packaging != nil {
		out["packaging"] = *s.Packaging
	}
	if s.Quantity != nil {
		out["quantity"] = *s.Quantity
	}
	if s.BudgetPerUnit != nil {
		out["budget_per_unit"] = *s.BudgetPerUnit
	}
	if s.ItemResolution != nil {
		out["item_resolution_kind"] = string(s.ItemResolution.Kind)
	}
	return out
}

// publicationEvents 是真的觀察，不是「應該沒有」的推定：數 run event 裡帶
// publication 的那些。零筆代表**觀察到零筆**。
func publicationEvents(types []string) int {
	n := 0
	for _, t := range types {
		if strings.Contains(t, "PUBLICATION") {
			n++
		}
	}
	return n
}

func replayScenario(t *testing.T, spec goldenset.Scenario) []goldenset.TurnObservation {
	t.Helper()

	newFixture, ok := scenarioFixtures[spec.Fixture]
	require.True(t, ok, "%s：找不到具名 fixture %q", spec.ID, spec.Fixture)
	fx := newFixture()

	patches := make([]domain.SlotPatch, 0, len(spec.Turns))
	for _, turn := range spec.Turns {
		patches = append(patches, slotPatchFromStub(t, turn.StubPatch))
	}
	extractor := &scenarioExtractor{patches: patches}

	// ⛔ 每個 scenario 自己的 session／revision／stub queue，彼此不共用。
	runner, resumeRunner, err := NewProcurementGraphRunner(extractor, fx.resolver, seqIDs(9800),
		session.InMemoryService(), nil, &fakeStepNarrator{text: "narrated"}, nil)
	require.NoError(t, err)

	runStore := newFakeRunStore()
	repo := newFakeRepo()
	deps := GraphControllerDeps{
		RunStore: runStore, RevisionStore: repo, Dispatcher: NewRunDispatcher(),
		// ResumeRunner 一併接上：目前 replay 只跑 OpTurn 走不到它，但留白會讓
		// 之後補 resume op 的人拿到一個 nil runner，症狀是空 session 被當新一輪跑。
		Runner: runner, ResumeRunner: resumeRunner, CropResolver: fx.resolver,
	}

	ctx := context.Background()
	buyerID := identity.BuyerID(98000)
	var requestID int64
	out := make([]goldenset.TurnObservation, 0, len(spec.Turns))

	for i, turn := range spec.Turns {
		if turn.Op != goldenset.OpTurn {
			// 其餘 op 的 replay 尚未接上；明確記成 evaluation_error，不假裝跑過。
			out = append(out, goldenset.TurnObservation{
				ErrorCode: "replay_op_not_supported:" + turn.Op,
			})
			continue
		}

		got, err := ClaimAndDispatchTurnViaGraph(ctx, TurnInput{
			BuyerID: buyerID, RequestID: requestID, Message: turn.Message,
			ClientRequestID: spec.ID + "-turn-" + strconv.Itoa(i),
		}, deps, TurnStreamHooks{})
		if err != nil {
			out = append(out, goldenset.TurnObservation{ErrorCode: err.Error()})
			continue
		}
		requestID = got.Result.RequestID

		missing := make([]string, 0, len(got.Result.MissingRequired))
		for _, k := range got.Result.MissingRequired {
			missing = append(missing, string(k))
		}
		count := publicationEvents(runStore.eventTypes())
		out = append(out, goldenset.TurnObservation{
			Slots:            slotsToMap(got.Result.Slots),
			MissingRequired:  missing,
			PublicationCount: &count,
		})
	}
	return out
}

// TestScenarioReplay 跑完整份 corpus。
//
// known_gap 的 case 照樣跑、照樣記結果，但不讓它把這條測試弄紅——它有 owner，
// 會在被修好的那天自己變成 active。
func TestScenarioReplay(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "goldenset", "testdata", "scenarios.yaml"))
	require.NoError(t, err)
	scenarios, err := goldenset.LoadScenarios(raw)
	require.NoError(t, err)

	results := make([]goldenset.ScenarioResult, 0, len(scenarios))
	for _, spec := range scenarios {
		spec := spec
		t.Run(spec.ID, func(t *testing.T) {
			observations := replayScenario(t, spec)
			failures := goldenset.CompareScenario(spec, observations)

			results = append(results, goldenset.ScenarioResult{
				ScenarioID: spec.ID, Capability: spec.Capability, Status: spec.Status,
				TrackedBy: spec.TrackedBy,
				TurnsRun:  len(observations), TurnsTotal: len(spec.Turns),
				Failures: failures,
			})

			if spec.Status == goldenset.ScenarioStatusKnownGap {
				t.Logf("known_gap（%s）：%d 筆差異", spec.TrackedBy, len(failures))
				return
			}
			require.Empty(t, failures, "%s 有差異：%+v", spec.ID, failures)
		})
	}

	report := goldenset.ScenarioReport{
		Metadata: goldenset.ReportMetadata{
			SchemaVersion: goldenset.ReportSchemaVersion,
			Mode:          "offline",
			CorpusHash:    goldenset.CorpusHash(scenarios),
			Model:         goldenset.Unavailable,
			Provider:      goldenset.Unavailable,
		},
		Results: results,
		Summary: goldenset.Summarize(results),
	}
	blob, err := json.Marshal(report)
	require.NoError(t, err)
	t.Logf("scenario report: %s", blob)
}
