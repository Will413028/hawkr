package forecast

import (
	"testing"
	"time"
)

// ==================== GroupUsability（ADR D2/D3） ====================
//
// 私人封存的 ADR 2026-08-07-pricebook-group-readiness-gate-resolution
// D2/D3 定義的三態矩陣。表格涵蓋 Status/AggregationMethod/MarketSupportCount
// 的每個獨立觸發條件，而不是只挑幾個代表值——因為三個 quality reason code
// 是各自獨立判斷（不是 if/elif chain），任何一條漏測都可能放過「兩個條件同時
// 成立卻只回一個 code」的迴歸。

func TestGroupUsability(t *testing.T) {
	const minSupport = 2

	for _, tc := range []struct {
		name          string
		status        PredictionStatus
		method        string
		marketSupport int
		wantUsability UsabilityStatus
		wantReasons   []QualityReasonCode
	}{
		{
			name:   "ready/two-stage/support>=min → usable, no reasons",
			status: PredictionStatusReady, method: AggregationMethodVolumeWeightedTwoStage, marketSupport: 2,
			wantUsability: UsabilityUsable, wantReasons: nil,
		},
		{
			name:   "ready/two-stage/support>min → usable, no reasons",
			status: PredictionStatusReady, method: AggregationMethodVolumeWeightedTwoStage, marketSupport: 5,
			wantUsability: UsabilityUsable, wantReasons: nil,
		},
		{
			name:   "partial/two-stage/support>=min → degraded, PARTIAL_VARIANT_COVERAGE",
			status: PredictionStatusPartial, method: AggregationMethodVolumeWeightedTwoStage, marketSupport: 2,
			wantUsability: UsabilityDegraded, wantReasons: []QualityReasonCode{QualityReasonPartialVariantCoverage},
		},
		{
			name:   "ready/equal-weight-fallback/support>=min → degraded, VOLUME_WEIGHT_FALLBACK",
			status: PredictionStatusReady, method: AggregationMethodEqualWeightFallback, marketSupport: 2,
			wantUsability: UsabilityDegraded, wantReasons: []QualityReasonCode{QualityReasonVolumeWeightFallback},
		},
		{
			name:   "ready/two-stage/0<support<min → degraded, INSUFFICIENT_MARKET_SUPPORT",
			status: PredictionStatusReady, method: AggregationMethodVolumeWeightedTwoStage, marketSupport: 1,
			wantUsability: UsabilityDegraded, wantReasons: []QualityReasonCode{QualityReasonInsufficientMarketSupport},
		},
		{
			name:   "partial AND equal-weight-fallback → degraded, BOTH reasons (independent checks, not if/elif)",
			status: PredictionStatusPartial, method: AggregationMethodEqualWeightFallback, marketSupport: 2,
			wantUsability: UsabilityDegraded,
			wantReasons:   []QualityReasonCode{QualityReasonPartialVariantCoverage, QualityReasonVolumeWeightFallback},
		},
		{
			name:   "partial AND fallback AND low support → degraded, ALL THREE reasons",
			status: PredictionStatusPartial, method: AggregationMethodEqualWeightFallback, marketSupport: 1,
			wantUsability: UsabilityDegraded,
			wantReasons: []QualityReasonCode{
				QualityReasonPartialVariantCoverage, QualityReasonVolumeWeightFallback, QualityReasonInsufficientMarketSupport,
			},
		},
		{
			name:   "insufficient_data → unusable regardless of method/support",
			status: PredictionStatusInsufficientData, method: AggregationMethodVolumeWeightedTwoStage, marketSupport: 5,
			wantUsability: UsabilityUnusable, wantReasons: nil,
		},
		{
			name:   "market_closed → unusable",
			status: PredictionStatusMarketClosed, method: AggregationMethodVolumeWeightedTwoStage, marketSupport: 0,
			wantUsability: UsabilityUnusable, wantReasons: nil,
		},
		{
			name:   "unavailable (freshness override) → unusable even if method/support still look healthy",
			status: PredictionStatusUnavailable, method: AggregationMethodVolumeWeightedTwoStage, marketSupport: 3,
			wantUsability: UsabilityUnusable, wantReasons: nil,
		},
		{
			name:   "unavailable with fallback still set from before the override → unusable, but VOLUME_WEIGHT_FALLBACK still reported (independent condition)",
			status: PredictionStatusUnavailable, method: AggregationMethodEqualWeightFallback, marketSupport: 3,
			wantUsability: UsabilityUnusable, wantReasons: []QualityReasonCode{QualityReasonVolumeWeightFallback},
		},
		{
			name:   "MarketSupportCount==0 → unusable even though that can't pair with a partial/ready status in practice",
			status: PredictionStatusReady, method: AggregationMethodVolumeWeightedTwoStage, marketSupport: 0,
			wantUsability: UsabilityUnusable, wantReasons: nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := GroupPoint{Status: tc.status, AggregationMethod: tc.method, MarketSupportCount: tc.marketSupport}
			gotUsability, gotReasons := GroupUsability(p, minSupport)
			if gotUsability != tc.wantUsability {
				t.Errorf("usability = %q, want %q", gotUsability, tc.wantUsability)
			}
			if !reasonCodesEqual(gotReasons, tc.wantReasons) {
				t.Errorf("reasons = %v, want %v", gotReasons, tc.wantReasons)
			}
		})
	}
}

func reasonCodesEqual(a, b []QualityReasonCode) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ==================== SeriesUsability（非大類路徑，沿用既有 PredictionStatus） ====================

