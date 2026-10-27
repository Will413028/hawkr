package forecast

import (
	"math"
	"testing"
)

// readyVariant 產一個 ready 的子品項點。
func readyVariant(marketID, cropID string, horizon int, p10, p50, p90 float64) VariantPoint {
	return VariantPoint{
		MarketID: marketID, CropID: cropID, HorizonDays: horizon,
		TargetDate: NewDate(2026, 7, 18).AddDays(horizon),
		Status:     PredictionStatusReady,
		P10:        price(p10), P50: price(p50), P90: price(p90),
	}
}

// sevenHorizons 把單一 horizon 的產生器展開成完整七日（AggregateGroup 要求
// 輸入涵蓋 1..7）。
func sevenHorizons(gen func(h int) []VariantPoint) []VariantPoint {
	var out []VariantPoint
	for h := 1; h <= 7; h++ {
		out = append(out, gen(h)...)
	}
	return out
}

func TestAggregateGroupTwoStageWeighting(t *testing.T) {
	// 市場 A：子品項 x（量 300，p50=30）、y（量 100，p50=50）
	//         → 市場內加權 = (30*300 + 50*100) / 400 = 35
	// 市場 B：子品項 x（量 100，p50=60）
	//         → 市場內加權 = 60
	// 跨市場：市場 A 量 400、市場 B 量 100
	//         → (35*400 + 60*100) / 500 = 40
	points := sevenHorizons(func(h int) []VariantPoint {
		return []VariantPoint{
			readyVariant("mA", "x", h, 20, 30, 40),
			readyVariant("mA", "y", h, 40, 50, 60),
			readyVariant("mB", "x", h, 50, 60, 70),
		}
	})
	vols := GroupVolumes{
		ByVariantMarket: map[VariantMarketKey]float64{
			{CropID: "x", MarketID: "mA"}: 300,
			{CropID: "y", MarketID: "mA"}: 100,
			{CropID: "x", MarketID: "mB"}: 100,
		},
		ByMarket: map[string]float64{"mA": 400, "mB": 100},
	}

	got, err := AggregateGroup(points, vols, 2)
	if err != nil {
		t.Fatalf("AggregateGroup: %v", err)
	}
	if len(got) != 7 {
		t.Fatalf("列數 = %d，預期 7", len(got))
	}
	g := got[0]
	if g.Status != PredictionStatusReady {
		t.Errorf("status = %q，預期 %q（兩個子品項全參與）", g.Status, PredictionStatusReady)
	}
	if g.AggregationMethod != AggregationMethodVolumeWeightedTwoStage {
		t.Errorf("aggregation_method = %q，預期 %q", g.AggregationMethod, AggregationMethodVolumeWeightedTwoStage)
	}
	if g.EffectiveVariantCount != 2 || g.TotalVariantCount != 2 || g.MarketSupportCount != 2 {
		t.Errorf("計數 = (effective=%d total=%d markets=%d)，預期 (2,2,2)", g.EffectiveVariantCount, g.TotalVariantCount, g.MarketSupportCount)
	}
	// p50：(30*300 + 50*100)/400 = 35 → (35*400 + 60*100)/500 = 40
	assertClose(t, "group p50", *g.P50, 40, 1e-12)
	// p10：(20*300 + 40*100)/400 = 25 → (25*400 + 50*100)/500 = 30
	assertClose(t, "group p10", *g.P10, 30, 1e-12)
	// p90：(40*300 + 60*100)/400 = 45 → (45*400 + 70*100)/500 = 50
	assertClose(t, "group p90", *g.P90, 50, 1e-12)
	if !(*g.P10 <= *g.P50 && *g.P50 <= *g.P90) {
		t.Error("加權後仍必須單調")
	}
}

