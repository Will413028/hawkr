package domain

import "testing"

func TestUntaxAndTaxOf(t *testing.T) {
	cases := []struct{ gross, untax, tax int64 }{
		{0, 0, 0},
		{-5, 0, -5}, // 負數不會單獨呼叫，但行為必須是定義好的
		{168, 160, 8},
		{900, 857, 43},
		{300, 286, 14},
		{150, 143, 7},
		{1, 1, 0},
		{21, 20, 1},
		{105, 100, 5},
	}
	for _, c := range cases {
		if got := Untax(c.gross); got != c.untax {
			t.Errorf("Untax(%d) = %d, want %d", c.gross, got, c.untax)
		}
		if got := TaxOf(c.gross); got != c.tax {
			t.Errorf("TaxOf(%d) = %d, want %d", c.gross, got, c.tax)
		}
	}
}

// Untax 等價於 Round(gross/1.05)，且 20·G/21 的小數部分恆不為 0.5
// ⇒ HALF_UP 與 HALF_EVEN 在這條式子上沒有鑑別力。全量列舉覆核。
func TestUntax_NoHalfwayCase(t *testing.T) {
	for g := int64(0); g <= 2_000_000; g++ {
		num := g * 20
		twice := 2 * (num % 21)
		if twice == 21 {
			t.Fatalf("found a .5 boundary at gross=%d — 捨入模式在此有鑑別力，本函式的前提被推翻", g)
		}
		want := (num + 10) / 21
		if got := Untax(g); got != want {
			t.Fatalf("Untax(%d) = %d, want %d", g, got, want)
		}
		if Untax(g)+TaxOf(g) != g {
			t.Fatalf("Untax(%d) + TaxOf(%d) != %d", g, g, g)
		}
	}
}
