//go:build integration

package catalog

import (
	"testing"
	"uuid"
)

// 種子分類：10 個頂層分組、52 個葉節點，葉節點都有各自的 Shopify 對照，名稱兩個語系都有。
func TestSeedTaxonomyShape(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	cats, err := f.m.Categories(f.ct)
	if err != nil {
		t.Fatal(err)
	}
	var tops, leaves int
	shopify := map[string]bool{}
	byID := map[uuid.UUID]Category{}
	for _, c := range cats {
		byID[c.ID] = c
		if c.NameZhTW == "" || c.NameEn == "" {
			t.Errorf("分類 %s 缺名稱：%+v", c.ID, c)
		}
		if c.IsLeaf() {
			leaves++
			if c.ShopifyTaxonomyID == "" || shopify[c.ShopifyTaxonomyID] {
				t.Errorf("葉節點 %s 的 Shopify 對照空白或重複：%q", c.NameZhTW, c.ShopifyTaxonomyID)
			}
			shopify[c.ShopifyTaxonomyID] = true
		} else {
			tops++
			if c.ShopifyTaxonomyID != "" {
				t.Errorf("頂層分組不對照 Shopify：%+v", c)
			}
		}
	}
	if tops != 10 || leaves != 52 {
		t.Fatalf("頂層 %d、葉節點 %d，want 10 與 52", tops, leaves)
	}
	for _, c := range cats {
		if c.IsLeaf() && byID[*c.ParentID].IsLeaf() {
			t.Errorf("樹只有兩層：%s 的上層不是頂層", c.NameZhTW)
		}
	}
	if cats[0].IsLeaf() {
		t.Error("頂層在前")
	}
}

func TestOnlyLeafCategoriesTakeProducts(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	cats, err := f.m.Categories(f.ct)
	if err != nil {
		t.Fatal(err)
	}
	var top, leaf Category
	for _, c := range cats {
		if c.IsLeaf() {
			leaf = c
		} else {
			top = c
		}
	}
	for name, c := range map[string]struct {
		id   uuid.UUID
		want bool
	}{"葉節點": {leaf.ID, true}, "頂層分組": {top.ID, false}, "不存在": {uuid.NewV7(), false}} {
		got, err := f.m.IsLeafCategory(f.ct, c.id)
		if err != nil || got != c.want {
			t.Errorf("%s：got %v err %v，want %v", name, got, err, c.want)
		}
	}
}

func TestCategoryNameMustCarryZhTW(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	wantConstraint(t, f.exec(`INSERT INTO catalog.categories (id, name, position) VALUES ($1, '{"en": "x"}', 0)`, uuid.NewV7()), "categories_name_check")
	wantConstraint(t, f.exec(`INSERT INTO catalog.categories (id, name, position) VALUES ($1, '{"zh-TW": ""}', 0)`, uuid.NewV7()), "categories_name_check")
}
