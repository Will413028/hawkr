package domain

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// updateSlotsApplyGolden 重新產生 testdata/slots_apply 下的 fixture，用法同
// artifact_golden_test.go 的 -update-artifact-golden：
// go test -run TestSlots_Apply_GoldenFixtures -update-slots-apply-golden
var updateSlotsApplyGolden = flag.Bool("update-slots-apply-golden", false, "重新產生 testdata/slots_apply 下的 fixture")

// TestSlots_Apply_GoldenFixtures 是 T4（見 dialogue_planner_golden_test.go 開頭
// doc comment，理由同）的一部分：Slots.Apply／CropGate 在 T8 會被重新接線成 ADK
// v2 workflow graph 的 FunctionNode（不像 PlanDialogue 整個退場，見 roadmap
// T8：「Slots.Apply／CropGate／price preview → FunctionNode；PlanDialogue →
// Route」），但接線方式改變本身就有回歸風險，所以同樣需要凍結現行決策當比對基準。
//
// 逐一對應 slots_test.go 裡「測 Apply 本身」的每一支測試（排除
// TestSlots_Hash_*／TestSlots_MissingRequired*——那些測的是 Slots 的其他方法，
// 不是 Apply，不在 T4 範圍）：釘住 (before, patch, now) → (after, rejected) 完整
// 一對。多輪 Apply 疊加的測試（PreservesUntouchedFields／
// PreservesUnitOnRejectionWhenNewUnitInvalid）在這裡把「前一輪」攤平成
// before 參數本身，只釘住被測的那一次呼叫——golden fixture 要能單獨重播，
// 不依賴呼叫順序。
func TestSlots_Apply_GoldenFixtures(t *testing.T) {
	preservesUntouchedBefore, _ := Slots{}.Apply(SlotPatch{Item: ptr("寶島梨禮盒"), Quantity: ptr(50)}, refNow)
	preservesUnitBefore, _ := Slots{}.Apply(SlotPatch{Unit: ptr("KG")}, refNow)

	longText := ""
	for i := 0; i < MaxTextLen+50; i++ {
		longText += "梨"
	}

	cases := []struct {
		name   string
		before Slots
		patch  SlotPatch
		now    time.Time
	}{
		{"accepts_valid_patch", Slots{}, SlotPatch{
			Item: ptr("寶島梨禮盒"), Quantity: ptr(50), Unit: ptr("KG"),
			BudgetPerUnit: ptr(800), DeliveryCity: ptr("桃園市"), DeliveryDate: ptr("2026-08-20"),
		}, refNow},
		{"drops_invalid_fields_but_keeps_valid_ones", Slots{}, SlotPatch{
			Item: ptr("寶島梨禮盒"), Quantity: ptr(0), Unit: ptr("箱子"),
			DeliveryCity: ptr("不存在市"), DeliveryDate: ptr("2026-08-01"),
		}, refNow},
		{"truncates_overlong_text", Slots{}, SlotPatch{Item: ptr(longText)}, refNow},
		{"rejects_date_too_far_out", Slots{}, SlotPatch{DeliveryDate: ptr("2028-01-01")}, refNow},
		{"preserves_untouched_fields", preservesUntouchedBefore, SlotPatch{Quantity: ptr(80)}, refNow},
		{"normalizes_city_alias", Slots{}, SlotPatch{DeliveryCity: ptr("桃園")}, refNow},
		{"ambiguous_city_stays_rejected", Slots{}, SlotPatch{DeliveryCity: ptr("新竹")}, refNow},
		{"normalizes_delivery_date_with_year", Slots{}, SlotPatch{DeliveryDate: ptr("2026/8/20")}, refNow},
		{"rejects_delivery_date_missing_year_with_clear_reason", Slots{}, SlotPatch{DeliveryDate: ptr("8/20")}, refNow},
		{"golden_request_from_flow_19", Slots{}, SlotPatch{
			Item: ptr("寶島梨禮盒"), Quantity: ptr(50), BudgetPerUnit: ptr(800),
			DeliveryCity: ptr("桃園"), DeliveryDate: ptr("8/20"),
		}, refNow},
		{"rejects_unit_not_in_allowed_units", Slots{}, SlotPatch{Unit: ptr("BOX")}, refNow},
		{"preserves_unit_on_rejection_when_new_unit_invalid", preservesUnitBefore, SlotPatch{Unit: ptr("BOX")}, refNow},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			after, rejected := tc.before.Apply(tc.patch, tc.now)

			fixture := struct {
				Before   Slots           `json:"before"`
				Patch    SlotPatch       `json:"patch"`
				Now      time.Time       `json:"now"`
				After    Slots           `json:"after"`
				Rejected []RejectedField `json:"rejected"`
			}{Before: tc.before, Patch: tc.patch, Now: tc.now, After: after, Rejected: rejected}

			wire, err := json.MarshalIndent(fixture, "", "  ")
			require.NoError(t, err)
			wire = append(wire, '\n')

			path := filepath.Join("testdata", "slots_apply", tc.name+".json")

			if *updateSlotsApplyGolden {
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
				require.NoError(t, os.WriteFile(path, wire, 0o644))
				return
			}

			want, err := os.ReadFile(path)
			require.NoErrorf(t, err, "讀不到 fixture（第一次請跑 go test -run TestSlots_Apply_GoldenFixtures -update-slots-apply-golden）")
			require.Truef(t, bytes.Equal(want, wire),
				"Slots.Apply golden fixture 不符。若是預期的行為變更，跑 -update-slots-apply-golden 重新產生。\ngot:  %s\nwant: %s",
				wire, want)
		})
	}
}
