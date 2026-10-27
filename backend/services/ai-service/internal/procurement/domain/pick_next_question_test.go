package domain

import (
	"strings"
	"testing"
)

func TestPickNextQuestion_RequiredWinsOverOptional(t *testing.T) {
	slots := Slots{}
	missing := []SlotKey{SlotDeliveryCity}

	got, capExhausted := PickNextQuestion(slots, missing, map[SlotKey]bool{}, 0)

	if capExhausted {
		t.Error("必填缺項優先，不該回報 capExhausted")
	}
	if got == nil || got.SlotKey != SlotDeliveryCity {
		t.Fatalf("want question for delivery_city, got %+v", got)
	}
	if got.Source != ClarifyQuestionSourceFixed {
		t.Errorf("Source = %q, want %q", got.Source, ClarifyQuestionSourceFixed)
	}
	if len(got.Options) != 6 {
		t.Errorf("delivery_city options len = %d, want 6（六都）", len(got.Options))
	}
	if !got.AllowFreeText {
		t.Error("AllowFreeText must be true")
	}
}

// TestPickNextQuestion_UnitOnlyMissingRequired_RoutesToQuantityCompoundQuestion
// 是 2026-08-23 unit 併入 requiredSlots（product 決策：price-preview 需要 unit
// 才能算價，見 slots.go requiredSlots 的 doc comment）後的一個死結 bug 的
// regression guard：buildQuestion 只有 key==SlotQuantity 才會轉走
// buildQuantityQuestion（複合問題卡，SlotKey 恆為 "quantity"），unit 一旦成為
// missingRequired[0]（PickNextQuestion 走必填分支，直接呼叫
// buildQuestion(SlotUnit, slots)）就會落進 buildQuestion 的通用分支，組出
// SlotKey: "unit" 的問題卡——但 /answers（usecase/answer.go Handle）對
// slotKey:"unit" 有一條無條件擋掉的 guard（unit 已經折進 quantity 的複合問題
// 卡，只能透過 buildQuantityPatch 一起送出），買家會永遠答不了這張卡、卡在
// CLARIFYING 出不去。修法是 buildQuestion 的路由條件也要接住 SlotUnit，讓它
// 一樣轉走 buildQuantityQuestion（返回的卡 SlotKey 仍是 "quantity"，/answers
// 既有的 buildQuantityPatch 路徑因此吃得下）。
func TestPickNextQuestion_UnitOnlyMissingRequired_RoutesToQuantityCompoundQuestion(t *testing.T) {
	item, city := "水果禮盒", "桃園市"
	slots := Slots{Item: &item, DeliveryCity: &city} // quantity/unit 都還沒填

	got, capExhausted := PickNextQuestion(slots, []SlotKey{SlotUnit}, map[SlotKey]bool{}, 0)

	if capExhausted {
		t.Error("必填缺項優先，不該回報 capExhausted")
	}
	if got == nil {
		t.Fatal("want a question card, got nil")
	}
	if got.SlotKey != SlotQuantity {
		t.Fatalf("SlotKey = %q, want %q（/answers 只認得 slotKey:\"quantity\"，"+
			"slotKey:\"unit\" 一律被 answer.go 的 guard 擋掉，回這個 SlotKey 會讓買家永遠答不了）",
			got.SlotKey, SlotQuantity)
	}
	if !got.AllowFreeText {
		t.Error("quantity 也還沒填，這張卡要能收數字輸入")
	}
	if len(got.UnitOptions) != 3 {
		t.Errorf("UnitOptions len = %d, want 3", len(got.UnitOptions))
	}
}

