package model

import (
	"slices"
	"testing"
)

// D4：交易必要的收件匣與 email 不可關；推播可關，只有賣家的 SHIPMENT_ANOMALY 推播不可關；互動與行銷全部可關。
func TestDisableableFollowsD4(t *testing.T) {
	for _, c := range Categories() {
		class, _ := ClassOf(c)
		for _, ch := range Channels() {
			var want bool
			switch class {
			case ClassTransactional:
				want = ch == ChannelPush && c != CategoryShipmentAnomaly
			case ClassInteraction, ClassMarketing:
				want = true
			}
			if got := Disableable(c, ch); got != want {
				t.Errorf("Disableable(%s, %s) = %v，D4 要求 %v", c, ch, got, want)
			}
		}
	}
}

func TestUnknownCategoryOrChannelIsNeverDisableable(t *testing.T) {
	if Disableable("NOT_A_CATEGORY", ChannelPush) || Disableable(CategoryChat, "SMS") {
		t.Error("清單外的分類或渠道不可關")
	}
}

// 每個分類都有大類，大類只有 D4 的三種；預設全開。
func TestEveryCategoryHasAClassAndDefaultsOn(t *testing.T) {
	seen := map[PrefCategory]bool{}
	for _, c := range Categories() {
		if seen[c] {
			t.Errorf("分類 %s 重複", c)
		}
		seen[c] = true
		class, ok := ClassOf(c)
		if !ok || !slices.Contains([]CategoryClass{ClassTransactional, ClassInteraction, ClassMarketing}, class) {
			t.Errorf("分類 %s 的大類 %q 不在 D4", c, class)
		}
		for _, ch := range Channels() {
			if !DefaultEnabled(c, ch) {
				t.Errorf("(%s, %s) 預設要開", c, ch)
			}
		}
	}
	if len(Categories()) != 14 {
		t.Errorf("D4 清單是 14 個分類，現在 %d", len(Categories()))
	}
}

func TestPresentationsAreTheTenOfD6(t *testing.T) {
	if got := len(Presentations()); got != 10 {
		t.Errorf("D6 有 10 個呈現分類，現在 %d", got)
	}
}
