package bizcalendar

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

const header = "西元日期,星期,是否放假,備註"

// csvOf 組出 year 全年的 CSV：週六日放假，其餘上班；edit 可改單日的列（回傳新的整列文字，空字串表示整列省略）。
func csvOf(year int, edit func(date string, line string) string) string {
	lines := []string{header}
	for d := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC); d.Year() == year; d = d.AddDate(0, 0, 1) {
		flag := "0"
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			flag = "2"
		}
		key := d.Format("20060102")
		line := fmt.Sprintf("%s,?,%s,", key, flag)
		if edit != nil {
			line = edit(key, line)
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\r\n") + "\r\n"
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestParseRealFiles(t *testing.T) {
	for _, tc := range []struct {
		file          string
		year, holiday int
	}{{"115.csv", 2026, 120}, {"116.csv", 2027, 121}} {
		days, err := ParseOfficeCalendar(tc.year, readFixture(t, tc.file))
		if err != nil {
			t.Fatalf("%s：%v", tc.file, err)
		}
		holidays := 0
		for _, d := range days {
			if d.Holiday {
				holidays++
			}
		}
		if len(days) != 365 || holidays != tc.holiday {
			t.Errorf("%s：%d 天、%d 個放假日，要 365 與 %d", tc.file, len(days), holidays, tc.holiday)
		}
		if days[0].Note != "開國紀念日" || !days[0].Holiday {
			t.Errorf("%s：第一天 = %+v", tc.file, days[0])
		}
	}
}

func TestParseAcceptsLFWithoutBOMAndNoTrailingNewline(t *testing.T) {
	content := strings.TrimSuffix(strings.ReplaceAll(csvOf(2026, nil), "\r\n", "\n"), "\n")
	if days, err := ParseOfficeCalendar(2026, content); err != nil || len(days) != 365 {
		t.Fatalf("days=%d err=%v", len(days), err)
	}
}

func TestParseLeapYear(t *testing.T) {
	days, err := ParseOfficeCalendar(2028, csvOf(2028, nil))
	if err != nil || len(days) != 366 {
		t.Fatalf("days=%d err=%v", len(days), err)
	}
	_, err = ParseOfficeCalendar(2028, csvOf(2028, func(date, line string) string {
		if date == "20280229" {
			return ""
		}
		return line
	}))
	if err == nil || !strings.Contains(err.Error(), "2028-02-29") || !strings.Contains(err.Error(), "應有 366 天，實際 365 天") {
		t.Fatalf("少 2 月 29 日要被擋下並指出日期：%v", err)
	}
	if _, err := ParseOfficeCalendar(2027, csvOf(2027, func(date, line string) string {
		if date == "20270228" {
			return line + "\r\n20270229,?,0,"
		}
		return line
	})); err == nil || !strings.Contains(err.Error(), "第 61 行") {
		t.Fatalf("平年的 2 月 29 日不是日期：%v", err)
	}
}

func TestParseRejects(t *testing.T) {
	full := csvOf(2026, nil)
	tests := []struct {
		name    string
		year    int
		content string
		field   string
		want    string
	}{
		{"空內容", 2026, "", "CSVContent", "空的"},
		{"只有 BOM", 2026, "\uFEFF", "CSVContent", "空的"},
		{"表頭不符", 2026, strings.Replace(full, header, "date,weekday,flag,note", 1), "CSVContent", "第 1 行：表頭"},
		{"沒有表頭", 2026, strings.Join(strings.Split(full, "\r\n")[1:], "\r\n"), "CSVContent", "第 1 行：表頭"},
		{"旗標 1", 2026, csvOf(2026, func(d, l string) string { return strings.Replace(l, ",?,0,", ",?,1,", 1) }), "CSVContent", "第 2 行：是否放假必須是 0 或 2，實際是 \"1\""},
		{"旗標空", 2026, csvOf(2026, func(d, l string) string {
			if d == "20260105" {
				return "20260105,一,,"
			}
			return l
		}), "CSVContent", "第 6 行"},
		{"欄數不足", 2026, csvOf(2026, func(d, l string) string {
			if d == "20260103" {
				return "20260103,六,2"
			}
			return l
		}), "CSVContent", "第 4 行：應有 4 欄，實際 3 欄"},
		{"欄數過多", 2026, csvOf(2026, func(d, l string) string {
			if d == "20260103" {
				return "20260103,六,2,a,b"
			}
			return l
		}), "CSVContent", "第 4 行：應有 4 欄，實際 5 欄"},
		{"日期不是 8 位", 2026, csvOf(2026, func(d, l string) string {
			if d == "20260103" {
				return "2026/1/3,六,2,"
			}
			return l
		}), "CSVContent", "第 4 行：日期"},
		{"不存在的日期", 2026, csvOf(2026, func(d, l string) string {
			if d == "20260103" {
				return "20260230,六,2,"
			}
			return l
		}), "CSVContent", "第 4 行：日期"},
		{"年份不符", 2027, full, "CSVContent", "第 2 行：日期 2026-01-01 不在 2027 年"},
		{"夾別年的一列", 2026, csvOf(2026, func(d, l string) string {
			if d == "20260103" {
				return "20270103,日,2,"
			}
			return l
		}), "CSVContent", "第 4 行：日期 2027-01-03 不在 2026 年"},
		{"重複的一天", 2026, csvOf(2026, func(d, l string) string {
			if d == "20260103" {
				return l + "\r\n" + l
			}
			return l
		}), "CSVContent", "第 5 行：日期 2026-01-03 與第 4 行重複"},
		{"缺一天", 2026, csvOf(2026, func(d, l string) string {
			if d == "20260610" {
				return ""
			}
			return l
		}), "CSVContent", "缺 2026-06-10"},
		{"缺最後一天", 2026, csvOf(2026, func(d, l string) string {
			if d == "20261231" {
				return ""
			}
			return l
		}), "CSVContent", "應有 365 天，實際 364 天，缺 2026-12-31"},
		{"缺第一天", 2026, csvOf(2026, func(d, l string) string {
			if d == "20260101" {
				return ""
			}
			return l
		}), "CSVContent", "缺 2026-01-01"},
		{"欄數不足", 2026, lineOf("20260103,?,2"), "CSVContent", "第 4 行：應有 4 欄，實際 3 欄"},
		{"欄數過多", 2026, lineOf("20260103,?,2,,x"), "CSVContent", "第 4 行：應有 4 欄，實際 5 欄"},
		{"引號沒有結尾", 2026, lineOf(`20260103,?,2,"abc`), "CSVContent", "第 4 行：引號格式不正確"},
		{"欄位中間的引號", 2026, lineOf(`20260103,?,2,a"b`), "CSVContent", "第 4 行：引號格式不正確"},
		{"表頭前有空行", 2026, "\n\n西元日期,星期,是否放假\n", "CSVContent", "第 3 行：表頭必須是"},
		{"表頭不符且前有空行", 2026, "\n\ndate,weekday,flag,note\n", "CSVContent", "第 3 行：表頭必須是"},
		{"備註含 NUL", 2026, lineOf("20260103,?,2,a\x00b"), "CSVContent", "第 4 行：欄位不得包含 NUL 字元"},
		{"日期欄含 NUL", 2026, lineOf("2026\x00103,?,2,"), "CSVContent", "第 4 行：欄位不得包含 NUL 字元"},
		{"只有空行", 2026, "\n\n", "CSVContent", "CSV 內容是空的"},
		{"表頭欄數不對", 2026, "西元日期,星期,是否放假\n", "CSVContent", "第 1 行：表頭必須是"},
		{"年份太小", 0, full, "Year", "年份必須在"},
		{"年份太大", 10000, full, "Year", "年份必須在"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			days, err := ParseOfficeCalendar(tc.year, tc.content)
			if err == nil {
				t.Fatalf("要被拒絕，得到 %d 天", len(days))
			}
			var ae *apperr.Error
			if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput {
				t.Fatalf("要是 InvalidInput：%v", err)
			}
			if v := ae.FieldViolations(); len(v) != 1 || v[0].Field != tc.field {
				t.Errorf("欄位 = %v，要 %s", v, tc.field)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("訊息 %q 要含 %q", err.Error(), tc.want)
			}
		})
	}
}

