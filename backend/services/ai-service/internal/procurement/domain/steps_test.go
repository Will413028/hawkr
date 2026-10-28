package domain

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

func TestStepLabels_CoversAllStepCodes(t *testing.T) {
	allCodes := []StepCode{
		StepUnderstandInput, StepExtractSlots, StepValidateSlots, StepCropGate, StepSlotsRejected,
		StepPickNextQuestion, StepRequirementReady, StepSearchCandidates, StepSearchPlatform,
		StepSearchExternal, StepReadSeller, StepCompare, StepVerifyOrigin,
		StepCheckStock, StepRankCandidates, StepSummarize, StepDegradedFallback,
	}
	require.Len(t, allCodes, 17, "白名單應為 17 碼——這個數字本身也是防呆：新增/刪除 code 卻忘記更新這份列表會在這裡先炸")
	require.Len(t, StepLabels, len(allCodes), "StepLabels 不得有這份列表之外的成員——否則刪 code 忘了刪 label 不會被上面那條抓到")
	for _, code := range allCodes {
		require.NotEmpty(t, StepLabels[code], "StepCode %q 缺少對應的 label", code)
	}
}

func TestComputeTurnSteps(t *testing.T) {
	timings := TurnStepTimings{
		UnderstandMs: 12, ExtractMs: 3400, ValidateMs: 1, DecideAndComposeMs: 8200,
	}

	tests := []struct {
		name                         string
		hasRejected, ready, degraded bool
		wantCodes                    []StepCode
	}{
		{"clarifying, no rejects", false, false, false,
			[]StepCode{StepUnderstandInput, StepExtractSlots, StepValidateSlots, StepPickNextQuestion}},
		{"ready, no rejects", false, true, false,
			[]StepCode{StepUnderstandInput, StepExtractSlots, StepValidateSlots, StepRequirementReady}},
		{"clarifying, has rejects", true, false, false,
			[]StepCode{StepUnderstandInput, StepExtractSlots, StepValidateSlots, StepSlotsRejected, StepPickNextQuestion}},
		{"ready, has rejects", true, true, false,
			[]StepCode{StepUnderstandInput, StepExtractSlots, StepValidateSlots, StepSlotsRejected, StepRequirementReady}},
		// degraded=true 時上游恆為 hasRejected=false（見本檔 Global Constraints）——
		// 以下兩案例不測 hasRejected=true 的組合，因為 Handle() 永遠不會產生它。
		{"degraded, clarifying", false, false, true,
			[]StepCode{StepUnderstandInput, StepExtractSlots, StepDegradedFallback, StepValidateSlots, StepPickNextQuestion}},
		{"degraded, already ready", false, true, true,
			[]StepCode{StepUnderstandInput, StepExtractSlots, StepDegradedFallback, StepValidateSlots, StepRequirementReady}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			steps := ComputeTurnSteps(timings, Slots{}, tt.hasRejected, tt.ready, tt.degraded, false, nil)

			gotCodes := make([]StepCode, len(steps))
			for i, s := range steps {
				gotCodes[i] = s.Code
				require.Equal(t, codemeta.ProcurementStepStatusSucceeded.Value, s.Status)
				require.Equal(t, StepLabels[s.Code], s.Label, "label 必須來自固定字串表")
			}
			require.Equal(t, tt.wantCodes, gotCodes)
		})
	}

	t.Run("duration values pass through from timings for timed steps", func(t *testing.T) {
		steps := ComputeTurnSteps(timings, Slots{}, false, false, false, false, nil)
		byCode := make(map[StepCode]int64, len(steps))
		for _, s := range steps {
			byCode[s.Code] = s.DurationMs
		}
		require.Equal(t, timings.UnderstandMs, byCode[StepUnderstandInput])
		require.Equal(t, timings.ExtractMs, byCode[StepExtractSlots])
		require.Equal(t, timings.ValidateMs, byCode[StepValidateSlots])
		require.Equal(t, timings.DecideAndComposeMs, byCode[StepPickNextQuestion])
	})

	t.Run("flag steps report zero duration, not a fabricated number", func(t *testing.T) {
		steps := ComputeTurnSteps(timings, Slots{}, true, false, true, false, nil)
		for _, s := range steps {
			if s.Code == StepSlotsRejected || s.Code == StepDegradedFallback {
				require.Zero(t, s.DurationMs, "%s 是零耗時的標記 step", s.Code)
			}
		}
	})
}

