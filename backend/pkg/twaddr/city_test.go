package twaddr

import "testing"

func TestCanonicalCity(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"台北市", "台北市"}, {"臺北市", "台北市"}, {" 臺東縣 ", "台東縣"},
	} {
		got, ok := CanonicalCity(tc.in)
		if !ok || got != tc.want {
			t.Fatalf("CanonicalCity(%q) = (%q, %v), want (%q, true)", tc.in, got, ok, tc.want)
		}
	}
	if _, ok := CanonicalCity("火星市"); ok {
		t.Fatal("unknown city must be rejected")
	}
	if len(TaiwanCities) != 22 {
		t.Fatalf("TaiwanCities count = %d, want 22", len(TaiwanCities))
	}
}

func TestCityCodeUsesStableISO31662TWCode(t *testing.T) {
	code, ok := CityCode("臺北市")
	if !ok || code != "TW-TPE" {
		t.Fatalf("CityCode(臺北市) = %q, %v; want TW-TPE, true", code, ok)
	}
	city, ok := CityFromCode("tw-ttt")
	if !ok || city != "台東縣" {
		t.Fatalf("CityFromCode(tw-ttt) = %q, %v; want 台東縣, true", city, ok)
	}
}

func TestTaiwanCityRegistryHasUniqueCodesForAll22Cities(t *testing.T) {
	seen := map[string]bool{}
	for _, city := range TaiwanCities {
		code, ok := CityCode(city)
		if !ok || code == "" {
			t.Fatalf("missing code for %s", city)
		}
		if seen[code] {
			t.Fatalf("duplicate code %s", code)
		}
		seen[code] = true
	}
	if len(seen) != 22 {
		t.Fatalf("unique codes = %d, want 22", len(seen))
	}
}
