// Package domain 是 procurement 域的純資料與規則層：沒有 DB、沒有 HTTP、沒有 LLM。
// 需求欄位的驗證與「不合格欄位丟棄而非整包失敗」的語意全部收斂在這裡，可獨立單測。
package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// SlotKey 是需求欄位的穩定識別字，同時是 LLM structured output 的 property 名稱、
// API 的欄位名與 DB slots_json 的 key——三處共用同一組常數，不容許各自寫字面值。
type SlotKey string

const (
	SlotItem SlotKey = "item"
	// SlotItemResolution 是 SlotItem 的解析結果——品項落在 Pricebook 分類體系的
	// 哪一格。刻意做成 item 的**兄弟 key** 而不是把 item 改成
	// {raw, resolved, provenance} 的巢狀結構：巢狀會讓既有每一筆 revision 的
	// slots_json（`"item": "高麗菜"`）反序列化直接失敗，rewind 與對話回放都得靠
	// 一支改寫全部歷史 revision 的資料 migration 才能活；兄弟 key 是純附加，舊
	// revision 照樣讀得出來。兩者的生命週期性質完全相同——同一個 struct、同一次
	// 寫入、同一次回退——ADR「解析結果與 item 原話同生命週期」的不變量一樣成立。
	//
	// 代價是兩個 key 理論上可能不同步，由 Apply 一處守住（見該函式內「item 一變動
	// 就清掉舊解析」那段）。
	SlotItemResolution     SlotKey = "item_resolution"
	SlotQuantity           SlotKey = "quantity"
	SlotUnit               SlotKey = "unit"
	SlotPackaging          SlotKey = "packaging"
	SlotBudgetPerUnit      SlotKey = "budget_per_unit"
	SlotDeliveryCity       SlotKey = "delivery_city"
	SlotDeliveryDate       SlotKey = "delivery_date"
	SlotExactMatchRequired SlotKey = "exact_match_required"
)

const (
	// MaxTextLen 是自由文字欄位的長度上限（以 rune 計）。使用者輸入沒有天然上限，
	// 邊界要設在「文字被寫進 DB／送進 prompt」這個唯一序列化點。
	MaxTextLen = 200
	// MaxQuantity／MaxBudgetPerUnit 防止離譜值進入後續價格與候選邏輯。
	MaxQuantity      = 1_000_000
	MaxBudgetPerUnit = 10_000_000
	// MaxPricePerKg 是「這個數字不可能是真的單價」的上界（新台幣元／公斤）。
	// 逃生艙自填價與市場行情取價共用同一個上界——usecase 的 maxPricePerKgNTD
	// 就是這個常數，不再各自宣告。
	MaxPricePerKg = 100_000_000
	// MaxLeadDays 是到貨日可接受的最遠天數。
	MaxLeadDays = 365
	// DateLayout 是到貨日的唯一格式。
	DateLayout = "2006-01-02"
)

// AllowedUnits 只允許重量單位：KG/CATTY/GRAM 都有天然公斤換算係數，可直接
// 用來把 Pricebook 的 TWD/kg 市場參考價換算成對應金額。BOX/CASE/PIECE/SET
// 等計數單位沒有天然公斤換算，Pricebook 對它們永遠算不出參考價，因此不在
// 允許清單內（2026-08-17 commit b45ce3858 已做過一次這個決定；本清單曾短暫
// 被再次擴張為包含計數單位，發現從未真正走通任何輸入路徑後撤回，理由同 b45ce3858）。
var AllowedUnits = []string{"KG", "CATTY", "GRAM"}

// AllowedCities 是台灣 22 個縣市。配送城市必須是可比對的固定值。
var AllowedCities = []string{
	"臺北市", "台北市", "新北市", "桃園市", "臺中市", "台中市", "臺南市", "台南市", "高雄市",
	"基隆市", "新竹市", "新竹縣", "苗栗縣", "彰化縣", "南投縣", "雲林縣", "嘉義市", "嘉義縣",
	"屏東縣", "宜蘭縣", "花蓮縣", "臺東縣", "台東縣", "澎湖縣", "金門縣", "連江縣",
}