func TestComputeTurnSteps_SafeArgs(t *testing.T) {
	timings := TurnStepTimings{UnderstandMs: 1, ExtractMs: 1, ValidateMs: 1, DecideAndComposeMs: 1}
	item := "香蕉"
	quantity := 20
	city := "台北市"

	t.Run("no slots known yet: no code carries safeArgs", func(t *testing.T) {
		steps := ComputeTurnSteps(timings, Slots{}, false, false, false, false, nil)
		for _, s := range steps {
			require.Nil(t, s.SafeArgs, "%s 不該有空 slots 生出的 safeArgs", s.Code)
		}
	})

	t.Run("known slots only reach EXTRACT_SLOTS and the terminal code, never VALIDATE_SLOTS", func(t *testing.T) {
		slots := Slots{Item: &item, Quantity: &quantity, DeliveryCity: &city}
		steps := ComputeTurnSteps(timings, slots, false, true, false, false, nil)
		byCode := make(map[StepCode]map[string]string, len(steps))
		for _, s := range steps {
			byCode[s.Code] = s.SafeArgs
		}
		want := map[string]string{"item": "香蕉", "quantity": "20", "city": "台北市"}
		require.Equal(t, want, byCode[StepExtractSlots])
		require.Equal(t, want, byCode[StepRequirementReady])
		require.Nil(t, byCode[StepValidateSlots], "VALIDATE_SLOTS 沒有對應的動態文案模板，不該帶 safeArgs")
	})

	t.Run("only present fields are included, absent slots leave no key", func(t *testing.T) {
		slots := Slots{Item: &item}
		steps := ComputeTurnSteps(timings, slots, false, false, false, false, nil)
		byCode := make(map[StepCode]map[string]string, len(steps))
		for _, s := range steps {
			byCode[s.Code] = s.SafeArgs
		}
		require.Equal(t, map[string]string{"item": "香蕉"}, byCode[StepExtractSlots])
		require.NotContains(t, byCode[StepExtractSlots], "quantity")
		require.NotContains(t, byCode[StepExtractSlots], "city")
	})
}

// TestComputeTurnSteps_NarrationLandsOnMatchingCodes 守住這次改動的核心：
// LLM 敘述必須填到對應 code 的 Step.Text，沒有敘述的步驟維持空字串（消費端靠
// 空值決定要不要退回 Label）。
func TestComputeTurnSteps_NarrationLandsOnMatchingCodes(t *testing.T) {
	narration := StepNarration{
		StepExtractSlots:  "正在讀你的需求，抓品項",
		StepValidateSlots: "甘藍條件有效，無拒絕欄位。",
	}
	steps := ComputeTurnSteps(TurnStepTimings{}, Slots{}, false, false, false, false, narration)

	byCode := map[StepCode]Step{}
	for _, s := range steps {
		byCode[s.Code] = s
	}
	if got := byCode[StepExtractSlots].Text; got != "正在讀你的需求，抓品項" {
		t.Fatalf("EXTRACT_SLOTS.Text = %q，敘述沒有填進去", got)
	}
	if got := byCode[StepValidateSlots].Text; got != "甘藍條件有效，無拒絕欄位。" {
		t.Fatalf("VALIDATE_SLOTS.Text = %q", got)
	}
	// 沒有敘述的步驟不得被填任何東西——尤其不得回填 Label，那會讓消費端分不出
	// 「這是 LLM 講的」還是「這是固定文案」。
	if got := byCode[StepUnderstandInput].Text; got != "" {
		t.Fatalf("UNDERSTAND_INPUT 沒有敘述，Text 應為空，實得 %q", got)
	}
	if byCode[StepUnderstandInput].Label == "" {
		t.Fatal("Label 仍必須有值——Text 是額外資訊，不是取代 Label")
	}
}