// TestPickNextQuestion_OptionalOrderAndDependency 沿著 optionalSlotOrder 走一遍。
// 序列在 issue #2442 之後是 quantity（複合問，含 unit）→ delivery_date →
// packaging → budget_per_unit（前兩個是報價單前提，見 optionalSlotOrder 註解）。
func TestPickNextQuestion_OptionalOrderAndDependency(t *testing.T) {
	item, city := "水果禮盒", "桃園市"
	slots := Slots{Item: &item, DeliveryCity: &city}

	// quantity 是優先序第一個未填欄位，且是複合問（unit 也未知）。
	got, _ := PickNextQuestion(slots, nil, map[SlotKey]bool{}, 0)
	if got == nil || got.SlotKey != SlotQuantity {
		t.Fatalf("want quantity, got %+v", got)
	}
	if !got.AllowFreeText || len(got.UnitOptions) != 3 {
		t.Fatalf("want compound question (AllowFreeText+3 unit options), got %+v", got)
	}

	// quantity 跟 unit 一起填了 → delivery_date（第二個報價單前提）。
	qty, unit := 10, "KG"
	slots2 := Slots{Item: &item, DeliveryCity: &city, Quantity: &qty, Unit: &unit}
	got2, _ := PickNextQuestion(slots2, nil, map[SlotKey]bool{}, 0)
	if got2 == nil || got2.SlotKey != SlotDeliveryDate {
		t.Fatalf("want delivery_date, got %+v", got2)
	}

	// 兩個報價單前提都填了 → 回到一般選填序列的第一個：packaging。
	date := "2026-09-01"
	slots3 := Slots{Item: &item, DeliveryCity: &city, Quantity: &qty, Unit: &unit, DeliveryDate: &date}
	got3, _ := PickNextQuestion(slots3, nil, map[SlotKey]bool{}, 0)
	if got3 == nil || got3.SlotKey != SlotPackaging {
		t.Fatalf("want packaging, got %+v", got3)
	}
}

func TestPickNextQuestion_BudgetPerUnitDependencyPositive(t *testing.T) {
	// unit 已知，budget 未填 → 應問 budget_per_unit。
	// （budget 排在序列最後，需先填完 quantity/delivery_date/unit/packaging 才輪到）
	item, city, unit, pkg, date := "水果禮盒", "桃園市", "KG", "簡易", "2026-09-01"
	qty := 10
	slots := Slots{
		Item: &item, DeliveryCity: &city, Quantity: &qty, DeliveryDate: &date,
		Unit: &unit, Packaging: &pkg,
	}

	got, _ := PickNextQuestion(slots, nil, map[SlotKey]bool{}, 0)
	if got == nil || got.SlotKey != SlotBudgetPerUnit {
		t.Fatalf("want budget_per_unit (unit present), got %+v", got)
	}
}

// TestPickNextQuestion_QuantityMissingUnit_AsksUnitFollowUp_NotBudget 取代原本的
// TestPickNextQuestion_BudgetPerUnitDependencyNegative：quantity 已知但 unit 未知
// （例如買家自由對話只講了數字）時，nextEligibleOptionalSlot 會在第一輪就攔下
// quantity 本身（此時尚未 filled），回「單位追問」問題卡；不會往後跑到
// budget_per_unit 的依賴檢查——因為 quantity 沒填完就無法通過迴圈裡更早的
// SlotQuantity 檢查，budget 的 unit-known 依賴在新設計下永遠不會在這個狀態被
// 觸發（quantity 一旦「filled」就保證 unit 已知，見 slotFilled）。
func TestPickNextQuestion_QuantityMissingUnit_AsksUnitFollowUp_NotBudget(t *testing.T) {
	item, city, date := "水果禮盒", "桃園市", "2026-09-01"
	qty := 10
	slots := Slots{
		Item: &item, DeliveryCity: &city, Quantity: &qty, DeliveryDate: &date,
	}

	got, capExhausted := PickNextQuestion(slots, nil, map[SlotKey]bool{}, 0)
	if got == nil || got.SlotKey != SlotQuantity {
		t.Fatalf("want quantity follow-up (unit missing), got %+v", got)
	}
	if got.AllowFreeText {
		t.Error("數量已知，這輪不該再收數字輸入")
	}
	if len(got.UnitOptions) != 3 {
		t.Errorf("UnitOptions len = %d, want 3", len(got.UnitOptions))
	}
	if capExhausted {
		t.Error("quantity 本身符合資格，不該回報 capExhausted")
	}
}

