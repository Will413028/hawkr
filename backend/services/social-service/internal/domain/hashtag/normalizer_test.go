package hashtag

import "testing"

func TestNormalize_table(t *testing.T) {
	cases := []struct {
		name          string
		raw           string
		wantCanonical string
		wantDisplay   string
		wantOK        bool
	}{
		{"empty", "", "", "", false},
		{"only spaces", "   ", "", "", false},
		{"only hash", "###", "", "", false},
		{"strip single hash", "#美食", "美食", "美食", true},
		{"strip multiple hash", "##美食", "美食", "美食", true},
		{"trim whitespace", "  美食 \t", "美食", "美食", true},
		{"trim NBSP", " 美食 ", "美食", "美食", true},
		{"trim full-width space", "　美食　", "美食", "美食", true},
		{"ascii lower canonical", "#FoodPorn", "foodporn", "FoodPorn", true},
		{"cjk unchanged", "美食", "美食", "美食", true},
		{"mixed", "#Foo美食Bar", "foo美食bar", "Foo美食Bar", true},
		{"digit preserved", "#2026Trend", "2026trend", "2026Trend", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, d, ok := Normalize(tc.raw)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if c != tc.wantCanonical {
				t.Errorf("canonical = %q, want %q", c, tc.wantCanonical)
			}
			if d != tc.wantDisplay {
				t.Errorf("display = %q, want %q", d, tc.wantDisplay)
			}
		})
	}
}

func TestNormalize_NFCMergesEquivalents(t *testing.T) {
	// pre-composed:  café  = "caf" + U+00E9
	// decomposed:    café = "caf" + U+0065 + U+0301
	c1, _, ok1 := Normalize("café")
	c2, _, ok2 := Normalize("café")
	if !ok1 || !ok2 {
		t.Fatal("both inputs should normalize OK")
	}
	if c1 != c2 {
		t.Errorf("NFC should produce equal canonical: %q vs %q", c1, c2)
	}
}

// TestNormalize_NonASCIICapsUnchanged guards lowerASCII against accidentally
// lower-casing non-ASCII capitals (e.g. Greek, full-width A-Z).
func TestNormalize_NonASCIICapsUnchanged(t *testing.T) {
	cases := []struct {
		raw, wantCanonical string
	}{
		// Greek capital Alpha (U+0391) must NOT become lower alpha.
		{"ΑBC", "Αbc"},
		// Full-width capital A (U+FF21) must NOT become full-width lower a.
		{"ＡBC", "Ａbc"},
	}
	for _, tc := range cases {
		c, _, ok := Normalize(tc.raw)
		if !ok {
			t.Fatalf("Normalize(%q) ok = false, want true", tc.raw)
		}
		if c != tc.wantCanonical {
			t.Errorf("canonical = %q, want %q", c, tc.wantCanonical)
		}
	}
}
