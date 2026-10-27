package domain

import "testing"

func TestComputeDealTemperatureCoverage(t *testing.T) {
	tests := []struct {
		name                       string
		refLower, refMid, refUpper float64
		buyerLower, buyerUpper     int64
		wantWithin, wantAtOrBelow  int64
		wantBand                   string
		wantReason                 string
	}{
		{
			// spec §2.2 的工作範例：F(58)=0.3667、F(70)=0.7769
			name: "設計書工作範例", refLower: 52, refMid: 61, refUpper: 74,
			buyerLower: 58, buyerUpper: 70,
			wantWithin: 4100, wantAtOrBelow: 7800, wantBand: "HOT",
		},
		{
			// 買家區間恰為 [P10, P90]：涵蓋率必為 80%、上界必為 90%
			name: "區間等於參考區間", refLower: 52, refMid: 61, refUpper: 74,
			buyerLower: 52, buyerUpper: 74,
			wantWithin: 8000, wantAtOrBelow: 9000, wantBand: "HOT",
		},
		{
			name: "完全低於參考區間", refLower: 52, refMid: 61, refUpper: 74,
			buyerLower: 30, buyerUpper: 40,
			wantReason: "RANGE_OUTSIDE_REFERENCE",
		},
		{
			name: "完全高於參考區間", refLower: 52, refMid: 61, refUpper: 74,
			buyerLower: 80, buyerUpper: 90,
			wantReason: "RANGE_OUTSIDE_REFERENCE",
		},
		{
			// 單點相接視為完全在外（spec §2.3 嚴格不等式判準）
			name: "上限恰等於參考下限", refLower: 52, refMid: 61, refUpper: 74,
			buyerLower: 40, buyerUpper: 52,
			wantReason: "RANGE_OUTSIDE_REFERENCE",
		},
		{
			// 下尾截斷：F(40) 取 0.10，所以 within = F(58)-0.10 = 0.2667 → 27%
			name: "下緣部分重疊，尾部截斷", refLower: 52, refMid: 61, refUpper: 74,
			buyerLower: 40, buyerUpper: 58,
			wantWithin: 2700, wantAtOrBelow: 3700, wantBand: "WARM",
		},
		{
			// 上尾截斷：F(90) 取 0.90，within = 0.90-F(70) = 0.1231 → 12%。
			// **BANDS_V2 的關鍵案例**：買家上限 90 遠高於行情上緣 74，對買家是
			// 最有利的情況（行情整段在你的上限之下），BANDS_V1 卻依嚴格界判 COLD
			// ——那是反指標。改吃寬鬆界後是 HOT。
			name: "上緣部分重疊，尾部截斷", refLower: 52, refMid: 61, refUpper: 74,
			buyerLower: 70, buyerUpper: 90,
			wantWithin: 1200, wantAtOrBelow: 9000, wantBand: "HOT",
		},
		{
			name: "編譯後三分位撞在一起", refLower: 3, refMid: 3, refUpper: 4,
			buyerLower: 3, buyerUpper: 4,
			wantReason: "QUANTILES_NOT_MONOTONIC",
		},
		{
			// 零寬度區間：數學上測度為 0，誠實回 0%。色帶看的是寬鬆界——上限
			// 恰在中位，50% 落在 BANDS_V2 的 WARM 區間（25%–75%）。
			name: "買家上下限相同", refLower: 52, refMid: 61, refUpper: 74,
			buyerLower: 61, buyerUpper: 61,
			wantWithin: 0, wantAtOrBelow: 5000, wantBand: "WARM",
		},
		{
			// COLD 的代表案例：上限 53 只比參考下限 52 高一點，行情幾乎整段
			// 高於買家願意付的價格。F(53)=0.10+(1/9)*0.40=0.1444 → 14%。
			name: "上限貼近參考下限", refLower: 52, refMid: 61, refUpper: 74,
			buyerLower: 52, buyerUpper: 53,
			wantWithin: 400, wantAtOrBelow: 1400, wantBand: "COLD",
		},
		{
			// 呼叫端不保證 lower<=upper（PRICEBOOK_GUIDED 路徑不走
			// validateBuyerRequestedRange），沒有守衛的話 within 會是負數
			name: "買家上下限顛倒", refLower: 52, refMid: 61, refUpper: 74,
			buyerLower: 70, buyerUpper: 58,
			wantReason: "BUYER_RANGE_NOT_SET",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, reason := ComputeDealTemperatureCoverage(
				tt.refLower, tt.refMid, tt.refUpper, tt.buyerLower, tt.buyerUpper)
			if reason != tt.wantReason {
				t.Fatalf("reason = %q，預期 %q", reason, tt.wantReason)
			}
			if tt.wantReason != "" {
				return
			}
			if got.WithinRangeBps != tt.wantWithin {
				t.Errorf("WithinRangeBps = %d，預期 %d", got.WithinRangeBps, tt.wantWithin)
			}
			if got.AtOrBelowUpperBps != tt.wantAtOrBelow {
				t.Errorf("AtOrBelowUpperBps = %d，預期 %d", got.AtOrBelowUpperBps, tt.wantAtOrBelow)
			}
			if got.Band != tt.wantBand {
				t.Errorf("Band = %q，預期 %q", got.Band, tt.wantBand)
			}
		})
	}
}

func TestComputeDealTemperatureFromPoint(t *testing.T) {
	cases := []struct {
		name       string
		refMid     float64
		lo, hi     int64
		wantWithin int64
		wantBand   string
	}{
		{"參考價落在買家區間內", 106, 90, 120, 10000, DealTemperatureBandHot},
		{"參考價高於買家上限", 106, 80, 100, 0, DealTemperatureBandCold},
		{"參考價低於買家下限", 106, 110, 130, 0, DealTemperatureBandHot},
		{"剛好等於上限仍算在內", 106, 90, 106, 10000, DealTemperatureBandHot},
	}
	for _, c := range cases {
		got, reason := ComputeDealTemperatureFromPoint(c.refMid, c.lo, c.hi)
		if reason != "" {
			t.Fatalf("%s: 非預期 reason %q", c.name, reason)
		}
		if got.WithinRangeBps != c.wantWithin {
			t.Errorf("%s: WithinRangeBps = %d，預期 %d", c.name, got.WithinRangeBps, c.wantWithin)
		}
		if got.Band != c.wantBand {
			t.Errorf("%s: Band = %q，預期 %q", c.name, got.Band, c.wantBand)
		}
	}
}

func TestComputeDealTemperatureFromPoint_OnlyExtremes(t *testing.T) {
	for _, mid := range []float64{50, 99.9, 100, 100.1, 150} {
		got, _ := ComputeDealTemperatureFromPoint(mid, 100, 120)
		for _, v := range []int64{got.WithinRangeBps, got.AtOrBelowUpperBps} {
			if v != 0 && v != 10000 {
				t.Errorf("refMid=%v 產生中間值 %d，單點估計不該有展開", mid, v)
			}
		}
	}
}

func TestComputeDealTemperatureFromPoint_InvertedBuyerRange(t *testing.T) {
	_, reason := ComputeDealTemperatureFromPoint(106, 120, 90)
	if reason != DealTemperatureReasonBuyerRangeNotSet {
		t.Errorf("顛倒區間 reason = %q，預期 BUYER_RANGE_NOT_SET", reason)
	}
}
