package domain

// ClarifyQuestion 是 POST /turns、/answers、/rewinds 共用的引導式問題卡 payload。
// 欄位形狀與 hawkr-website features/procurement-assistant/types.ts 的
// ClarifyQuestion 對齊，不得單方面改動。
type ClarifyQuestion struct {
	SlotKey       SlotKey          `json:"slotKey"`
	Prompt        string           `json:"prompt"`
	Options       []QuestionOption `json:"options"`
	UnitOptions   []QuestionOption `json:"unitOptions,omitempty"`
	AllowFreeText bool             `json:"allowFreeText"`
	Source        string           `json:"source"`
	CanSkip       bool             `json:"canSkip"`
}

// QuestionOption 是問題卡上的一個選項卡。Value 一律出自伺服器白名單，
// client 只回傳 OptionID（見 /answers 端點契約）。
type QuestionOption struct {
	OptionID string `json:"optionId"`
	Label    string `json:"label"`
	Value    string `json:"value"`
}

// ClarifyQuestionSourceFixed 是本輪唯一會出現的 Source 值——不建 Phrase LLM，
// 詳見 docs/superpowers/specs/2026-08-13-procurement-guided-clarification-design.md §3。
// 型別上保留 "PHRASE_LLM" 分支（前端 types.ts 已宣告）供未來真有需要時擴充。
const ClarifyQuestionSourceFixed = "FIXED"

// SlotCropDisambiguation 是消歧問句的路由用哨兵值——刻意不加進 domain.Slots
// struct，品項解析狀態存在 procurement_requests 的獨立欄位，不是 slots_json
// 的一部分。只用於 ClarifyQuestion.SlotKey／AnswerInput.SlotKey 的路由判斷，
// 比照 SlotQuantity 身兼「複合問題卡路由鍵」的既有用法（見 spec §5）。
const SlotCropDisambiguation SlotKey = "crop_disambiguation"

// CropOtherOptionID 是消歧問句固定附加的「D 其他」選項 ID——買家選這個時
// 自填描述＋自填價格（spec §5），usecase.AnswerUseCase.Handle 用這個常數
// 比對，不是逐字面字串。
const CropOtherOptionID = "crop-other"

// MaxConsecutiveOptionalQuestions 是連續選填提問的上限。問滿這個數字後
// PickNextQuestion 不再主動發問，買家仍可自行打字提供（Extract 照常收）。
const MaxConsecutiveOptionalQuestions = 3

// optionalSlotOrder 是選填欄位的固定優先序（issue #2343 原提案序列，扣掉本輪排除的
// temperature；packaging 原本「只在 GIFT 才問」的依賴一併拿掉，改為無條件按序詢問——
// 見設計書 §5）。spec（品相／規格）已於 2026-08-17 全面移除，不再是任何形式的
// slot——不主動問、LLM 不抽取、候選比對不使用。
//
// quantity 這一題現在同時承載數量與單位（複合問題卡，見 buildQuantityQuestion），unit
// 不再是這張清單裡的獨立項目。quantity／delivery_date 被提到最前面（報價單設計書 §3.2）：
// 這兩個是「寄送報價單」的前提條件（QuoteRequiredOptionalSlots，見 quote.go），若排在
// 後面，買家在連續選填上限（MaxConsecutiveOptionalQuestions＝3 題）耗盡前根本問不到，
// 報價單就永遠寄不出去。其餘兩個欄位（packaging/budget_per_unit）維持原本的相對
// 順序，budget_per_unit 仍排在最後（nextEligibleOptionalSlot 的依賴規則要求 unit 已知
// 才問 budget；unit 現已成為 quantity 的一部分，一旦 quantity filled 即保證 unit 已知）。
var optionalSlotOrder = []SlotKey{
	SlotQuantity, SlotDeliveryDate, SlotPackaging, SlotBudgetPerUnit,
}