// TestComputeAnswerSteps_NilNarration_LeavesTextEmpty 確認沒接 narrator 的路徑
// 行為完全不變（Text 全空），這是這次改動對既有消費端向後相容的保證。
func TestComputeAnswerSteps_NilNarration_LeavesTextEmpty(t *testing.T) {
	for _, s := range ComputeAnswerSteps(0, 0, Slots{}, false, false, false, nil) {
		if s.Text != "" {
			t.Fatalf("%s.Text = %q，nil narration 不該產生任何文字", s.Code, s.Text)
		}
	}
}

func codesOf(steps []Step) []StepCode {
	out := make([]StepCode, 0, len(steps))
	for _, s := range steps {
		out = append(out, s.Code)
	}
	return out
}

// TestComputeSteps_CropGateEmittedOnlyWhenEvaluated 釘住這次改動的核心：
// CROP_GATE 這一碼在 gate 真的做過判定時才出現，且位置對齊真正的執行順序
// （validate_slots 之後、SLOTS_REJECTED 與末碼之前）。
//
// 這一碼以前完全不會被任何 Compute*Steps 產生——crop_gate node／answer.go／
// rewind.go 三處算好的敘述因此每輪都被 applyStepNarration 丟掉。
func TestComputeSteps_CropGateEmittedOnlyWhenEvaluated(t *testing.T) {
	t.Run("turn：未判定時不出現", func(t *testing.T) {
		require.NotContains(t,
			codesOf(ComputeTurnSteps(TurnStepTimings{}, Slots{}, false, false, false, false, nil)),
			StepCropGate)
	})

	t.Run("turn：判定後出現在 VALIDATE_SLOTS 之後、SLOTS_REJECTED 之前", func(t *testing.T) {
		require.Equal(t,
			[]StepCode{StepUnderstandInput, StepExtractSlots, StepValidateSlots, StepCropGate, StepSlotsRejected, StepPickNextQuestion},
			codesOf(ComputeTurnSteps(TurnStepTimings{}, Slots{}, true, false, false, true, nil)))
	})

	t.Run("answer：判定後出現在 VALIDATE_SLOTS 之後", func(t *testing.T) {
		require.Equal(t,
			[]StepCode{StepValidateSlots, StepCropGate, StepRequirementReady},
			codesOf(ComputeAnswerSteps(0, 0, Slots{}, false, true, true, nil)))
	})

	t.Run("answer：未判定時維持原本兩碼", func(t *testing.T) {
		require.Equal(t,
			[]StepCode{StepValidateSlots, StepRequirementReady},
			codesOf(ComputeAnswerSteps(0, 0, Slots{}, false, true, false, nil)))
	})

	t.Run("rewind：判定後 CROP_GATE 排在末碼之前", func(t *testing.T) {
		require.Equal(t,
			[]StepCode{StepCropGate, StepPickNextQuestion},
			codesOf(ComputeRewindSteps(0, Slots{}, false, true, nil)))
	})

	t.Run("rewind：未判定時仍然只有末碼一個", func(t *testing.T) {
		require.Equal(t,
			[]StepCode{StepPickNextQuestion},
			codesOf(ComputeRewindSteps(0, Slots{}, false, false, nil)))
	})
}

