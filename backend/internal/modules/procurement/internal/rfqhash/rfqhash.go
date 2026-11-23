package rfqhash

import "time"

// SchemaV1 是第一版原像的 schema_version。
const SchemaV1 = "v1"

// Date 把日期欄位轉成 hash 輸入的 YYYY-MM-DD 字串；nil 是沒有值。
func Date(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.DateOnly)
	return &s
}