// SlotPrompts 是伺服器固定問句字串表——比照 steps.go 的 StepLabels 模式，
// question.prompt 永遠查這張表，結構上不可能回填模型原文。
var SlotPrompts = map[SlotKey]string{
	SlotItem:          "這次想採購什麼品項？",
	SlotDeliveryCity:  "要送到哪個城市呢？",
	SlotQuantity:      "大概需要多少數量？",
	SlotUnit:          "這批貨要用什麼單位計算？",
	SlotPackaging:     "包裝上有什麼需求嗎？",
	SlotBudgetPerUnit: "單價預算大概抓多少？",
	SlotDeliveryDate:  "希望什麼時候送達？",
}

// SlotOptions 是每個 slot 的固定選項清單，value 一律出自伺服器白名單。沒有列在這裡的
// slot（item/quantity/packaging/budget_per_unit/delivery_date）是純自由輸入，
// buildQuestion 對它們回空 slice。
var SlotOptions = map[SlotKey][]QuestionOption{
	SlotDeliveryCity: {
		{OptionID: "city-taipei", Label: "臺北市", Value: "臺北市"},
		{OptionID: "city-newtaipei", Label: "新北市", Value: "新北市"},
		{OptionID: "city-taoyuan", Label: "桃園市", Value: "桃園市"},
		{OptionID: "city-taichung", Label: "臺中市", Value: "臺中市"},
		{OptionID: "city-tainan", Label: "臺南市", Value: "臺南市"},
		{OptionID: "city-kaohsiung", Label: "高雄市", Value: "高雄市"},
	},
	SlotUnit: {
		{OptionID: "unit-kg", Label: "公斤", Value: "KG"},
		{OptionID: "unit-catty", Label: "台斤", Value: "CATTY"},
		{OptionID: "unit-gram", Label: "公克", Value: "GRAM"},
	},
}

// PickNextQuestion 決定這一輪要不要問、問哪個欄位。必填缺項優先（missingRequired[0]，
// 呼叫端已依 requiredSlots 固定序算好）；必填齊備後才輪到選填，依 optionalSlotOrder
// 掃描，跳過已有值／已被買家明確跳過（skipped）的欄位；budget_per_unit 需要 unit
// 已知才問。連續問滿 MaxConsecutiveOptionalQuestions 題選填後回 nil。
//
// 純函式：不碰 DB、不呼叫 LLM。skipped／consecutiveOptionalStreak 由呼叫端傳入
// （見 usecase.previousOptionalStreak／deriveSkippedSlots）。
//
// capExhausted 是第二個回傳值：question == nil 有兩種完全不同的成因——「選填欄位
// 真的都問完/被跳過了」（capExhausted=false）跟「找到了一個本來可以問的選填欄位，
// 但連續選填 cap 已經打滿，主動不問」（capExhausted=true）。這兩種 nil 對呼叫端
// 意義不同：後者代表 cap 狀態要原封不動延續到下一輪（不然 cap 會在下一輪失效重新
// 問滿 3 題）；前者代表「這裡沒有東西可保護」，selectedStreak 可以放心歸零，即使
// 之後有欄位被 rewind 重新打開也該給它公平的機會被問到。呼叫端（usecase 層的
// computeOptionalStreak）靠這個訊號分辨兩者，不必自己重新掃一遍
// optionalSlotOrder／skipped／dependency 才能得到同樣的答案——那樣就是複製一份
// 這裡的優先序邏輯，兩處早晚會 drift。
func PickNextQuestion(slots Slots, missingRequired []SlotKey, skipped map[SlotKey]bool, consecutiveOptionalStreak int) (question *ClarifyQuestion, capExhausted bool) {
	if len(missingRequired) > 0 {
		return buildQuestion(missingRequired[0], slots), false
	}
	key, ok := nextEligibleOptionalSlot(slots, skipped)
	if !ok {
		return nil, false
	}
	if consecutiveOptionalStreak >= MaxConsecutiveOptionalQuestions {
		return nil, true
	}
	return buildQuestion(key, slots), false
}

