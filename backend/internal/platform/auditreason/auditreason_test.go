package auditreason

import (
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func TestRequired(t *testing.T) {
	for _, raw := range []string{"", " ", " \n\t ", strings.Repeat("理", MaxRunes+1), " " + strings.Repeat("理", MaxRunes+1) + " "} {
		if _, err := Required(raw, "Reason"); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("%q：err=%v", raw, err)
		}
	}
	got, err := Required("  違規經營 \n", "Reason")
	if err != nil || got != "違規經營" {
		t.Fatalf("got=%q err=%v", got, err)
	}
	// 去掉空白之後剛好 1000 字可以。
	if got, err := Required(" "+strings.Repeat("理", MaxRunes)+" ", "Reason"); err != nil || len([]rune(got)) != MaxRunes {
		t.Fatalf("boundary: %v", err)
	}
}

func TestOptional(t *testing.T) {
	for _, raw := range []string{strings.Repeat("理", MaxRunes+1), " " + strings.Repeat("理", MaxRunes+1) + " "} {
		if _, err := Optional(raw, "Comment"); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("%q：err=%v", raw[:6], err)
		}
	}
	for raw, want := range map[string]string{"": "", " \n\t ": "", "  資料齊全 \n": "資料齊全"} {
		if got, err := Optional(raw, "Comment"); err != nil || got != want {
			t.Fatalf("%q：got=%q err=%v", raw, got, err)
		}
	}
	if got, err := Optional(" "+strings.Repeat("理", MaxRunes)+" ", "Comment"); err != nil || len([]rune(got)) != MaxRunes {
		t.Fatalf("boundary: %v", err)
	}
}
