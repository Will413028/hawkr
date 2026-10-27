package postgres_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// schema.sql 的 generated column 與 surface_form.sql 查詢參數用的正規化運算式必須
// 逐字相同——這是「analyzer 綁定索引」在 Atlas OSS（不能用 SQL function）下的唯一
// 機械保證。任一邊改了、另一邊沒改，索引與查詢就對不上，症狀是零命中，不是報錯。
func TestSurfaceFormNormExpr_SchemaAndQueryIdentical(t *testing.T) {
	schemaExpr := normExprBetweenMarkers(t, "../../../db/schema/schema.sql", "form_raw")
	queryExpr := normExprBetweenMarkers(t, "../../../db/queries/surface_form.sql", "sqlc.arg(query)::text")
	require.Equal(t, schemaExpr, queryExpr,
		"schema.sql 與 surface_form.sql 的 norm-expr 區塊不一致（已把各自的輸入欄位/參數替換成佔位符後比對）")
}

func normExprBetweenMarkers(t *testing.T, path, inputToken string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	s := string(raw)
	const begin, end = "-- norm-expr-begin", "-- norm-expr-end"
	start := strings.Index(s, begin)
	stop := strings.Index(s, end)
	require.True(t, start >= 0 && stop > start, "%s 缺標記行 %s / %s", path, begin, end)
	require.Equal(t, 1, strings.Count(s, begin), "%s 的標記行應恰好一組", path)
	expr := s[start+len(begin) : stop]
	require.Equal(t, 1, strings.Count(expr, inputToken), "%s 的運算式應恰好引用一次輸入 %q", path, inputToken)
	expr = strings.ReplaceAll(expr, inputToken, "<INPUT>")
	return strings.Join(strings.Fields(expr), " ") // 只容忍空白差異，其餘逐字元
}