func TestAggregateGroupQuantilesWeightedIndependentlyNotDerivedFromP50(t *testing.T) {
	// §8 line 395：P10／P90 各自獨立加權，不從 P50 推導。
	// 用兩個子品項、區間寬度差很多的資料：若 P10/P90 是從 P50 推的（例如
	// 用某個固定比例），寬度會被抹平。
	points := sevenHorizons(func(h int) []VariantPoint {
		return []VariantPoint{
			readyVariant("mA", "narrow", h, 49, 50, 51),
			readyVariant("mA", "wide", h, 10, 50, 90),
		}
	})
	vols := GroupVolumes{
		ByVariantMarket: map[VariantMarketKey]float64{
			{CropID: "narrow", MarketID: "mA"}: 1,
			{CropID: "wide", MarketID: "mA"}:   1,
		},
		ByMarket: map[string]float64{"mA": 2},
	}
	got, err := AggregateGroup(points, vols, 2)
	if err != nil {
		t.Fatalf("AggregateGroup: %v", err)
	}
	assertClose(t, "p50", *got[0].P50, 50, 1e-12)
	assertClose(t, "p10", *got[0].P10, 29.5, 1e-12)
	assertClose(t, "p90", *got[0].P90, 70.5, 1e-12)
}

func TestAggregateGroupEqualWeightFallbackWhenAllVolumesZero(t *testing.T) {
	points := sevenHorizons(func(h int) []VariantPoint {
		return []VariantPoint{
			readyVariant("mA", "x", h, 20, 30, 40),
			readyVariant("mB", "x", h, 40, 50, 60),
		}
	})
	got, err := AggregateGroup(points, GroupVolumes{}, 1) // 完全沒有成交量資料
	if err != nil {
		t.Fatalf("AggregateGroup: %v", err)
	}
	if got[0].AggregationMethod != AggregationMethodEqualWeightFallback {
		t.Errorf("aggregation_method = %q，預期 %q（§8 line 392：權重全 0 要標出來，不能靜默退化）",
			got[0].AggregationMethod, AggregationMethodEqualWeightFallback)
	}
	assertClose(t, "等權 p50", *got[0].P50, 40, 1e-12) // (30+50)/2
}

func TestAggregateGroupSingleParticipantMissingWeightDoesNotMislabelHorizon(t *testing.T) {
	// P4 review followup：市場 mB 只有一個子品項（z），且該子品項的成交量沒有
	// 登記進 vols.ByVariantMarket（map 零值 0）。WeightedAverage 對單一參與者
	// 退成等權分支——但單一參與者的等權平均在數學上就等於加權平均本身
	// （sum/1 == v），不是真的退化。修正前這裡會讓整個 horizon 誤標成
	// equal_weight_fallback，即使市場內（mA）與跨市場層都是真實成交量加權。
	points := sevenHorizons(func(h int) []VariantPoint {
		return []VariantPoint{
			readyVariant("mA", "x", h, 20, 30, 40),
			readyVariant("mA", "y", h, 40, 50, 60),
			readyVariant("mB", "z", h, 50, 60, 70), // 唯一參與者，成交量未登記
		}
	})
	vols := GroupVolumes{
		ByVariantMarket: map[VariantMarketKey]float64{
			{CropID: "x", MarketID: "mA"}: 300,
			{CropID: "y", MarketID: "mA"}: 100,
			// {CropID: "z", MarketID: "mB"} 刻意不登記
		},
		ByMarket: map[string]float64{"mA": 400, "mB": 100}, // 跨市場層有真實權重
	}
	got, err := AggregateGroup(points, vols, 3)
	if err != nil {
		t.Fatalf("AggregateGroup: %v", err)
	}
	if got[0].AggregationMethod != AggregationMethodVolumeWeightedTwoStage {
		t.Errorf("aggregation_method = %q，預期 %q（單一參與者缺成交量不該讓整個 horizon 誤標成 fallback）",
			got[0].AggregationMethod, AggregationMethodVolumeWeightedTwoStage)
	}
}

