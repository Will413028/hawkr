package main

import (
	"fmt"
	"strings"
	"time"
)

func renderMigration(annotations []annotation) string {
	var b strings.Builder
	total := 0
	for _, a := range annotations {
		total += len(a.CategoryIDs)
	}
	fmt.Fprintf(&b, "-- 站外業者的結構化分類標籤，由 cmd/prospect-category-gen 於 %s 生成。\n", time.Now().UTC().Format("2006-01-02"))
	fmt.Fprintf(&b, "-- %d 個業者、%d 筆標籤。值域是平台分類樹的節點 id，允許 L1 或 L2。\n", len(annotations), total)
	fmt.Fprintf(&b, "-- 反向操作：DELETE FROM seller_prospect_categories WHERE prospect_id IN (…)。\n\n")
	fmt.Fprintf(&b, "-- JOIN seller_prospects 是刻意的：乾淨環境（測試容器、尚未遷移的 prod）沒有這些\n")
	fmt.Fprintf(&b, "-- 業者，直接 INSERT 會違反 FK 讓整支 migration 失敗。JOIN 讓它安全地插入 0 列。\n\n")
	b.WriteString("INSERT INTO seller_prospect_categories (prospect_id, category_id)\n")
	b.WriteString("SELECT v.prospect_id, v.category_id\nFROM (VALUES\n")

	first := true
	for _, a := range annotations {
		for _, id := range a.CategoryIDs {
			if !first {
				b.WriteString(",\n")
			}
			first = false
			fmt.Fprintf(&b, "  (%d::bigint, '%s'::varchar)", a.ProspectID, escape(id))
		}
	}
	if first {
		return "-- 沒有任何標籤可寫入。\n"
	}
	b.WriteString("\n) AS v(prospect_id, category_id)\n")
	b.WriteString("JOIN seller_prospects sp ON sp.id = v.prospect_id\n")
	b.WriteString("ON CONFLICT DO NOTHING;\n")
	return b.String()
}

func escape(s string) string { return strings.ReplaceAll(s, "'", "''") }
