// Package postgres 提供 pricebook-service 的 Postgres 儲存層。
package postgres

import (
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

// pgtype.Numeric 的字串橋接。
//
// 本服務是 repo 內第一個使用 NUMERIC 欄位的 Go 服務，`pkg/pgxutil` 沒有對應 helper；
// 這裡刻意留在 pricebook repository，避免把 precision-sensitive numeric 轉換
// 混進共用 nullable boundary。
//
// 🔴 一律走字串，不要經過 float64：
//  1. float64 會在 NUMERIC(12,4) / NUMERIC(14,3) 的位數上失去精度；
//  2. float64 沒有 NULL，`NumericValue() float64` 對 NULL 會回 0 —— 而 0 是合法價格，
//     那等於把「缺值」寫成一個看起來正常的觀測值，正是 schema 註解禁止的缺值哨兵。
//
// 這個承諾是靠 NumericFromString 擋掉科學記號輸入來兌現的，不是靠自律：
// `pgtype.Numeric.ScanScientific` 對含 e/E 的輸入會走 `big.Float.Float64()` +
// `FormatFloat`（pgx pgtype/numeric.go:130-137），那條路真的會經過 float64。
// 擋在入口，這份檔案裡就不存在任何 float64 路徑。
//
// NULL 的表達方式是 `pgtype.Numeric` 的零值（`Valid: false`）：缺值時**不要**呼叫
// NumericFromString，直接傳零值即可。

// NumericFromString 把十進位字串（例如 "46.2000"、"0"、"-3.5"）轉成 pgtype.Numeric。
//
// 以下輸入一律回錯誤，而**不會**默默變成 0、NULL 或某個看起來正常的值 ——
// 缺值請直接用 pgtype.Numeric 零值，讓呼叫端在「沒有值」與「值壞掉」之間必須做出選擇：
//
//   - 空字串與無法解析的字串。
//   - 科學記號（含 e/E）：pgx 那條路會經過 float64，超過約 17 位有效數字就失真。
//   - "NaN" / "Infinity" / "-Infinity"：Postgres 的 numeric 收這三個字面值，
//     pgx 也會把它們掃成 `Valid: true`，於是它們會像一筆正常價格那樣寫進 DB。
//     🔴 這是第二種「披著真實價格外衣的缺值哨兵」，而且比 0 更危險 ——
//     NaN 在 Postgres 排序中大於一切有限值，Task 9 的對帳會讀到一筆歷史新高而不是錯誤。
func NumericFromString(s string) (pgtype.Numeric, error) {
	if strings.ContainsAny(s, "eE") {
		return pgtype.Numeric{}, fmt.Errorf("parse numeric %q: 不接受科學記號（該路徑會經過 float64 而失真）", s)
	}
	var n pgtype.Numeric
	if err := n.ScanScientific(s); err != nil {
		return pgtype.Numeric{}, fmt.Errorf("parse numeric %q: %w", s, err)
	}
	if n.NaN || n.InfinityModifier != pgtype.Finite {
		return pgtype.Numeric{}, fmt.Errorf("parse numeric %q: 不接受非有限值（NaN / Infinity）", s)
	}
	return n, nil
}

// NumericToPtr 把 pgtype.Numeric 轉回十進位字串指標。
//
// NULL（`Valid == false`）回 nil；其餘回指向完整精度字串的指標。回傳 *string 而非 string
// 就是為了讓 NULL 與 "0" 在型別上無法互相冒充。
//
// 編碼失敗時同樣回 nil。pgx 掃出來的值與 NumericFromString 產出的值都必定可編碼，
// 因此這個分支在正常路徑上不可達。
//
// 兩點對呼叫端的提醒：
//   - 若 DB 裡真的存在 NaN / Infinity（本服務的寫入路徑已擋掉，但別的工具可能寫進去），
//     這裡會原樣回 "NaN" / "Infinity"，不會偽裝成 nil 或 0 —— 讓它看得見，而不是被吞掉。
//   - **不要手工組 pgtype.Numeric**。`pgtype.Numeric{Valid: true, Int: nil}` 這種手工值
//     不會回 nil：pgx v5.9.2 會渲染成 "0"（numeric.go:267 的 nil 短路），
//     v5.8.0 則渲染成 "<nil>"。一律用 NumericFromString 或 pgx 掃出來的值。
func NumericToPtr(n pgtype.Numeric) *string {
	if !n.Valid {
		return nil
	}
	v, err := n.Value()
	if err != nil {
		return nil
	}
	s, ok := v.(string)
	if !ok {
		return nil
	}
	return &s
}
