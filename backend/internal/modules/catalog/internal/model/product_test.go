package model_test

import (
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
