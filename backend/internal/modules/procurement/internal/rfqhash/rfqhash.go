package rfqhash

import "time"

// SchemaV1 是第一版原像的 schema_version。
const SchemaV1 = "v1"

// SchemaV2 是賣家回價原像的第二版（以 spec_note 取代品級與檢驗備註）；受眾與賣家可見內容仍是 SchemaV1。
const SchemaV2 = "v2"

// Date 把日期欄位轉成 hash 輸入的 YYYY-MM-DD 字串；nil 是沒有值。
func Date(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.DateOnly)
	return &s
}