func TestSeriesUsability(t *testing.T) {
	for _, tc := range []struct {
		status PredictionStatus
		want   UsabilityStatus
	}{
		{PredictionStatusReady, UsabilityUsable},
		{PredictionStatusPartial, UsabilityUsable},
		{PredictionStatusStale, UsabilityDegraded},
		{PredictionStatusInsufficientData, UsabilityUnusable},
		{PredictionStatusMarketClosed, UsabilityUnusable},
		{PredictionStatusNotPredicted, UsabilityUnusable},
		{PredictionStatusUnavailable, UsabilityUnusable},
		{PredictionStatus("brand_new_future_status"), UsabilityUnusable}, // 未知值保守處理，不能悄悄變 usable
	} {
		t.Run(string(tc.status), func(t *testing.T) {
			if got := SeriesUsability(tc.status); got != tc.want {
				t.Errorf("SeriesUsability(%q) = %q, want %q", tc.status, got, tc.want)
			}
		})
	}
}

// ==================== ObservedUsabilityFromFreshness（observed 路徑，無 PredictionStatus 概念） ====================

func TestObservedUsabilityFromFreshness(t *testing.T) {
	for _, tc := range []struct {
		freshness FreshnessLabel
		want      UsabilityStatus
	}{
		{FreshnessFresh, UsabilityUsable},
		{FreshnessStale, UsabilityDegraded},
		{FreshnessUnavailable, UsabilityUnusable},
	} {
		t.Run(string(tc.freshness), func(t *testing.T) {
			if got := ObservedUsabilityFromFreshness(tc.freshness); got != tc.want {
				t.Errorf("ObservedUsabilityFromFreshness(%q) = %q, want %q", tc.freshness, got, tc.want)
			}
		})
	}
}

// ==================== DatasetDigestFromSourceDigests（Rule 3：cross-row policy） ====================

func TestDatasetDigestFromSourceDigests(t *testing.T) {
	digestA := repeatHex('a')
	digestB := repeatHex('b')
	digestC := repeatHex('c')

	t.Run("no rows → nil (nothing to describe)", func(t *testing.T) {
		if got := DatasetDigestFromSourceDigests(nil); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})

	t.Run("single distinct digest collapses to that digest, unhashed", func(t *testing.T) {
		got := DatasetDigestFromSourceDigests([]string{digestA})
		want := "sha256:" + digestA
		if got == nil || *got != want {
			t.Errorf("got %v, want %q", got, want)
		}
	})

	t.Run("repeated rows with the same digest still collapse to the single-digest case", func(t *testing.T) {
		got := DatasetDigestFromSourceDigests([]string{digestA, digestA, digestA})
		want := "sha256:" + digestA
		if got == nil || *got != want {
			t.Errorf("got %v, want %q (duplicates must not accidentally hit the composite-hash branch)", got, want)
		}
	})

	t.Run("two distinct digests → sorted, joined, sha256'd composite", func(t *testing.T) {
		got := DatasetDigestFromSourceDigests([]string{digestB, digestA}) // deliberately unsorted input order
		want := "sha256:dfb484fa87938837327c83b4a7e3483a33a3dcd868655a1495e63b0886ae0b76"
		if got == nil || *got != want {
			t.Errorf("got %v, want %q (independently computed via python hashlib.sha256(('a'*64)+'|'+('b'*64)))", got, want)
		}
	})

	t.Run("input order does not change the result (sorted before hashing)", func(t *testing.T) {
		got1 := DatasetDigestFromSourceDigests([]string{digestA, digestB})
		got2 := DatasetDigestFromSourceDigests([]string{digestB, digestA})
		if got1 == nil || got2 == nil || *got1 != *got2 {
			t.Errorf("got1=%v got2=%v, want equal regardless of input order", got1, got2)
		}
	})

	t.Run("three distinct digests → composite matches independently-computed oracle", func(t *testing.T) {
		got := DatasetDigestFromSourceDigests([]string{digestC, digestA, digestB})
		want := "sha256:66fa93456cb49c6803f1e9a84ec0bf299b8982046e460d8167fd64c9a995fa26"
		if got == nil || *got != want {
			t.Errorf("got %v, want %q", got, want)
		}
	})

	t.Run("empty-string entries are ignored, not treated as a real digest", func(t *testing.T) {
		got := DatasetDigestFromSourceDigests([]string{"", digestA, ""})
		want := "sha256:" + digestA
		if got == nil || *got != want {
			t.Errorf("got %v, want %q", got, want)
		}
	})
}

// ==================== SourceDataAsOfUTC（Rule: sourceDataAsOf 是「來源截止」，
// 台北交易日的日終，wire format 仍是 UTC "Z" — docs/開發規範/Java 微服務開發
// 規範.md §2：時間一律 UTC，API 進出 YYYY-MM-DDTHH:mm:ssZ；issue 範例裡的
// +08:00 只是人類可讀範例，不能照抄成實際 wire format） ====================

func TestSourceDataAsOfUTC(t *testing.T) {
	// 2026-08-05 台北時間 23:59:59 = UTC 2026-08-05 15:59:59（台北固定 UTC+8，
	// 無 DST）。
	d := NewDate(2026, 8, 5)
	want := time.Date(2026, 8, 5, 15, 59, 59, 0, time.UTC)
	got := SourceDataAsOfUTC(d)
	if !got.Equal(want) {
		t.Errorf("SourceDataAsOfUTC(%s) = %s, want %s", d, got, want)
	}
	if got.Location() != time.UTC {
		t.Errorf("SourceDataAsOfUTC(%s).Location() = %v, want UTC (wire format must be Z, not +08:00)", d, got.Location())
	}
}

func repeatHex(r byte) string {
	b := make([]byte, 64)
	for i := range b {
		b[i] = r
	}
	return string(b)
}
