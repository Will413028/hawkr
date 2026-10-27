package goldenset

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func loadCorpus(t *testing.T) []Scenario {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "scenarios.yaml"))
	require.NoError(t, err)
	got, err := LoadScenarios(raw)
	require.NoError(t, err)
	return got
}

func TestLoadScenarios_Corpus(t *testing.T) {
	got := loadCorpus(t)
	require.NotEmpty(t, got)

	for _, s := range got {
		require.NoError(t, ValidateScenario(s), "corpus 自己必須先合法：%s", s.ID)
		if s.Status == ScenarioStatusKnownGap {
			require.NotEmpty(t, s.TrackedBy, "%s：known_gap 必須有 owner", s.ID)
		}
	}
}

// TestLoadScenarios_RejectsMalformed：loader 嚴格是刻意的。打錯一個 key 就被靜默
// 忽略的 loader，會讓人以為某條關係有被驗到，實際上那一整段從來沒跑過。
func TestLoadScenarios_RejectsMalformed(t *testing.T) {
	valid := `
schema_version: 1
scenarios:
  - id: a
    capability: supported
    fixture: f
    frozen_at: "2026-09-19T00:00:00Z"
    status: active
    turns:
      - op: turn
        message: "我要芭樂"
`
	_, err := LoadScenarios([]byte(valid))
	require.NoError(t, err, "正控制：合法輸入必須載得進來")

	cases := map[string]string{
		"未知的 schema_version": `
schema_version: 2
scenarios:
  - {id: a, capability: supported, fixture: f, frozen_at: "t", status: active, turns: [{op: turn, message: m}]}
`,
		"未知的 key": `
schema_version: 1
scenarios:
  - {id: a, capability: supported, fixture: f, frozen_at: "t", status: active, typo_key: x, turns: [{op: turn, message: m}]}
`,
		"重複的 id": `
schema_version: 1
scenarios:
  - {id: a, capability: supported, fixture: f, frozen_at: "t", status: active, turns: [{op: turn, message: m}]}
  - {id: a, capability: supported, fixture: f, frozen_at: "t", status: active, turns: [{op: turn, message: m}]}
`,
		"未知的 op": `
schema_version: 1
scenarios:
  - {id: a, capability: supported, fixture: f, frozen_at: "t", status: active, turns: [{op: teleport}]}
`,
		"known_gap 沒有 tracked_by": `
schema_version: 1
scenarios:
  - {id: a, capability: not_modeled, fixture: f, frozen_at: "t", status: known_gap, turns: [{op: turn, message: m}]}
`,
		"未知的 capability": `
schema_version: 1
scenarios:
  - {id: a, capability: someday, fixture: f, frozen_at: "t", status: active, turns: [{op: turn, message: m}]}
`,
		"turn 沒有 message": `
schema_version: 1
scenarios:
  - {id: a, capability: supported, fixture: f, frozen_at: "t", status: active, turns: [{op: turn}]}
`,
		"answer 沒有 slot_key": `
schema_version: 1
scenarios:
  - {id: a, capability: supported, fixture: f, frozen_at: "t", status: active, turns: [{op: answer}]}
`,
		"空的 scenarios": "schema_version: 1\nscenarios: []\n",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := LoadScenarios([]byte(raw))
			require.Error(t, err)
		})
	}
}

func observationFor(slots map[string]any, missing []string, next string) TurnObservation {
	zero := 0
	return TurnObservation{
		Slots: slots, MissingRequired: missing, NextAction: next, PublicationCount: &zero,
	}
}

// TestCompareScenario_SingleMutationFails 是 corpus 鑑別力的核心：fixture 只要有
// **一個**明確變異就必須失敗，而且指得出是哪一個 case／turn／field。
// 全部一起變的測試無法分辨「四個都驗到了」與「只有一個驗到、其餘順帶」。
func TestCompareScenario_SingleMutationFails(t *testing.T) {
	spec := Scenario{
		ID: "s", Capability: CapabilitySupported, Fixture: "f",
		FrozenAt: "t", Status: ScenarioStatusActive,
		Turns: []ScenarioTurn{{
			Op: OpTurn, Message: "我要芭樂",
			Expect: ScenarioExpectation{
				Slots:           map[string]any{"item": "芭樂", "delivery_city": "桃園市"},
				MissingRequired: &[]string{},
				NextAction:      "REVIEW_PRICE_PREVIEW",
				NoPublication:   true,
			},
		}},
	}
	pass := observationFor(map[string]any{"item": "芭樂", "delivery_city": "桃園市"}, nil, "REVIEW_PRICE_PREVIEW")
	require.Empty(t, CompareScenario(spec, []TurnObservation{pass}), "正控制：完全相符不得回報差異")

	mutations := map[string]struct {
		obs   TurnObservation
		field string
	}{
		"錯城市":          {observationFor(map[string]any{"item": "芭樂", "delivery_city": "新北市"}, nil, "REVIEW_PRICE_PREVIEW"), "slots.delivery_city"},
		"丟欄位":          {observationFor(map[string]any{"item": "芭樂"}, nil, "REVIEW_PRICE_PREVIEW"), "slots.delivery_city"},
		"錯 nextAction": {observationFor(map[string]any{"item": "芭樂", "delivery_city": "桃園市"}, nil, "ASK_NEXT_QUESTION"), "next_action"},
		"多了缺漏欄位":       {observationFor(map[string]any{"item": "芭樂", "delivery_city": "桃園市"}, []string{"unit"}, "REVIEW_PRICE_PREVIEW"), "missing_required"},
	}
	for name, tc := range mutations {
		t.Run(name, func(t *testing.T) {
			got := CompareScenario(spec, []TurnObservation{tc.obs})
			require.Len(t, got, 1, "一個變異應該只指出一個欄位：%+v", got)
			require.Equal(t, tc.field, got[0].Field)
			require.Equal(t, 0, got[0].Turn)
			require.Equal(t, "s", got[0].ScenarioID)
		})
	}
}

