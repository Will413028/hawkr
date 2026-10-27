package domain

import (
	"time"

	"github.com/Will413028/hawkr/go-pkg/timeutil"
)

// 申訴 SLA 的工作天曆（§7.3.5「3 個工作天」的定義 ＋ §5.1.14 的唯讀鏡像）。
//
// 🔴 與 StatutoryDeadline 是**兩套曆制、兩個函式**（§9 X 表逐字）：法定存證期限走自然日、
// 末日不因週六日或國定假日延長；申訴 SLA 扣週六日與國定假日。共用一個函式的那一刻，
// 其中一邊就錯了——而錯的那一邊不會有任何東西轉紅，只會讓賣家在連假期間被誤判逾期，
// 逾期計進 C1 的 90 天違規窗口，3 次即停權建議。
//
// 本檔全部是純函式：假日集合由呼叫端從 invoice_business_day_mirror 取出後傳入
// （§7.3.5 接法③逐字「不碰 DB、不碰網路」）。
//
// ⚠️ 設計書把算式指名放在 `pkg/bizcalendar`。本波沒有第二個消費者（全 repo 只有
// invoice-service 需要工作天曆），依重複實作防護的反面——「跨服務共用不等於把 domain
// logic 塞進無邊界的 common package」——先留在本層。要抽上去時整個檔案照搬即可：
// 它對 invoice-service 零相依。缺口登記在交付回報。

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

	// 鏡像的同步窗口（§5.1.14：INV.gov 每日同步「今天 −30 天 ～ +400 天」）。
	//
	// 它是「這一天有沒有被涵蓋」的唯一依據：鏡像**只存假日**，兩個假日之間的空白
	// 既可能是工作天、也可能是「還沒同步到那一段」。少了涵蓋區間就無法分辨這兩者，
	// 而把後者當成工作天正是上面那條紅字要防的偏早。
	businessDayMirrorPastDays   = 30
	businessDayMirrorFutureDays = 400
)

// BusinessDayMirrorWindow 回同步窗口的台北日閉區間（§5.1.14：今天 −30 ～ +400 天）。
//
// 🔴 它與 NewBusinessCalendar 的 coveredFrom／coveredTo **必須用同一組常數**：
// 同步器寫進去的區間比行事曆宣稱涵蓋的窄一天，那一天就會被當成假日（IsBusinessDay
// 對未涵蓋日回 false），而沒有任何東西會轉紅。共用常數是唯一擋得住這件事的機制。
//
// ⚠️ 測試不得拿本函式的回傳值當期望值——那是自己跟自己對，改壞常數照樣全綠
// （2026-09-04 實測：把 +400 改成 +399，測試 exit=0）。期望值要硬寫日期。
func BusinessDayMirrorWindow(now time.Time) (from, to time.Time) {
	base := taipeiMidnight(now)
	return base.AddDate(0, 0, -businessDayMirrorPastDays), base.AddDate(0, 0, businessDayMirrorFutureDays)
}

// BusinessCalendar 是一份台北時區的工作天曆快照。
//
// 零值是**零涵蓋**的行事曆（每一天都不是工作天），不是「每一天都是工作天」——
// 呼叫端忘了載入鏡像時，得到的是偏晚的硬上限而不是偏早的自然日。
type BusinessCalendar struct {
	holidays map[string]struct{}
	// coveredFrom／coveredTo 是台北日界的閉區間；covered=false 代表鏡像從未同步過。
	coveredFrom time.Time
	coveredTo   time.Time
	covered     bool
}

// NewBusinessCalendar 由鏡像的假日清單與 max(synced_at) 組出行事曆。
//
// syncedAt 為 nil（鏡像空表 ⇒ GetBusinessDayMirrorSyncedAt 回 ErrNoRows）代表零涵蓋。
func NewBusinessCalendar(holidays []time.Time, syncedAt *time.Time) BusinessCalendar {
	c := BusinessCalendar{holidays: make(map[string]struct{}, len(holidays))}
	for _, h := range holidays {
		c.holidays[taipeiDayKey(h)] = struct{}{}
	}
	if syncedAt == nil {
		return c
	}
	base := taipeiMidnight(*syncedAt)
	c.coveredFrom = base.AddDate(0, 0, -businessDayMirrorPastDays)
	c.coveredTo = base.AddDate(0, 0, businessDayMirrorFutureDays)
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
			// 這一天是起算日本身（次一個工作日），不計入 3 天。
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

// taipeiMidnight 把任一時刻收斂成它所屬台北日的 00:00。
//
// 🔴 日界是台北不是 UTC：直接用 UTC 日界會讓每天 00:00–08:00（UTC+8）建立的申訴
// 落到前一天，而錯誤只在每天的前八小時出現（同一個坑見 ParseInvoiceDateRange）。
func taipeiMidnight(t time.Time) time.Time {
	d := t.In(timeutil.LocationTaipei)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, timeutil.LocationTaipei)
}

func taipeiDayKey(t time.Time) string {
	return t.In(timeutil.LocationTaipei).Format("2006-01-02")
}
