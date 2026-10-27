// services/ai-service/internal/procurement/domain/dialogue_templates_golden_test.go
package domain

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestRenderPlan_GoldenFixtures_AllPlannableActs 逐字釘住 domain.RenderPlan
// 對每個 plannable act 的模板輸出（Kind 與 Text 都比對，不是只比長度／Kind，
// 見 dialogue_templates_test.go 既有測試）。
//
// 動機：dialogue_templates_test.go 只斷言 Kind／長度，surface_golden_test.go
// 比對的是 realizeOrFallback 對 domain.RenderPlan(plan, args) 的即時呼叫——
// 兩邊會一起變，對文案本身是恆真的。全 repo在這次之前沒有任何測試逐字釘住
// 任一個 act 的 fallback 文案；模板是 fallback 層，模型失敗時買家看到的正是
// 它，這正是「報價單」這種混稱能存在好幾個月沒被發現的原因。
//
// 迴圈跑 PlannableActCodes（比照 surface_golden_test.go:26 的既有做法）：新增
// plannable act 卻沒有補 testdata/dialogue_templates/<act>.json 時，這裡會
// Fatal 而不是靜默漏測。
//
// 每個 act 的 fixture 涵蓋兩種 SafeRenderArgs：全空值（emptyArgsBlocks）與本
// 測試共用的 populatedArgs（populatedArgsBlocks）——部分模板（ActAckAndAskRequired
// 等）文字依 args 分支，部分（ActRequestBasicsReady 等）完全忽略 args；兩種
// 情況都要釘住，才能同時涵蓋「有分支」與「無分支但仍要防止意外開始讀 args」
// 兩種回歸。
//
// fixture 走 testdata/ JSON（沿用 domain/testdata/dialogue_plan/ 的既有慣例），
// 不用 table-driven 常數字串：這裡的 golden 值是「目前逐字文案」，跟
// dialogue_plan 的輸入/輸出快照一樣屬於「資料」而非「邏輯」，放 JSON 才能讓
// diff 只顯示文案本身的變化，不會跟 Go 語法混在一起。
func TestRenderPlan_GoldenFixtures_AllPlannableActs(t *testing.T) {
	populatedArgs := SafeRenderArgs{
		MissingRequiredLabels: []string{"配送城市", "數量"},
		SalientFactLabels:     []string{"品項：寶島梨"},
		NextQuestionPrompt:    "需要指定包裝方式嗎？",
		RejectedLabels:        []string{"配送城市：不在服務範圍"},
	}

	for _, act := range PlannableActCodes {
		act := act
		t.Run(string(act), func(t *testing.T) {
			fixture := loadTemplateGoldenFixture(t, act)

			gotEmpty := RenderPlan(DialoguePlanV1{ActCode: act}, SafeRenderArgs{})
			assertBlocksMatchGolden(t, act, "emptyArgs", gotEmpty, fixture.EmptyArgsBlocks)

			gotPopulated := RenderPlan(DialoguePlanV1{ActCode: act}, populatedArgs)
			assertBlocksMatchGolden(t, act, "populatedArgs", gotPopulated, fixture.PopulatedArgsBlocks)
		})
	}
}

type templateGoldenBlock struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

type templateGoldenFixture struct {
	Act                 string                `json:"act"`
	EmptyArgsBlocks     []templateGoldenBlock `json:"emptyArgsBlocks"`
	PopulatedArgsBlocks []templateGoldenBlock `json:"populatedArgsBlocks"`
}

func loadTemplateGoldenFixture(t *testing.T, act ActCode) templateGoldenFixture {
	t.Helper()
	path := fmt.Sprintf("testdata/dialogue_templates/%s.json", strings.ToLower(string(act)))
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("act %s: missing golden fixture %s (new plannable act must add its own golden file): %v", act, path, err)
	}
	var fixture templateGoldenFixture
	if err := json.Unmarshal(b, &fixture); err != nil {
		t.Fatalf("act %s: invalid golden fixture %s: %v", act, path, err)
	}
	if fixture.Act != string(act) {
		t.Fatalf("act %s: fixture %s declares act=%q, filename/act mismatch", act, path, fixture.Act)
	}
	return fixture
}

// assertBlocksMatchGolden 逐 block 逐欄位比對，失敗訊息帶 act、scenario（args
// 情況）、block index 與哪個欄位（Kind／Text）不一致，一眼看出是哪個 act 的
// 哪個 block 變了——不是丟一個整個 slice 的 diff。
func assertBlocksMatchGolden(t *testing.T, act ActCode, scenario string, got []Block, want []templateGoldenBlock) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("act=%s scenario=%s: block count = %d, want %d\n  got:  %+v\n  want: %+v", act, scenario, len(got), len(want), got, want)
	}
	for i := range want {
		if string(got[i].Kind) != want[i].Kind {
			t.Errorf("act=%s scenario=%s block[%d].Kind = %q, want %q", act, scenario, i, got[i].Kind, want[i].Kind)
		}
		if got[i].Text != want[i].Text {
			t.Errorf("act=%s scenario=%s block[%d].Text = %q, want %q", act, scenario, i, got[i].Text, want[i].Text)
		}
	}
}
