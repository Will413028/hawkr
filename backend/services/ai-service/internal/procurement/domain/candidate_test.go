package domain

import (
	"testing"
	"time"
)

func TestMatchFactFor_ReturnsChineseLabel(t *testing.T) {
	got := MatchFactFor(SlotItem)
	if got.Key != SlotItem {
		t.Errorf("Key = %q, want %q", got.Key, SlotItem)
	}
	if got.Label != "品項" {
		t.Errorf("Label = %q, want 品項", got.Label)
	}
}

func TestNeedConfirmReason_QuantityIncludesActualValue(t *testing.T) {
	qty := 50
	unit := "BOX"
	s := Slots{Quantity: &qty, Unit: &unit}

	got := NeedConfirmReason(SlotQuantity, s)

	want := "需向賣家確認是否備有 50 BOX 的供應量"
	if got != want {
		t.Errorf("NeedConfirmReason = %q, want %q", got, want)
	}
}

func TestNeedConfirmReason_DeliveryCityIncludesCityName(t *testing.T) {
	city := "桃園市"
	s := Slots{DeliveryCity: &city}

	got := NeedConfirmReason(SlotDeliveryCity, s)

	want := "需向賣家確認是否配送至桃園市"
	if got != want {
		t.Errorf("NeedConfirmReason = %q, want %q", got, want)
	}
}

func TestNeedConfirmReason_DeliveryDateIncludesDate(t *testing.T) {
	date := "2026-08-20"
	s := Slots{DeliveryDate: &date}

	got := NeedConfirmReason(SlotDeliveryDate, s)

	want := "需向賣家確認能否於 2026-08-20 前到貨"
	if got != want {
		t.Errorf("NeedConfirmReason = %q, want %q", got, want)
	}
}

func TestNeedConfirmReason_FallsBackToGenericLabelWhenSlotEmpty(t *testing.T) {
	// Quantity 沒填時不能 panic 或印出零值，退回通用文案。
	got := NeedConfirmReason(SlotQuantity, Slots{})

	want := "需向賣家確認數量"
	if got != want {
		t.Errorf("NeedConfirmReason = %q, want %q", got, want)
	}
}

func TestNeedConfirmReason_DifferentSlotsProduceDifferentText(t *testing.T) {
	// D1 決策的核心斷言：不是固定字串，依維度動態組字。
	city := "台中市"
	date := "2026-09-01"
	s := Slots{DeliveryCity: &city, DeliveryDate: &date}

	cityReason := NeedConfirmReason(SlotDeliveryCity, s)
	dateReason := NeedConfirmReason(SlotDeliveryDate, s)

	if cityReason == dateReason {
		t.Errorf("兩個不同 slot 的文案不應相同，都得到 %q", cityReason)
	}
}

// TestExternalLaneDisclosure_CarriesDoc57Text 釘住 doc 57 §1.6 的常設揭露：
// EXTERNAL lane 不做事前驗證（2026-08-26 決策），email 往返本身就是篩選器，
// 代價是買家清單看起來像經過篩選而其實沒有——這句揭露就是那個策略成立的前提，
// 不是可有可無的補充說明，文案改動必須同步回 doc 57。
func TestExternalLaneDisclosure_CarriesDoc57Text(t *testing.T) {
	got := ExternalLaneDisclosure()

	if got.Key != EvidenceExternalUnverified {
		t.Errorf("Key = %q, want %q", got.Key, EvidenceExternalUnverified)
	}
	want := "外部名單，供應能力未經平台驗證，以賣家回覆為準"
	if got.Reason != want {
		t.Errorf("Reason = %q, want %q", got.Reason, want)
	}
}

// TestEvidenceExternalUnverified_IsNotASlot 釘住命名邊界：這個 key 走
// UnverifiableFact.Key 的既有型別（同 SlotCropDisambiguation／SlotItemResolution
// 那兩個既有的非填答用值），但它描述的是候選的來源性質、不是一個買家填答維度，
// 因此不得與任何真實 slot 的值相撞——撞了會讓前端把來源揭露誤當成某個維度的缺口。
func TestEvidenceExternalUnverified_IsNotASlot(t *testing.T) {
	for key := range slotLabel {
		if key == EvidenceExternalUnverified {
			t.Fatalf("%q 與真實 slot 值相撞", EvidenceExternalUnverified)
		}
	}
}

// TestValidateCandidateEvidence 守 doc 57 §1.6 evidence contract 的三欄。
//
// 目前刻意只擋「結構上不可能被正確渲染」的三種形狀（null 列表、零值 factsAsOf），
// 不擋空陣列——「不得只是空陣列」那條在 doc 57 只對 EXTERNAL 明文要求常設揭露，
// PLATFORM 半邊的需求尚未確定（Will 2026-09-06 裁決：先寬鬆、之後好改）。要收緊時
// 改這一支函式即可，呼叫端不必動。
func TestValidateCandidateEvidence(t *testing.T) {
	asOf := time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name         string
		matched      []byte
		unverifiable []byte
		factsAsOf    time.Time
		wantErr      bool
	}{
		{"兩個空陣列＋有 factsAsOf 通過", []byte("[]"), []byte("[]"), asOf, false},
		{"有內容通過", []byte(`[{"key":"item","label":"品項"}]`), []byte("[]"), asOf, false},
		{"matchedFacts 為 null 擋下", []byte("null"), []byte("[]"), asOf, true},
		{"unverifiableFacts 為 null 擋下", []byte("[]"), []byte("null"), asOf, true},
		{"matchedFacts 為空 bytes 擋下", nil, []byte("[]"), asOf, true},
		{"unverifiableFacts 為空 bytes 擋下", []byte("[]"), nil, asOf, true},
		{"factsAsOf 零值擋下", []byte("[]"), []byte("[]"), time.Time{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCandidateEvidence(tt.matched, tt.unverifiable, tt.factsAsOf)
			if tt.wantErr && err == nil {
				t.Errorf("ValidateCandidateEvidence() = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidateCandidateEvidence() = %v, want nil", err)
			}
		})
	}
}
