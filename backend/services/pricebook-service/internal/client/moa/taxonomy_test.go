package moa

import "testing"

func TestSplitCropName(t *testing.T) {
	cases := []struct{ in, group, variant string }{
		{"甘藍-初秋", "甘藍", "初秋"},
		{"甘藍-進口 初秋", "甘藍", "進口 初秋"},
		{"花胡瓜", "花胡瓜", ""},
		{"包心白菜-進口 其他", "包心白菜", "進口 其他"},
		// 子品項本身帶連字號：用第一個連字號切才對，用最後一個連字號切
		// 會把「臺農57號-紅心」誤拆成大類的一部分。這個案例會在有人把
		// strings.Index 換成 strings.LastIndex 時失敗，之前的案例都只有
		// 一個連字號、切不出差異，defend 不了這個選擇。
		{"甘薯-臺農57號-紅心", "甘薯", "臺農57號-紅心"},
		// 連字號在開頭：連字號前沒有任何字元，不算真的切出了大類，
		// 整串應當回退成大類本身，而不是寫一個空字串當大類名。
		{"-初秋", "-初秋", ""},
	}
	for _, c := range cases {
		g, v := SplitCropName(c.in)
		if g != c.group || v != c.variant {
			t.Errorf("SplitCropName(%q) = (%q,%q)，預期 (%q,%q)", c.in, g, v, c.group, c.variant)
		}
	}
}

func TestGroupCodeIsStableAndPrefixed(t *testing.T) {
	a := GroupCode("甘藍", "N04")
	b := GroupCode("甘藍", "N04")
	if a != b {
		t.Errorf("GroupCode 不穩定: %q vs %q", a, b)
	}
	if len(a) < len("moa-group:") || a[:len("moa-group:")] != "moa-group:" {
		t.Errorf("GroupCode 缺少 moa-group: 前綴: %q", a)
	}
	if GroupCode("甘藍", "N05") == a {
		t.Error("不同 category_code 應產生不同 group code")
	}
}
