package description

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/model"
)

func reason(t *testing.T, err error) (string, string) {
	t.Helper()
	var ie *model.InvalidInputError
	if !errors.As(err, &ie) || !errors.Is(err, model.ErrInvalidInput) {
		t.Fatalf("want InvalidInputError, got %v", err)
	}
	return ie.Field, ie.Reason
}

func TestNormalizeAcceptsFourBlockKinds(t *testing.T) {
	in := []model.Block{
		{Kind: model.BlockParagraph, Text: "  hello\nworld  "},
		{Kind: model.BlockImage, Image: &model.Image{Token: "t1", Alt: " a "}},
		{Kind: model.BlockList, Items: []string{" x ", "y"}},
		{Kind: model.BlockTable, Rows: [][]string{{"a", ""}, {"b", "c"}}},
	}
	out, err := Normalize("Description", in)
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Text != "hello\nworld" || out[1].Image.Alt != "a" || out[2].Items[0] != "x" {
		t.Fatalf("%+v", out)
	}
	if got := Text(out); got != "hello\nworld x y a b c" {
		t.Fatalf("Text = %q", got)
	}
}

func TestNormalizeRejects(t *testing.T) {
	long := strings.Repeat("字", model.MaxDescriptionRunes+1)
	cell := strings.Repeat("a", model.MaxTableCellRunes)
	list := make([]string, model.MaxListItems)
	for i := range list {
		list[i] = cell
	}
	cases := []struct {
		name   string
		blocks []model.Block
		field  string
		reason string
	}{
		{"unknown kind", []model.Block{{Kind: "html"}}, "Description[0].Kind", model.ReasonUnknown},
		{"empty paragraph", []model.Block{{Kind: model.BlockParagraph, Text: " "}}, "Description[0].Text", model.ReasonRequired},
		{"long paragraph", []model.Block{{Kind: model.BlockParagraph, Text: long}}, "Description", model.ReasonTooLong},
		{"total across paragraph, list and table", []model.Block{
			{Kind: model.BlockParagraph, Text: strings.Repeat("字", model.MaxDescriptionRunes-model.MaxListItemRunes*2)},
			{Kind: model.BlockList, Items: []string{cell, cell}},
			{Kind: model.BlockTable, Rows: [][]string{{"x"}}},
		}, "Description", model.ReasonTooLong},
		{"total across list items", []model.Block{{Kind: model.BlockList, Items: list}, {Kind: model.BlockList, Items: list}}, "Description", model.ReasonTooLong},
		{"image without ref", []model.Block{{Kind: model.BlockImage, Image: &model.Image{}}}, "Description[0].Image", model.ReasonInvalid},
		{"image with both", []model.Block{{Kind: model.BlockImage, Image: &model.Image{Token: "t", AssetID: uuid.NewV7()}}}, "Description[0].Image", model.ReasonInvalid},
		{"empty list", []model.Block{{Kind: model.BlockList}}, "Description[0].Items", model.ReasonRequired},
		{"too many items", []model.Block{{Kind: model.BlockList, Items: make([]string, model.MaxListItems+1)}}, "Description[0].Items", model.ReasonTooMany},
		{"ragged table", []model.Block{{Kind: model.BlockTable, Rows: [][]string{{"a", "b"}, {"c"}}}}, "Description[0].Rows[1]", model.ReasonInvalid},
		{"too many cols", []model.Block{{Kind: model.BlockTable, Rows: [][]string{make([]string, model.MaxTableCols+1)}}}, "Description[0].Rows", model.ReasonTooMany},
		{"too many rows", []model.Block{{Kind: model.BlockTable, Rows: make([][]string, model.MaxTableRows+1)}}, "Description[0].Rows", model.ReasonTooMany},
		{"long cell", []model.Block{{Kind: model.BlockTable, Rows: [][]string{{strings.Repeat("a", model.MaxTableCellRunes+1)}}}}, "Description[0].Rows[0][0]", model.ReasonTooLong},
		{"too many blocks", make([]model.Block, model.MaxDescriptionBlocks+1), "Description", model.ReasonTooMany},
	}
	for _, c := range cases {
		_, err := Normalize("Description", c.blocks)
		f, r := reason(t, err)
		if f != c.field || r != c.reason {
			t.Errorf("%s: got %s %s, want %s %s", c.name, f, r, c.field, c.reason)
		}
	}
	// 邊界值可以。
	edge := []model.Block{
		{Kind: model.BlockParagraph, Text: strings.Repeat("字", model.MaxDescriptionRunes)},
		{Kind: model.BlockTable, Rows: [][]string{make([]string, model.MaxTableCols)}},
		{Kind: model.BlockImage, Image: &model.Image{Token: "t", Alt: strings.Repeat("字", model.MaxAltRunes)}},
	}
	if _, err := Normalize("Description", edge); err != nil {
		t.Fatalf("edge (exact total, empty cells and alt do not count): %v", err)
	}
	// 只差一個字就超過：同一個總數，段落先佔 MaxDescriptionRunes-1 再一個格子兩字。
	over := []model.Block{
		{Kind: model.BlockParagraph, Text: strings.Repeat("字", model.MaxDescriptionRunes-1)},
		{Kind: model.BlockTable, Rows: [][]string{{"ab"}}},
	}
	if _, err := Normalize("Description", over); err == nil {
		t.Fatal("one rune over the total must fail")
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	asset := uuid.NewV7()
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "p"},
		{Kind: model.BlockImage, Image: &model.Image{AssetID: asset}},
		{Kind: model.BlockList, Items: []string{"a"}},
		{Kind: model.BlockTable, Rows: [][]string{{"1", "2"}}},
	}
	raw, err := Encode(blocks)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(raw, map[uuid.UUID]model.Image{asset: {AssetID: asset, ObjectKey: "k", Alt: "alt"}})
	if err != nil {
		t.Fatal(err)
	}
	want := append([]model.Block(nil), blocks...)
	want[1] = model.Block{Kind: model.BlockImage, Image: &model.Image{AssetID: asset, ObjectKey: "k", Alt: "alt"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
	if _, err := Encode([]model.Block{{Kind: model.BlockImage, Image: &model.Image{Token: "t"}}}); err == nil {
		t.Fatal("encoding an unconfirmed image must fail")
	}
}
