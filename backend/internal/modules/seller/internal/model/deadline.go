package model

import "time"

// Deadline 是一個有期限的義務：從 Start 起算，到 Due 到期。兩者都由 SQL 的審閱政策函式（db/objects.sql 的 seller.review_<種類>(at)）算出，
// 或是存下的值；Go 不寫時長，也不從別的欄位重建起算規則。只在義務還在時有值（資源上是 *Deadline，nil 表示沒有義務）。
type Deadline struct {
	Start, Due time.Time
}
