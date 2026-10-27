package icu

import (
	"strings"
	"testing"
)

func TestParseAccepts(t *testing.T) {
	cases := []string{
		"純文字",
		"風險：{ground}",
		"{kind, select, A {甲} B {乙} other {其他}}",
		"{ kind , select , A { 甲 } other { 其他 } }",
		"{days, plural, other {{days} 日}}",
		"{days, plural, one {{days} day} other {{days} days}}",
		"{a, select, X {{b, select, Y {y} other {z}}} other {w}}",
		"don't 和 it's",
		"全形「引號」與 / 斜線",
	}
	for _, src := range cases {
		if _, err := Parse(src); err != nil {
			t.Errorf("Parse(%q) 失敗：%v", src, err)
		}
	}
}

func TestParseRejects(t *testing.T) {
	cases := []struct{ src, want string }{
		{"第 # 筆", "禁止"},
		{"{n, plural, other {# 筆}}", "禁止"},
		{"<b>粗體</b>", "禁止"},
		{"a > b", "禁止"},
		{"{n, number}", "不支援參數型別"},
		{"{d, date}", "不支援參數型別"},
		{"{n, selectordinal, other {x}}", "不支援參數型別"},
		{"{n, plural, =0 {無} other {x}}", "精確分支"},
		{"{n, plural, many2 {x} other {x}}", "plural 的分支"},
		{"{k, select, A {a}}", "缺少 other"},
		{"{k, select, A {a} A {b} other {c}}", "重複"},
		{"{k, select, A {a} other {c}", "沒有以 } 結束"},
		{"{k", "只能是"},
		{"沒有開頭 }", "多出的 }"},
		{"'{x}", "apostrophe"},
		{"x'}", "apostrophe"},
		{"''", "連續兩個"},
		{"{a, select, X {{b, select, Y {{c, select, Z {z} other {z}}} other {y}}} other {w}}", "巢狀超過"},
		{"{ , select, other {x}}", "缺少參數名稱"},
	}
	for _, c := range cases {
		_, err := Parse(c.src)
		if err == nil {
			t.Errorf("Parse(%q) 應該失敗", c.src)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("Parse(%q) 的錯誤 %q 不含 %q", c.src, err, c.want)
		}
	}
}

func TestApostropheIsLiteral(t *testing.T) {
	m, err := Parse("it's {x}'s")
	if err != nil {
		t.Fatal(err)
	}
	got, err := Format(m, "en", map[string]any{"x": "A"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "it's A's" {
		t.Errorf("got %q", got)
	}
}

func TestFormat(t *testing.T) {
	m, err := Parse("{kind, select, T {合作已終止} other {停權中}}，{days, plural, one {{days} day} other {{days} days}}")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		locale string
		args   map[string]any
		want   string
	}{
		{"en", map[string]any{"kind": "T", "days": 1}, "合作已終止，1 day"},
		{"en", map[string]any{"kind": "S", "days": 2}, "停權中，2 days"},
		{"zh-TW", map[string]any{"kind": "", "days": 1}, "停權中，1 days"},
		{"en", map[string]any{"kind": "T", "days": 1234}, "合作已終止，1234 days"},
	}
	for _, c := range cases {
		got, err := Format(m, c.locale, c.args)
		if err != nil {
			t.Errorf("%v：%v", c.args, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s %v：got %q want %q", c.locale, c.args, got, c.want)
		}
	}
	if _, err := Format(m, "en", map[string]any{"kind": "T"}); err == nil {
		t.Error("缺 days 應該失敗")
	}
	if _, err := Format(m, "fr", map[string]any{"kind": "T", "days": 1}); err == nil {
		t.Error("不認得的語系應該失敗")
	}
}

func TestCategories(t *testing.T) {
	for locale, want := range map[string]string{"zh-TW": "other", "en": "other"} {
		got, err := Category(locale, 5)
		if err != nil || got != want {
			t.Errorf("%s：%q %v", locale, got, err)
		}
	}
	if got, _ := Category("en", 1); got != "one" {
		t.Errorf("en 1：%q", got)
	}
	if got, _ := Category("zh-TW", 1); got != "other" {
		t.Errorf("zh-TW 1：%q", got)
	}
	if _, err := Categories("fr"); err == nil {
		t.Error("不認得的語系應該失敗")
	}
}

func TestUses(t *testing.T) {
	m, err := Parse("{k, select, A {{n}} other {{k2, select, X {x} other {y}}}} {n, plural, other {z}}")
	if err != nil {
		t.Fatal(err)
	}
	uses, conflict := m.Uses()
	if conflict != "" {
		t.Errorf("plural 的分支裡用 {n} 不是衝突：%q", conflict)
	}
	if len(uses) != 3 || uses[1].Name != "n" || uses[1].Kind != UsePlural {
		t.Errorf("uses=%v", uses)
	}
	m, err = Parse("{k, select, A {x} other {y}} {k}")
	if err != nil {
		t.Fatal(err)
	}
	if _, conflict := m.Uses(); conflict != "k" {
		t.Errorf("select 與 arg 同名要回報衝突，conflict=%q", conflict)
	}
}