// requiredSlots 是形成詢價規格的最小必填集合。
//
// 2026-08-09 從 6 個縮成 2 個（item／delivery_city），其餘 4 個欄位改選填，
// 換取更自然的多輪對話體驗——見
// docs/superpowers/specs/2026-08-09-procurement-clarify-adk-redesign-design.md。
// 已知影響：PLATFORM lane 的 budget_per_unit 價格篩選在此欄位常態缺值時會變寬鬆，
// 這是已與 Will 確認接受的 tradeoff，不在本次變更範圍內調整篩選邏輯本身。
//
// 2026-08-23 追加 unit，變回 3 個必填欄位——這是 issue #2403 買家報價試算
// （price-preview）功能上線後與上面那個決策相衝的地方：Pricebook 的
// domain.CompileReferencePrice 沒有 unit 就換算不出參考價（ErrPriceBasisUnavailable），
// 但買家原本可能在完全沒填 unit 的情況下就到達 REQUIREMENT_READY、看到
// NextAction:"REVIEW_PRICE_PREVIEW" 提示，下一步呼叫報價試算端點必定失敗。
// Will（product owner）明確決定把 unit 併回必填集合，用「多問一題」換「不會把
// 買家導向一個註定失敗的下一步」。這個新增觸發了 domain.buildQuestion 的一個路由
// 修正（見該檔案 doc comment）：unit 一旦是 missingRequired[0]，問題卡仍必須折進
// quantity 的複合問題型態，不能出現 slotKey:"unit"（/answers 對這個 slotKey 有
// 無條件拒答的 guard，見 usecase/answer.go）。
var requiredSlots = []SlotKey{SlotItem, SlotDeliveryCity, SlotUnit}

// Slots 是一個需求版本的完整內容。所有欄位用指標表示「未填」與「填了零值」的差別。
type Slots struct {
	Item               *string         `json:"item,omitempty"`
	ItemResolution     *ItemResolution `json:"item_resolution,omitempty"`
	Quantity           *int            `json:"quantity,omitempty"`
	Unit               *string         `json:"unit,omitempty"`
	Packaging          *string         `json:"packaging,omitempty"`
	BudgetPerUnit      *int            `json:"budget_per_unit,omitempty"`
	DeliveryCity       *string         `json:"delivery_city,omitempty"`
	DeliveryDate       *string         `json:"delivery_date,omitempty"`
	ExactMatchRequired *bool           `json:"exact_match_required,omitempty"`
}

// SlotPatch 是一次改動：只帶要改的欄位，nil 表示不動。
type SlotPatch Slots

// RejectedField 記錄被丟棄或被修正的欄位與原因。原因會回給前端顯示，
// 讓使用者知道系統為什麼沒採用他說的某個值——不靜默丟棄。
type RejectedField struct {
	Key    string `json:"key"`
	Reason string `json:"reason"`
}