// nextEligibleOptionalSlot 掃 optionalSlotOrder 找出第一個「還沒有值、沒被買家
// 明確跳過、依賴已滿足（budget_per_unit 要求 unit 已知）」的選填欄位，不理會 cap——
// PickNextQuestion 拿它來判斷「若沒有 cap，這一輪本來會問哪個欄位」，藉此把
// 「cap 擋下一個本來會問的問題」跟「根本沒有問題可問」分開。
func nextEligibleOptionalSlot(slots Slots, skipped map[SlotKey]bool) (key SlotKey, ok bool) {
	for _, k := range optionalSlotOrder {
		if skipped[k] {
			continue
		}
		if slotFilled(slots, k) {
			continue
		}
		if k == SlotBudgetPerUnit && slots.Unit == nil {
			continue // 依賴：budget 要先有 unit
		}
		return k, true
	}
	return "", false
}

// BuildQuestionFor 直接組出指定欄位的問題卡，不經過 PickNextQuestion 的優先序／
// 連續選填 cap 邏輯（見原本的用途說明，不變）。slots 是呼叫端解出的目標狀態——
// quantity 這格需要它來決定要問「複合問」還是「單位追問」（見 buildQuantityQuestion）。
func BuildQuestionFor(key SlotKey, slots Slots) *ClarifyQuestion {
	return buildQuestion(key, slots)
}

func buildQuestion(key SlotKey, slots Slots) *ClarifyQuestion {
	// unit 已經折進 quantity 的複合問題卡（見 buildQuantityQuestion），沒有獨立的
	// 問題型態——SlotKey: "unit" 的卡片組不出來給誰用：/answers（usecase/answer.go
	// Handle）對 slotKey:"unit" 有一條無條件擋掉的 guard，只能透過
	// slotKey:"quantity" 的 buildQuantityPatch 一起送出。unit 併入 requiredSlots
	// （2026-08-23，見 slots.go 的決策說明）之後，PickNextQuestion 的必填分支會直接
	// 呼叫 buildQuestion(SlotUnit, slots)（missingRequired[0] == SlotUnit 時）——
	// 這裡若不轉走 buildQuantityQuestion，就會落進下面的通用分支組出答不了的
	// slotKey:"unit" 卡，買家永遠卡在 CLARIFYING 出不去（見
	// TestPickNextQuestion_UnitOnlyMissingRequired_RoutesToQuantityCompoundQuestion）。
	if key == SlotQuantity || key == SlotUnit {
		return buildQuantityQuestion(slots)
	}
	opts := SlotOptions[key]
	if opts == nil {
		opts = []QuestionOption{}
	}
	return &ClarifyQuestion{
		SlotKey: key, Prompt: SlotPrompts[key], Options: opts,
		AllowFreeText: true, Source: ClarifyQuestionSourceFixed,
		CanSkip: !IsRequiredSlot(key) && !QuoteRequiredOptionalSlots[key],
	}
}

// buildQuantityQuestion 決定 quantity 這格問題卡的型態。quantity 跟 unit 是同一個
// 複合值（沒有單位的數量在這個網域不合法，見 issue #2442）：needsAmount／needsUnit
// 各自代表「這一半還缺不缺」，allowFreeText 的算式涵蓋四種組合——兩者都缺是複合問
// （兩個輸入都給）；只缺單位是單位追問（只給單位選項，不再收數字）；只缺數量的
// 狀態理論上不會在一般引導流程出現，但 rewind 落在某個 checkpoint 時可能發生；
// 兩者都已知（rewind 落在「當時兩者都已填」的舊 checkpoint）則只重問數字，不強迫
// 買家重選已經確認過的單位。
func buildQuantityQuestion(slots Slots) *ClarifyQuestion {
	needsAmount := slots.Quantity == nil
	needsUnit := slots.Unit == nil

	prompt := SlotPrompts[SlotQuantity]
	if !needsAmount && needsUnit {
		prompt = SlotPrompts[SlotUnit]
	}

	var unitOpts []QuestionOption
	if needsUnit {
		unitOpts = SlotOptions[SlotUnit]
	}

	return &ClarifyQuestion{
		SlotKey: SlotQuantity, Prompt: prompt, Options: []QuestionOption{},
		UnitOptions: unitOpts, AllowFreeText: needsAmount || !needsUnit,
		Source: ClarifyQuestionSourceFixed, CanSkip: false,
	}
}

