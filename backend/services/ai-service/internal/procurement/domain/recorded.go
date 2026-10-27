package domain

import "strconv"

// RecordedField 是三段式回覆①確認段（issue #2343 Ask 5）的單一 chip：這一輪
// 新記下的欄位與內容。label 由伺服器固定字串表組，不回填模型原文或前端自己
// 組字——與 StepLabels 同一種「結構性防呆」（見 steps.go）。
type RecordedField struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value string `json:"value"`
}

// SlotLabels 是 SlotKey 的中文顯示字串表。逐字對齊 hawkr-website
// features/procurement-assistant/session-logic.ts 的 SLOT_LABELS——前端已照這組
// 文案上線 mock 驗證過，後端改字面值會讓兩邊 label 不一致（前端目前直接信任
// 後端回傳的 label，不會自己覆寫）。
var SlotLabels = map[SlotKey]string{
	SlotItem:               "品項",
	SlotItemResolution:     "品項分類",
	SlotQuantity:           "數量",
	SlotUnit:               "單位",
	SlotPackaging:          "包裝",
	SlotBudgetPerUnit:      "單價預算",
	SlotDeliveryCity:       "配送城市",
	SlotDeliveryDate:       "到貨日期",
	SlotExactMatchRequired: "是否需完全相符",
}

// slotOrder 決定 RecordedThisTurn 輸出的固定順序，與 Slots struct 欄位順序
// （＝前端 SLOT_LABELS 的宣告順序）一致，讓多欄位同時變動時的 chip 排列可預期、
// 可寫 golden 測試。
var slotOrder = []SlotKey{
	SlotItem, SlotItemResolution, SlotQuantity, SlotUnit, SlotPackaging,
	SlotBudgetPerUnit, SlotDeliveryCity, SlotDeliveryDate, SlotExactMatchRequired,
}

// RecordedThisTurn 算出這一輪要顯示在①確認段的欄位清單：patch 裡「有值」且
// Apply() 後 next 真的跟 current（套用前的狀態）不同的欄位，value 讀 next
// （Apply 後的正典值，例如城市已 normalizeCity、文字已截斷）。
//
// 用「current 跟 next 是否不同」判斷要不要收，而不是看 rejected 清單：
// Slots.Apply() 的 rejected 清單混合了兩種情況——完全丟棄（空字串／不合法數值，
// next 該欄位維持原值不變）與截斷但有採用（文字超長，next 該欄位變成截斷後的
// 新值，仍是「這輪記下的東西」）。只看 rejected 清單會把兩種情況一視同仁，讓
// 截斷但有採用的欄位也從確認清單消失——但那個欄位其實真的被記下了（截斷後的
// 值），買家應該在①確認段看到它，「異常段」的截斷警告是另外一件事、兩者不
// 衝突。current/next 比對法自然統一處理三種情況：完全丟棄（不變，排除）、
// 截斷但有採用（改變，納入且顯示截斷值）、正常更正（改變，納入）。
//
// 只看 patch 是否非 nil 決定「這個 key 這輪有沒有被 patch 到」，仍然不看 next
// 是否非 nil：ExactMatchRequired 在 Apply() 有「patch 沒帶、且 next 仍是 nil
// 時補預設值 true」的既有邏輯（slots.go Apply 尾段）——那是系統補值，不是買家
// 這輪說的，不能算「這輪記錄」。
func RecordedThisTurn(patch SlotPatch, current, next Slots) []RecordedField {
	out := []RecordedField{}
	for _, k := range slotOrder {
		v, ok := formatIfChanged(k, patch, current, next)
		if !ok {
			continue
		}
		out = append(out, RecordedField{Key: string(k), Label: SlotLabels[k], Value: v})
	}
	return out
}

// formatIfChanged 判斷 key 這個欄位這一輪是否「被 patch 到且 Apply 後真的
// 改變」；是的話從 next 讀出正典值並格式化成顯示字串。ok=false 代表這個 key
// 這輪不該出現在 recordedThisTurn（沒被 patch，或 patch 了但 Apply 判定不變）。
func formatIfChanged(key SlotKey, patch SlotPatch, current, next Slots) (value string, ok bool) {
	switch key {
	case SlotItem:
		if patch.Item == nil || stringPtrEqual(current.Item, next.Item) {
			return "", false
		}
		return derefOr(next.Item), true
	case SlotItemResolution:
		// 這是全 slotOrder 唯一一個由伺服器（crop_gate／消歧答題）而非買家原話
		// 填入的欄位，仍然要出現在①確認段：它決定報價的價格基準，買家看不到就
		// 沒機會發現系統挑錯了。ExactMatchRequired 的系統補值刻意不收（見
		// RecordedThisTurn doc comment），兩者的差別是後果——補一個預設值不會
		// 讓買家多付錢，挑錯品項分類會。
		if patch.ItemResolution == nil || current.ItemResolution.Equal(next.ItemResolution) || next.ItemResolution == nil {
			return "", false
		}
		return next.ItemResolution.DisplayValue(), true
	case SlotPackaging:
		if patch.Packaging == nil || stringPtrEqual(current.Packaging, next.Packaging) {
			return "", false
		}
		return derefOr(next.Packaging), true
	case SlotDeliveryCity:
		if patch.DeliveryCity == nil || stringPtrEqual(current.DeliveryCity, next.DeliveryCity) {
			return "", false
		}
		return derefOr(next.DeliveryCity), true
	case SlotDeliveryDate:
		if patch.DeliveryDate == nil || stringPtrEqual(current.DeliveryDate, next.DeliveryDate) {
			return "", false
		}
		return derefOr(next.DeliveryDate), true
	case SlotUnit:
		if patch.Unit == nil || stringPtrEqual(current.Unit, next.Unit) {
			return "", false
		}
		return derefOr(next.Unit), true
	case SlotQuantity:
		if patch.Quantity == nil || intPtrEqual(current.Quantity, next.Quantity) || next.Quantity == nil {
			return "", false
		}
		return strconv.Itoa(*next.Quantity), true
	case SlotBudgetPerUnit:
		if patch.BudgetPerUnit == nil || intPtrEqual(current.BudgetPerUnit, next.BudgetPerUnit) || next.BudgetPerUnit == nil {
			return "", false
		}
		return strconv.Itoa(*next.BudgetPerUnit), true
	case SlotExactMatchRequired:
		if patch.ExactMatchRequired == nil || boolPtrEqual(current.ExactMatchRequired, next.ExactMatchRequired) || next.ExactMatchRequired == nil {
			return "", false
		}
		if *next.ExactMatchRequired {
			return "是", true
		}
		return "否", true
	default:
		return "", false
	}
}

func derefOr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func stringPtrEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func intPtrEqual(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func boolPtrEqual(a, b *bool) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