func TestAggregateGroupOnlyReadyVariantsParticipate(t *testing.T) {
	// §8 line 391：只有 ready 的子品項參與加權；被擋掉的不以 0 計入。
	// 若被擋掉的以 0 計入，p50 會被拉低成 15（(30+0)/2）而不是 30。
	points := sevenHorizons(func(h int) []VariantPoint {
		blocked := readyVariant("mA", "y", h, 0, 0, 0)
		blocked.Status = PredictionStatusInsufficientData
		blocked.P10, blocked.P50, blocked.P90 = nil, nil, nil
		return []VariantPoint{readyVariant("mA", "x", h, 20, 30, 40), blocked}
	})
	vols := GroupVolumes{
		ByVariantMarket: map[VariantMarketKey]float64{
			{CropID: "x", MarketID: "mA"}: 100,
			{CropID: "y", MarketID: "mA"}: 900, // 大權重，若參與會嚴重拉偏
		},
		ByMarket: map[string]float64{"mA": 1000},
	}
	got, err := AggregateGroup(points, vols, 2)
	if err != nil {
		t.Fatalf("AggregateGroup: %v", err)
	}
	assertClose(t, "p50", *got[0].P50, 30, 1e-12)
	if got[0].Status != PredictionStatusPartial {
		t.Errorf("status = %q，預期 %q（2 個子品項只有 1 個 ready）", got[0].Status, PredictionStatusPartial)
	}
	if got[0].EffectiveVariantCount != 1 || got[0].TotalVariantCount != 2 {
		t.Errorf("計數 = (%d/%d)，預期 (1/2)", got[0].EffectiveVariantCount, got[0].TotalVariantCount)
	}
}

func TestAggregateGroupZeroEffectiveVariants(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     PredictionStatus
		wantStatus PredictionStatus
		wantReason MissingReason
	}{
		{"全部休市 → 大類也是 market_closed（§8 沒寫，照上游 all_child_markets_closed 的區分）",
			PredictionStatusMarketClosed, PredictionStatusMarketClosed, MissingReasonMarketClosed},
		{"全部資料不足 → insufficient_data（§8 line 393）",
			PredictionStatusInsufficientData, PredictionStatusInsufficientData, MissingReasonInsufficientRecentData},
	} {
		t.Run(tc.name, func(t *testing.T) {
			points := sevenHorizons(func(h int) []VariantPoint {
				p := readyVariant("mA", "x", h, 0, 0, 0)
				p.Status = tc.status
				p.P10, p.P50, p.P90 = nil, nil, nil
				p.MissingReason = tc.wantReason
				return []VariantPoint{p}
			})
			got, err := AggregateGroup(points, GroupVolumes{}, 1)
			if err != nil {
				t.Fatalf("AggregateGroup: %v", err)
			}
			if got[0].Status != tc.wantStatus {
				t.Errorf("status = %q，預期 %q", got[0].Status, tc.wantStatus)
			}
			if got[0].P50 != nil {
				t.Error("有效子品項數 0 時不得回價格")
			}
			if got[0].EffectiveVariantCount != 0 {
				t.Errorf("effective = %d，預期 0", got[0].EffectiveVariantCount)
			}
		})
	}
}

func TestAggregateGroupRejectsIncompleteHorizonSet(t *testing.T) {
	// 與 EvaluateHorizonSet（Task 7）同一條不變量，但型別不同所以各自實作：
	// merge 多個子品項的列是「七日缺日或重複」唯一真正可達的路徑。
	partial := []VariantPoint{readyVariant("mA", "x", 1, 20, 30, 40)}
	if _, err := AggregateGroup(partial, GroupVolumes{}, 1); err == nil {
		t.Fatal("只有 D+1 的輸入必須報錯——大類的七日表格不得缺格")
	}
	dup := sevenHorizons(func(h int) []VariantPoint {
		p := readyVariant("mA", "x", h, 20, 30, 40)
		p.TargetDate = NewDate(2026, 7, 19) // 七個 horizon 全指同一天
		return []VariantPoint{p}
	})
	if _, err := AggregateGroup(dup, GroupVolumes{}, 1); err == nil {
		t.Fatal("target_date 重複必須報錯")
	}
}

