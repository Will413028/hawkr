package textfilter

import (
	"slices"
	"testing"
)

func TestCheckAgainst(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		words []string
		want  []string
	}{
		{"空字串不命中", "", []string{"壞"}, nil},
		{"子字串命中", "你這個壞人", []string{"壞人"}, []string{"壞人"}},
		{"不分大小寫", "You Are BAD", []string{"bad"}, []string{"bad"}},
		{"依詞典的順序回傳", "shit and fuck", []string{"fuck", "shit", "damn"}, []string{"fuck", "shit"}},
		{"空詞略過", "任何文字", []string{"", "壞"}, nil},
		{"沒有詞典", "任何文字", nil, nil},
	}
	for _, c := range cases {
		hit, got := CheckAgainst(c.text, c.words)
		if hit != (len(c.want) > 0) || !slices.Equal(got, c.want) {
			t.Errorf("%s：CheckAgainst(%q) = %v, %v，預期 %v", c.name, c.text, hit, got, c.want)
		}
	}
}

func TestCheckUsesTheDictionary(t *testing.T) {
	if hit, words := Check("你去死吧"); !hit || !slices.Equal(words, []string{"去死"}) {
		t.Errorf("Check = %v, %v", hit, words)
	}
	if hit, _ := Check("今天的蔬菜很新鮮"); hit {
		t.Error("正常文案不該命中")
	}
	for _, w := range Words {
		if w == "" {
			t.Error("詞典不得有空詞")
		}
	}
}
