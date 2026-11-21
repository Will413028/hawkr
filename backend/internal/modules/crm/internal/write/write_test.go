package write

import (
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func TestNormalizeTagName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"一般", "VIP 客戶", "VIP 客戶", true},
		{"去頭尾空白", "  常客  ", "常客", true},
		{"去全形空白與 tab", "　常客\t", "常客", true},
		{"空字串", "", "", false},
		{"只有空白", " 　\t", "", false},
		{"剛好 50 個字", strings.Repeat("標", 50), strings.Repeat("標", 50), true},
		{"51 個字", strings.Repeat("標", 51), "", false},
		{"以字元計不是位元組：50 個四位元組字元", strings.Repeat("😀", 50), strings.Repeat("😀", 50), true},
		{"51 個四位元組字元", strings.Repeat("😀", 51), "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := NormalizeTagName(t.Context(), tc.in)
			if tc.ok {
				if err != nil || got != tc.want {
					t.Fatalf("got %q, %v，預期 %q", got, err, tc.want)
				}
				return
			}
			if apperr.KindOf(err) != apperr.KindInvalidInput {
				t.Fatalf("err = %v，預期 InvalidInput", err)
			}
		})
	}
}
