package usecase

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/stretchr/testify/require"
)

// TestPublicShortTextRejectsUnperformedTransaction：同一句不實交易敘述，在
// ShortIntent 與 step 敘述兩個表面都不得流出。doc 57 §3.1 第 3 條「不承載事實」。
func TestPublicShortTextRejectsUnperformedTransaction(t *testing.T) {
	unsafe := []string{
		"已付款100元給某某商店",
		"已經幫你找到三個合適的賣家",
		"已送出給賣家了",
		"已下單，等待出貨",
		"這批要 5 公斤",
	}
	for _, s := range unsafe {
		t.Run(s, func(t *testing.T) {
			require.NotEmpty(t, publicTextRejection(s), "必須被判定為越界")

			// 表面一：Activity 的短句。
			require.NotEqual(t, s, guardShortIntent(s), "ShortIntent 不得原樣放行")

			// 表面二：step 敘述。
			got := narrateStepOutcome(context.Background(), &fakeStepNarrator{text: s},
				domain.StepValidateSlots, "這一輪沒有新記下任何欄位；目前條件皆有效。", "fallback")
			require.Equal(t, "fallback", got, "step 敘述必須退回確定性文案")
		})
	}
}

// TestPublicShortTextRejectsWireUnitFacts 釘住 enum 那道繞道。
//
// 這幾句不是編出來的：2026-09-19 在 dev 上跑真對話時，VALIDATE_SLOTS 實際產出
// 「已記下高麗菜 20KG、單價 60、臺中市、2026-09-30 到貨，條件皆有效。」並**通過**
// 了當時的判準——單位那一組只收中文，而 slots.unit 存的就是 AllowedUnits 的 enum
// 字面，prompt 把它餵給模型，模型照著寫。
func TestPublicShortTextRejectsWireUnitFacts(t *testing.T) {
	unsafe := []string{
		"已記下高麗菜 20KG、單價 60、臺中市、2026-09-30 到貨，條件皆有效。",
		"已記下 20kg",
		"已記下 20 KG",
		"這批要 5 CATTY",
		"這批要 500 GRAM",
		"報價 30800 TWD",
		"每單位 60 NTD",
	}
	for _, s := range unsafe {
		t.Run(s, func(t *testing.T) {
			require.Equal(t, "quantified_fact", publicTextRejection(s))
			require.NotEqual(t, s, guardShortIntent(s), "ShortIntent 不得原樣放行")

			got := narrateStepOutcome(context.Background(), &fakeStepNarrator{text: s},
				domain.StepValidateSlots, "這一輪沒有新記下任何欄位；目前條件皆有效。", "fallback")
			require.Equal(t, "fallback", got, "step 敘述必須退回確定性文案")
		})
	}
}

// TestQuantifiedFactPatternCoversEveryAllowedUnit 讓值域與判準綁在一起：
// 之後往 AllowedUnits 加一個單位而忘了想 policy 這件事，這條會紅。
// 光有上面那組字面 case 做不到——它們只涵蓋當下的三個值。
func TestQuantifiedFactPatternCoversEveryAllowedUnit(t *testing.T) {
	require.NotEmpty(t, domain.AllowedUnits)
	for _, u := range domain.AllowedUnits {
		t.Run(u, func(t *testing.T) {
			require.Equal(t, "quantified_fact", publicTextRejection("這批要 5 "+u),
				"AllowedUnits 的 %q 沒有被事實邊界涵蓋", u)
		})
	}
}

// TestPublicShortTextAllowsLegitimateIntent 是正控制。少了它，「一律 fallback」
// 也會讓上面那條變綠——而那正是計畫警告過的失敗模式：把 blocks 的全套 schema
// 套到裸字串上，結果每一輪都退回固定標籤。
func TestPublicShortTextAllowsLegitimateIntent(t *testing.T) {
	safe := []string{
		"正在整理你的需求",
		"還缺 2 個條件才能開始比價",
		"已記下品項與配送地",
		"需要你確認一下品項分類",
		// (?i) 與英文單位不得把這些誤判成講了數量：單位必須緊鄰在數字之後，
		// 而這幾句的數字後面接的是別的字。domain 的 unitAliases 註解記過同一
		// 類子字串誤命中（program／logo／巧克力），那裡沒有數字前綴可以依賴、
		// 只能少收，這裡有，所以可以收得比它寬。
		"已排入第 2 個 program 流程",
		"已讀取 3 張 diagram",
		"第 1 組 catty-shaped 包裝待確認",
	}
	for _, s := range safe {
		t.Run(s, func(t *testing.T) {
			require.Empty(t, publicTextRejection(s), "合法意圖不得被擋")
			require.Equal(t, s, guardShortIntent(s))

			got := narrateStepOutcome(context.Background(), &fakeStepNarrator{text: s},
				domain.StepValidateSlots, "摘要", "fallback")
			require.Equal(t, s, got)
		})
	}
}

// TestPublicTextRejection_NegationIsNotAViolation 承接既有黑名單的否定偵測：
// 免責文案（「不代表供應商已報價」）含有被列的子字串，但那是否定敘述。
func TestPublicTextRejection_NegationIsNotAViolation(t *testing.T) {
	require.Empty(t, publicTextRejection("這會發布一筆詢價需求，不代表供應商已報價"))
}

// TestPublicTextRejection_ReasonsAreLowCardinality 守住 metric label 的基數：
// 回傳值只能是固定字面值，不得把買家的原話帶出來。
func TestPublicTextRejection_ReasonsAreLowCardinality(t *testing.T) {
	allowed := map[string]bool{"forbidden_phrase": true, "quantified_fact": true}
	for _, s := range []string{"已付款100元給某某商店", "這批要 5 公斤", "已成交"} {
		require.True(t, allowed[publicTextRejection(s)], "未預期的 reason：%q", publicTextRejection(s))
	}
}