// TestComputeSteps_CropGateNarrationLands 是上面那支的另一半，也是這次修的
// 真正病灶：以前 narration 帶著 CROP_GATE 的句子進來，卻沒有對應的 step 可以
// 填，整句被靜默丟棄。三條路徑都必須把它填進去。
func TestComputeSteps_CropGateNarrationLands(t *testing.T) {
	const want = "品項「高麗菜」的分類判斷已完成。"
	narration := StepNarration{StepCropGate: want}

	find := func(t *testing.T, steps []Step) string {
		t.Helper()
		for _, s := range steps {
			if s.Code == StepCropGate {
				return s.Text
			}
		}
		t.Fatal("步驟清單裡沒有 CROP_GATE，敘述無處可填")
		return ""
	}

	require.Equal(t, want, find(t, ComputeTurnSteps(TurnStepTimings{}, Slots{}, false, false, false, true, narration)))
	require.Equal(t, want, find(t, ComputeAnswerSteps(0, 0, Slots{}, false, false, true, narration)))
	require.Equal(t, want, find(t, ComputeRewindSteps(0, Slots{}, false, true, narration)))
}

// TestCandidateSearchRailPlan_ShapeAndNesting 釘住候選搜尋 rail 的靜態形狀：
// 順序即顯示順序，兩條 lane 掛在父列底下，其餘皆 top-level。
func TestCandidateSearchRailPlan_ShapeAndNesting(t *testing.T) {
	plan := CandidateSearchRailPlan()

	require.Equal(t, []RailRow{
		{Code: StepSearchCandidates},
		{Code: StepSearchPlatform, Parent: StepSearchCandidates},
		{Code: StepSearchExternal, Parent: StepSearchCandidates},
		{Code: StepSummarize},
	}, plan)

	for _, row := range plan {
		require.NotEmpty(t, StepLabels[row.Code], "rail 上的每一列都必須有 server 端固定 label：%q", row.Code)
		if row.Parent != "" {
			require.NotEmpty(t, StepLabels[row.Parent], "父列也是 rail 上的一列，同樣要有 label")
		}
	}
}

// TestCandidateSearchRailPlan_OmitsStepsWithNoRealWork 是本輪最重要的一條防線：
// usecase.CandidateSearchUseCase.run 全部的工作是 claim → 兩條 lane 併發 →
// finalize，這 5 碼命名的工作在碼上完全不存在（沒有逐家讀賣家、規格比對、
// 履歷／產地／供應量查核；排序是在各 lane 的 Search 內部做完的，不是跨 lane
// 的獨立階段）。把它們排進 rail 等於替買家編造進度——doc57 §1.1 evidence
// contract 擋的正是這件事。
//
// 這條測試是「有人照著舊計畫把 9 碼全接上」時唯一會轉紅的地方。要放行其中
// 任何一碼，前提是那一步真的被執行且可觀測，而不是為了讓 rail 看起來完整。
func TestCandidateSearchRailPlan_OmitsStepsWithNoRealWork(t *testing.T) {
	inPlan := map[StepCode]bool{}
	for _, row := range CandidateSearchRailPlan() {
		inPlan[row.Code] = true
	}
	for _, code := range []StepCode{
		StepReadSeller, StepCompare,
		StepVerifyOrigin, StepCheckStock, StepRankCandidates,
	} {
		require.False(t, inPlan[code],
			"%q 在 candidate_search.go 沒有對應的實際工作，不得排進 rail", code)
	}
}

// TestStepCodes_NoFailedCode 釘住 T23 的裁決：失敗住 status 軸
// （codemeta.ProcurementStepStatusFailed），不住 code 軸。這裡用字面值比對而
// 不是引用一個常數——常數已經被刪掉了，這條測試擋的正是「有人把它加回來」。
func TestStepCodes_NoFailedCode(t *testing.T) {
	require.Empty(t, StepLabels[StepCode("FAILED")],
		"FAILED 不得回到 code 軸：失敗是 ProcurementStepStatus 的一個值")
	require.Equal(t, "FAILED", codemeta.ProcurementStepStatusFailed.Value,
		"status 軸必須真的有 FAILED 這個值，否則上面那條等於把失敗語意整個弄丟")
}
