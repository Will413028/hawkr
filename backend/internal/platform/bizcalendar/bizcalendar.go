// Package bizcalendar 是平台的工作日行事曆：platform 擁有 schema bizcalendar（db/schema.sql），資料是人事行政總處辦公日曆表逐日 CSV，
// 一年一份、每天一列（日期、是否放假、備註）。不依賴任何模組；消費者經 Load 取得 Calendar，之後的計算都是純函式。
//
// 涵蓋以年為單位：匯入的年份才算涵蓋（Import 以一個 transaction 寫滿全年，已匯入的年份重匯是整年取代）。沒有涵蓋的日子不是工作天也不是假日，
// 是「算不出來」（ok = false）：平台不提供週六日的預設、天數上限或偏早偏晚的取捨，那是呼叫者依自己義務決定的事。
// 一天是不是工作天只看是否放假，星期幾不參與，所以補班的週六是工作天。日界是台北（platform/twtime）。
//
// Import 把後台紀錄（platform/adminaudit）寫在同一個 transaction 內：紀錄與資料同生同滅。
//
// Import 在同一個 transaction 內發布 CalendarImported，讓需要同步行事曆的訂閱者（遷移期的 internal/legacy/bizcalendarrelay）在同一個 transaction 排入工作。
package bizcalendar

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

//go:embed db/migrations/*.sql db/migrations/atlas.sum
var migrationFiles embed.FS

// Schema 是 bizcalendar 的 schema 名稱。
const Schema = "bizcalendar"

// AuditTargetType 是 Import 寫進後台紀錄的對象種類；對象 ID 是年份。
const AuditTargetType = "holiday_calendar"

// Migrations 回傳 bizcalendar schema 的 migration，給 cmd/hawkr 登記。
func Migrations() fs.FS {
	sub, err := fs.Sub(migrationFiles, "db/migrations")
	if err != nil {
		panic(err) // 只有 embed 的路徑寫錯才會發生
	}
	return sub
}

// Store 讀寫行事曆資料。
type Store struct {
	db    *db.DB
	audit *adminaudit.Log
	bus   *events.Bus
}

// CalendarImported 是某一年的行事曆匯入後在同一個 transaction 內發布的事件。
type CalendarImported struct {
	Year int
}

// New 建立 Store。
func New(d *db.DB, audit *adminaudit.Log, bus *events.Bus) *Store {
	if d == nil || audit == nil || bus == nil {
		panic("bizcalendar: missing dependency")
	}
	return &Store{db: d, audit: audit, bus: bus}
}

// ImportResult 是一次匯入的結果。
type ImportResult struct {
	Year int
	// DayCount 與 HolidayCount 是匯入後該年的天數與放假天數。
	DayCount     int
	HolidayCount int
	// ReplacedDayCount 是被取代的該年原有天數；第一次匯入該年時為 0。
	ReplacedDayCount int
}

const (
	// 一次匯入就是整年每一天，所以逐日 upsert 就是整年取代；同時匯入同一年的兩個 transaction 由主鍵與列鎖序列化，不會留下半年資料。
	// 輸入已依 day 升序（importColumns），兩個 transaction 取列鎖的順序一致，不會死結。xmax <> 0 是被取代的既有列。
	upsertSQL = `WITH up AS (
INSERT INTO bizcalendar.days (day, holiday, note)
SELECT t.day, t.holiday, t.note FROM unnest($1::date[], $2::boolean[], $3::text[]) AS t(day, holiday, note) ORDER BY t.day
ON CONFLICT (day) DO UPDATE SET holiday = EXCLUDED.holiday, note = EXCLUDED.note
RETURNING (xmax <> 0) AS replaced)
SELECT count(*) FILTER (WHERE replaced) FROM up`
)

// importColumns 把 days 依日期升序攤成 upsert 的三個欄位陣列（同層多列依 ID 升序的規則）；不改動 days。
func importColumns(days []Day) (dates []time.Time, holidays []bool, notes []string) {
	sorted := slices.SortedFunc(slices.Values(days), func(a, b Day) int { return a.Date.Compare(b.Date) })
	dates = make([]time.Time, len(sorted))
	holidays = make([]bool, len(sorted))
	notes = make([]string, len(sorted))
	for i, d := range sorted {
		dates[i], holidays[i], notes[i] = d.Date, d.Holiday, d.Note
	}
	return dates, holidays, notes
}

// Import 解析 content（ParseOfficeCalendar 的格式）並整年取代 year 的資料，同一個 transaction 寫一筆後台紀錄。解析失敗時不寫入任何東西。
func (s *Store) Import(ctx context.Context, actor adminaudit.Actor, year int, content string) (ImportResult, error) {
	days, err := ParseOfficeCalendar(year, content)
	if err != nil {
		return ImportResult{}, err
	}
	dates, holidays, notes := importColumns(days)
	res := ImportResult{Year: year, DayCount: len(days)}
	for _, d := range days {
		if d.Holiday {
			res.HolidayCount++
		}
	}
	err = s.db.InTx(ctx, func(ctx context.Context) error {
		conn := s.db.Conn(ctx)
		if err := conn.QueryRow(ctx, upsertSQL, dates, holidays, notes).Scan(&res.ReplacedDayCount); err != nil {
			return fmt.Errorf("bizcalendar: import year: %w", err)
		}
		if err := s.audit.Record(ctx, adminaudit.Entry{
			Actor: actor, TargetType: AuditTargetType, TargetID: fmt.Sprint(year),
			Detail: map[string]any{"days": res.DayCount, "holidays": res.HolidayCount, "replaced_days": res.ReplacedDayCount},
		}); err != nil {
			return err
		}
		return events.Publish(ctx, s.bus, CalendarImported{Year: year})
	})
	if err != nil {
		return ImportResult{}, err
	}
	return res, nil
}

const loadSQL = `SELECT day, holiday, note FROM bizcalendar.days ORDER BY day`

// Load 讀出目前全部已匯入的資料。一個語句讀完，所以結果是一致的快照（不會讀到匯入到一半的一年）；資料量是年數乘 365，不分頁。
func (s *Store) Load(ctx context.Context) (*Calendar, error) {
	rows, err := s.db.Conn(ctx).Query(ctx, loadSQL)
	if err != nil {
		return nil, fmt.Errorf("bizcalendar: load: %w", err)
	}
	days, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Day, error) {
		var d Day
		err := r.Scan(&d.Date, &d.Holiday, &d.Note)
		return d, err
	})
	if err != nil {
		return nil, fmt.Errorf("bizcalendar: load: %w", err)
	}
	return NewCalendar(days)
}
