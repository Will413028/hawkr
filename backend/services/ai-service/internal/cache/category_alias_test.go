package cache

import "testing"

// 人工層必須壓過生成層：發現某條生成別名是錯的時候，往 synonyms 加一行就能
// 修掉，不必重跑整個 LLM pipeline。這是保留人工層的實際用途。
func TestCanonicalCategoryName_ManualLayerWinsOverGenerated(t *testing.T) {
	orig := generatedCategoryAliases
	t.Cleanup(func() { generatedCategoryAliases = orig })
	generatedCategoryAliases = map[string]string{
		"高麗菜": "錯誤目標",
		"捲心菜": "甘藍",
	}

	if got := CanonicalCategoryName("高麗菜"); got != "甘藍" {
		t.Errorf("人工層應勝出，got %q, want 甘藍", got)
	}
	if got := CanonicalCategoryName("捲心菜"); got != "甘藍" {
		t.Errorf("只有生成層時應採生成層，got %q, want 甘藍", got)
	}
	if got := CanonicalCategoryName("不存在的品項ZZZ"); got != "不存在的品項ZZZ" {
		t.Errorf("兩層都沒有時原樣回傳，got %q", got)
	}
	if got := CanonicalCategoryName(" 捲心菜 "); got != "甘藍" {
		t.Errorf("生成層也要吃 TrimSpace 後的 key，got %q", got)
	}
}

func TestCanonicalCategoryName_MeatCutsCollapseToL2(t *testing.T) {
	for _, cut := range []string{
		"豬五花", "五花肉", "三層肉", "梅花肉", "里肌肉", "大里肌",
		"腿庫", "松阪豬", "豬絞肉", "排骨",
	} {
		if got := CanonicalCategoryName(cut); got != "豬肉" {
			t.Errorf("CanonicalCategoryName(%q) = %q，預期 豬肉", cut, got)
		}
	}
	for _, cut := range []string{"雞胸肉", "棒棒腿", "三節翅", "雞絞肉"} {
		if got := CanonicalCategoryName(cut); got != "雞肉" {
			t.Errorf("CanonicalCategoryName(%q) = %q，預期 雞肉", cut, got)
		}
	}
}

func TestCanonicalCategoryName_FishSpeciesAreNotCollapsed(t *testing.T) {
	for _, species := range []string{"鱸魚", "吳郭魚", "虱目魚", "白帶魚"} {
		if got := CanonicalCategoryName(species); got == "魚類" {
			t.Errorf("%q 不該被收斂到魚類——魚種各有各的成交價，應解析到具體魚種", species)
		}
	}
}
