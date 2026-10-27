package domain

import "testing"

func TestCanonicalPricePerKg(t *testing.T) {
	cases := []struct {
		name  string
		price float64
		unit  string
		want  float64
	}{
		{"台斤", 42.5, UnitCatty, 70.83333333333333},

		{"百公斤", 4633, UnitHectokilogram, 46.33},
		{"公斤不變", 106.03, UnitKilogram, 106.03},
	}
	for _, c := range cases {
		got, ok := CanonicalPricePerKg(c.price, c.unit)
		if !ok {
			t.Fatalf("%s: 應可換算", c.name)
		}
		if diff := got - c.want; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("%s: = %v，預期 %v", c.name, got, c.want)
		}
	}
}

func TestCanonicalPricePerKg_CattyGoesUpNotDown(t *testing.T) {
	got, _ := CanonicalPricePerKg(42.5, UnitCatty)
	if got <= 42.5 {
		t.Fatalf("台斤換公斤應變大，got %v（方向寫反了）", got)
	}
}

func TestCanonicalPricePerKg_PackagingUnitsRefused(t *testing.T) {
	for _, u := range []string{"box", "piece", "bundle", "", "公斤", "KG"} {
		if _, ok := CanonicalPricePerKg(100, u); ok {
			t.Errorf("單位 %q 不該有 canonical 換算", u)
		}
		if CanonicalUnitFor(u) != "" {
			t.Errorf("單位 %q 不該回 canonical 單位", u)
		}
	}
}

func TestCanonicalUnitFor(t *testing.T) {
	for _, u := range []string{UnitKilogram, UnitCatty, UnitHectokilogram} {
		if CanonicalUnitFor(u) != UnitKilogram {
			t.Errorf("CanonicalUnitFor(%q) 應為 kg", u)
		}
	}
}
