package hashtag

import (
	"slices"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

func TestNormalize(t *testing.T) {
	cases := []struct {
		raw, name, display string
		ok                 bool
	}{
		{"Fruit", "fruit", "Fruit", true},
		{"  #Fruit　", "fruit", "Fruit", true},
		{"##有機", "有機", "有機", true},
		{"é", "é", "é", true}, // 顯示是 NFC；名稱在字元集驗證時被拒絕（é 不是 a-z）
		{"", "", "", false},
		{"#", "", "", false},
		{" \t ", "", "", false},
	}
	for _, c := range cases {
		name, display, ok := Normalize(c.raw)
		if ok != c.ok || (ok && display != c.display) || (ok && c.display != "é" && name != c.name) {
			t.Errorf("Normalize(%q) = %q, %q, %v", c.raw, name, display, ok)
		}
	}
}

func TestRequireSet(t *testing.T) {
	got, err := RequireSet(uitext.ZhTW, []string{"#Fruit", "FRUIT", "水果", "abc123"})
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, tag := range got {
		names = append(names, tag.Name)
	}
	if !slices.Equal(names, []string{"fruit", "水果", "abc123"}) || got[0].Display != "Fruit" {
		t.Fatalf("%+v", got)
	}
	if got, err := RequireSet(uitext.ZhTW, nil); got != nil || err != nil {
		t.Fatalf("空輸入：%v %v", got, err)
	}
	bad := [][]string{{"a b"}, {"a-b"}, {"é"}, {"😀"}, {"a.b"}, {""}, {"#"}, {strings.Repeat("a", 41)}, {"%"}, {"a_b"}}
	for _, in := range bad {
		if _, err := RequireSet(uitext.ZhTW, in); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%q 要被拒絕：%v", in, err)
		}
	}
	if _, err := RequireSet(uitext.ZhTW, []string{strings.Repeat("a", 40)}); err != nil {
		t.Errorf("40 個字元可以：%v", err)
	}
	var many []string
	for i := range MaxCount + 1 {
		many = append(many, strings.Repeat("a", i+1))
	}
	if _, err := RequireSet(uitext.ZhTW, many); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Errorf("超過 %d 個：%v", MaxCount, err)
	}
	if _, err := RequireSet(uitext.ZhTW, many[:MaxCount]); err != nil {
		t.Errorf("剛好 %d 個可以：%v", MaxCount, err)
	}
	// 重複的不算數量。
	dup := []string{"a", "A", "a"}
	if got, err := RequireSet(uitext.ZhTW, dup); err != nil || len(got) != 1 {
		t.Errorf("%v %v", got, err)
	}
}

func TestSearchPatternRejectsLikeWildcards(t *testing.T) {
	for _, q := range []string{"%", "_", "a%", "a_b", "a b"} {
		if _, _, err := SearchPattern(uitext.ZhTW, q, false); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%q：%v", q, err)
		}
	}
	if p, ok, err := SearchPattern(uitext.ZhTW, "#Org", false); p != "org%" || !ok || err != nil {
		t.Errorf("前綴 %q %v %v", p, ok, err)
	}
	if p, ok, err := SearchPattern(uitext.ZhTW, "機", true); p != "%機%" || !ok || err != nil {
		t.Errorf("包含 %q %v %v", p, ok, err)
	}
	if _, ok, err := SearchPattern(uitext.ZhTW, "###", false); ok || err != nil {
		t.Errorf("正規化後是空的不是錯誤：%v %v", ok, err)
	}
}
