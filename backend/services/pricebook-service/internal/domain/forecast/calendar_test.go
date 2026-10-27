package forecast

import (
	"testing"
	"time"
)

func strptr(s string) *string { return &s }

func TestCategoryCodeForMarketType(t *testing.T) {
	// market_categories.py:5-17：休市日曆用 V/F/L，行情列用 N04/N05/N06。
	for _, tc := range []struct{ marketType, want string }{
		{"V", "N04"}, {"F", "N05"}, {"L", "N06"},
	} {
		got, ok := CategoryCodeForMarketType(tc.marketType)
		if !ok || got != tc.want {
			t.Errorf("CategoryCodeForMarketType(%q) = (%q, %v)，預期 (%q, true)", tc.marketType, got, ok, tc.want)
		}
		back, ok := MarketTypeForCategoryCode(tc.want)
		if !ok || back != tc.marketType {
			t.Errorf("MarketTypeForCategoryCode(%q) = (%q, %v)，預期 (%q, true)", tc.want, back, ok, tc.marketType)
		}
	}
	if _, ok := CategoryCodeForMarketType("X"); ok {
		t.Error("未知的 MarketType 必須回 false，不能靜默對應到某個 category")
	}
	if _, ok := CategoryCodeForMarketType("v"); ok {
		t.Error("大小寫必須由呼叫端正規化，這裡不做隱式 upper——MOA 回的是大寫，靜默接受小寫會遮蔽上游改格式")
	}
}

func TestClosuresIsClosedUsesTwoLevelKey(t *testing.T) {
	// 這是 parity fixture 的真實形狀：同一市場 moa:109，2026-07-21 只有 N04 休市，
	// N05 開市。單層 market 鍵會把 N05 一起關掉。
	d := NewDate(2026, 7, 21)
	c := NewClosures()
	c.AddMarketCategory("moa:109", "N04", d)

	if !c.IsClosed("moa:109", strptr("N04"), d) {
		t.Error("N04 應為休市")
	}
	if c.IsClosed("moa:109", strptr("N05"), d) {
		t.Error("N05 不該被 N04 的休市列波及——這正是兩層鍵存在的理由")
	}
	if c.IsClosed("moa:109", nil, d) {
		t.Error("category 為 nil 時只看市場層級休市列，不該命中 category 層級的列")
	}
	if c.IsClosed("moa:104", strptr("N04"), d) {
		t.Error("別的市場不該被波及")
	}
	if c.IsClosed("moa:109", strptr("N04"), NewDate(2026, 7, 22)) {
		t.Error("別的日期不該被波及")
	}
}

func TestClosuresMarketWideAppliesToEveryCategory(t *testing.T) {
	d := NewDate(2026, 7, 21)
	c := NewClosures()
	c.AddMarket("moa:109", d)
	for _, cat := range []*string{nil, strptr("N04"), strptr("N05"), strptr("N06")} {
		if !c.IsClosed("moa:109", cat, d) {
			t.Errorf("市場層級休市必須涵蓋所有 category（含 nil），category=%v 沒命中", cat)
		}
	}
}

func TestClosuresAddFromPointsFiltersByCutoffAndFlags(t *testing.T) {
	// 鏡射 forecasting.py:84-97 的 _known_closed_from_points：略過 !is_active、
	// !is_market_closed、以及 available_at > cutoff 的點；category 為 nil 的落到
	// 市場層級，否則落到 category 層級。
	cutoff := time.Date(2026, 7, 18, 15, 45, 0, 0, time.UTC)
	mk := func(tradeDate Date, closed, active bool, cat *string, availableAt time.Time) PricePoint {
		isActive := active
		p, err := NewPricePoint(PricePointInput{
			SeriesID: "s", MarketID: "moa:109", CropID: "moa:LA1",
			TradeDate: tradeDate, AvailableAt: availableAt,
			IsMarketClosed: closed, CategoryCode: cat, IsActive: &isActive,
		})
		if err != nil {
			t.Fatalf("NewPricePoint: %v", err)
		}
		return p
	}
	inWindow := cutoff.Add(-time.Hour)
	future := cutoff.Add(time.Hour)

	c := NewClosures()
	c.AddFromPoints([]PricePoint{
		mk(NewDate(2026, 7, 10), true, true, strptr("N04"), inWindow),  // 收
		mk(NewDate(2026, 7, 11), true, true, nil, inWindow),            // 收（市場層級）
		mk(NewDate(2026, 7, 12), false, true, strptr("N04"), inWindow), // 略過：非休市
		mk(NewDate(2026, 7, 13), true, false, strptr("N04"), inWindow), // 略過：非 active
		mk(NewDate(2026, 7, 14), true, true, strptr("N04"), future),    // 略過：cutoff 之後才可見
	}, cutoff)

	for _, tc := range []struct {
		day  Date
		cat  *string
		want bool
	}{
		{NewDate(2026, 7, 10), strptr("N04"), true},
		{NewDate(2026, 7, 10), strptr("N05"), false},
		{NewDate(2026, 7, 11), strptr("N05"), true}, // 市場層級涵蓋所有 category
		{NewDate(2026, 7, 12), strptr("N04"), false},
		{NewDate(2026, 7, 13), strptr("N04"), false},
		{NewDate(2026, 7, 14), strptr("N04"), false},
	} {
		if got := c.IsClosed("moa:109", tc.cat, tc.day); got != tc.want {
			t.Errorf("IsClosed(%s, %v) = %v，預期 %v", tc.day, tc.cat, got, tc.want)
		}
	}
}

func TestVerifiedCalendar(t *testing.T) {
	keys := []VerifiedCalendarKey{{MarketID: "moa:109", CategoryCode: "N04", Year: 2026}}
	v := NewVerifiedCalendar(keys)

	if !v.IsVerified("moa:109", strptr("N04"), 2026) {
		t.Error("已登記的 (market, category, year) 應為已驗證")
	}
	if v.IsVerified("moa:109", strptr("N04"), 2027) {
		t.Error("跨年沒有覆蓋時必須回 false——D+1~D+7 跨年時這是真實會走到的路徑")
	}
	if v.IsVerified("moa:109", strptr("N05"), 2026) {
		t.Error("同市場但沒登記的 category 必須回 false")
	}
	if v.IsVerified("moa:109", nil, 2026) {
		t.Error("category 為 nil 時必須回 false（鏡射上游 str(None)==\"None\" 永遠不會命中的行為）")
	}

	// 上游的 verified_calendar_market_categories=None 代表「完全跳過驗證」。
	if !UnenforcedVerifiedCalendar().IsVerified("moa:999", nil, 1999) {
		t.Error("未啟用驗證時一律視為已驗證")
	}
}
