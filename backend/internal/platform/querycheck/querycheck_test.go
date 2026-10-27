package querycheck

import (
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want []string // 違規訊息要包含的片段；空表示通過
	}{
		{"自己的表與允許的 pub", `SELECT p.id FROM social.posts p WHERE EXISTS (SELECT 1 FROM seller.pub_exposable_stores($1) e WHERE e.store_id = p.store_id)`, nil},
		{"CTE 與內建函式", `WITH x AS (SELECT count(*) AS n FROM social.posts) SELECT n FROM x`, nil},
		{"別的模組的表", `SELECT 1 FROM social.posts p JOIN seller.stores s ON s.id = p.store_id`, []string{"seller.stores"}},
		{"沒有 schema 的表", `SELECT 1 FROM posts`, []string{"表 .posts"}},
		{"別的模組的非 pub 函式", `SELECT seller.effective_measures(now())`, []string{"seller.effective_measures"}},
		{"沒有依賴的模組的 pub", `SELECT 1 FROM ordering.pub_orders() o`, []string{"ordering.pub_orders"}},
		{"不帶 schema 的未知函式", `SELECT do_something(1)`, []string{"do_something"}},
		{"now() 不允許", `SELECT 1 FROM social.posts p WHERE p.published_at < now()`, []string{"取得現在時間"}},
		{"帶 schema 的 now 也不允許", `SELECT pg_catalog.now()`, []string{"pg_catalog.now"}},
		{"clock_timestamp 不允許", `SELECT clock_timestamp()`, []string{"clock_timestamp"}},
		{"CURRENT_TIMESTAMP 不允許", `SELECT 1 FROM social.posts p WHERE p.published_at < CURRENT_TIMESTAMP`, []string{"CURRENT_TIMESTAMP"}},
		{"UPDATE 的目標也算", `UPDATE seller.stores SET name = 'x'`, []string{"seller.stores"}},
	}
	for _, tc := range tests {
		got, err := Check(tc.sql, "social", []string{"buyer", "seller", "media"})
		if err != nil {
			t.Fatalf("%s：%v", tc.name, err)
		}
		if len(tc.want) == 0 && len(got) != 0 {
			t.Errorf("%s：不該有違規，卻是 %v", tc.name, got)
		}
		for _, w := range tc.want {
			if !strings.Contains(strings.Join(got, "\n"), w) {
				t.Errorf("%s：違規 %v 沒有提到 %q", tc.name, got, w)
			}
		}
	}
	if _, err := Check(`SELEC 1`, "social", nil); err == nil {
		t.Error("語法錯誤的 SQL 要回錯誤，不能當成通過")
	}
}
