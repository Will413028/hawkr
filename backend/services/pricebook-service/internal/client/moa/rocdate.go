package moa

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ToROCDate 西元日期 → 民國年格式 YYY.MM.DD（MOA API 的輸入格式）
func ToROCDate(t time.Time) string {
	return fmt.Sprintf("%d.%02d.%02d", t.Year()-1911, t.Month(), t.Day())
}

// ParseROCDate 民國年格式 YYY.MM.DD → 西元日期（UTC 零時）。
//
// 嚴格拒絕行事曆上不存在的日期（例："115.02.30"）：range-check 只擋得住
// day > 31，"115.02.30" 這種月份合法、日期不合法的輸入會通過 range-check，
// 而 time.Date 對超出當月天數的 day 是靜默正規化成下個月（30 日的 2 月
// → 3 月 2 日），不是報錯。Task 5 的休市日曆 ingest 明確要防「一個打錯的
// 哨兵日期悄悄變成另一個合法日期流進資料庫」，所以在這裡的源頭就用
// round-trip 比對擋掉，而不是留給下游猜。
func ParseROCDate(s string) (time.Time, error) {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("民國年格式錯誤: %q", s)
	}
	year, err := strconv.Atoi(parts[0])
	if err != nil {
		return time.Time{}, fmt.Errorf("民國年年份無法解析: %q", s)
	}
	// strconv.Atoi 沒有下限檢查："0.07.24"、"-1.07.24" 都會被解析成合法整數，
	// 靜默對應到 1911-07-24 / 1910-07-24——與 :42 的 round-trip 檢查是同一類
	// 缺陷（不合理輸入被悄悄接受成一個看起來合法的日期），只是觸發情境比較
	// 少見。農業部行情資料最早只到民國 101 年，年份非正整數必然是輸入錯誤。
	if year < 1 {
		return time.Time{}, fmt.Errorf("民國年年份無效: %q", s)
	}
	month, err := strconv.Atoi(parts[1])
	if err != nil || month < 1 || month > 12 {
		return time.Time{}, fmt.Errorf("民國年月份無效: %q", s)
	}
	day, err := strconv.Atoi(parts[2])
	if err != nil || day < 1 || day > 31 {
		return time.Time{}, fmt.Errorf("民國年日期無效: %q", s)
	}

	t := time.Date(year+1911, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if t.Year() != year+1911 || int(t.Month()) != month || t.Day() != day {
		return time.Time{}, fmt.Errorf("民國年日期在行事曆上不存在: %q", s)
	}
	return t, nil
}
