package timeutil

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// FormatISO8601Ptr 格式化 *time.Time 為 ISO 8601 UTC 字串。
// nil 或零值回傳空字串。
func FormatISO8601Ptr(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return FormatISO8601(*t)
}

// FormatROCDate 將 *time.Time 轉換為民國年日期格式（YYY/MM/DD）。
// nil 或零值回傳空字串。
func FormatROCDate(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	rocYear := t.Year() - 1911
	return fmt.Sprintf("%03d/%02d/%02d", rocYear, int(t.Month()), t.Day())
}

// ParseROCDate 解析民國年日期字串（YYY/MM/DD）為 *time.Time（UTC）。
// 空字串回傳 (nil, nil)。格式錯誤回傳 error。
func ParseROCDate(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}

	parts := strings.Split(s, "/")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid ROC date format: %q (expected YYY/MM/DD)", s)
	}

	// 驗證年份為三位數
	if len(parts[0]) != 3 {
		return nil, fmt.Errorf("invalid ROC date format: %q (year must be 3 digits)", s)
	}
	// 驗證月日為兩位數
	if len(parts[1]) != 2 || len(parts[2]) != 2 {
		return nil, fmt.Errorf("invalid ROC date format: %q (month/day must be 2 digits)", s)
	}

	rocYear, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid ROC year: %q", parts[0])
	}
	month, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid month: %q", parts[1])
	}
	day, err := strconv.Atoi(parts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid day: %q", parts[2])
	}

	t := time.Date(rocYear+1911, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return &t, nil
}
