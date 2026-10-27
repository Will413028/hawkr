package domain

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// updateDialoguePlanGolden 重新產生 testdata/dialogue_plan 下的 fixture，用法同
// artifact_golden_test.go 的 -update-artifact-golden：
// go test -run TestPlanDialogue_GoldenFixtures -update-dialogue-plan-golden
var updateDialoguePlanGolden = flag.Bool("update-dialogue-plan-golden", false, "重新產生 testdata/dialogue_plan 下的 fixture")

// TestPlanDialogue_GoldenFixtures 是 roadmap T4（2026-08-26 架構轉向）的「保命動作」：
// PlanDialogue 這個 deterministic pure-function planner 即將退場，控制流改交給模型
// 走 ADK v2 的 workflow graph engine。adk-go 目前零 eval framework，拆掉 PlanDialogue
// 的那一刻，「同一個情境下新舊控制流是否做出同樣決策」這件事就再也無法比對——除非
// 事先把它每一個既有單元測試情境的 (input, output) 原樣凍結下來。
//
// 這份 fixture 逐一對應 dialogue_planner_test.go 現有的每一支測試（除了
// TestPlanDialogue_PlannableActCodes_AllReachableViaSomeFixture，那支本身是「窮舉
// 既有 fixture 有沒有覆蓋全部 act code」的聚合測試，不是單一情境，不轉存）：同一個
// input 逐位元組釘住完整的 DialoguePlanV1 輸出，不只釘 ActCode——ReadinessGate／
// ClaimCodes／SalientFactRefs 等欄位往後同樣可能在新架構下悄悄跑掉。
//
// 這兩份既有測試檔（dialogue_planner_test.go／slots_test.go）本身完全不動：golden
// fixture 是新增的第二層安全網，不是既有單元測試的替代品。
func TestPlanDialogue_GoldenFixtures(t *testing.T) {
	cases := []struct {
		name  string
		input DialoguePlanInput
	}{
		{"missing_required_selects_ack_and_ask_required", DialoguePlanInput{
			Current: Slots{}, Next: Slots{Item: item("蘋果")},
			Missing: []SlotKey{SlotDeliveryCity, SlotUnit}, Ready: false,
		}},
		{"degraded_takes_priority_over_everything_else", DialoguePlanInput{
			Degraded: true, Ready: false, HasRejected: true,
		}},
		{"rejected_takes_priority_over_correction", DialoguePlanInput{
			HasRejected: true, Ready: true,
			Current: Slots{Item: item("蘋果")}, Next: Slots{Item: item("蘋果")},
			Recorded: []RecordedField{{Key: "item", Value: "蘋果"}},
		}},
		{"correction_of_existing_field_selects_correction_applied", DialoguePlanInput{
			Ready: true, Question: nil,
			Current:  Slots{Item: item("蘋果")},
			Next:     Slots{Item: item("富士蘋果")},
			Recorded: []RecordedField{{Key: "item", Value: "富士蘋果"}},
		}},
		{"first_time_fill_does_not_select_correction_applied", DialoguePlanInput{
			Ready: true, Question: nil,
			Current:  Slots{},
			Next:     Slots{Item: item("蘋果")},
			Recorded: []RecordedField{{Key: "item", Value: "蘋果"}},
		}},
		{"ready_with_follow_up_question_selects_ack_and_ask_optional", DialoguePlanInput{
			Ready: true, Question: &ClarifyQuestion{SlotKey: SlotQuantity},
		}},
		{"ready_no_follow_up_selects_request_basics_ready", DialoguePlanInput{
			Ready: true, Question: nil,
		}},
		{"price_preview_available_selects_price_preview_available", DialoguePlanInput{
			Ready: true, Question: nil, PricePreviewAvailable: true,
		}},
		{"rfq_stale_selects_rfq_reconfirm_required", DialoguePlanInput{
			Ready: true, PricePreviewAvailable: true, RfqConfirmationState: RfqStale,
		}},
		{"rfq_confirmed_selects_publication_approval_required_not_rfq_confirmed", DialoguePlanInput{
			Ready: true, PricePreviewAvailable: true, RfqConfirmationState: RfqConfirmed,
		}},
		{"correction_takes_priority_over_not_ready", DialoguePlanInput{
			Ready: false, Missing: []SlotKey{SlotDeliveryCity},
			Current:  Slots{Item: item("蘋果")},
			Next:     Slots{Item: item("富士蘋果")},
			Recorded: []RecordedField{{Key: "item", Value: "富士蘋果"}},
		}},
		{"question_takes_priority_over_price_preview_available", DialoguePlanInput{
			Ready: true, PricePreviewAvailable: true,
			Question: &ClarifyQuestion{SlotKey: SlotQuantity},
		}},
		{"crop_disambiguation_pending_takes_priority_over_not_ready", DialoguePlanInput{
			Ready:                        false,
			CropDisambiguationPending:    true,
			CropDisambiguationCandidates: []CropDisambiguationOption{{Label: "小白菜"}},
		}},
		{"has_rejected_takes_priority_over_crop_disambiguation_pending", DialoguePlanInput{
			HasRejected: true, CropDisambiguationPending: true,
		}},
		{"claim_codes_item_needs_disambiguation", DialoguePlanInput{
			CropDisambiguationPending: true,
		}},
		{"claim_codes_per_act_ack_and_ask_required", DialoguePlanInput{
			Ready: false,
		}},
		{"claim_codes_per_act_request_basics_ready_forbids_search_started", DialoguePlanInput{
			Ready: true, PricePreviewAvailable: false,
		}},
		{"claim_codes_per_act_publication_approval_required_forbids_rfq_published", DialoguePlanInput{
			Ready: true, PricePreviewAvailable: true, RfqConfirmationState: RfqConfirmed,
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			output := PlanDialogue(tc.input)

			fixture := struct {
				Input  DialoguePlanInput `json:"input"`
				Output DialoguePlanV1    `json:"output"`
			}{Input: tc.input, Output: output}

			wire, err := json.MarshalIndent(fixture, "", "  ")
			require.NoError(t, err)
			wire = append(wire, '\n')

			path := filepath.Join("testdata", "dialogue_plan", tc.name+".json")

			if *updateDialoguePlanGolden {
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
				require.NoError(t, os.WriteFile(path, wire, 0o644))
				return
			}

			want, err := os.ReadFile(path)
			require.NoErrorf(t, err, "讀不到 fixture（第一次請跑 go test -run TestPlanDialogue_GoldenFixtures -update-dialogue-plan-golden）")
			require.Truef(t, bytes.Equal(want, wire),
				"PlanDialogue golden fixture 不符。若是預期的決策變更，跑 -update-dialogue-plan-golden 重新產生。\ngot:  %s\nwant: %s",
				wire, want)
		})
	}
}
