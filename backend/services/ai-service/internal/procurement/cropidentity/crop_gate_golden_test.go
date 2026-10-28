// services/ai-service/internal/procurement/cropidentity/crop_gate_golden_test.go
package cropidentity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/testkit"
	"github.com/stretchr/testify/require"
)

// updateCropGateGolden 重新產生 testdata/crop_gate 下的 fixture，用法同
// domain/artifact_golden_test.go 的 -update-artifact-golden：
// go test -run TestComputeCropGate_GoldenFixtures -update-crop-gate-golden
var updateCropGateGolden = flag.Bool("update-crop-gate-golden", false, "重新產生 testdata/crop_gate 下的 fixture")

// cropGateGoldenResult 把 GateResult 攤平成可序列化的形狀——evaluated 是
// GateResult 的未匯出欄位，本檔與 crop_gate.go 同一個 package（cropidentity），
// 可以直接讀，但 encoding/json 不會匯出它，所以另外包一層。
//
// FactsAsOf 只凍「有沒有值」不凍值本身：它是 time.Now()，凍住真值會讓 fixture
// 每次重跑都不同。有沒有值才是這裡真正要釘的判定（gate 有沒有真的查過分類樹）。
type cropGateGoldenResult struct {
	Item          *string                           `json:"item"`
	Pending       bool                              `json:"pending"`
	Candidates    []domain.CropDisambiguationOption `json:"candidates"`
	PendingReason domain.CropPendingReason          `json:"pendingReason"`
	Resolution    *domain.ItemResolution            `json:"resolution,omitempty"`
	Evaluated     bool                              `json:"evaluated"`
	FactsAsOfSet  bool                              `json:"factsAsOfSet"`
}

