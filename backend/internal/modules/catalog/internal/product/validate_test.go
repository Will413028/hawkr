package product

import (
	"errors"
	"strings"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/hashtag"
)

func ptr[T any](v T) *T { return &v }

func base() model.ProductInput {
	return model.ProductInput{
		Title: " 蘋果 ", CategoryID: uuid.NewV7(),
		Skus: []model.SkuInput{{Price: 100}},
	}
}

func wantInvalid(t *testing.T, err error, field, reason string) {
	t.Helper()
	var ie *model.InvalidInputError
	if !errors.As(err, &ie) {
		t.Fatalf("want InvalidInputError %s/%s, got %v", field, reason, err)
	}
	if ie.Field != field || ie.Reason != reason {
		t.Fatalf("got %s/%s want %s/%s", ie.Field, ie.Reason, field, reason)
	}
}

func TestPrepareDefaultSku(t *testing.T) {
	p, err := prepare(base())
	if err != nil {
		t.Fatal(err)
	}
	if p.title != "蘋果" || len(p.skus) != 1 {
		t.Fatalf("%+v", p)
	}
	in := base()
	in.Skus = append(in.Skus, model.SkuInput{Option1: "x", Price: 1})
	_, err = prepare(in)
	wantInvalid(t, err, "Skus", model.ReasonTooMany)
	in = base()
	in.Skus = []model.SkuInput{{Option1: "x", Price: 1}}
	_, err = prepare(in)
	wantInvalid(t, err, "Skus[0].Option1", model.ReasonUnknown)
}

func TestSkuCombinationsMustFallWithinOptionValues(t *testing.T) {
	axes := []model.Axis{{Name: "顏色", Values: []string{"紅", "藍"}}, {Name: "尺寸", Values: []string{"S", "M"}}}
	good := func() model.ProductInput {
		in := base()
		in.Axes = axes
		in.Skus = []model.SkuInput{{Option1: "紅", Option2: "S", Price: 10}, {Option1: "藍", Option2: "M", Price: 20}}
		return in
	}
	if _, err := prepare(good()); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name          string
		mutate        func(*model.ProductInput)
		field, reason string
	}{
		{"value outside axis", func(in *model.ProductInput) { in.Skus[0].Option1 = "綠" }, "Skus[0].Option1", model.ReasonUnknown},
		{"value from the other axis", func(in *model.ProductInput) { in.Skus[0].Option2 = "紅" }, "Skus[0].Option2", model.ReasonUnknown},
		{"missing second option", func(in *model.ProductInput) { in.Skus[1].Option2 = "" }, "Skus[1].Option2", model.ReasonRequired},
		{"duplicate combination", func(in *model.ProductInput) { in.Skus[1] = model.SkuInput{Option1: " 紅 ", Option2: "S", Price: 5} }, "Skus[1]", model.ReasonDuplicate},
		{"price zero", func(in *model.ProductInput) { in.Skus[0].Price = 0 }, "Skus[0].Price", model.ReasonOutOfRange},
		{"compare at not above price", func(in *model.ProductInput) { in.Skus[0].CompareAtPrice = ptr(int64(10)) }, "Skus[0].CompareAtPrice", model.ReasonOutOfRange},
		{"negative stock", func(in *model.ProductInput) { in.Skus[0].StockQty = -1 }, "Skus[0].StockQty", model.ReasonOutOfRange},
		{"no skus", func(in *model.ProductInput) { in.Skus = nil }, "Skus", model.ReasonRequired},
		{"three axes", func(in *model.ProductInput) { in.Axes = append(in.Axes, model.Axis{Name: "x", Values: []string{"1"}}) }, "Axes", model.ReasonTooMany},
		{"duplicate axis name", func(in *model.ProductInput) { in.Axes[1].Name = "顏色" }, "Axes[1].Name", model.ReasonDuplicate},
		{"duplicate value", func(in *model.ProductInput) { in.Axes[0].Values = []string{"紅", "紅"} }, "Axes[0].Values[1]", model.ReasonDuplicate},
		{"axis without values", func(in *model.ProductInput) { in.Axes[0].Values = nil }, "Axes[0].Values", model.ReasonRequired},
	}
	for _, c := range cases {
		in := good()
		in.Axes = append([]model.Axis(nil), axes...)
		in.Axes[0].Values = append([]string(nil), axes[0].Values...)
		c.mutate(&in)
		_, err := prepare(in)
		t.Run(c.name, func(t *testing.T) { wantInvalid(t, err, c.field, c.reason) })
	}
	one := base()
	one.Axes = axes[:1]
	one.Skus = []model.SkuInput{{Option1: "紅", Option2: "S", Price: 1}}
	_, err := prepare(one)
	wantInvalid(t, err, "Skus[0].Option2", model.ReasonUnknown)
}

func TestPrepareTitleAndHashtags(t *testing.T) {
	in := base()
	in.Title = " "
	_, err := prepare(in)
	wantInvalid(t, err, "Title", model.ReasonRequired)
	in.Title = strings.Repeat("a", model.MaxProductInputTitleRunes+1)
	_, err = prepare(in)
	wantInvalid(t, err, "Title", model.ReasonTooLong)

	in = base()
	in.Hashtags = []string{"#Fruit", "FRUIT", "水果"}
	p, err := prepare(in)
	if err != nil || len(p.tags) != 2 || p.tags[0].Name != "fruit" || p.tags[0].Display != "Fruit" {
		t.Fatalf("%+v %v", p.tags, err)
	}
	in.Hashtags = []string{"a b"}
	_, err = prepare(in)
	wantInvalid(t, err, "Hashtags", model.ReasonInvalid)
	if !errors.Is(err, hashtag.ErrInvalidChars) {
		t.Fatalf("cause lost: %v", err)
	}
}

func TestPrepareRejectsDuplicateImageRefs(t *testing.T) {
	in := base()
	in.Gallery = []model.Image{{Token: "t"}}
	in.Description = []model.Block{{Kind: model.BlockImage, Image: &model.Image{Token: "t"}}}
	_, err := prepare(in)
	wantInvalid(t, err, "Description.Image[0]", model.ReasonDuplicate)
	in.Gallery = make([]model.Image, model.MaxProductInputGallery+1)
	_, err = prepare(in)
	wantInvalid(t, err, "Gallery", model.ReasonTooMany)
}
