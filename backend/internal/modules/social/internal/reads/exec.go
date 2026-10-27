// Package reads 是 social 帶「公開可讀、可見、計入」規則的 query（A7-1a）。它們用 go-jet 在 visibility 的述詞上建，
// 在呼叫方的 db.Conn(ctx) 上執行，以 pgx 的 RowToStructByName 掃描；其餘靜態的 query（寫入、鎖、清除、媒體引用、店主的讀取）是 sqlc。
//
// 守住 sqlc 原本給的保證的測試（golden_test.go、guard_test.go，以及根套件 reads_integration_test.go 的 Describe 形狀測試）：
//   - 邊界（單體規則 2）：登記的每支 query 渲染出來的 SQL，以 pg_query 解析（platform/querycheck），表只能是 social.*，函式只能是 social.*、內建白名單或依賴表允許的 <擁有者>.pub_*；
//   - 結果的形狀：每支 query 對測試資料庫 Describe（不執行），欄位名稱與型別必須對上 row struct；
//   - 渲染出來的 SQL 有 golden 檔（testdata/golden/*.sql），改 query 就看得到 SQL 的差異；
//   - 公開表（posts、comments、forum_entries）只經 visibility 的入口取得，原始 SQL 只出現在 pubsql 的生成檔（AST 測試）。
package reads

import (
	"context"
	"fmt"

	. "github.com/go-jet/jet/v2/postgres"
	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// Stmt 是 reads 能執行的語句（jet 的 Statement）；呼叫方只選要建哪一支，不碰 jet 的型別。
type Stmt = Statement

// Many 執行 st，把每一列掃進 R。欄位與 R 的欄位必須一一對應（pgx.RowToStructByName：多或少都是錯誤）。
func Many[R any](ctx context.Context, c db.DBTX, st Statement) ([]R, error) {
	q, args := st.Sql()
	rows, err := c.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("reads：執行：%w", err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByName[R])
	if err != nil {
		return nil, fmt.Errorf("reads：掃描：%w", err)
	}
	return out, nil
}

// One 執行 st 並回傳第一列；沒有列是 pgx.ErrNoRows。
func One[R any](ctx context.Context, c db.DBTX, st Statement) (R, error) {
	q, args := st.Sql()
	rows, err := c.Query(ctx, q, args...)
	if err != nil {
		var zero R
		return zero, fmt.Errorf("reads：執行：%w", err)
	}
	return pgx.CollectOneRow(rows, pgx.RowToStructByName[R])
}

// plain 把欄位逐一別名成不帶表名的欄位名：jet 預設的別名是 "表.欄"，而 pgx 的 RowToStructByName 以欄位名對應 struct。
func plain(cols ColumnList) ProjectionList {
	out := make(ProjectionList, len(cols))
	for i, c := range cols {
		out[i] = c.AS(c.Name())
	}
	return out
}

func project(parts ...ProjectionList) ProjectionList {
	var out ProjectionList
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// Flag 執行只回一個旗標的語句。
func Flag(ctx context.Context, c db.DBTX, st Statement) (bool, error) {
	r, err := One[BoolRow](ctx, c, st)
	return r.Value, err
}
