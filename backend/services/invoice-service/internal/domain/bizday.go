package domain

import (
	"time"

	"github.com/Will413028/hawkr/go-pkg/timeutil"
)


const (
	// ComplaintSlaBusinessDays 是賣家回覆申訴的 SLA（§0.1 關鍵參數：3 工作天）。
	ComplaintSlaBusinessDays = 3

	// ComplaintSlaHardCapDays 是行事曆耗盡時的硬上限（§5.1.14 表格／§7.3.5 逐字
	// 「以『起算日 + 14 自然日』為硬上限」）。
	//
	// 🔴 沒有它，「未涵蓋的日期一律視為假日」會讓推進迴圈永遠找不到第 3 個工作天。
	// 方向必須偏晚：算偏早 ⇒ 賣家被誤判逾期 ⇒ 違規計數 +1。人事行政總處通常前一年
	// 8–9 月才公布次年行事曆，缺口是常態不是例外。
	ComplaintSlaHardCapDays = 14
)

// BusinessCalendar 是一份台北時區的工作天曆快照。
//
// 零值是**零涵蓋**的行事曆（每一天都不是工作天），不是「每一天都是工作天」——
// 呼叫端忘了載入鏡像時，得到的是偏晚的硬上限而不是偏早的自然日。
type BusinessCalendar struct {
	holidays map[string]struct{}
	coveredFrom time.Time
	coveredTo   time.Time
	covered     bool
}

// BusinessDayCoverage 是單體推送時明確帶入的涵蓋區間（台北日的閉區間）與推送時間。
type BusinessDayCoverage struct {
	From     time.Time
	To       time.Time
	SyncedAt time.Time
}

// NewBusinessCalendar 由鏡像的假日清單與儲存的涵蓋區間組出行事曆。
//
// coverage 為 nil（鏡像從未被推送過）代表零涵蓋。涵蓋區間是推送端依它真正持有資料的年份決定的，
// 不由推送時間加上固定窗口推得：後者會對沒有資料的年份宣稱已涵蓋而沒有假日，方向偏早。
func NewBusinessCalendar(holidays []time.Time, coverage *BusinessDayCoverage) BusinessCalendar {
	c := BusinessCalendar{holidays: make(map[string]struct{}, len(holidays))}
	for _, h := range holidays {
		c.holidays[taipeiDayKey(h)] = struct{}{}
	}
	if coverage == nil {
		return c
	}
	c.coveredFrom = taipeiMidnight(coverage.From)
	c.coveredTo = taipeiMidnight(coverage.To)
	c.covered = true
	return c
}

// Covers 回報整個 [from, to] 台北日區間都在鏡像的同步窗口內。
//
// 呼叫端用它決定「算得出來」與「算不出來」：算不出來時 remainingBusinessDays 回空
// （§6.1.6 第 1 條：UI 顯示「處理期限計算中」），**不得退化成日曆天**。
func (c BusinessCalendar) Covers(from, to time.Time) bool {
	if !c.covered {
		return false
	}
	f := taipeiMidnight(from)
	t := taipeiMidnight(to)
	return !f.Before(c.coveredFrom) && !t.After(c.coveredTo)
}

// IsBusinessDay 回報某一天是不是工作天（台北日界）。
//
// 未涵蓋的日期一律回 false（視為假日）——這是 §5.1.14 指定的 fallback 方向。
func (c BusinessCalendar) IsBusinessDay(day time.Time) bool {
	d := taipeiMidnight(day)
	if !c.covered || d.Before(c.coveredFrom) || d.After(c.coveredTo) {
		return false
	}
	switch d.Weekday() {
	case time.Saturday, time.Sunday:
		return false
	}
	_, isHoliday := c.holidays[taipeiDayKey(d)]
	return !isHoliday
}

// ComplaintSlaDueAt 由申訴建立時刻算出 sla_due_at（絕對 UTC 時刻）。
//
// 算式（§7.3.5 表格逐字）：
//   - 起算 ＝ createdAt 的**次一個工作日 00:00（Asia/Taipei）**
//   - 期限 ＝ 自起算日再推 ComplaintSlaBusinessDays 個工作天的 00:00
//   - 硬上限 ＝ createdAt 當日 00:00 + ComplaintSlaHardCapDays 個**自然日**
//
// 回傳恆有值：invoice_complaints.sla_due_at 是 NOT NULL 且必須在建案當下落庫，
// 沒有「算不出來就不寫」這個選項（那會讓買家在鏡像過期時完全送不出申訴）。
// 行事曆涵蓋不到時落在硬上限，方向偏晚。
func (c BusinessCalendar) ComplaintSlaDueAt(createdAt time.Time) time.Time {
	base := taipeiMidnight(createdAt)
	capAt := base.AddDate(0, 0, ComplaintSlaHardCapDays)

	cursor := base
	started := false
	counted := 0
	for {
		cursor = cursor.AddDate(0, 0, 1)
		if cursor.After(capAt) {
			return capAt.UTC()
		}
		if !c.IsBusinessDay(cursor) {
			continue
		}
		if !started {
			started = true
			continue
		}
		counted++
		if counted == ComplaintSlaBusinessDays {
			return cursor.UTC()
		}
	}
}

// RemainingBusinessDays 回報距離 dueAt 還剩幾個工作天，可為負（已逾期）。
//
// ok=false 代表這段區間落在鏡像的涵蓋範圍外 ⇒ 呼叫端回空字串而不是 0：
// 0 會被前端讀成「今天到期」，而那是一個憑空造出來的事實（§6.1.6 第 1 條）。
//
// 口徑：以台北日界計，剩餘 ＝ (今天, 到期日] 之間的工作天數；已逾期 ＝ 該值為負，
// 絕對值 ＝ (到期日, 今天] 之間的工作天數。到期日當天為 0。
func (c BusinessCalendar) RemainingBusinessDays(now, dueAt time.Time) (int32, bool) {
	today := taipeiMidnight(now)
	due := taipeiMidnight(dueAt)
	from, to := today, due
	if due.Before(today) {
		from, to = due, today
	}
	if !c.Covers(from, to) {
		return 0, false
	}
	n := int32(0)
	for cursor := from.AddDate(0, 0, 1); !cursor.After(to); cursor = cursor.AddDate(0, 0, 1) {
		if c.IsBusinessDay(cursor) {
			n++
		}
	}
	if due.Before(today) {
		return -n, true
	}
	return n, true
}

func taipeiMidnight(t time.Time) time.Time {
	d := t.In(timeutil.LocationTaipei)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, timeutil.LocationTaipei)
}

func taipeiDayKey(t time.Time) string {
	return t.In(timeutil.LocationTaipei).Format("2006-01-02")
}
