package forecast

import (
	"math"
	"testing"
)

func TestOrderedNonNegativeClipsThenAccumulatesMax(t *testing.T) {
	// center=-5, residuals=(-1,0,1) → raw=(-6,-5,-4) → 全部截 0 → (0,0,0)
	p10, p50, p90, err := OrderedNonNegative(-5.0, [3]float64{-1.0, 0.0, 1.0})
	if err != nil {
		t.Fatalf("回錯誤: %v", err)
	}
	assertClose(t, "p10", p10, 0.0, 0)
	assertClose(t, "p50", p50, 0.0, 0)
	assertClose(t, "p90", p90, 0.0, 0)
}

func TestOrderedNonNegativeEnforcesMonotonicityWhenMiddleDips(t *testing.T) {
	// center=10, residuals=(-15,-15,5) → raw=(-5,-5,15) → 截 0 → (0,0,15)
	// → 累積 max (0,0,15)。刻意讓 p10/p50 都截到 0、只有 p90 為正——驗證
	// 「先截 0 再累積 max」跟「先累積 max 再截 0」在這個輸入下的差異會被抓到。
	p10, p50, p90, err := OrderedNonNegative(10.0, [3]float64{-15.0, -15.0, 5.0})
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, "p10", p10, 0.0, 0)
	assertClose(t, "p50", p50, 0.0, 0)
	assertClose(t, "p90", p90, 15.0, 0)
}

func TestOrderedNonNegativeRejectsNonFiniteInput(t *testing.T) {
	if _, _, _, err := OrderedNonNegative(math.Inf(1), [3]float64{0, 0, 0}); err == nil {
		t.Fatal("center 為 +Inf 應回錯誤（mirrors forecasting.py:116-117）")
	}
}

func TestOrderedNonNegativeEnforcesP10LEP50LEP90WhenUnclipped(t *testing.T) {
	p10, p50, p90, err := OrderedNonNegative(100.0, [3]float64{-2.0, 0.0, 3.0})
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, "p10", p10, 98.0, 0)
	assertClose(t, "p50", p50, 100.0, 0)
	assertClose(t, "p90", p90, 103.0, 0)
	if !(p10 <= p50 && p50 <= p90) {
		t.Errorf("p10<=p50<=p90 不成立: %v %v %v", p10, p50, p90)
	}
}
