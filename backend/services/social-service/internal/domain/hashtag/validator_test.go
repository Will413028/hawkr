package hashtag

import (
	"errors"
	"strings"
	"testing"
)

func TestValidate_EmptyInputReturnsNil(t *testing.T) {
	out, err := RequireSet(nil)
	if err != nil {
		t.Fatalf("nil input err = %v, want nil", err)
	}
	if len(out) != 0 {
		t.Errorf("len = %d, want 0", len(out))
	}
	out, err = RequireSet([]string{})
	if err != nil || len(out) != 0 {
		t.Fatalf("empty slice should return ([], nil), got (%v, %v)", out, err)
	}
}

func TestValidate_EmptyAfterNormalize(t *testing.T) {
	_, err := RequireSet([]string{"###"})
	if !errors.Is(err, ErrEmptyAfterNormalize) {
		t.Errorf("err = %v, want ErrEmptyAfterNormalize", err)
	}
}

func TestValidate_InvalidChars(t *testing.T) {
	cases := []string{
		"#has space",   // space
		"#hash#inside", // # in middle
		"#emoji🍔",      // emoji
		"#punct,here",  // ASCII punctuation
		"#tab\there",   // control
		"#full！width",  // full-width punctuation
	}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			_, err := RequireSet([]string{raw})
			if !errors.Is(err, ErrInvalidChar) {
				t.Errorf("raw=%q err=%v, want ErrInvalidChar", raw, err)
			}
		})
	}
}

// TestValidate_RejectsZeroWidthAndFormatChars locks in the cross-layer contract
// that the validator rejects zero-width / format characters that Normalize's
// unicode.IsSpace trim does NOT catch (U+200B, U+200E, U+FEFF).
func TestValidate_RejectsZeroWidthAndFormatChars(t *testing.T) {
	// Use \u escapes so the source file itself stays free of BOM / ZWSP /
	// LRM bytes (Go forbids U+FEFF in source).
	cases := map[string]string{
		"zero-width space": "美\u200b食", // 美 + ZWSP + 食 (escape so source is ASCII-clean for ZWSP/LRM/BOM)
		"LTR mark":         "美\u200e食", // 美 + LRM + 食
		"BOM":              "\ufeff美食", // BOM + 美食
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := RequireSet([]string{raw})
			if !errors.Is(err, ErrInvalidChar) {
				t.Errorf("raw=%q err=%v, want ErrInvalidChar", raw, err)
			}
		})
	}
}

func TestValidate_AllowedChars(t *testing.T) {
	// Spec §3.2: ASCII letter (a-z), ASCII digit (0-9), CJK Han only.
	// Latin-1 accented letters (é) are NOT in the allowed set — see also
	// the Java reference impl HashtagInputValidator.isAllowedCodePoint.
	raw := []string{"abc", "美食", "2026", "Mixed美食2026"}
	wantCanonical := []string{"abc", "美食", "2026", "mixed美食2026"}
	out, err := RequireSet(raw)
	if err != nil {
		t.Fatalf("unexpected err = %v", err)
	}
	if len(out) != len(wantCanonical) {
		t.Fatalf("len = %d, want %d", len(out), len(wantCanonical))
	}
	for i, want := range wantCanonical {
		if out[i].Canonical != want {
			t.Errorf("[%d] canonical = %q, want %q", i, out[i].Canonical, want)
		}
	}
}

func TestValidate_LengthBoundary(t *testing.T) {
	// ASCII: bytes == runes
	tag40 := strings.Repeat("a", 40)
	tag41 := strings.Repeat("a", 41)
	if _, err := RequireSet([]string{tag40}); err != nil {
		t.Errorf("40 ASCII chars should pass, got %v", err)
	}
	if _, err := RequireSet([]string{tag41}); !errors.Is(err, ErrLengthExceeded) {
		t.Errorf("41 ASCII chars err = %v, want ErrLengthExceeded", err)
	}
	// CJK: 1 rune = 3 bytes — verify length counts code points, not bytes
	cjk40 := strings.Repeat("美", 40)
	cjk41 := strings.Repeat("美", 41)
	if _, err := RequireSet([]string{cjk40}); err != nil {
		t.Errorf("40 CJK runes (120 bytes) should pass, got %v", err)
	}
	if _, err := RequireSet([]string{cjk41}); !errors.Is(err, ErrLengthExceeded) {
		t.Errorf("41 CJK runes err = %v, want ErrLengthExceeded", err)
	}
}

func TestValidate_CountBoundary(t *testing.T) {
	mk := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = strings.Repeat("a", i+1) // distinct lengths → distinct canonical
		}
		return out
	}
	if _, err := RequireSet(mk(10)); err != nil {
		t.Errorf("10 tags should pass, got %v", err)
	}
	if _, err := RequireSet(mk(11)); !errors.Is(err, ErrCountExceeded) {
		t.Errorf("11 tags err = %v, want ErrCountExceeded", err)
	}
}

func TestValidate_DedupKeepsFirstDisplay(t *testing.T) {
	out, err := RequireSet([]string{"#FOOD", "#food", "#Food"})
	if err != nil {
		t.Fatalf("unexpected err = %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("len = %d, want 1 (deduped)", len(out))
	}
	if out[0].Canonical != "food" {
		t.Errorf("canonical = %q, want %q", out[0].Canonical, "food")
	}
	if out[0].Display != "FOOD" {
		t.Errorf("display = %q, want %q (first input wins)", out[0].Display, "FOOD")
	}
}