// Apply 把 patch 套到現有 slots，回傳新的 Slots 與被拒欄位清單。
//
// 語意刻意設計成「逐欄位驗證、不合格者丟棄，其餘照收」：LLM 抽錯一個欄位時，
// 使用者說對的其他欄位不該一起消失。文字過長是截斷而非丟棄（保留可用資訊），
// 但同樣要回報，不靜默處理。
func (s Slots) Apply(p SlotPatch, now time.Time) (Slots, []RejectedField) {
	out := s
	var rejected []RejectedField
	reject := func(k SlotKey, reason string) {
		rejected = append(rejected, RejectedField{Key: string(k), Reason: reason})
	}

	applyText := func(key SlotKey, in *string, dst **string) {
		if in == nil {
			return
		}
		v := strings.TrimSpace(*in)
		if v == "" {
			reject(key, "空值，未採用")
			return
		}
		if r := []rune(v); len(r) > MaxTextLen {
			v = string(r[:MaxTextLen])
			reject(key, fmt.Sprintf("超過 %d 字，已截斷", MaxTextLen))
		}
		*dst = &v
	}

	applyText(SlotItem, p.Item, &out.Item)
	applyText(SlotPackaging, p.Packaging, &out.Packaging)

	// item 一變動就清掉舊解析——這是「解析結果與 item 原話同生命週期」這條不變量
	// 的唯一守點，所有路徑（turn／answer／rewind）都經過這裡。
	//
	// 舊形狀（解析結果住 procurement_requests 的七個欄位）沒有這個位置可守：欄位
	// 跟 slots 不在同一次寫入裡，只能靠各個呼叫端記得清，於是 rewind 需要一支
	// withClearedResolution 特判，而 Pricebook 查詢失敗那條降級路徑要另外寫一次
	// 「全零 persist」。留著舊解析的代價不是資料難看——下游取價的
	// 報價就緒門檻只看解析結果非空、看不出它對應的是哪個 item，於是買家會拿 A 品項
	// 的行情對 B 報價、把邀請寄給真實賣家，寄出去就收不回來。
	//
	// 判準是「Apply 前後的 item 值不同」而不是 p.Item != nil：patch 帶了 item 但被
	// 上面的驗證退回時，request 內容其實沒變，不該連帶清掉仍然正確的解析結果。
	if !stringPtrEqual(s.Item, out.Item) {
		out.ItemResolution = nil
	}

	if p.ItemResolution != nil {
		v := *p.ItemResolution
		// 沒有品項卻帶著解析結果，代表寫入端把兩者拆開了——這正是本次重構要消滅
		// 的形狀，不容許它從 patch 這一側重新長出來。
		reason := "沒有品項可以解析"
		if out.Item != nil {
			reason = validateItemResolution(v)
		}
		if reason != "" {
			reject(SlotItemResolution, reason)
		} else {
			out.ItemResolution = &v
		}
	}

	if p.Quantity != nil {
		if *p.Quantity < 1 || *p.Quantity > MaxQuantity {
			reject(SlotQuantity, fmt.Sprintf("數量需介於 1 與 %d 之間", MaxQuantity))
		} else {
			v := *p.Quantity
			out.Quantity = &v
		}
	}

	if p.BudgetPerUnit != nil {
		if *p.BudgetPerUnit < 1 || *p.BudgetPerUnit > MaxBudgetPerUnit {
			reject(SlotBudgetPerUnit, fmt.Sprintf("單價預算需介於 1 與 %d 元之間", MaxBudgetPerUnit))
		} else {
			v := *p.BudgetPerUnit
			out.BudgetPerUnit = &v
		}
	}

	if p.Unit != nil {
		v := strings.ToUpper(strings.TrimSpace(*p.Unit))
		if !contains(AllowedUnits, v) {
			reject(SlotUnit, "單位不在允許清單內："+strings.Join(AllowedUnits, "/"))
		} else {
			out.Unit = &v
		}
	}

	if p.DeliveryCity != nil {
		v := strings.TrimSpace(*p.DeliveryCity)
		if canonical, ok := normalizeCity(v); ok {
			v = canonical
		}
		if !contains(AllowedCities, v) {
			reject(SlotDeliveryCity, "不是可辨識的縣市名稱")
		} else {
			out.DeliveryCity = &v
		}
	}

	if p.DeliveryDate != nil {
		v := strings.TrimSpace(*p.DeliveryDate)
		if canonical, ok := normalizeDeliveryDate(v); ok {
			v = canonical
		}
		d, err := time.ParseInLocation(DateLayout, v, time.UTC)
		switch {
		case err != nil:
			reject(SlotDeliveryDate, deliveryDateRejectReason(v))
		case !d.After(now.AddDate(0, 0, -1)):
			reject(SlotDeliveryDate, "到貨日不可早於今天")
		case d.After(now.AddDate(0, 0, MaxLeadDays)):
			reject(SlotDeliveryDate, fmt.Sprintf("到貨日不可超過 %d 天後", MaxLeadDays))
		default:
			out.DeliveryDate = &v
		}
	}

	if p.ExactMatchRequired != nil {
		v := *p.ExactMatchRequired
		out.ExactMatchRequired = &v
	}
	if out.ExactMatchRequired == nil {
		// 預設只找完全相符品項；要放寬必須是買家明確同意的動作（設計書 §4.1）。
		def := true
		out.ExactMatchRequired = &def
	}

	return out, rejected
}

// MissingRequired 回傳仍然缺少的必填欄位，順序固定（供 UI 穩定顯示）。
//
// 全部填齊時回傳空 slice 而非 nil：這個回傳值會直接序列化成 API 的
// missingRequired，nil 會變成 JSON null、空 slice 才是 []。而「全部填齊」正是
// 前端要偵測的「需求就緒」轉換點——恰好在最關鍵的那一刻改變 JSON 型別，會逼前端
// 多寫一層 null 判斷。與同一個 payload 的 rejected 保持一致：空集合就是 []。
func (s Slots) MissingRequired() []SlotKey {
	filled := map[SlotKey]bool{
		SlotItem:          s.Item != nil,
		SlotQuantity:      s.Quantity != nil,
		SlotUnit:          s.Unit != nil,
		SlotBudgetPerUnit: s.BudgetPerUnit != nil,
		SlotDeliveryCity:  s.DeliveryCity != nil,
		SlotDeliveryDate:  s.DeliveryDate != nil,
	}
	out := []SlotKey{}
	for _, k := range requiredSlots {
		if !filled[k] {
			out = append(out, k)
		}
	}
	return out
}

// IsRequiredSlot 回傳這個欄位是不是必填欄位——pickNextSlot 的連續計數判斷「有沒有被
// 必填問題打斷」時要用（見 usecase.deriveQuestionHistory）。
func IsRequiredSlot(key SlotKey) bool {
	for _, k := range requiredSlots {
		if k == key {
			return true
		}
	}
	return false
}

// Hash 是內容指紋，用於 artifact 的 input_hash 比對與樂觀鎖輔助判斷。
// 走 encoding/json 序列化：struct 欄位順序固定，因此同內容必得同 hash。
func (s Slots) Hash() string {
	b, err := json.Marshal(s)
	if err != nil {
		// Slots 只含基本型別，json.Marshal 不會失敗；真發生就讓 hash 明顯異常而非 panic。
		return "hash-error"
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