// TestComputeCropGate_GoldenFixtures 是 T4（見 domain 套件
// dialogue_planner_golden_test.go 開頭 doc comment，理由同）的第三部分。
// ComputeGate 不是純函式——依賴注入的 Resolver 會查分類樹——所以
// fixture 除了 (before, after) 兩個輸入，還要把 resolver 這次回傳的
// canned 值也一併凍住，重播時才不需要真的接一個分類樹替身。
//
// 逐一對應 crop_gate_test.go 裡「測 ComputeGate 本身」的每一支測試。
//
// 刀 2 起 gate 不再自己落庫（待答狀態隨中斷一起 INSERT），所以 fixture 凍的是
// 「這一輪判定成什麼」而不是「要寫什麼進 request」；四個以 snapshot 餵入舊持久化
// 候選的 case 也一併移除——那條跨輪讀取路徑已經沒有生產者（見 ComputeGate
// 的說明）。
func TestComputeCropGate_GoldenFixtures(t *testing.T) {
	cases := []struct {
		name          string
		resolverNil   bool
		resolveResult DisambiguationResult
		resolveErr    error
		before        domain.Slots
		after         domain.Slots
	}{
		{
			// 已經有解析結果的品項＝真正的 no-op：不重查分類樹。
			name: "item_unchanged_already_resolved_no_op",
			before: domain.Slots{Item: item("芭樂"),
				ItemResolution: domain.NewCategoryConfirmedItemResolution(&domain.CropSelection{CategoryID: "7", Label: "芭樂"})},
			after: domain.Slots{Item: item("芭樂"),
				ItemResolution: domain.NewCategoryConfirmedItemResolution(&domain.CropSelection{CategoryID: "7", Label: "芭樂"})},
		},
		{
			// 品項沒變、但還沒有解析結果——上一輪失敗過（失敗會清掉既有解析），
			// 這一輪要重試。原本這個 case 叫 item_unchanged_no_op，而那個 no-op
			// 正是「依賴恢復後永遠不會重試」的成因。
			name:   "item_unchanged_unresolved_retries",
			before: domain.Slots{Item: item("芭樂")},
			after:  domain.Slots{Item: item("芭樂")},
		},
		{
			// rewind 把 item 一路退回 nil：跟「item 沒變動」走同一個分支，兩者
			// 現在的處置相同（都沒有東西要重新解析）。
			name:   "item_rewound_to_nil_no_op",
			before: domain.Slots{Item: item("小白菜")},
			after:  domain.Slots{Item: nil},
		},
		{
			name:          "unavailable_marks_pending_with_no_candidates",
			resolveResult: DisambiguationResult{Unavailable: true},
			before:        domain.Slots{Item: item("舊品項")},
			after:         domain.Slots{Item: item("查無此物")},
		},
		{
			name: "tree_confirmed_category_passes_with_resolution",
			resolveResult: DisambiguationResult{Unavailable: true, Selection: &domain.CropSelection{
				Stage: domain.CropSelectionDescribe, CategoryID: "24", Label: "五花肉", ParentCategoryID: "8", ParentLabel: "豬肉",
			}},
			before: domain.Slots{Item: item("舊品項")},
			after:  domain.Slots{Item: item("五花肉")},
		},
		{
			name: "ambiguous_keeps_item_and_marks_pending",
			resolveResult: DisambiguationResult{
				NeedsDisambiguation: true,
				Candidates:          []domain.CropDisambiguationOption{{Label: "有機小白菜"}},
			},
			before: domain.Slots{},
			after:  domain.Slots{Item: item("小白菜")},
		},
		{
			name:       "resolver_error_degrades_without_blocking",
			resolveErr: errors.New("category tree unavailable"),
			before:     domain.Slots{},
			after:      domain.Slots{Item: item("芭樂")},
		},
		{
			name:        "nil_resolver_does_not_resolve",
			resolverNil: true,
			before:      domain.Slots{},
			after:       domain.Slots{Item: item("芭樂")},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var resolver Resolver
			if !tc.resolverNil {
				resolver = &testkit.FakeResolver[DisambiguationResult]{ResolveResult: tc.resolveResult, ResolveErr: tc.resolveErr}
			}

			g := ComputeGate(context.Background(), resolver, tc.before, tc.after)

			resultOut := cropGateGoldenResult{
				Item: g.Item, Pending: g.Pending,
				Candidates: g.Candidates, PendingReason: g.PendingReason, Resolution: g.Resolution,
				Evaluated: g.evaluated, FactsAsOfSet: !g.FactsAsOf.IsZero(),
			}

			fixture := struct {
				ResolverNil   bool                 `json:"resolverNil"`
				ResolveResult DisambiguationResult `json:"resolveResult"`
				ResolveErr    string               `json:"resolveErr,omitempty"`
				Before        domain.Slots         `json:"before"`
				After         domain.Slots         `json:"after"`
				Result        cropGateGoldenResult `json:"result"`
			}{
				ResolverNil: tc.resolverNil, ResolveResult: tc.resolveResult,
				Before: tc.before, After: tc.after, Result: resultOut,
			}
			if tc.resolveErr != nil {
				fixture.ResolveErr = tc.resolveErr.Error()
			}

			wire, err := json.MarshalIndent(fixture, "", "  ")
			require.NoError(t, err)
			wire = append(wire, '\n')

			path := filepath.Join("testdata", "crop_gate", tc.name+".json")

			if *updateCropGateGolden {
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
				require.NoError(t, os.WriteFile(path, wire, 0o644))
				return
			}

			want, err := os.ReadFile(path)
			require.NoErrorf(t, err, "讀不到 fixture（第一次請跑 go test -run TestComputeCropGate_GoldenFixtures -update-crop-gate-golden）")
			require.Truef(t, bytes.Equal(want, wire),
				"ComputeGate golden fixture 不符。若是預期的行為變更，跑 -update-crop-gate-golden 重新產生。\ngot:  %s\nwant: %s",
				wire, want)
		})
	}
}

func ptrStr(v string) *string { return &v }