func TestAggregateGroupRejectsNonFiniteWeights(t *testing.T) {
	points := sevenHorizons(func(h int) []VariantPoint {
		return []VariantPoint{readyVariant("mA", "x", h, 20, 30, 40)}
	})
	vols := GroupVolumes{
		ByVariantMarket: map[VariantMarketKey]float64{{CropID: "x", MarketID: "mA"}: math.NaN()},
		ByMarket:        map[string]float64{"mA": 100},
	}
	if _, err := AggregateGroup(points, vols, 1); err == nil {
		t.Fatal("NaN 權重必須報錯而不是靜默產出 NaN 價格——NUMERIC 的 NaN 在排序上大於一切有限值")
	}
}

// TestAggregateGroupStage2IsDeterministicAcrossRepeatedCalls 守住 stage 2 的
// sort.Strings(marketIDs) 這條不變量。
//
// 為什麼用逐位元比較（!=）而不是 assertClose 容差比較：這裡要測的不是「值算得
// 對不對」（那已經由 TestAggregateGroupTwoStageWeighting 守），而是「同一組輸入
// 重複呼叫，結果是否每次都完全相同」。3 個市場、非對稱、非 2 的冪次權重時，
// map 迭代順序不同確實會讓浮點加總的排列產生差異——但實測（200,000 組隨機
// (3 市場值, 3 權重) 窮舉 6 種排列重算）約 32% 組合出現排列相依差異，最大相對
// 誤差只有 4.4e-16（ULP 級），比任何合理的 assertClose 容差（例如本檔慣用的
// 1e-12）小 4 個數量級。用容差斷言測這條不變量，不管 sort.Strings 在不在都會
// 通過——容差窗把該抓的差異蓋過去了。所以這裡刻意不用 assertClose，改成直接
// 比較 float64 是否逐位元相同。**不要把這個斷言「順手」改成 assertClose**：
// 那會讓這個測試永遠測不出排序被拿掉。
func TestAggregateGroupStage2IsDeterministicAcrossRepeatedCalls(t *testing.T) {
	// 3+ 個市場、非對稱、非 2 的冪次權重，讓浮點加總順序真的可能影響結果
	// （若只有 <=2 個市場，兩項加法逐位元可交換，測不出這條不變量）。
	points := sevenHorizons(func(h int) []VariantPoint {
		return []VariantPoint{
			readyVariant("mA", "x", h, 10, 20, 30),
			readyVariant("mB", "x", h, 15, 25, 35),
			readyVariant("mC", "x", h, 40, 50, 60),
		}
	})
	vols := GroupVolumes{
		ByVariantMarket: map[VariantMarketKey]float64{
			{CropID: "x", MarketID: "mA"}: 1,
			{CropID: "x", MarketID: "mB"}: 1,
			{CropID: "x", MarketID: "mC"}: 1,
		},
		ByMarket: map[string]float64{"mA": 574.4393126237192, "mB": 678.1534676385334, "mC": 264.73523684162325},
	}

	first, err := AggregateGroup(points, vols, 1)
	if err != nil {
		t.Fatalf("AggregateGroup: %v", err)
	}
	for i := 0; i < 50; i++ {
		got, err := AggregateGroup(points, vols, 1)
		if err != nil {
			t.Fatalf("AggregateGroup: %v", err)
		}
		if *got[0].P10 != *first[0].P10 || *got[0].P50 != *first[0].P50 || *got[0].P90 != *first[0].P90 {
			t.Fatalf("第 %d 次呼叫結果與第一次不是逐位元相同：\n  first = (%v,%v,%v)\n  got   = (%v,%v,%v)",
				i, *first[0].P10, *first[0].P50, *first[0].P90, *got[0].P10, *got[0].P50, *got[0].P90)
		}
	}
}