// TestPickNextQuestion_ConsecutiveCapStopsAsking 驗證 cap 打滿時 question 回
// nil——同時驗證 capExhausted 回 true，因為 quantity 這個欄位本來是符合資格、只是被
// cap 擋下（跟「根本沒東西可問」是不同的 nil 成因，見 capExhausted 的 doc comment）。
func TestPickNextQuestion_ConsecutiveCapStopsAsking(t *testing.T) {
	item, city := "水果禮盒", "桃園市"
	slots := Slots{Item: &item, DeliveryCity: &city}

	got, capExhausted := PickNextQuestion(slots, nil, map[SlotKey]bool{}, MaxConsecutiveOptionalQuestions)
	if got != nil {
		t.Fatalf("want nil once cap reached, got %+v", got)
	}
	if !capExhausted {
		t.Error("quantity 本來符合資格、只是被 cap 擋下，capExhausted 必須是 true")
	}
}

// TestPickNextQuestion_NilWhenNothingLeftToAsk 驗證「真的沒東西可問」時 question
// 回 nil，且 capExhausted 必須是 false——即使呼叫端傳進來的 streak 已經 >= cap，
// 也不該被誤判成「cap 擋下了一個本來會問的問題」，因為根本沒有符合資格的欄位。
func TestPickNextQuestion_NilWhenNothingLeftToAsk(t *testing.T) {
	item, city, unit, pkg, date := "水果禮盒", "桃園市", "KG", "簡易", "2026-09-01"
	qty, budget := 10, 500
	slots := Slots{
		Item: &item, DeliveryCity: &city, Quantity: &qty,
		Unit: &unit, Packaging: &pkg, BudgetPerUnit: &budget, DeliveryDate: &date,
	}

	got, capExhausted := PickNextQuestion(slots, nil, map[SlotKey]bool{}, 0)
	if got != nil {
		t.Fatalf("want nil when every slot filled, got %+v", got)
	}
	if capExhausted {
		t.Error("沒有任何欄位符合資格，不該回報 capExhausted")
	}

	// 即使 streak 已經 >= cap，「真的沒東西可問」的 nil 仍然不是 cap 造成的。
	gotAtCap, capExhaustedAtCap := PickNextQuestion(slots, nil, map[SlotKey]bool{}, MaxConsecutiveOptionalQuestions)
	if gotAtCap != nil {
		t.Fatalf("want nil when every slot filled, got %+v", gotAtCap)
	}
	if capExhaustedAtCap {
		t.Error("沒有任何欄位符合資格時，即使 streak 已達 cap，capExhausted 仍必須是 false")
	}
}

func TestPickNextQuestion_FreeTextSlotHasEmptyOptions(t *testing.T) {
	got := buildQuestion(SlotPackaging, Slots{})
	if got.Options == nil || len(got.Options) != 0 {
		t.Errorf("packaging Options = %+v, want empty non-nil slice", got.Options)
	}
}

// TestBuildQuantityQuestion_BothUnknown_IsCompound 驗證兩者都缺時是「複合問」：
// 保留原本的 prompt，AllowFreeText=true（要收數字），附上 7 個單位選項。
func TestBuildQuantityQuestion_BothUnknown_IsCompound(t *testing.T) {
	got := buildQuestion(SlotQuantity, Slots{})

	if got.SlotKey != SlotQuantity {
		t.Fatalf("SlotKey = %v, want SlotQuantity", got.SlotKey)
	}
	if got.Prompt != SlotPrompts[SlotQuantity] {
		t.Errorf("Prompt = %q, want %q", got.Prompt, SlotPrompts[SlotQuantity])
	}
	if !got.AllowFreeText {
		t.Error("兩者皆未知時應該收數字輸入，AllowFreeText 必須是 true")
	}
	if len(got.UnitOptions) != 3 {
		t.Errorf("UnitOptions len = %d, want 3", len(got.UnitOptions))
	}
	if got.CanSkip {
		t.Error("quantity 恆為 QuoteRequiredOptionalSlots，CanSkip 必須是 false")
	}
}

