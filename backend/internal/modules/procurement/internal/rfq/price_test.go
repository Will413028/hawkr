package rfq

import (
	"math"
	"testing"
)

func TestBasis(t *testing.T) {
	for unit, want := range map[string]int32{"GRAM": 1000, "KG": 1, "CATTY": 1} {
		if q, u := Basis(unit); q != want || u != unit {
			t.Errorf("Basis(%s) = %d %s", unit, q, u)
		}
	}
}

func TestValidRange(t *testing.T) {
	for _, tc := range []struct {
		lower, upper int64
		want         bool
	}{
		{1, 1, true}, {100, 200, true}, {0, 10, false}, {10, 0, false}, {-1, 5, false}, {5, -1, false}, {201, 200, false},
	} {
		if got := ValidRange(tc.lower, tc.upper); got != tc.want {
			t.Errorf("ValidRange(%d, %d) = %v", tc.lower, tc.upper, got)
		}
	}
}

func TestTotalRoundsHalfUpPerBasis(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		lower, upper         int64
		quantity, basis      int32
		wantLower, wantUpper int64
	}{
		{"kg", 100, 200, 3, 1, 300, 600},
		{"gram per 1000", 100, 200, 250, 1000, 25, 50},
		{"rounds half up", 1, 3, 500, 1000, 1, 2},
		{"rounds down below half", 1, 1, 499, 1000, 0, 0},
	} {
		lo, up, ok := Total(tc.lower, tc.upper, tc.quantity, tc.basis)
		if !ok || lo != tc.wantLower || up != tc.wantUpper {
			t.Errorf("%s：Total = %d %d %v，要 %d %d", tc.name, lo, up, ok, tc.wantLower, tc.wantUpper)
		}
	}
}

func TestTotalReportsOverflow(t *testing.T) {
	if _, _, ok := Total(math.MaxInt64, math.MaxInt64, 2, 1); ok {
		t.Error("乘法溢位要回 ok=false")
	}
	if _, _, ok := Total(1, math.MaxInt64, 1, 1000); ok {
		t.Error("加進四捨五入的 basis/2 溢位要回 ok=false")
	}
	if _, _, ok := Total(math.MaxInt64/2, 5, 1, 1); !ok {
		t.Error("沒溢位的上界不能被擋")
	}
}