func slotFilled(s Slots, key SlotKey) bool {
	switch key {
	case SlotQuantity:
		return s.Quantity != nil && s.Unit != nil
	case SlotUnit:
		return s.Unit != nil
	case SlotPackaging:
		return s.Packaging != nil
	case SlotBudgetPerUnit:
		return s.BudgetPerUnit != nil
	case SlotDeliveryDate:
		return s.DeliveryDate != nil
	default:
		return false
	}
}

// BuildCropDisambiguationQuestion 把品項解析的候選清單組成一張消歧問題卡
// （spec §5）。AllowFreeText 恆為 false——只能從候選白名單挑，或挑 D，不接受
// 直接自由輸入取代選擇動作（跟消歧問句的既有獨立端點一致）。CanSkip 恆為
// false——品項身份未定時無法計價、無法建 RFQ，不能跳過。
//
// candidates 為空（Cut 1：檢索用盡仍查無候選）時換一句 Prompt，選項只剩固定的
// 「其他（自己輸入）」——迴圈本身不需要另外分支，len(candidates)==0 時 opts
// 自然只含這一個選項。
func BuildCropDisambiguationQuestion(candidates []CropDisambiguationOption, selections ...*CropSelection) *ClarifyQuestion {
	opts := make([]QuestionOption, 0, len(candidates)+1)
	for _, c := range candidates {
		opts = append(opts, QuestionOption{
			OptionID: CropDisambiguationOptionID(c),
			Label:    c.Label,
			Value:    c.Label,
		})
	}
	opts = append(opts, QuestionOption{
		OptionID: CropOtherOptionID, Label: "其他（自己輸入）", Value: "OTHER",
	})
	prompt := "找到幾個相符的品項，麻煩選一下你要的是哪一個："
	if len(candidates) == 0 {
		prompt = "找不到吻合的品項分類，可以自己輸入品項描述並填單價："
	}
	allowFreeText := false
	if len(selections) > 0 && selections[0] != nil {
		selection := selections[0]
		switch selection.Stage {
		case CropSelectionGroup:
			prompt = "先確認你要採購的大品項，請選擇一項，或輸入品項名稱："
			allowFreeText = true
		case CropSelectionItem:
			prompt = "接著確認「" + selection.ParentLabel + "」的細項，請選擇一項，或輸入更具體的名稱："
			allowFreeText = true
		case CropSelectionPrice:
			prompt = "目前沒有可用的對應行情，請確認品項描述並填入單價："
			if categoryID, label := selection.ConfirmedCategory(); categoryID != "" && label != "" {
				prompt = "已確認品項「" + label + "」，目前沒有可用的對應行情，請填入單價："
			}
		}
		opts[len(opts)-1].Label = "沒有符合的品項，自行填寫單價"
	}
	return &ClarifyQuestion{
		SlotKey: SlotCropDisambiguation,
		Prompt:  prompt,
		Options: opts, AllowFreeText: allowFreeText,
		Source: ClarifyQuestionSourceFixed, CanSkip: false,
	}
}

// CropDisambiguationOptionID 衍生一個穩定的 OptionID——CropCode／GroupCode
// 跟 domain.CropResolution 一樣互斥，恰好一個非 nil（見 crop_resolution.go
// 的 doc comment）。匯出（大寫開頭）是因為 usecase.findCropDisambiguationOption
// （answer.go，Task 10）要用同一份衍生規則比對買家送回來的 OptionID，兩處
// 各自重寫一份前綴字串邏輯遲早會 drift。
func CropDisambiguationOptionID(c CropDisambiguationOption) string {
	if c.CategoryID != "" {
		return "category-" + c.CategoryID
	}
	return CropOtherOptionID // 理論上不會發生（候選一定有 code），防禦性預設
}