// TestCompareScenario_RepeatedPublicationIsCaught：重複發布是這條 corpus 最貴的
// 回歸，必須指名到 publication_count。
func TestCompareScenario_RepeatedPublicationIsCaught(t *testing.T) {
	spec := Scenario{ID: "s", Capability: CapabilitySupported, Fixture: "f", FrozenAt: "t",
		Status: ScenarioStatusActive,
		Turns:  []ScenarioTurn{{Op: OpTurn, Message: "m", Expect: ScenarioExpectation{NoPublication: true}}}}

	two := 2
	got := CompareScenario(spec, []TurnObservation{{PublicationCount: &two}})
	require.Len(t, got, 1)
	require.Equal(t, "publication_count", got[0].Field)
	require.Equal(t, CategoryStateTransition, got[0].Category)
}

// TestCompareScenario_UnobservedIsNotZero：沒觀察到副作用 ≠ 沒有副作用。
// 這是這類測試最容易說謊的地方，所以它要回 evaluation_error 而不是靜默通過。
func TestCompareScenario_UnobservedIsNotZero(t *testing.T) {
	spec := Scenario{ID: "s", Capability: CapabilitySupported, Fixture: "f", FrozenAt: "t",
		Status: ScenarioStatusActive,
		Turns:  []ScenarioTurn{{Op: OpTurn, Message: "m", Expect: ScenarioExpectation{NoPublication: true}}}}

	got := CompareScenario(spec, []TurnObservation{{PublicationCount: nil}})
	require.Len(t, got, 1)
	require.Equal(t, CategoryEvaluationError, got[0].Category)
}

// TestCompareScenario_UnspecifiedIsNotAssertion：沒寫的欄位不斷言；明寫成空才要求為空。
func TestCompareScenario_UnspecifiedIsNotAssertion(t *testing.T) {
	unspecified := Scenario{ID: "s", Capability: CapabilitySupported, Fixture: "f", FrozenAt: "t",
		Status: ScenarioStatusActive,
		Turns:  []ScenarioTurn{{Op: OpTurn, Message: "m"}}}
	require.Empty(t, CompareScenario(unspecified, []TurnObservation{
		observationFor(map[string]any{"item": "芭樂"}, []string{"unit"}, "ASK_NEXT_QUESTION"),
	}), "沒指定的欄位不得被當成「必須為空」")

	explicitEmpty := unspecified
	explicitEmpty.Turns = []ScenarioTurn{{Op: OpTurn, Message: "m",
		Expect: ScenarioExpectation{MissingRequired: &[]string{}}}}
	got := CompareScenario(explicitEmpty, []TurnObservation{
		observationFor(map[string]any{}, []string{"unit"}, ""),
	})
	require.Len(t, got, 1)
	require.Equal(t, "missing_required", got[0].Field)
}

// TestCompareScenario_NumericNormalization：YAML 的 int 與 JSON 往返後的 float64
// 是同一個值，不得報成 regression；ID 一律當字串，不轉數值（精度）。
func TestCompareScenario_NumericNormalization(t *testing.T) {
	spec := Scenario{ID: "s", Capability: CapabilitySupported, Fixture: "f", FrozenAt: "t",
		Status: ScenarioStatusActive,
		Turns: []ScenarioTurn{{Op: OpTurn, Message: "m", Expect: ScenarioExpectation{
			Slots: map[string]any{"quantity": 50},
		}}}}

	require.Empty(t, CompareScenario(spec, []TurnObservation{
		{Slots: map[string]any{"quantity": float64(50)}},
	}), "int 與 float64 的 50 是同一個值")

	got := CompareScenario(spec, []TurnObservation{{Slots: map[string]any{"quantity": float64(51)}}})
	require.Len(t, got, 1, "真的不同仍然要抓到")
}
