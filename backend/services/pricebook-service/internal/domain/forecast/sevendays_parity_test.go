package forecast

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type calendarFixture struct {
	ClosedByMarket         map[string][]string `json:"closed_by_market"`
	ClosedByMarketCategory []struct {
		MarketID     string   `json:"market_id"`
		CategoryCode string   `json:"category_code"`
		Dates        []string `json:"dates"`
	} `json:"closed_by_market_category"`
	VerifiedCalendar []struct {
		MarketID     string `json:"market_id"`
		CategoryCode string `json:"category_code"`
		Year         int    `json:"year"`
	} `json:"verified_calendar"`
}

func loadParityCalendar(t *testing.T) (*Closures, VerifiedCalendar) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(parityTestdataDir, "calendar.json"))
	if err != nil {
		t.Fatalf("讀 calendar.json 失敗: %v（先跑 Task 1 產生它）", err)
	}
	var fx calendarFixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("解析 calendar.json 失敗: %v", err)
	}
	closures := NewClosures()
	for marketID, dates := range fx.ClosedByMarket {
		for _, s := range dates {
			d, err := ParseDate(s)
			if err != nil {
				t.Fatalf("解析 closed_by_market 日期 %q 失敗: %v", s, err)
			}
			closures.AddMarket(marketID, d)
		}
	}
	for _, e := range fx.ClosedByMarketCategory {
		for _, s := range e.Dates {
			d, err := ParseDate(s)
			if err != nil {
				t.Fatalf("解析 closed_by_market_category 日期 %q 失敗: %v", s, err)
			}
			closures.AddMarketCategory(e.MarketID, e.CategoryCode, d)
		}
	}
	keys := make([]VerifiedCalendarKey, 0, len(fx.VerifiedCalendar))
	for _, e := range fx.VerifiedCalendar {
		keys = append(keys, VerifiedCalendarKey{MarketID: e.MarketID, CategoryCode: e.CategoryCode, Year: e.Year})
	}
	return closures, NewVerifiedCalendar(keys)
}

