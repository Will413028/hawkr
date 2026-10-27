package domain

import "testing"

// TestCompileReferencePrice_KgUnit_RoundsToIntegerTwd (Task 18／Fix 2) 取代原本
// 「ReturnsSourcePriceUnchanged」的斷言——CompileReferencePrice 現在會在回傳前
// math.Round 成整數元（CLAUDE.md §4：金額以新台幣「元」的整數表示，禁止浮點
// 運算），KG 的換算係數雖然是 1，但來源 P10/P50/P90 本身仍可能是任意浮點數，
// 取整發生在這裡，不是「原樣不動」。
func TestCompileReferencePrice_KgUnit_RoundsToIntegerTwd(t *testing.T) {
	got, err := CompileReferencePrice(35.5, "KG", PriceComponentGoodsOnly)
	if err != nil {
		t.Fatalf("CompileReferencePrice() error = %v, want nil", err)
	}
	if got != 36 {
		t.Errorf("got = %v, want 36（math.Round(35.5) 四捨五入，Go 對 .5 取「遠離零」方向）", got)
	}
}

// TestCompileReferencePrice_CattyUnit_AppliesWeightFactorAndRounds (Task 18／
// Fix 2) 取代原本只驗證「套用換算係數」的版本，一併驗證取整：35.5*0.6=21.3，
// math.Round 後是 21，不是 21.3。
func TestCompileReferencePrice_CattyUnit_AppliesWeightFactorAndRounds(t *testing.T) {
	got, err := CompileReferencePrice(35.5, "CATTY", PriceComponentGoodsOnly)
	if err != nil {
		t.Fatalf("CompileReferencePrice() error = %v, want nil", err)
	}
	if got != 21 {
		t.Errorf("got = %v, want 21（35.5*0.6=21.3，math.Round 後取整為 21）", got)
	}
}

// TestCompileReferencePrice_CattyUnit_RegressionFloatPrecisionArtifact_ProducesCleanInteger
// (Task 18／Fix 2) 涵蓋 whole-branch review 找到的具體案例：CATTY 的 0.6 係數
// 是 IEEE754 無法精確表示的小數，100.1*0.6 在取整前的原始值是
// 60.059999999999995（浮點精度殘留）——修前這個殘留值會被
// usecase.formatAmount／gRPC handler 的 formatPriceAmount 原樣印成
// "60.059999999999995" 這種違反 CLAUDE.md §4 的字串。math.Round 在
// CompileReferencePrice 這裡（唯一的浮點入口）就地取整一次，之後不再有小數
// 重新進入這條路徑。
func TestCompileReferencePrice_CattyUnit_RegressionFloatPrecisionArtifact_ProducesCleanInteger(t *testing.T) {
	got, err := CompileReferencePrice(100.1, "CATTY", PriceComponentGoodsOnly)
	if err != nil {
		t.Fatalf("CompileReferencePrice() error = %v, want nil", err)
	}
	if got != 60 {
		t.Errorf("got = %v, want 60（100.1*0.6=60.059999999999995 的浮點殘留必須被 math.Round 收斂成整數）", got)
	}
}

// TestCompileReferencePrice_UnsupportedUnit_ReturnsError 驗證計數單位（BOX，
// 曾在 domain.AllowedUnits 內短暫出現過但已撤回——沒有天然公斤換算，Pricebook
// 永遠算不出參考價）直接傳進 CompileReferencePrice 時明確報錯，而非靜默算出
// 一個看似合理的數字。
func TestCompileReferencePrice_UnsupportedUnit_ReturnsError(t *testing.T) {
	_, err := CompileReferencePrice(35.5, "BOX", PriceComponentGoodsOnly)
	if err == nil {
		t.Fatal("CompileReferencePrice() error = nil, want error")
	}
}

func TestCompileReferencePrice_UnsupportedPriceComponent_ReturnsError(t *testing.T) {
	_, err := CompileReferencePrice(35.5, "KG", "INCLUDE_FREIGHT")
	if err == nil {
		t.Fatal("CompileReferencePrice() error = nil, want error")
	}
}

// TestCompileReferencePrice_AllowedUnitsCoupling validates that CompileReferencePrice
// correctly handles every unit in domain.AllowedUnits. This coupling ensures that if
// AllowedUnits ever gains a new entry, CompileReferencePrice will either work correctly
// (if a weightConversionFactor is added) or fail loudly (if forgotten), rather than
// silently drifting into inconsistency.
func TestCompileReferencePrice_AllowedUnitsCoupling(t *testing.T) {
	for _, unit := range AllowedUnits {
		t.Run(unit, func(t *testing.T) {
			got, err := CompileReferencePrice(100.0, unit, PriceComponentGoodsOnly)
			if err != nil {
				t.Errorf("CompileReferencePrice() for allowed unit %q should not error, got %v", unit, err)
			}
			if got == 0 {
				t.Errorf("CompileReferencePrice() for unit %q should produce non-zero result, got %v", unit, got)
			}
		})
	}
}

// TestWeightConversionFactor_UnmappedUnit_ReturnsError verifies that weightConversionFactor
// fails loudly (not silently) when passed a unit string that is not KG/CATTY/GRAM.
// This is a guard against silent drift: if AllowedUnits ever gains a new entry without
// a matching case in the switch, CompileReferencePrice will error instead of returning 1.0.
func TestWeightConversionFactor_UnmappedUnit_ReturnsError(t *testing.T) {
	_, err := weightConversionFactor("BUNCH")
	if err == nil {
		t.Fatal("weightConversionFactor() for unmapped unit should return error, got nil")
	}
}
