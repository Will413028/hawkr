package trimtext

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

func isSpaceRunes() []rune {
	var out []rune
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if unicode.IsSpace(r) {
			out = append(out, r)
		}
	}
	return out
}

// SpaceClass 與 unicode.IsSpace 是同一個集合、同一個順序，且只用 \uXXXX（沒有範圍或類別）。
func TestSpaceClassIsUnicodeIsSpace(t *testing.T) {
	inner, ok := strings.CutPrefix(SpaceClass, "[")
	if !ok || !strings.HasSuffix(inner, "]") {
		t.Fatalf("SpaceClass = %q，要是方括號表示式", SpaceClass)
	}
	inner = strings.TrimSuffix(inner, "]")
	if !regexp.MustCompile(`^(\\u[0-9a-f]{4})+$`).MatchString(inner) {
		t.Fatalf("SpaceClass 只能列 \\uXXXX：%q", inner)
	}
	var got []rune
	for i := 0; i < len(inner); i += 6 {
		v, err := strconv.ParseUint(inner[i+2:i+6], 16, 32)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, rune(v))
	}
	want := isSpaceRunes()
	if fmt.Sprint(got) != fmt.Sprint(want) || len(want) != 25 {
		t.Errorf("SpaceClass = %U，unicode.IsSpace = %U", got, want)
	}
}

// Trim 去掉的正好是 SpaceClass 的碼位：每個空白在頭、尾都被去掉，不是空白的相鄰碼位（ZWSP、U+180E、BOM 等）保留，中間的空白保留。
func TestTrimRemovesExactlyTheSpaceClass(t *testing.T) {
	for _, r := range isSpaceRunes() {
		s := string(r)
		if got := Trim(s + "名" + s + "字" + s); got != "名"+s+"字" {
			t.Errorf("Trim(%+q) = %+q", s+"名"+s+"字"+s, got)
		}
	}
	for _, r := range []rune{0x200B, 0x180E, 0xFEFF, 0x2060, 0x0008, 0x001F, 0x3001} {
		if s := string(r); Trim(s) != s {
			t.Errorf("Trim(%+q) 不該去掉它", s)
		}
	}
}

func TestCheckExpandsToTheColumnAndPattern(t *testing.T) {
	if got, want := Check("store_name"), "store_name !~ '"+Pattern+"'"; got != want {
		t.Errorf("Check = %q，要 %q", got, want)
	}
}

// 寫入端用 trimtext.Trim 或 strings.TrimSpace 都可以（單體規則 9）：對每一個碼位，兩者去掉的完全相同，而且去掉的正好是
// unicode.IsSpace 的碼位（SpaceClass 的集合，TestSpaceClassIsUnicodeIsSpace）。逐碼位檢查頭、尾與單獨一個字元。
func TestTrimAndStringsTrimSpaceRemoveTheSameSetOverAllRunes(t *testing.T) {
	removed := 0
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if r >= 0xD800 && r <= 0xDFFF {
			continue // 代理碼位不是合法的 UTF-8 字元
		}
		s := string(r)
		for _, v := range []string{s, s + "字", "字" + s} {
			if got, want := Trim(v), strings.TrimSpace(v); got != want {
				t.Fatalf("Trim(%+q) = %+q，strings.TrimSpace = %+q", v, got, want)
			}
		}
		if gone := Trim(s) == ""; gone != unicode.IsSpace(r) {
			t.Fatalf("%U：Trim 去掉 = %v，unicode.IsSpace = %v", r, gone, unicode.IsSpace(r))
		}
		if Trim(s) == "" {
			removed++
		}
	}
	if removed != len(isSpaceRunes()) {
		t.Fatalf("Trim 去掉 %d 個碼位，unicode.IsSpace 有 %d 個", removed, len(isSpaceRunes()))
	}
}