// TestBuildQuantityQuestion_AmountKnownUnitMissing_IsFollowUp 驗證買家透過自由
// 對話只給了數字（LLM Extract 沒抓到單位）時，問題卡換成「單位追問」：prompt
// 換掉、不再收數字輸入、只附單位選項。
func TestBuildQuantityQuestion_AmountKnownUnitMissing_IsFollowUp(t *testing.T) {
	qty := 50
	got := buildQuestion(SlotQuantity, Slots{Quantity: &qty})

	if got.Prompt != SlotPrompts[SlotUnit] {
		t.Errorf("Prompt = %q, want %q（單位追問要換 prompt）", got.Prompt, SlotPrompts[SlotUnit])
	}
	if got.AllowFreeText {
		t.Error("數量已知，這輪不該再收數字輸入，AllowFreeText 必須是 false")
	}
	if len(got.UnitOptions) != 3 {
		t.Errorf("UnitOptions len = %d, want 3", len(got.UnitOptions))
	}
}

// TestBuildQuantityQuestion_BothKnown_AsksAmountOnly 驗證 rewind 可能落在「兩者
// 在 target 都已知」的 checkpoint（quantity 曾被單獨修正過、unit 沒變動）：這時
// 候只重問數字，不強迫買家重選已經確認過的單位。
func TestBuildQuantityQuestion_BothKnown_AsksAmountOnly(t *testing.T) {
	qty := 50
	unit := "KG"
	got := buildQuestion(SlotQuantity, Slots{Quantity: &qty, Unit: &unit})

	if got.Prompt != SlotPrompts[SlotQuantity] {
		t.Errorf("Prompt = %q, want %q", got.Prompt, SlotPrompts[SlotQuantity])
	}
	if !got.AllowFreeText {
		t.Error("兩者皆已知時仍要能重新輸入數字，AllowFreeText 必須是 true")
	}
	if len(got.UnitOptions) != 0 {
		t.Errorf("UnitOptions len = %d, want 0（單位已知，不用再附選項）", len(got.UnitOptions))
	}
}

func TestBuildQuestion_CanSkip_RequiredAndQuoteRequiredSlotsAreFalse(t *testing.T) {
	cases := map[SlotKey]bool{
		SlotItem:          false, // 必填
		SlotDeliveryCity:  false, // 必填
		SlotDeliveryDate:  false, // QuoteRequiredOptionalSlots
		SlotPackaging:     true,
		SlotBudgetPerUnit: true,
	}
	for key, want := range cases {
		got := buildQuestion(key, Slots{})
		if got.CanSkip != want {
			t.Errorf("buildQuestion(%s).CanSkip = %v, want %v", key, got.CanSkip, want)
		}
	}
}

func TestIsRequiredSlot(t *testing.T) {
	cases := map[SlotKey]bool{
		SlotItem: true, SlotDeliveryCity: true, SlotUnit: true,
		SlotQuantity: false, // unit 併入必填（2026-08-23）後，quantity 依然是選填
	}
	for key, want := range cases {
		if got := IsRequiredSlot(key); got != want {
			t.Errorf("IsRequiredSlot(%s) = %v, want %v", key, got, want)
		}
	}
}

