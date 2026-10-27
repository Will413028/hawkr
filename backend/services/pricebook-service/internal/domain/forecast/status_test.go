package forecast

import (
	"testing"
	"time"
)

func TestOuterStatusMappingTable(t *testing.T) {
	for _, tc := range []struct {
		inner      PointStatus
		reason     MissingReason
		gatePassed bool
		gateReason MissingReason
		want       PredictionStatus
		wantReason MissingReason
	}{
		{PointStatusNotApplicable, MissingReasonMarketClosed, true, "", PredictionStatusMarketClosed, MissingReasonMarketClosed},
		{PointStatusNotPredicted, MissingReasonInsufficientRecentData, true, "", PredictionStatusNotPredicted, MissingReasonInsufficientRecentData},
		{PointStatusNotPredicted, MissingReasonInsufficientCalibrationData, true, "", PredictionStatusInsufficientData, MissingReasonInsufficientCalibrationData},
		{PointStatusNotPredicted, MissingReasonInvalidCalibrationLineage, true, "", PredictionStatusInsufficientData, MissingReasonInvalidCalibrationLineage},
		{PointStatusNotPredicted, MissingReasonNonFinitePrice, true, "", PredictionStatusInsufficientData, MissingReasonNonFinitePrice},
		{PointStatusPredicted, MissingReasonCalendarUnverified, true, "", PredictionStatusInsufficientData, MissingReasonCalendarUnverified},
		{PointStatusPredicted, "", false, MissingReasonBelowRidgeSampleThreshold, PredictionStatusInsufficientData, MissingReasonBelowRidgeSampleThreshold},
		{PointStatusPredicted, "", false, MissingReasonNonPositiveP50, PredictionStatusInsufficientData, MissingReasonNonPositiveP50},
		{PointStatusPredicted, "", true, "", PredictionStatusReady, ""},
	} {
		name := string(tc.inner) + "/" + string(tc.reason) + "/gate=" + string(tc.gateReason)
		t.Run(name, func(t *testing.T) {
			p := SeriesPoint{PointStatus: tc.inner, MissingReason: tc.reason}
			v := GateVerdict{Passed: tc.gatePassed, Reason: tc.gateReason}
			gotStatus, gotReason := OuterStatus(p, v)
			if gotStatus != tc.want || gotReason != tc.wantReason {
				t.Errorf("OuterStatus = (%q, %q)，預期 (%q, %q)", gotStatus, gotReason, tc.want, tc.wantReason)
			}
		})
	}
}

func TestOuterStatusUnknownInnerStatusIsNotSilentlyReady(t *testing.T) {
	// 未來若有人在 PointStatus 加新值卻忘了更新這張表，預設必須是保守值
	// （insufficient_data），不能是 ready——那會讓一個沒人想過的狀態直接出價。
	got, _ := OuterStatus(SeriesPoint{PointStatus: PointStatus("brand_new")}, GateVerdict{Passed: true})
	if got != PredictionStatusInsufficientData {
		t.Errorf("未知內層狀態 → %q，預期 %q", got, PredictionStatusInsufficientData)
	}
}

func TestFreshnessClassify(t *testing.T) {
	cfg := DefaultFreshnessConfig()
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	at := func(h float64) *time.Time {
		v := now.Add(-time.Duration(h * float64(time.Hour)))
		return &v
	}
	for _, tc := range []struct {
		name string
		in   *time.Time
		want FreshnessLabel
	}{
		{"剛發布", at(0), FreshnessFresh},
		{"35h → fresh", at(35), FreshnessFresh},
		{"36h 整 → stale（含頭）", at(36), FreshnessStale},
		{"71h → stale", at(71), FreshnessStale},
		{"72h 整 → unavailable", at(72), FreshnessUnavailable},
		{"完全沒有資料 → unavailable", nil, FreshnessUnavailable},
		{"時鐘倒退（未來時間）→ fresh", at(-1), FreshnessFresh},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := cfg.Classify(now, tc.in); got != tc.want {
				t.Errorf("Classify = %q，預期 %q", got, tc.want)
			}
		})
	}
}

func TestFreshnessConfigValidate(t *testing.T) {
	if err := (FreshnessConfig{FreshWithin: 72 * time.Hour, StaleWithin: 36 * time.Hour}).Validate(); err == nil {
		t.Error("FreshWithin 大於 StaleWithin 必須報錯——那會讓 stale 這個區間消失")
	}
	if err := DefaultFreshnessConfig().Validate(); err != nil {
		t.Errorf("預設值必須合法: %v", err)
	}
}
