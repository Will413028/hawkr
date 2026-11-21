// Package planner 決定一輪釐清要問什麼、要對買家說哪一類話：純函式，同樣輸入必得同樣輸出，不碰 DB、LLM。
// 問句與選項只帶代碼，文字由 reply 套件依語系組出。
package planner

import (
	"strconv"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/slots"
)

// Prompt 是問題卡問句的代碼。
type Prompt string

// 問句。
const (
	PromptItem          Prompt = "ITEM"
	PromptDeliveryCity  Prompt = "DELIVERY_CITY"
	PromptQuantity      Prompt = "QUANTITY"
	PromptUnit          Prompt = "UNIT"
	PromptPackaging     Prompt = "PACKAGING"
	PromptBudgetPerUnit Prompt = "BUDGET_PER_UNIT"
	PromptDeliveryDate  Prompt = "DELIVERY_DATE"
	// PromptItemChoices 是「有幾個相符的品項，請選一個」；PromptItemDescribe 是「找不到，請描述」。
	PromptItemChoices  Prompt = "ITEM_CHOICES"
	PromptItemDescribe Prompt = "ITEM_DESCRIBE"
)

// OptionKind 決定選項的顯示文字從哪來。
type OptionKind string

// 選項種類。
const (
	// OptionValue 的顯示文字就是 Value（縣市名、分類名）。
	OptionValue OptionKind = "VALUE"
	// OptionUnit 的顯示文字依 Value（單位代碼）查目錄。
	OptionUnit OptionKind = "UNIT"
	// OptionOther 是品項消歧固定附加的「其他，自己輸入」；OptionDescribe 是沒有候選時的「自行描述品項」。
	OptionOther    OptionKind = "OTHER"
	OptionDescribe OptionKind = "DESCRIBE"
)

// ItemOtherOptionID 是品項消歧固定附加的「其他」選項 ID；回答時用它辨認買家要自行描述品項。
const ItemOtherOptionID = "item-other"

// Option 是問題卡上的一個選項。Value 只出自伺服器白名單；client 只回傳 ID。
type Option struct {
	ID    string
	Value string
	Kind  OptionKind
}

// Question 是伺服器要買家回答的一個問題。
type Question struct {
	Slot          model.SlotKey
	Prompt        Prompt
	Options       []Option
	UnitOptions   []Option
	AllowFreeText bool
	CanSkip       bool
}

// MaxConsecutiveOptionalQuestions 是連續選填提問的上限；問滿後不再主動問，買家仍可自己打字提供。
const MaxConsecutiveOptionalQuestions = 3

// optionalSlotOrder 是選填欄位的固定優先序。數量（含單位）與到貨日排最前：它們是寄送報價單的前提，排後面的話
// 連續選填上限用完前問不到。
var optionalSlotOrder = []model.SlotKey{
	slots.Quantity, slots.DeliveryDate, slots.Packaging, slots.BudgetPerUnit,
}

// quoteRequiredOptional 是選填但寄報價單要有的欄位，不可跳過。
var quoteRequiredOptional = map[model.SlotKey]bool{
	slots.Quantity:     true,
	slots.DeliveryDate: true,
}

var prompts = map[model.SlotKey]Prompt{
	slots.Item:          PromptItem,
	slots.DeliveryCity:  PromptDeliveryCity,
	slots.Quantity:      PromptQuantity,
	slots.Unit:          PromptUnit,
	slots.Packaging:     PromptPackaging,
	slots.BudgetPerUnit: PromptBudgetPerUnit,
	slots.DeliveryDate:  PromptDeliveryDate,
}

// 固定選項的白名單。沒有列在這裡的欄位是純自由輸入。
var (
	cityOptions = []Option{
		{ID: "city-taipei", Value: "臺北市", Kind: OptionValue},
		{ID: "city-newtaipei", Value: "新北市", Kind: OptionValue},
		{ID: "city-taoyuan", Value: "桃園市", Kind: OptionValue},
		{ID: "city-taichung", Value: "臺中市", Kind: OptionValue},
		{ID: "city-tainan", Value: "臺南市", Kind: OptionValue},
		{ID: "city-kaohsiung", Value: "高雄市", Kind: OptionValue},
	}
	unitOptions = []Option{
		{ID: "unit-kg", Value: "KG", Kind: OptionUnit},
		{ID: "unit-catty", Value: "CATTY", Kind: OptionUnit},
		{ID: "unit-gram", Value: "GRAM", Kind: OptionUnit},
	}
)

// PickNext 決定這一輪要不要問、問哪個欄位。必填缺項優先（missing[0]，呼叫端已依固定序算好）；必填齊備後依 optionalSlotOrder
// 找第一個沒有值、沒被買家明確跳過、依賴已滿足的選填欄位；連續問滿上限後回 nil。
//
// capExhausted 分辨兩種 nil：本來有欄位可問但被上限擋下（true），與真的沒東西可問（false）。前者的連續計數要原封不動延續到下一輪，
// 後者才可歸零；呼叫端靠它分辨，不必另掃一遍優先序。
func PickNext(s slots.Set, missing []model.SlotKey, skipped map[model.SlotKey]bool, streak int) (q *Question, capExhausted bool) {
	if len(missing) > 0 {
		return BuildFor(missing[0], s), false
	}
	key, ok := nextEligibleOptional(s, skipped)
	if !ok {
		return nil, false
	}
	if streak >= MaxConsecutiveOptionalQuestions {
		return nil, true
	}
	return BuildFor(key, s), false
}