// TestPickNextQuestion_QuantityAndDeliveryDateAskedBeforeOtherOptionals 釘住設計書
// §3.2 的優先序調整：quantity／delivery_date 是報價單的前提條件，必須在連續選填上限
// （MaxConsecutiveOptionalQuestions＝3 題）耗盡前就被問到，因此排在其餘選填欄位之前。
func TestPickNextQuestion_QuantityAndDeliveryDateAskedBeforeOtherOptionals(t *testing.T) {
	item, city := "高麗菜", "桃園市"
	slots := Slots{Item: &item, DeliveryCity: &city} // 兩個必填已滿足

	got, capExhausted := PickNextQuestion(slots, nil, map[SlotKey]bool{}, 0)
	if capExhausted {
		t.Fatal("不應該一開始就 capExhausted")
	}
	if got == nil || got.SlotKey != SlotQuantity {
		t.Fatalf("第一題選填應該問 quantity，got %+v", got)
	}

	// quantity 跟 unit 一起填了之後，第二題必須是 delivery_date（另一個報價單前提），
	// 不是 packaging 這類一般選填。
	qty, unit := 10, "KG"
	slots2 := Slots{Item: &item, DeliveryCity: &city, Quantity: &qty, Unit: &unit}
	got2, _ := PickNextQuestion(slots2, nil, map[SlotKey]bool{}, 0)
	if got2 == nil || got2.SlotKey != SlotDeliveryDate {
		t.Fatalf("第二題選填應該問 delivery_date，got %+v", got2)
	}
}

func TestBuildCropDisambiguationQuestion_IncludesRealCandidatesAndOtherOption(t *testing.T) {
	q := BuildCropDisambiguationQuestion([]CropDisambiguationOption{
		{Label: "有機小白菜", CategoryID: "18", CategoryLevel: 3},
	})
	if q.SlotKey != SlotCropDisambiguation {
		t.Errorf("SlotKey = %q, want %q", q.SlotKey, SlotCropDisambiguation)
	}
	if q.AllowFreeText {
		t.Error("AllowFreeText must be false")
	}
	if q.CanSkip {
		t.Error("CanSkip must be false")
	}
	if len(q.Options) != 2 { // 1 real + D other
		t.Errorf("Options len = %d, want 2", len(q.Options))
	}
	if q.Options[0].OptionID != "category-18" {
		t.Errorf("Options[0].OptionID = %q, want %q", q.Options[0].OptionID, "category-18")
	}
	if q.Options[0].Label != "有機小白菜" {
		t.Errorf("Options[0].Label = %q, want %q", q.Options[0].Label, "有機小白菜")
	}
	last := q.Options[len(q.Options)-1]
	if last.OptionID != CropOtherOptionID {
		t.Errorf("last.OptionID = %q, want %q", last.OptionID, CropOtherOptionID)
	}
	if last.Value != "OTHER" {
		t.Errorf("last.Value = %q, want %q", last.Value, "OTHER")
	}
}

// TestBuildCropDisambiguationQuestion_NoCandidates_OnlyOtherOptionWithDedicatedPrompt
// 是 Cut 1（品項別名檢索死路解鎖）新增的分支：候選為空時只剩「其他（自己
// 輸入）」，Prompt 換成「找不到」的措辭——沿用「找到幾個相符的品項」對 0 候選
// 是假話（沒有找到任何東西）。
func TestBuildCropDisambiguationQuestion_NoCandidates_OnlyOtherOptionWithDedicatedPrompt(t *testing.T) {
	q := BuildCropDisambiguationQuestion(nil)
	if len(q.Options) != 1 {
		t.Fatalf("Options len = %d, want 1", len(q.Options))
	}
	if q.Options[0].OptionID != CropOtherOptionID {
		t.Errorf("Options[0].OptionID = %q, want %q", q.Options[0].OptionID, CropOtherOptionID)
	}
	if strings.Contains(q.Prompt, "找到") {
		t.Errorf("Prompt = %q，0 候選時不該說「找到」", q.Prompt)
	}
	if q.AllowFreeText {
		t.Error("AllowFreeText must be false even with zero candidates")
	}
	if q.CanSkip {
		t.Error("CanSkip must be false even with zero candidates")
	}
}