func mustDate(t *testing.T, s string) Date {
	t.Helper()
	d, err := ParseDate(s)
	if err != nil {
		t.Fatalf("ParseDate(%q): %v", s, err)
	}
	return d
}

func TestAggregateGroupObservations_TwoStageWeighting(t *testing.T) {
	d1 := mustDate(t, "2026-08-01")
	points := []DailyVariantPrice{
		// Market A, two crops trading the same day — stage 1 weights by
		// each crop's volume *within market A*.
		{MarketID: "moa:A", CropID: "moa:crop1", TradeDate: d1, Price: 100},
		{MarketID: "moa:A", CropID: "moa:crop2", TradeDate: d1, Price: 200},
		// Market B, one crop trading the same day.
		{MarketID: "moa:B", CropID: "moa:crop1", TradeDate: d1, Price: 300},
	}
	vols := GroupVolumes{
		ByVariantMarket: map[VariantMarketKey]float64{
			{CropID: "moa:crop1", MarketID: "moa:A"}: 300,
			{CropID: "moa:crop2", MarketID: "moa:A"}: 100,
			{CropID: "moa:crop1", MarketID: "moa:B"}: 50,
		},
		ByMarket: map[string]float64{"moa:A": 400, "moa:B": 50},
	}

	got := AggregateGroupObservations(points, vols)

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1 (single trade date)", len(got))
	}
	// Stage 1, market A: (100*300 + 200*100) / 400 = 125.
	// Stage 1, market B: 300 (single participant).
	// Stage 2, across markets: (125*400 + 300*50) / 450 = 138.888...
	want := (125.0*400 + 300.0*50) / 450.0
	if got[0].TradeDate != d1 {
		t.Errorf("TradeDate = %s, want %s", got[0].TradeDate, d1)
	}
	if got[0].Value == nil || math.Abs(*got[0].Value-want) > 1e-9 {
		t.Errorf("Value = %v, want %v", got[0].Value, want)
	}
}

func TestAggregateGroupObservations_MultipleDatesSortedAscending(t *testing.T) {
	d1 := mustDate(t, "2026-08-01")
	d2 := mustDate(t, "2026-08-02")
	points := []DailyVariantPrice{
		{MarketID: "moa:A", CropID: "moa:crop1", TradeDate: d2, Price: 200},
		{MarketID: "moa:A", CropID: "moa:crop1", TradeDate: d1, Price: 100},
	}
	vols := GroupVolumes{
		ByVariantMarket: map[VariantMarketKey]float64{{CropID: "moa:crop1", MarketID: "moa:A"}: 1},
		ByMarket:        map[string]float64{"moa:A": 1},
	}

	got := AggregateGroupObservations(points, vols)

	if len(got) != 2 || got[0].TradeDate != d1 || got[1].TradeDate != d2 {
		t.Fatalf("got = %+v, want [d1, d2] in ascending order", got)
	}
}

func TestAggregateGroupObservations_EmptyInputReturnsEmpty(t *testing.T) {
	got := AggregateGroupObservations(nil, GroupVolumes{})
	if len(got) != 0 {
		t.Errorf("len(got) = %d, want 0", len(got))
	}
}

func TestAggregateGroupObservationsCarriesVolume(t *testing.T) {
	points := []DailyVariantPrice{
		{MarketID: "mA", CropID: "x", TradeDate: NewDate(2026, 7, 20), Price: 30, Volume: price(300)},
		{MarketID: "mA", CropID: "y", TradeDate: NewDate(2026, 7, 20), Price: 50, Volume: price(100)},
		{MarketID: "mB", CropID: "x", TradeDate: NewDate(2026, 7, 20), Price: 60, Volume: price(100)},
	}
	vols := GroupVolumes{
		ByVariantMarket: map[VariantMarketKey]float64{
			{CropID: "x", MarketID: "mA"}: 300, {CropID: "y", MarketID: "mA"}: 100, {CropID: "x", MarketID: "mB"}: 100,
		},
		ByMarket: map[string]float64{"mA": 400, "mB": 100},
	}
	out := AggregateGroupObservations(points, vols)
	if len(out) != 1 {
		t.Fatalf("want 1 point, got %d", len(out))
	}
	if out[0].Volume == nil {
		t.Fatal("want non-nil Volume")
	}
	// Sum of the raw contributing rows' volumes, not the (unrelated) spatial
	// weighting inputs — 300+100+100 = 500.
	if *out[0].Volume != 500 {
		t.Errorf("Volume = %v, want 500", *out[0].Volume)
	}
}

