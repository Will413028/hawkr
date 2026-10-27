package appversion

import "testing"

func TestSemverComparesEachPartAsANumber(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.10", "1.0.9", 1}, // 字串比較會得到相反的結果
		{"1.2.3", "1.2.3", 0},
		{"0.9.9", "1.0.0", -1},
		{"2.0.0", "10.0.0", -1},
	}
	for _, c := range cases {
		a, okA := parseSemver(c.a)
		b, okB := parseSemver(c.b)
		if !okA || !okB {
			t.Fatalf("%s 或 %s 解析失敗", c.a, c.b)
		}
		if got := compareSemver(a, b); got != c.want {
			t.Errorf("compare(%s, %s) = %d，預期 %d", c.a, c.b, got, c.want)
		}
	}
}

func TestSemverAcceptsOnlyThreeNumericParts(t *testing.T) {
	for _, v := range []string{"", "1.0", "1.0.0.0", "1.0.3(7)", "v1.0.0", "1.+1.0", "1.-1.0", "1..0", "1.0.x"} {
		if _, ok := parseSemver(v); ok {
			t.Errorf("parseSemver(%q) 應失敗", v)
		}
	}
}

func TestStoreURLSchemes(t *testing.T) {
	for v, want := range map[string]bool{
		"":                                 true,
		"https://apps.apple.com/app/id1":   true,
		"market://details?id=example":      false,
		"itms-apps://apps.apple.com/app/1": true,
		"http://example.com":               false,
		"javascript:alert(1)":              false,
	} {
		if got := validStoreURL(v); got != want {
			t.Errorf("validStoreURL(%q) = %v", v, got)
		}
	}
}

// 筆數照 Google AIP-158：0 用預設值、超過上限以上限計、負數是呼叫方的錯。