func nextEligibleOptional(s slots.Set, skipped map[model.SlotKey]bool) (model.SlotKey, bool) {
	for _, k := range optionalSlotOrder {
		if skipped[k] || filled(s, k) {
			continue
		}
		if k == slots.BudgetPerUnit && s.Unit == nil {
			continue
		}
		return k, true
	}
	return "", false
}

// NextOptionalStreak 算這一輪要持久化的連續選填計數，供下一輪當 PickNext 的 streak。依序判斷：
// 必填還沒齊（ready=false）歸零；品項消歧問句延續（它是持久的關卡，不是選填，疊加會讓之後報價必填的欄位永遠問不到）；
// 問了一題選填加一；被上限擋下延續；其餘（真的沒東西可問）歸零。
func NextOptionalStreak(ready bool, q *Question, capExhausted bool, prev int) int {
	switch {
	case !ready:
		return 0
	case q != nil && q.Slot == slots.ItemDisambiguation:
		return prev
	case q != nil && !slots.IsRequired(q.Slot):
		return prev + 1
	case q == nil && capExhausted:
		return prev
	}
	return 0
}

// BuildFor 直接組出指定欄位的問題卡，不經過優先序與上限（回答、回退時用）。s 是目標狀態：數量這格要靠它決定問複合問題或單位追問。
func BuildFor(key model.SlotKey, s slots.Set) *Question {
	// 單位折進數量的複合問題卡，沒有獨立的問題型態：獨立的 UNIT 卡沒有路徑能回答。
	if key == slots.Quantity || key == slots.Unit {
		return buildQuantity(s)
	}
	var opts []Option
	if key == slots.DeliveryCity {
		opts = cityOptions
	}
	return &Question{
		Slot: key, Prompt: prompts[key], Options: copyOptions(opts), AllowFreeText: true,
		CanSkip: !slots.IsRequired(key) && !quoteRequiredOptional[key],
	}
}

// buildQuantity 決定數量這格的型態。數量與單位是同一個複合值：兩者都缺是複合問（兩個輸入都給）；只缺單位是單位追問
// （只給單位選項）；只缺數量或兩者都已知（回退到舊版本時）只問數字，不強迫買家重選已確認的單位。
func buildQuantity(s slots.Set) *Question {
	needsAmount := s.Quantity == nil
	needsUnit := s.Unit == nil

	p := PromptQuantity
	if !needsAmount && needsUnit {
		p = PromptUnit
	}
	var unitOpts []Option
	if needsUnit {
		unitOpts = copyOptions(unitOptions)
	}
	return &Question{
		Slot: slots.Quantity, Prompt: p, Options: []Option{},
		UnitOptions: unitOpts, AllowFreeText: needsAmount || !needsUnit,
	}
}

func filled(s slots.Set, key model.SlotKey) bool {
	switch key {
	case slots.Quantity:
		return s.Quantity != nil && s.Unit != nil
	case slots.Unit:
		return s.Unit != nil
	case slots.Packaging:
		return s.Packaging != nil
	case slots.BudgetPerUnit:
		return s.BudgetPerUnit != nil
	case slots.DeliveryDate:
		return s.DeliveryDate != nil
	}
	return false
}

// ItemCandidate 是品項消歧要買家選的一個分類。
type ItemCandidate struct {
	CategoryID int64
	Label      string
}

// ItemOptionID 是候選的選項 ID；回答時用同一個規則比對買家送回來的 ID。
func ItemOptionID(c ItemCandidate) string { return "category-" + strconv.FormatInt(c.CategoryID, 10) }

// BuildItemDisambiguation 把候選分類組成品項消歧問題卡。只能從白名單挑或選「其他」，不接受自由輸入取代選擇；不可跳過，
// 品項身份未定時無法計價也無法詢價。沒有候選時只剩「自行描述品項」。
func BuildItemDisambiguation(cands []ItemCandidate) *Question {
	opts := make([]Option, 0, len(cands)+1)
	for _, c := range cands {
		opts = append(opts, Option{ID: ItemOptionID(c), Value: c.Label, Kind: OptionValue})
	}
	if len(cands) == 0 {
		opts = append(opts, Option{ID: ItemOtherOptionID, Value: "OTHER", Kind: OptionDescribe})
		return &Question{Slot: slots.ItemDisambiguation, Prompt: PromptItemDescribe, Options: opts}
	}
	opts = append(opts, Option{ID: ItemOtherOptionID, Value: "OTHER", Kind: OptionOther})
	return &Question{Slot: slots.ItemDisambiguation, Prompt: PromptItemChoices, Options: opts}
}

func copyOptions(in []Option) []Option {
	out := make([]Option, len(in))
	copy(out, in)
	return out
}
