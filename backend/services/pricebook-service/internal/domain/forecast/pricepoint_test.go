package forecast

import (
	"testing"
	"time"
)

func price(v float64) *float64 { return &v }

func TestNewPricePointRejectsNegativePrice(t *testing.T) {
	_, err := NewPricePoint(PricePointInput{
		SeriesID: "moa:104:LA1", MarketID: "moa:104", CropID: "moa:LA1",
		TradeDate:    NewDate(2025, 1, 7),
		AveragePrice: price(-1.0),
		AvailableAt:  time.Date(2025, 1, 7, 18, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatal("負數 average_price 應回錯誤（mirrors features.py:121-122）")
	}
}

func TestNewPricePointDefaultsIsActiveTrueAndQualityObserved(t *testing.T) {
	p, err := NewPricePoint(PricePointInput{
		SeriesID: "moa:104:LA1", MarketID: "moa:104", CropID: "moa:LA1",
		TradeDate:    NewDate(2025, 1, 7),
		AveragePrice: price(17.0),
		AvailableAt:  time.Date(2025, 1, 7, 18, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("建立 PricePoint 失敗: %v", err)
	}
	if !p.IsActive {
		t.Error("未指定 IsActive 時預設應為 true（mirrors dataclass default is_active=True）")
	}
	if p.AvailabilityQuality != "observed" {
		t.Errorf("AvailabilityQuality = %q，預設應為 observed", p.AvailabilityQuality)
	}
}

func TestNewPricePointRejectsUnknownAvailabilityQuality(t *testing.T) {
	_, err := NewPricePoint(PricePointInput{
		SeriesID: "moa:104:LA1", MarketID: "moa:104", CropID: "moa:LA1",
		TradeDate:           NewDate(2025, 1, 7),
		AveragePrice:        price(17.0),
		AvailableAt:         time.Date(2025, 1, 7, 18, 0, 0, 0, time.UTC),
		AvailabilityQuality: "guessed",
	})
	if err == nil {
		t.Fatal("未知的 availability_quality 應回錯誤（mirrors features.py:114-117 的 allowed_quality 集合）")
	}
}