// runSevenDaysParity 用 SevenDays 重算整份 fixture，逐欄位比對。
//
// 與 parity_test.go 的 runParityFixture 的分工：那支驗「給定特徵向量，數學算對了」
// （P3 範圍），這支驗「給定歷史與日曆，分支選對了、狀態標對了」（P4 範圍）。
// 最關鍵的差異是 not_applicable（休市）case：P3 只能斷言 fixture 的 features
// 為 null（parity_test.go:156-161 的註解說明那是刻意的範圍排除），這支是真的
// 讓 Go 自己判出休市。
func runSevenDaysParity(t *testing.T, fixtureFile, modelFile string, wantNotApplicable, wantPredicted int) {
	t.Helper()
	history := loadParityHistory(t)
	closures, verified := loadParityCalendar(t)

	raw, err := os.ReadFile(filepath.Join(parityTestdataDir, fixtureFile))
	if err != nil {
		t.Fatalf("讀 %s 失敗: %v", fixtureFile, err)
	}
	var fx parityFixtureFile
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("解析 %s 失敗: %v", fixtureFile, err)
	}
	artifact, err := LoadArtifact(filepath.Join(parityTestdataDir, modelFile))
	if err != nil {
		t.Fatalf("LoadArtifact(%s) 失敗: %v", modelFile, err)
	}
	origin, err := time.Parse(time.RFC3339, fx.Source.ForecastOriginLocal)
	if err != nil {
		t.Fatalf("解析 forecast_origin_local 失敗: %v", err)
	}

	got, err := SevenDays(SevenDaysInput{
		Artifact:        artifact,
		HistoryBySeries: history,
		Origin:          origin,
		Closures:        closures,
		Verified:        verified,
	})
	if err != nil {
		t.Fatalf("SevenDays 失敗: %v", err)
	}
	if len(got) != len(fx.Cases) {
		t.Fatalf("SevenDays 回 %d 列，fixture 有 %d case", len(got), len(fx.Cases))
	}

	byKey := map[string]SeriesPoint{}
	for _, p := range got {
		byKey[fmt.Sprintf("%s|%d", p.SeriesID, p.HorizonDays)] = p
	}

	gotNotApplicable, gotPredicted := 0, 0
	for _, c := range fx.Cases {
		c := c
		t.Run(fmt.Sprintf("%s_D+%d", c.SeriesID, c.HorizonDays), func(t *testing.T) {
			p, ok := byKey[fmt.Sprintf("%s|%d", c.SeriesID, c.HorizonDays)]
			if !ok {
				t.Fatalf("SevenDays 沒有產出 %s D+%d", c.SeriesID, c.HorizonDays)
			}
			if got, want := string(p.PointStatus), c.PredictionStatus; got != want {
				t.Errorf("prediction_status = %q，預期 %q", got, want)
			}
			wantMissing := ""
			if c.MissingReason != nil {
				wantMissing = *c.MissingReason
			}
			if got := string(p.MissingReason); got != wantMissing {
				t.Errorf("missing_reason = %q，預期 %q", got, wantMissing)
			}
			if p.CalendarStatus != c.CalendarStatus {
				t.Errorf("calendar_status = %q，預期 %q", p.CalendarStatus, c.CalendarStatus)
			}
			if p.MarketStatus != c.MarketStatus {
				t.Errorf("market_status = %q，預期 %q", p.MarketStatus, c.MarketStatus)
			}
			if p.ModelKind != c.ModelKind {
				t.Errorf("model_kind = %q，預期 %q", p.ModelKind, c.ModelKind)
			}
			if p.TargetDate.String() != c.TargetDate {
				t.Errorf("target_date = %s，預期 %s", p.TargetDate, c.TargetDate)
			}
			if c.PredictionStatus == string(PointStatusNotApplicable) {
				if p.P10 == nil || p.P50 == nil || p.P90 == nil {
					t.Errorf("休市點應帶 conditional 價格，實際 p10=%v p50=%v p90=%v", p.P10, p.P50, p.P90)
				}
			} else {
				assertClosePtr(t, "price_p10", p.P10, c.PriceP10, 1e-9)
				assertClosePtr(t, "price_p50", p.P50, c.PriceP50, 1e-9)
				assertClosePtr(t, "price_p90", p.P90, c.PriceP90, 1e-9)
			}
		})
		switch c.PredictionStatus {
		case string(PointStatusNotApplicable):
			gotNotApplicable++
		case string(PointStatusPredicted):
			gotPredicted++
		}
	}
	// 守門：fixture 若哪天重新產生後休市 case 消失，上面的迴圈仍會全綠。
	if gotNotApplicable != wantNotApplicable || gotPredicted != wantPredicted {
		t.Fatalf("case 分佈 = not_applicable=%d predicted=%d，預期 not_applicable=%d predicted=%d",
			gotNotApplicable, gotPredicted, wantNotApplicable, wantPredicted)
	}
}

// assertClosePtr 比較兩個可為 nil 的 float64：nil 與非 nil 必須一致。
func assertClosePtr(t *testing.T, label string, got *float64, want *float64, tolerance float64) {
	t.Helper()
	switch {
	case got == nil && want == nil:
		return
	case got == nil:
		t.Errorf("%s = nil，預期 %v", label, *want)
	case want == nil:
		t.Errorf("%s = %v，預期 nil（沒有價格的點不得帶價格）", label, *got)
	default:
		assertClose(t, label, *got, *want, tolerance)
	}
}

func TestSevenDaysParityDemo(t *testing.T) {
	runSevenDaysParity(t, "parity-fixture.json", "model-demo.json", 1, 13)
}

func TestSevenDaysParityFallback(t *testing.T) {
	runSevenDaysParity(t, "parity-fixture-fallback.json", "model-fallback.json", 1, 13)
}
