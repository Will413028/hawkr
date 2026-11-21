package retention

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

var (
	createTable = regexp.MustCompile(`(?s)CREATE TABLE procurement\.(\w+) \((.*?)\n\);`)
	sqlComment  = regexp.MustCompile(`--[^\n]*`)
)

// buyerIDTables 列出 schema 文字中含 buyer_id 欄位的表。
func buyerIDTables(schema string) []string {
	schema = sqlComment.ReplaceAllString(schema, "")
	var out []string
	for _, m := range createTable.FindAllStringSubmatch(schema, -1) {
		for _, line := range strings.Split(m[2], "\n") {
			if f := strings.Fields(line); len(f) > 0 && f[0] == "buyer_id" {
				out = append(out, m[1])
				break
			}
		}
	}
	return out
}

func missing(schema string, covered map[string]string) []string {
	var out []string
	for _, tbl := range buyerIDTables(schema) {
		if _, ok := covered[tbl]; !ok {
			out = append(out, tbl)
		}
	}
	return out
}

// 含 buyer_id 的表都要有清除步驟或明列豁免；新增一張含 buyer_id 的表而沒登記，BuyerDeleted 就會漏掉它。
func TestRetentionCoversEveryBuyerIdTable(t *testing.T) {
	b, err := os.ReadFile("../../db/schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	if len(buyerIDTables(string(b))) < 5 {
		t.Fatalf("解析不到含 buyer_id 的表：%v", buyerIDTables(string(b)))
	}
	if m := missing(string(b), Covered()); len(m) > 0 {
		t.Fatalf("這些含 buyer_id 的表沒有出現在 retention 的清單（登記步驟，或在 exempt 寫理由）：%v", m)
	}
}

// 守門本身要抓得到新增的、沒登記的表。
func TestRetentionGuardCatchesAnUnregisteredBuyerIdTable(t *testing.T) {
	schema := "CREATE TABLE procurement.conversations (\n    id UUID,\n    buyer_id BIGINT NOT NULL\n);\n" +
		"CREATE TABLE procurement.brand_new (\n    id UUID,\n    buyer_id BIGINT -- 註解\n);\n" +
		"CREATE TABLE procurement.other (\n    id UUID,\n    note TEXT\n);\n"
	got := missing(schema, Covered())
	if len(got) != 1 || got[0] != "brand_new" {
		t.Fatalf("missing = %v，要剛好是 brand_new", got)
	}
}

func TestExemptionsStateAReason(t *testing.T) {
	for tbl, why := range exempt {
		if strings.TrimSpace(why) == "" {
			t.Errorf("%s 的豁免沒有理由", tbl)
		}
	}
}
