package product

import (
	"bytes"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/model"
)

func baseInput() model.ProductInput {
	return model.ProductInput{
		Title: "山藥", CategoryID: uuid.NewV7(), Hashtags: []string{"#a"},
		Gallery: []model.Image{{Token: "t1"}},
		Skus:    []model.SkuInput{{Price: 100, StockQty: 3}},
	}
}

func TestInputHashIsStableAndSeesEveryField(t *testing.T) {
	t.Parallel()
	a := baseInput()
	if !bytes.Equal(inputHash(a), inputHash(a)) {
		t.Fatal("same input must hash the same")
	}
	three := int32(3)
	changes := map[string]func(*model.ProductInput){
		"title":     func(p *model.ProductInput) { p.Title = "地瓜" },
		"category":  func(p *model.ProductInput) { p.CategoryID = uuid.NewV7() },
		"hashtag":   func(p *model.ProductInput) { p.Hashtags = []string{"#b"} },
		"token":     func(p *model.ProductInput) { p.Gallery = []model.Image{{Token: "t2"}} },
		"alt":       func(p *model.ProductInput) { p.Gallery = []model.Image{{Token: "t1", Alt: "x"}} },
		"price":     func(p *model.ProductInput) { p.Skus = []model.SkuInput{{Price: 101, StockQty: 3}} },
		"stock":     func(p *model.ProductInput) { p.Skus = []model.SkuInput{{Price: 100, StockQty: 4}} },
		"threshold": func(p *model.ProductInput) { p.LowStockThreshold = &three },
		"axes":      func(p *model.ProductInput) { p.Axes = []model.Axis{{Name: "色", Values: []string{"紅"}}} },
		"desc":      func(p *model.ProductInput) { p.Description = []model.Block{{Kind: model.BlockParagraph, Text: "x"}} },
	}
	base := inputHash(a)
	for name, mutate := range changes {
		b := baseInput()
		b.CategoryID, b.Gallery, b.Skus, b.Hashtags = a.CategoryID, a.Gallery, a.Skus, a.Hashtags
		mutate(&b)
		if bytes.Equal(base, inputHash(b)) {
			t.Errorf("changing %s must change the hash", name)
		}
	}
}

func TestLikePatternEscapesWildcards(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"":         "",
		"   ":      "",
		" 蘋果 ":     "%蘋果%",
		"100%":     `%100\%%`,
		"a_b":      `%a\_b%`,
		`back\\sl`: `%back\\\\sl%`,
	} {
		if got := likePattern(in); got != want {
			t.Errorf("likePattern(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLowStockThresholdRange(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		v  int32
		ok bool
	}{{-1, false}, {0, true}, {9999, true}, {10000, false}} {
		in := baseInput()
		in.LowStockThreshold = &c.v
		_, err := prepare(in)
		if (err == nil) != c.ok {
			t.Errorf("threshold %d: err = %v, want ok=%v", c.v, err, c.ok)
		}
	}
}