func TestAggregateGroupObservationsNoParticipantsVolumeNil(t *testing.T) {
	out := aggregateOneDate(NewDate(2026, 7, 20), nil, GroupVolumes{ByVariantMarket: map[VariantMarketKey]float64{}, ByMarket: map[string]float64{}})
	if out.Value != nil {
		t.Error("want nil Value for zero participants")
	}
	if out.Volume != nil {
		t.Error("want nil Volume for zero participants")
	}
}

// TestAggregateGroupObservationsNilVolumeVsRealZeroVolume pins the I3 fix
// (final review, 2026-08-08): a row's Volume==nil ("this row's volume_kg is
// SQL NULL, genuinely unknown") must NOT be summed as a 0 contribution, and
// a date where EVERY contributing row has Volume==nil must produce a nil
// GroupObservedPoint.Volume — not a fabricated &0.0 — distinct from a date
// that has a real, known zero (Volume: price(0)). Before the fix,
// trend_summary.go's groupPath constructed DailyVariantPrice.Volume as a
// plain float64 defaulting unknown rows to 0.0, which this struct's old
// aggregateOneDate had no way to tell apart from a genuine zero.
func TestAggregateGroupObservationsNilVolumeVsRealZeroVolume(t *testing.T) {
	unknownDate := NewDate(2026, 7, 20)
	realZeroDate := NewDate(2026, 7, 21)
	points := []DailyVariantPrice{
		// unknownDate: both contributing rows have a genuinely unknown
		// (NULL) volume_kg — Volume left nil, not zero-filled.
		{MarketID: "mA", CropID: "x", TradeDate: unknownDate, Price: 30, Volume: nil},
		{MarketID: "mB", CropID: "x", TradeDate: unknownDate, Price: 40, Volume: nil},
		// realZeroDate: one row has a real, known zero volume.
		{MarketID: "mA", CropID: "x", TradeDate: realZeroDate, Price: 35, Volume: price(0)},
	}
	vols := GroupVolumes{
		ByVariantMarket: map[VariantMarketKey]float64{
			{CropID: "x", MarketID: "mA"}: 1, {CropID: "x", MarketID: "mB"}: 1,
		},
		ByMarket: map[string]float64{"mA": 1, "mB": 1},
	}

	out := AggregateGroupObservations(points, vols)
	if len(out) != 2 {
		t.Fatalf("len(out) = %d, want 2 (two distinct trade dates)", len(out))
	}

	byDate := map[Date]GroupObservedPoint{}
	for _, p := range out {
		byDate[p.TradeDate] = p
	}

	unknown := byDate[unknownDate]
	if unknown.Value == nil {
		t.Fatal("want non-nil Value on unknownDate (rows had real prices, only Volume was unknown)")
	}
	if unknown.Volume != nil {
		t.Errorf("Volume = %v, want nil (every contributing row had Volume==nil — must not fabricate a 0)", *unknown.Volume)
	}

	realZero := byDate[realZeroDate]
	if realZero.Volume == nil {
		t.Fatal("want non-nil Volume on realZeroDate (a row had a real, known Volume: price(0))")
	}
	if *realZero.Volume != 0 {
		t.Errorf("Volume = %v, want 0 (real zero)", *realZero.Volume)
	}
}
