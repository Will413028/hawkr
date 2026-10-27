package domain

import "testing"

func TestQuoteRequiredOptionalSlots_ContainsExactlyQuantityAndDeliveryDate(t *testing.T) {
	want := map[SlotKey]bool{
		SlotQuantity: true, SlotDeliveryDate: true,
		SlotUnit: false, SlotPackaging: false, SlotBudgetPerUnit: false,
		SlotItem: false, SlotDeliveryCity: false,
	}
	for key, expected := range want {
		if got := QuoteRequiredOptionalSlots[key]; got != expected {
			t.Errorf("QuoteRequiredOptionalSlots[%s] = %v, want %v", key, got, expected)
		}
	}
	if len(QuoteRequiredOptionalSlots) != 2 {
		t.Errorf("表內欄位數 = %d, want 2（只有 quantity/delivery_date）", len(QuoteRequiredOptionalSlots))
	}
}
