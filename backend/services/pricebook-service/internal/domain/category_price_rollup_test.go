package domain

import (
	"encoding/json"
	"testing"
)

func TestRollupForecastPoint_JSONRoundTrip(t *testing.T) {
	p50 := 51.3
	point := RollupForecastPoint{HorizonDays: 1, TargetDate: "2026-09-01", P50: &p50}

	data, err := json.Marshal(point)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if got := string(data); got != `{"horizonDays":1,"targetDate":"2026-09-01","p50":51.3}` {
		t.Errorf("Marshal = %s, want p10/p90 omitted (nil) and p50 present", got)
	}

	var decoded RollupForecastPoint
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.HorizonDays != 1 || decoded.TargetDate != "2026-09-01" || decoded.P50 == nil || *decoded.P50 != 51.3 {
		t.Errorf("decoded = %+v, want round-trip match", decoded)
	}
}
