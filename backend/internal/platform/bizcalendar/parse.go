package bizcalendar

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

// 辦公日曆表 CSV 的格式（人事行政總處，data.gov.tw 資料集 14718；實測 115、116 年檔）。
const (
	officeCalendarHeader = "西元日期,星期,是否放假,備註"
	fieldsPerRecord      = 4
	flagWorking          = "0"
	flagHoliday          = "2"
	// MinYear 與 MaxYear 是可匯入的西元年範圍：日期欄 YYYYMMDD 的四位年份能表示的範圍。
	MinYear = 1
	MaxYear = 9999
)

// ParseOfficeCalendar 解析一年的辦公日曆表 CSV，回傳該年每一天各一筆（依檔案的列序）。格式嚴格，任何偏差都整份拒絕，不略過不合的列：
//   - 內容是 UTF-8，可帶 BOM；依 RFC 4180 以 encoding/csv 讀，換行是 LF 或 CRLF，空行略過，欄位可以加引號（備註含逗號時需要）；
//   - 第 1 行是表頭 `西元日期,星期,是否放假,備註`，之後每列恰好四欄；
//   - 日期是 `YYYYMMDD` 的真實日期，年份等於 year，同一天只能一列；是否放假只有 `0`（上班）與 `2`（放假）；備註去頭尾空白、可以空；
//   - 星期欄不參與判斷：一天是不是工作天只看是否放假；
//   - 全年每一天都要有（365 或 366 列），缺的日子在錯誤訊息裡列出第一天。
//
// 錯誤是 InvalidInput，欄位為 CSVContent（或 Year），訊息是繁體中文並帶行號（欄位內含換行時是該列開始的行）；csv 套件的英文錯誤不會傳給使用者。
func ParseOfficeCalendar(year int, content string) ([]Day, error) {
	if year < MinYear || year > MaxYear {
		return nil, apperr.InvalidInputOn([]string{"Year"}, "年份必須在 %d 到 %d 之間", MinYear, MaxYear)
	}
	bad := func(line int, format string, args ...any) error {
		return apperr.InvalidInputOn([]string{"CSVContent"}, "第 %d 行：%s", line, fmt.Sprintf(format, args...))
	}
	r := csv.NewReader(strings.NewReader(strings.TrimPrefix(content, "\uFEFF")))
	r.FieldsPerRecord = fieldsPerRecord
	header, err := r.Read()
	if errors.Is(err, io.EOF) {
		return nil, apperr.InvalidInputOn([]string{"CSVContent"}, "CSV 內容是空的")
	}
	if err != nil {
		return nil, readError(err, header, bad, true)
	}
	if strings.Join(header, ",") != officeCalendarHeader {
		line, _ := r.FieldPos(0)
		return nil, bad(line, "表頭必須是 %s", officeCalendarHeader)
	}
	out := make([]Day, 0, daysInYear(year))
	seen := make(map[int64]int, daysInYear(year))
	for {
		cols, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, readError(err, cols, bad, false)
		}
		n, _ := r.FieldPos(0)
		if slices.ContainsFunc(cols, func(f string) bool { return strings.ContainsRune(f, 0) }) {
			return nil, bad(n, "欄位不得包含 NUL 字元")
		}
		date, err := parseCompactDate(cols[0])
		if err != nil {
			return nil, bad(n, "日期 %q 不是 YYYYMMDD 的有效日期", cols[0])
		}
		if date.Year() != year {
			return nil, bad(n, "日期 %s 不在 %d 年", date.Format(time.DateOnly), year)
		}
		if first, dup := seen[dayNumber(date)]; dup {
			return nil, bad(n, "日期 %s 與第 %d 行重複", date.Format(time.DateOnly), first)
		}
		seen[dayNumber(date)] = n
		var holiday bool
		switch cols[2] {
		case flagHoliday:
			holiday = true
		case flagWorking:
		default:
			return nil, bad(n, "是否放假必須是 %s 或 %s，實際是 %q", flagWorking, flagHoliday, cols[2])
		}
		out = append(out, Day{Date: date, Holiday: holiday, Note: trimtext.Trim(cols[3])})
	}
	for d := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC); d.Year() == year; d = d.AddDate(0, 0, 1) {
		if _, ok := seen[dayNumber(d)]; !ok {
			return nil, apperr.InvalidInputOn([]string{"CSVContent"}, "%d 年應有 %d 天，實際 %d 天，缺 %s", year, daysInYear(year), len(out), d.Format(time.DateOnly))
		}
	}
	return out, nil
}

// readError 把 csv.Reader 的錯誤換成繁體中文的訊息；表頭列（isHeader）欄數不對視為表頭不合。
func readError(err error, record []string, bad func(int, string, ...any) error, isHeader bool) error {
	var pe *csv.ParseError
	if !errors.As(err, &pe) {
		return apperr.InvalidInputOn([]string{"CSVContent"}, "CSV 內容無法讀取")
	}
	line := pe.StartLine
	switch {
	case errors.Is(err, csv.ErrFieldCount) && isHeader:
		return bad(line, "表頭必須是 %s", officeCalendarHeader)
	case errors.Is(err, csv.ErrFieldCount):
		return bad(line, "應有 %d 欄，實際 %d 欄", fieldsPerRecord, len(record))
	default:
		return bad(line, "引號格式不正確")
	}
}

// parseCompactDate 解析恰好 8 位數字的 YYYYMMDD；time.Parse 會拒絕不存在的日期（2 月 30 日）。
func parseCompactDate(s string) (time.Time, error) {
	if len(s) != 8 {
		return time.Time{}, fmt.Errorf("length")
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return time.Time{}, fmt.Errorf("digit")
		}
	}
	return time.Parse("20060102", s)
}
