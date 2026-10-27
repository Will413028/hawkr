package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRecordedThisTurn_EmptyPatchProducesEmptySlice(t *testing.T) {
	next, _ := Slots{}.Apply(SlotPatch{}, refNow)
	got := RecordedThisTurn(SlotPatch{}, Slots{}, next)
	require.Empty(t, got, "沒有 patch 任何欄位時不該有任何 recorded chip")
}

func TestRecordedThisTurn_SystemDefaultNotCountedAsRecorded(t *testing.T) {
	// Slots.Apply 對 ExactMatchRequired 有「patch 沒帶、next 仍是 nil 時補預設值
	// true」的既有邏輯（slots.go Apply 尾段）——這是系統補值，不是買家這輪說的。
	patch := SlotPatch{Item: ptr("寶島梨禮盒")}
	next, _ := Slots{}.Apply(patch, refNow)
	require.True(t, *next.ExactMatchRequired, "前置條件：Apply 應該已經補上預設值")

	got := RecordedThisTurn(patch, Slots{}, next)
	require.Equal(t, []RecordedField{{Key: "item", Label: "品項", Value: "寶島梨禮盒"}}, got,
		"exact_match_required 是系統補的，不該出現")
}

func TestRecordedThisTurn_FormatsStringFields(t *testing.T) {
	patch := SlotPatch{Item: ptr("寶島梨禮盒"), DeliveryCity: ptr("桃園市")}
	next, rejected := Slots{}.Apply(patch, refNow)
	require.Empty(t, rejected)

	got := RecordedThisTurn(patch, Slots{}, next)
	require.Equal(t, []RecordedField{
		{Key: "item", Label: "品項", Value: "寶島梨禮盒"},
		{Key: "delivery_city", Label: "配送城市", Value: "桃園市"},
	}, got, "順序須依 slotOrder（＝ Slots struct 欄位順序），不是 patch 設定的順序")
}

func TestRecordedThisTurn_FormatsNumberAndBoolFields(t *testing.T) {
	patch := SlotPatch{
		Quantity: ptr(50), BudgetPerUnit: ptr(800), ExactMatchRequired: ptr(false),
	}
	next, rejected := Slots{}.Apply(patch, refNow)
	require.Empty(t, rejected)

	got := RecordedThisTurn(patch, Slots{}, next)
	require.Equal(t, []RecordedField{
		{Key: "quantity", Label: "數量", Value: "50"},
		{Key: "budget_per_unit", Label: "單價預算", Value: "800"},
		{Key: "exact_match_required", Label: "是否需完全相符", Value: "否"},
	}, got)
}

func TestRecordedThisTurn_ExcludesRejectedFields(t *testing.T) {
	patch := SlotPatch{Item: ptr("寶島梨禮盒"), Quantity: ptr(0)} // 0 不合法
	next, rejected := Slots{}.Apply(patch, refNow)
	require.Len(t, rejected, 1)
	require.Equal(t, "quantity", rejected[0].Key)

	got := RecordedThisTurn(patch, Slots{}, next)
	require.Equal(t, []RecordedField{{Key: "item", Label: "品項", Value: "寶島梨禮盒"}}, got,
		"被拒收的 quantity 不該出現在 recordedThisTurn")
}

func TestRecordedThisTurn_UnchangedFieldsFromPreviousTurnsNotRepeated(t *testing.T) {
	first, _ := Slots{}.Apply(SlotPatch{Item: ptr("寶島梨禮盒")}, refNow)
	secondPatch := SlotPatch{DeliveryCity: ptr("桃園市")}
	second, rejected := first.Apply(secondPatch, refNow)
	require.Empty(t, rejected)

	got := RecordedThisTurn(secondPatch, first, second)
	require.Equal(t, []RecordedField{{Key: "delivery_city", Label: "配送城市", Value: "桃園市"}}, got,
		"item 是上一輪記錄的，這輪 patch 沒帶到就不該重複出現")
}

func TestRecordedThisTurn_CorrectionReappearsInLaterTurn(t *testing.T) {
	first, _ := Slots{}.Apply(SlotPatch{DeliveryCity: ptr("台北市")}, refNow)
	correctionPatch := SlotPatch{DeliveryCity: ptr("桃園市")}
	second, rejected := first.Apply(correctionPatch, refNow)
	require.Empty(t, rejected)

	got := RecordedThisTurn(correctionPatch, first, second)
	require.Equal(t, []RecordedField{{Key: "delivery_city", Label: "配送城市", Value: "桃園市"}}, got,
		"買家更正欄位時，這輪的 patch 有帶到，應該再次出現在 recordedThisTurn")
}

func TestRecordedThisTurn_TruncatedTextFieldStillRecordsWithTruncatedValue(t *testing.T) {
	longItem := strings.Repeat("寶", 205)
	patch := SlotPatch{Item: ptr(longItem)}
	next, rejected := Slots{}.Apply(patch, refNow)
	require.Len(t, rejected, 1)
	require.Contains(t, rejected[0].Reason, "已截斷")
	wantTruncated := strings.Repeat("寶", 200)
	require.Equal(t, wantTruncated, *next.Item)

	got := RecordedThisTurn(patch, Slots{}, next)
	require.Equal(t, []RecordedField{{Key: "item", Label: "品項", Value: wantTruncated}}, got,
		"截斷但有採用的欄位仍要出現在 recordedThisTurn，顯示截斷後的值")
}

func TestRecordedThisTurn_FullyDiscardedFieldNotRecorded(t *testing.T) {
	patch := SlotPatch{Item: ptr("")} // 空字串，完全丟棄
	next, rejected := Slots{}.Apply(patch, refNow)
	require.Len(t, rejected, 1)
	require.Nil(t, next.Item, "完全丟棄時 next 不變")

	got := RecordedThisTurn(patch, Slots{}, next)
	require.Empty(t, got, "完全丟棄的欄位不該出現在 recordedThisTurn")
}

func TestRecordedThisTurn_NilNextValueDoesNotPanic(t *testing.T) {
	// Apply() 今天不會產生「patch 有帶、current 有值，但 next 卻退回 nil」這種
	// 狀態，但 formatIfChanged 不該只靠這個呼叫端不變量活著——直接構造這個 next
	// struct literal，驗證 int/bool 分支各自的 next == nil guard 真的擋得住，
	// 不會對 *next.Quantity / *next.BudgetPerUnit / *next.ExactMatchRequired 解參考。
	current := Slots{
		Quantity:           ptr(50),
		BudgetPerUnit:      ptr(800),
		ExactMatchRequired: ptr(true),
	}
	patch := SlotPatch{
		Quantity:           ptr(60),
		BudgetPerUnit:      ptr(900),
		ExactMatchRequired: ptr(false),
	}
	next := Slots{
		Quantity:           nil,
		BudgetPerUnit:      nil,
		ExactMatchRequired: nil,
	}

	var got []RecordedField
	require.NotPanics(t, func() {
		got = RecordedThisTurn(patch, current, next)
	})
	require.Empty(t, got, "next 該欄位是 nil 時不該當成這輪記錄，也不能 panic")
}
