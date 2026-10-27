package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 資訊單調：買家每補一項資訊，「還缺什麼」只能變短或不變，絕不能變長。
// 這是 metamorphic 不變式——不斷言某一步缺哪幾欄（那是 TestSlots_MissingRequired 的事），
// 只斷言相鄰兩步之間的關係，所以必填欄位集合日後調整也不會讓它假紅。
func TestSlots_MissingRequiredIsMonotonicAsBuyerAddsInformation(t *testing.T) {
	steps := []struct {
		name  string
		patch SlotPatch
	}{
		{"只有品項", SlotPatch{Item: ptr("高麗菜")}},
		{"補數量", SlotPatch{Quantity: ptr(50)}},
		{"補單位", SlotPatch{Unit: ptr("KG")}},
		{"補城市", SlotPatch{DeliveryCity: ptr("桃園市")}},
		{"補選填的日期", SlotPatch{DeliveryDate: ptr("2026-10-20")}},
		{"補選填的預算", SlotPatch{BudgetPerUnit: ptr(30)}},
		{"補選填的包裝", SlotPatch{Packaging: ptr("紙箱")}},
	}

	slots := Slots{}
	prev := map[SlotKey]bool{}
	first := true
	for _, step := range steps {
		next, _ := slots.Apply(step.patch, refNow)
		cur := map[SlotKey]bool{}
		for _, k := range next.MissingRequired() {
			cur[k] = true
		}

		if !first {
			for k := range cur {
				require.Truef(t, prev[k],
					"%s 之後「%s」變成缺的，而前一步不缺——補資訊只能讓缺漏清單變短。"+
						"買家會看到剛回答完又被問回去，那是最傷信任的一種對話錯誤", step.name, k)
			}
			require.LessOrEqualf(t, len(cur), len(prev),
				"%s 之後缺漏數從 %d 變成 %d", step.name, len(prev), len(cur))
		}
		slots, prev, first = next, cur, false
	}

	require.Empty(t, slots.MissingRequired(), "全部補完之後不該還缺必填欄位")
}
