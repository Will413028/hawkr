package forecast

import "time"

// marketTypeToCategoryCode 鏡射上游 market_categories.py:5-17。
//
// MOA 的休市日曆（MarketRestDayFarmWCF）用 V/F/L 標營業類別，行情列
// （AgriProductsTransType）的 TcType 用 N04/N05/N06 —— 同一件事的兩套代碼。
// 上游註解記錄了兩個實例：市場 105 只賣花，在日曆是 L、在行情是 N06；
// 市場 423 只賣果，在日曆是 F、在行情是 N05。
var marketTypeToCategoryCode = map[string]string{
	"V": "N04",
	"F": "N05",
	"L": "N06",
}

var categoryCodeToMarketType = func() map[string]string {
	out := make(map[string]string, len(marketTypeToCategoryCode))
	for marketType, categoryCode := range marketTypeToCategoryCode {
		out[categoryCode] = marketType
	}
	return out
}()

// CategoryCodeForMarketType 把休市日曆的營業類別碼轉成行情列的 category code。
// 未知輸入回 ("", false) —— 刻意不做大小寫正規化也不給預設值：MOA 回的是大寫，
// 靜默接受其他形式會讓「上游改了代碼」看起來像「這個市場沒有休市」。
func CategoryCodeForMarketType(marketType string) (string, bool) {
	code, ok := marketTypeToCategoryCode[marketType]
	return code, ok
}

// MarketTypeForCategoryCode 是 CategoryCodeForMarketType 的反向查表。
func MarketTypeForCategoryCode(categoryCode string) (string, bool) {
	marketType, ok := categoryCodeToMarketType[categoryCode]
	return marketType, ok
}

type marketDateKey struct {
	marketID string
	date     Date
}

type marketCategoryDateKey struct {
	marketID     string
	categoryCode string
	date         Date
}

// Closures 是「某市場（或某市場的某營業類別）在某天休市」的集合。
//
// 🔴 兩層鍵不是過度設計：2026-07-30 實打 MOA 休市日曆量測，21 個市場中有 3 個
// （台中市 400 / 高雄市 800 / 溪湖鎮 514）的類別間排程不同，民國 115 年各差
// 71-73 天。parity fixture 本身就是證據——moa:109:A1 是 N05、moa:109:LA1 是
// N04，同市場同一天 2026-07-21 只有 N04 休市。
//
// 上游把「傳入的日曆」與「從觀測點推出的休市」分成兩組結構（forecasting.py:73-108
// 的 _normalize_closed_dates / _known_closed_from_points），但兩組在 is_closed
// 裡是純聯集（forecasting.py:230-249），所以這裡合成一份 —— 差別只在來源，
// 不在語意。
type Closures struct {
	byMarket         map[marketDateKey]bool
	byMarketCategory map[marketCategoryDateKey]bool
}

func NewClosures() *Closures {
	return &Closures{
		byMarket:         map[marketDateKey]bool{},
		byMarketCategory: map[marketCategoryDateKey]bool{},
	}
}

// AddMarket 記一筆市場層級休市（涵蓋該市場所有營業類別）。
func (c *Closures) AddMarket(marketID string, d Date) {
	c.byMarket[marketDateKey{marketID: marketID, date: d}] = true
}

// AddMarketCategory 記一筆某市場某營業類別的休市。
func (c *Closures) AddMarketCategory(marketID, categoryCode string, d Date) {
	c.byMarketCategory[marketCategoryDateKey{marketID: marketID, categoryCode: categoryCode, date: d}] = true
}

// AddFromPoints 從觀測點推出休市，鏡射 _known_closed_from_points
// （forecasting.py:84-97）：只收 is_active && is_market_closed && available_at <= cutoff
// 的點；category_code 為 nil 落市場層級，否則落 category 層級。
func (c *Closures) AddFromPoints(points []PricePoint, cutoff time.Time) {
	for _, p := range points {
		if !p.IsActive || !p.IsMarketClosed || p.AvailableAt.After(cutoff) {
			continue
		}
		if p.CategoryCode == nil {
			c.AddMarket(p.MarketID, p.TradeDate)
			continue
		}
		c.AddMarketCategory(p.MarketID, *p.CategoryCode, p.TradeDate)
	}
}

// IsClosed 鏡射 forecasting.py:230-249 的 is_closed 判斷。
func (c *Closures) IsClosed(marketID string, categoryCode *string, d Date) bool {
	if c.byMarket[marketDateKey{marketID: marketID, date: d}] {
		return true
	}
	if categoryCode == nil {
		return false
	}
	return c.byMarketCategory[marketCategoryDateKey{
		marketID:     marketID,
		categoryCode: *categoryCode,
		date:         d,
	}]
}

// VerifiedCalendarKey 是「我方對 (市場, 營業類別, 年) 有完整休市日曆覆蓋」的鍵。
type VerifiedCalendarKey struct {
	MarketID     string
	CategoryCode string
	Year         int
}

// VerifiedCalendar 回答「這個 (市場, 營業類別, 年) 的休市日曆是否可信」。
//
// 它與 Closures 是兩件不同的事：Closures 說「這天休市」，VerifiedCalendar 說
// 「我們對這一年的休市排程有沒有把握」。少了後者，「沒有休市列」與「沒有日曆
// 資料」就會塌縮成同一件事 —— 設計書 §8 line 421 明文禁止這個塌縮
// （「缺日曆資料的日期一律回 insufficient_data，不可假設『沒資料就是休市』」）。
type VerifiedCalendar struct {
	enforced bool
	keys     map[VerifiedCalendarKey]bool
}

// NewVerifiedCalendar 建立一個啟用驗證的日曆。
func NewVerifiedCalendar(keys []VerifiedCalendarKey) VerifiedCalendar {
	set := make(map[VerifiedCalendarKey]bool, len(keys))
	for _, k := range keys {
		set[k] = true
	}
	return VerifiedCalendar{enforced: true, keys: set}
}

// UnenforcedVerifiedCalendar 對應上游 verified_calendar_market_categories=None：
// 完全跳過驗證，一律視為已驗證。只給測試與明確不需要驗證的呼叫端用。
func UnenforcedVerifiedCalendar() VerifiedCalendar {
	return VerifiedCalendar{enforced: false}
}

// IsVerified 鏡射 forecasting.py:221-229 的 calendar_verified 判斷。
//
// categoryCode 為 nil 一律回 false：上游用 str(series.category_code) 組鍵，
// None 會 stringify 成 "None" 而永遠不在集合裡（forecasting.py:225）。那是個
// 意外而不是設計，但它是可觀察行為的一部分，且方向是安全的（無 category 的
// series 保守地視為未驗證），所以照樣移植。
func (v VerifiedCalendar) IsVerified(marketID string, categoryCode *string, year int) bool {
	if !v.enforced {
		return true
	}
	if categoryCode == nil {
		return false
	}
	return v.keys[VerifiedCalendarKey{MarketID: marketID, CategoryCode: *categoryCode, Year: year}]
}
