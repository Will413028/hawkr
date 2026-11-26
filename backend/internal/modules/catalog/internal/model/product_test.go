package model_test

import (
	"errors"
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/model"
)

// 儲存端 CHECK 的字面上限要等於生成的常數（上限的唯一來源是 api/ 的 limits option）。
func TestSchemaCheckLimitsMatchGeneratedConstants(t *testing.T) {
	raw, err := os.ReadFile("../../db/schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	schema := string(raw)
	atoi := func(s string) int {
		n, err := strconv.Atoi(s)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	// 名為 constraint 的 CHECK 裡 char_length(column) <= N 的 N。
	maxLen := func(constraint, column string) int {
		re := regexp.MustCompile(`CONSTRAINT ` + constraint + ` CHECK \([^\n]*char_length\(` + column + `\) <= (\d+)`)
		m := re.FindStringSubmatch(schema)
		if m == nil {
			t.Fatalf("schema.sql 沒有 %s 的 char_length(%s) <= N", constraint, column)
		}
		return atoi(m[1])
	}
	for _, c := range []struct {
		constraint, column string
		want               int
	}{
		{"products_title_check", "title", model.MaxProductInputTitleRunes},
		{"option_axes_name_check", "name", model.MaxOptionAxisInputNameRunes},
		{"option_values_value_check", "value", model.MaxOptionAxisInputValuesRunes},
		{"product_images_alt_check", "alt", model.MaxImageInputAltRunes},
	} {
		if got := maxLen(c.constraint, c.column); got != c.want {
			t.Errorf("%s: schema.sql 的上限 %d，生成的常數 %d", c.constraint, got, c.want)
		}
	}
	m := regexp.MustCompile(`CONSTRAINT option_axes_position_check CHECK \(position IN \(([\d, ]+)\)\)`).FindStringSubmatch(schema)
	if m == nil {
		t.Fatal("schema.sql 沒有 option_axes_position_check 的 IN 清單")
	}
	var last int
	for _, n := range regexp.MustCompile(`\d+`).FindAllString(m[1], -1) {
		last = atoi(n)
	}
	if last != model.MaxProductInputAxes {
		t.Errorf("option_axes_position_check 的最大位置 %d，MaxProductInputAxes %d", last, model.MaxProductInputAxes)
	}
}

func TestSuggestedSafetyStock(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		weekly []int32
		want   int32
	}{
		{"沒有銷量是 10", nil, 10},
		{"全部為 0 是 10", []int32{0, 0, 0, 0, 0, 0, 0, 0}, 10},
		{"銷量很低時取下限 5", []int32{1, 0, 0, 0, 0, 0, 0, 0}, 5},
		{"剛好在下限：8 週共 20 件，平均 2.5，兩週 5", []int32{3, 2, 3, 2, 3, 2, 3, 2}, 5},
		{"超過下限：8 週共 80 件，平均 10，兩週 20", []int32{10, 10, 10, 10, 10, 10, 10, 10}, 20},
		{"無條件進位：8 週共 22 件 → 兩週 5.5 → 6", []int32{3, 3, 3, 3, 3, 3, 2, 2}, 6},
		{"無條件進位：8 週共 25 件 → 兩週 6.25 → 7", []int32{4, 3, 3, 3, 3, 3, 3, 3}, 7},
		{"週數不足的缺的週算 0", []int32{40}, 10},
		{"超過 8 週只取最後 8 週：前 4 週各 100、後 8 週各 10 → 20", []int32{100, 100, 100, 100, 10, 10, 10, 10, 10, 10, 10, 10}, 20},
	}
	for _, c := range cases {
		if got := model.SuggestedSafetyStock(c.weekly); got != c.want {
			t.Errorf("%s：got %d, want %d", c.name, got, c.want)
		}
	}
}

func TestEtagRoundTripsAndRejectsGarbage(t *testing.T) {
	t.Parallel()
	for _, v := range []int64{1, 2, 99} {
		got, err := model.ParseEtag(model.Etag(v))
		if err != nil || got != v {
			t.Errorf("ParseEtag(Etag(%d)) = %d, %v", v, got, err)
		}
	}
	for _, bad := range []string{"", "0", "-1", "abc", "01", "1.5", " 1"} {
		if _, err := model.ParseEtag(bad); !errors.Is(err, model.ErrInvalidInput) {
			t.Errorf("ParseEtag(%q) 應回 ErrInvalidInput：%v", bad, err)
		}
	}
}
