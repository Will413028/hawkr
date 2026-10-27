package timeutil

import (
	"fmt"
	"time"
)

// FormatISO8601 將 time.Time 格式化為 ISO 8601 UTC 字串。
// 輸出格式：2026-03-31T08:30:00Z（不帶小數秒）。
func FormatISO8601(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// ParseISO8601 解析 ISO 8601 時間字串，支援帶/不帶小數秒。
// Java DateTimeFormatter.ISO_OFFSET_DATE_TIME 在 nano 非零時會輸出小數秒，
// Go 的 time.RFC3339 不接受小數秒，因此需要 fallback 到 RFC3339Nano。
func ParseISO8601(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339Nano, s)
}
