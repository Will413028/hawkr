// Package slots 是採購需求欄位的規則層：沒有 DB、HTTP、LLM。逐欄位驗證，不合格的欄位丟棄並回報原因，其餘照收（單一欄位抽錯不讓整包消失）。
// 原因是代碼（Reason），文字由 reply 套件依語系組出，這裡沒有使用者面字串。
package slots

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

// 需求欄位的代碼，值等於 proto SlotKey 的值名（去掉前綴）。
const (
	Item               model.SlotKey = "ITEM"
	ItemResolution     model.SlotKey = "ITEM_RESOLUTION"
	Quantity           model.SlotKey = "QUANTITY"
	Unit               model.SlotKey = "UNIT"
	Packaging          model.SlotKey = "PACKAGING"
	BudgetPerUnit      model.SlotKey = "BUDGET_PER_UNIT"
	DeliveryCity       model.SlotKey = "DELIVERY_CITY"
	DeliveryDate       model.SlotKey = "DELIVERY_DATE"
	ExactMatchRequired model.SlotKey = "EXACT_MATCH_REQUIRED"
	// ItemDisambiguation 只當問題的路由鍵，不是 Set 的欄位。
	ItemDisambiguation model.SlotKey = "ITEM_DISAMBIGUATION"
)

const (
	// MaxTextLen 是自由文字欄位的長度上限（以 rune 計），與資料表的 CHECK 一致。
	MaxTextLen = 200
	// MaxQuantity、MaxBudgetPerUnit 擋離譜值。
	MaxQuantity      = 1_000_000
	MaxBudgetPerUnit = 10_000_000
	// MaxLeadDays 是到貨日可接受的最遠天數（從台北的今天算）。
	MaxLeadDays = 365
	// DateLayout 是到貨日的唯一文字格式。
	DateLayout = "2006-01-02"
)

// AllowedUnits 只允許重量單位：KG、CATTY、GRAM 都有公斤換算係數，詢價的計價基準據此決定。
var AllowedUnits = []string{"KG", "CATTY", "GRAM"}

// AllowedCities 是台灣 22 縣市，臺／台兩種寫法都收。
var AllowedCities = []string{
	"臺北市", "台北市", "新北市", "桃園市", "臺中市", "台中市", "臺南市", "台南市", "高雄市",
	"基隆市", "新竹市", "新竹縣", "苗栗縣", "彰化縣", "南投縣", "雲林縣", "嘉義市", "嘉義縣",
	"屏東縣", "宜蘭縣", "花蓮縣", "臺東縣", "台東縣", "澎湖縣", "金門縣", "連江縣",
}

// 必填欄位：形成詢價規格的最小集合，順序固定。
var requiredSlots = []model.SlotKey{Item, DeliveryCity, Unit}

// ItemResolutionKind 是品項被判定的方式，值等於資料表的 item_resolution_kind。
type ItemResolutionKind string

const (
	// CategoryConfirmed 是買家確認過的平台分類。
	CategoryConfirmed ItemResolutionKind = "CATEGORY_CONFIRMED"
	// BuyerDescribed 是買家在「找不到符合的品項」時自己描述的品項。
	BuyerDescribed ItemResolutionKind = "BUYER_DESCRIBED"
)

// Resolution 是品項原話被判定成什麼。
type Resolution struct {
	Kind  ItemResolutionKind
	Label string
}

// Set 是一個需求版本的內容；指標欄位 nil 表示買家還沒填。DeliveryDate 是 UTC 午夜表示的日曆日。
type Set struct {
	Item               *string
	Resolution         *Resolution
	Quantity           *int32
	Unit               *string
	Packaging          *string
	BudgetPerUnit      *int64
	DeliveryCity       *string
	DeliveryDate       *time.Time
	ExactMatchRequired *bool
}

// Patch 是一次改動：只帶要改的欄位，nil 表示不動。數字用 int64 收，超出範圍時要能被拒絕而不是溢位；到貨日收買家的原文。
type Patch struct {
	Item               *string
	Resolution         *Resolution
	Quantity           *int64
	Unit               *string
	Packaging          *string
	BudgetPerUnit      *int64
	DeliveryCity       *string
	DeliveryDate       *string
	ExactMatchRequired *bool
}

// Reason 是欄位被丟棄或被修正的原因代碼。
type Reason string

// 欄位被拒的原因。
const (
	ReasonEmpty                 Reason = "EMPTY"
	ReasonTruncated             Reason = "TRUNCATED"
	ReasonQuantityRange         Reason = "QUANTITY_RANGE"
	ReasonBudgetRange           Reason = "BUDGET_RANGE"
	ReasonUnitNotAllowed        Reason = "UNIT_NOT_ALLOWED"
	ReasonCityUnrecognized      Reason = "CITY_UNRECOGNIZED"
	ReasonDateFormat            Reason = "DATE_FORMAT"
	ReasonDateMissingYear       Reason = "DATE_MISSING_YEAR"
	ReasonDatePast              Reason = "DATE_PAST"
	ReasonDateTooFar            Reason = "DATE_TOO_FAR"
	ReasonResolutionNoItem      Reason = "RESOLUTION_NO_ITEM"
	ReasonResolutionNoLabel     Reason = "RESOLUTION_NO_LABEL"
	ReasonResolutionUnknownKind Reason = "RESOLUTION_UNKNOWN_KIND"
)

// Rejection 記錄被丟棄或被修正的欄位與原因；不靜默處理。
type Rejection struct {
	Key    model.SlotKey
	Reason Reason
}