func TestParseTrimsNoteAndKeepsInnerSpace(t *testing.T) {
	days, err := ParseOfficeCalendar(2026, csvOf(2026, func(d, l string) string {
		if d == "20260101" {
			return "20260101,四,2,　開國 紀念日 \t"
		}
		return l
	}))
	if err != nil || days[0].Note != "開國 紀念日" {
		t.Fatalf("note = %q err=%v", days[0].Note, err)
	}
}

func TestParseIgnoresWeekdayColumn(t *testing.T) {
	if _, err := ParseOfficeCalendar(2026, csvOf(2026, nil)); err != nil {
		t.Fatalf("星期欄是問號也要通過：%v", err)
	}
}

func TestParseAcceptsBOM(t *testing.T) {
	if days, err := ParseOfficeCalendar(2026, "\uFEFF"+csvOf(2026, nil)); err != nil || len(days) != 365 {
		t.Fatalf("days=%d err=%v", len(days), err)
	}
}

func lineOf(line string) string {
	return csvOf(2026, func(d, l string) string {
		if d == "20260103" {
			return line
		}
		return l
	})
}

func quotedNote(note string) func(d, l string) string {
	return func(d, l string) string {
		if d == "20260103" {
			return `20260103,?,2,` + note
		}
		return l
	}
}

func TestParseAcceptsQuotedFieldWithComma(t *testing.T) {
	days := mustParse(t, 2026, csvOf(2026, quotedNote(`"春節,補假"`)))
	if got := days[2].Note; got != "春節,補假" {
		t.Fatalf("note = %q", got)
	}
}

func TestParseAcceptsQuotedHeader(t *testing.T) {
	content := strings.Replace(csvOf(2026, nil), officeCalendarHeader, `"西元日期","星期","是否放假","備註"`, 1)
	mustParse(t, 2026, content)
}

func TestParseSkipsBlankLines(t *testing.T) {
	content := strings.Replace(csvOf(2026, nil), "20260103", "\r\n\r\n20260103", 1)
	if days := mustParse(t, 2026, content); len(days) != 365 {
		t.Fatalf("days = %d", len(days))
	}
}

// 引號欄位內含換行時，後面的錯誤行號仍是檔案的實際行號。
func TestParseReportsTheRealLineAfterAMultiLineQuotedField(t *testing.T) {
	content := csvOf(2026, func(d, l string) string {
		switch d {
		case "20260103":
			return "20260103,?,2,\"a\nb\""
		case "20260105":
			return "20260105,?,9,"
		}
		return l
	})
	_, err := ParseOfficeCalendar(2026, content)
	if err == nil || !strings.Contains(err.Error(), "第 7 行：是否放假必須是") {
		t.Fatalf("err = %v", err)
	}
}