// Apply 把 patch 套到現有內容，回傳新內容與被拒欄位。文字過長是截斷而不是丟棄，同樣回報。
func (s Set) Apply(p Patch, now time.Time) (Set, []Rejection) {
	out := s
	var rejected []Rejection
	reject := func(k model.SlotKey, r Reason) { rejected = append(rejected, Rejection{Key: k, Reason: r}) }

	applyText := func(key model.SlotKey, in *string, dst **string) {
		if in == nil {
			return
		}
		if v, ok := boundedText(*in, reject, key); ok {
			*dst = &v
		}
	}
	applyText(Item, p.Item, &out.Item)
	applyText(Packaging, p.Packaging, &out.Packaging)

	// 品項一變動就清掉舊解析：解析結果與品項原話同生命週期，這裡是唯一的守點。判準是 Apply 前後的值不同，不是 p.Item 非 nil——
	// patch 的品項被驗證退回時 request 沒變，不該連帶清掉仍然正確的解析。
	if !equalPtr(s.Item, out.Item) {
		out.Resolution = nil
	}

	if p.Resolution != nil {
		if out.Item == nil {
			reject(ItemResolution, ReasonResolutionNoItem)
		} else if r, ok := validResolution(*p.Resolution, reject); ok {
			out.Resolution = &r
		}
	}

	if p.Quantity != nil {
		if *p.Quantity < 1 || *p.Quantity > MaxQuantity {
			reject(Quantity, ReasonQuantityRange)
		} else {
			v := int32(*p.Quantity)
			out.Quantity = &v
		}
	}

	if p.BudgetPerUnit != nil {
		if *p.BudgetPerUnit < 1 || *p.BudgetPerUnit > MaxBudgetPerUnit {
			reject(BudgetPerUnit, ReasonBudgetRange)
		} else {
			v := *p.BudgetPerUnit
			out.BudgetPerUnit = &v
		}
	}

	if p.Unit != nil {
		v := strings.ToUpper(trimtext.Trim(*p.Unit))
		if !contains(AllowedUnits, v) {
			reject(Unit, ReasonUnitNotAllowed)
		} else {
			out.Unit = &v
		}
	}

	if p.DeliveryCity != nil {
		v := trimtext.Trim(*p.DeliveryCity)
		if canonical, ok := normalizeCity(v); ok {
			v = canonical
		}
		if !contains(AllowedCities, v) {
			reject(DeliveryCity, ReasonCityUnrecognized)
		} else {
			out.DeliveryCity = &v
		}
	}

	if p.DeliveryDate != nil {
		raw := trimtext.Trim(*p.DeliveryDate)
		if canonical, ok := normalizeDeliveryDate(raw); ok {
			raw = canonical
		}
		d, err := time.ParseInLocation(DateLayout, raw, time.UTC)
		today := twtime.Day(now)
		switch {
		case err != nil:
			reject(DeliveryDate, dateFormatReason(raw))
		case d.Before(today):
			reject(DeliveryDate, ReasonDatePast)
		case d.After(today.AddDate(0, 0, MaxLeadDays)):
			reject(DeliveryDate, ReasonDateTooFar)
		default:
			out.DeliveryDate = &d
		}
	}

	if p.ExactMatchRequired != nil {
		v := *p.ExactMatchRequired
		out.ExactMatchRequired = &v
	}
	if out.ExactMatchRequired == nil {
		// 預設只找完全相符的品項；放寬必須是買家明確同意的動作。
		def := true
		out.ExactMatchRequired = &def
	}
	return out, rejected
}

// boundedText 去頭尾空白、拒絕空值，超過 MaxTextLen 的截斷（以 rune 計）並回報。截斷後再去一次頭尾空白，資料表的 CHECK 不收尾端空白。
func boundedText(raw string, reject func(model.SlotKey, Reason), key model.SlotKey) (string, bool) {
	v := trimtext.Trim(raw)
	if v == "" {
		reject(key, ReasonEmpty)
		return "", false
	}
	if utf8.RuneCountInString(v) > MaxTextLen {
		v = trimtext.Trim(string([]rune(v)[:MaxTextLen]))
		reject(key, ReasonTruncated)
	}
	return v, true
}

func validResolution(r Resolution, reject func(model.SlotKey, Reason)) (Resolution, bool) {
	if r.Kind != CategoryConfirmed && r.Kind != BuyerDescribed {
		reject(ItemResolution, ReasonResolutionUnknownKind)
		return Resolution{}, false
	}
	label, ok := boundedText(r.Label, func(k model.SlotKey, why Reason) {
		if why == ReasonEmpty {
			why = ReasonResolutionNoLabel
		}
		reject(k, why)
	}, ItemResolution)
	if !ok {
		return Resolution{}, false
	}
	return Resolution{Kind: r.Kind, Label: label}, true
}

// MissingRequired 回傳仍缺的必填欄位，順序固定；全部填齊時是空 slice，不是 nil。
func (s Set) MissingRequired() []model.SlotKey {
	filled := map[model.SlotKey]bool{
		Item:         s.Item != nil,
		DeliveryCity: s.DeliveryCity != nil,
		Unit:         s.Unit != nil,
	}
	out := []model.SlotKey{}
	for _, k := range requiredSlots {
		if !filled[k] {
			out = append(out, k)
		}
	}
	return out
}

// IsRequired 回傳這個欄位是不是必填。
func IsRequired(key model.SlotKey) bool {
	for _, k := range requiredSlots {
		if k == key {
			return true
		}
	}
	return false
}

// DisplayValue 是品項解析給買家看的字串。
func (r Resolution) DisplayValue() string { return r.Label }

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func equalPtr[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
