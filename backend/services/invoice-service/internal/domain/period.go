// Package domain 承載 invoice-service 的實體、狀態轉移規則、金額不變式與期別換算。
//
// 本 package 是純函式層：零 I/O、零框架相依（不 import pgx／fx／grpc／net/http），
// 所有輸入輸出皆為值型別。理由見 58 號設計書 §4.2.2 的模組責任表——AMEGO 對金額不平
// 一律回萬用碼 3040174，訊息還會指向錯誤的欄位，我方無法靠 provider 的回應定位問題，
// 所有不變式只能在送出前於這一層算完並斷言。
package domain

import (
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/timeutil"
)

// InvoiceDateTaipei 把 UTC 時刻轉成發票上的曆日（§5.3.4）。
//
// 呼叫端取值一律是 COALESCE(payment.provider_paid_at, payment.pay_time)：pay_time 是我方
// server 的 nowUtc，provider_paid_at 才是 PayUni 端的真實付款時間，而後者可為 NULL
// （ATM webhook 通常無時間欄位）。
func InvoiceDateTaipei(paidAtUTC time.Time) time.Time {
	return timeutil.Date(paidAtUTC.In(timeutil.LocationTaipei))
}

// AmegoPeriod 回 AMEGO 的 Year 與 Period（0–5 的雙月制期別代碼：
// 0=01-02月 1=03-04月 2=05-06月 3=07-08月 4=09-10月 5=11-12月）。
//
// 服務內部所有時間都是 UTC，但發票期別以台灣當地月份分期。直接拿 UTC 的月份會在每兩個月
// 的 1 日 00:00–08:00 (UTC) 落到前一期，而那正好是跨期判定最敏感的那八個小時 —— 錯誤只在
// 每兩個月的邊界日出現，任何一次 dev 驗證都極難重現。
//
// Period 是期別代碼不是民國年月：送 {"Period":"11508"} 當場回 {"code":99,"msg":"Period 錯誤"}。
func AmegoPeriod(t time.Time) (year, period int) {
	lt := t.In(timeutil.LocationTaipei)
	return lt.Year(), (int(lt.Month()) - 1) / 2
}

// TaxPeriodCode 回傳財政部期別碼：民國年 3 碼 + 期別 2 碼（01=1-2月 … 06=11-12月）。
// 例：2026-08-15（台北）→ 民國 115 年、第 4 期 → "11504"。
//
// 期別有兩套編碼、差 1 且互不轉換：存庫用本函式（1–6、民國年），送 AMEGO 用 AmegoPeriod
// （0–5、西元年）。要送 AMEGO 就呼叫 AmegoPeriod，不要拿 tax_period_code[3:5] 減一——
// 兩者的 year 基準也不同，只換期別那一半會產生一個「看起來對、每次都錯一期」的值。
func TaxPeriodCode(dateTaipei time.Time) string {
	lt := dateTaipei.In(timeutil.LocationTaipei)
	rocYear := lt.Year() - 1911
	period := (int(lt.Month()) + 1) / 2 // 1–6
	return fmt.Sprintf("%03d%02d", rocYear, period)
}

// ParseTaxPeriodCode 把 "11504" 拆回西元年與期別（1–6）。
func ParseTaxPeriodCode(code string) (year, period int, err error) {
	if len(code) != 5 {
		return 0, 0, fmt.Errorf("invoice: tax period code must be 5 chars: %q", code)
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return 0, 0, fmt.Errorf("invoice: tax period code must be numeric: %q", code)
		}
	}
	roc := int(code[0]-'0')*100 + int(code[1]-'0')*10 + int(code[2]-'0')
	period = int(code[3]-'0')*10 + int(code[4]-'0')
	if period < 1 || period > 6 {
		return 0, 0, fmt.Errorf("invoice: tax period out of range 1..6: %q", code)
	}
	return roc + 1911, period, nil
}

// periodBoundsTaipei 回傳該期的第一個月與「次期首月」（＝跨期後的那個單月）。
// 期別 p（1–6）涵蓋的月份是 2p-1 與 2p；次期首月即 2p+1（p=6 時跨到隔年 1 月）。
func periodBoundsTaipei(year, period int) (firstMonth time.Month, nextOddYear int, nextOddMonth time.Month) {
	firstMonth = time.Month(2*period - 1)
	nextOddYear, nextOddMonth = year, time.Month(2*period+1)
	if period == 6 {
		nextOddYear, nextOddMonth = year+1, time.January
	}
	return firstMonth, nextOddYear, nextOddMonth
}

// PeriodEndTaipei 回傳該期末月最後一日 23:59:59（Asia/Taipei）。
func PeriodEndTaipei(taxPeriodCode string) (time.Time, error) {
	year, period, err := ParseTaxPeriodCode(taxPeriodCode)
	if err != nil {
		return time.Time{}, err
	}
	_, ny, nm := periodBoundsTaipei(year, period)
	// 次期首月 1 日往前一秒 ＝ 本期末月最後一日 23:59:59。
	return time.Date(ny, nm, 1, 0, 0, 0, 0, timeutil.LocationTaipei).Add(-time.Second), nil
}

// PeriodFilingDueTaipei 回傳法定申報期限：次期首月 15 日 23:59:59（營業稅法 §35 I）。
func PeriodFilingDueTaipei(taxPeriodCode string) (time.Time, error) {
	year, period, err := ParseTaxPeriodCode(taxPeriodCode)
	if err != nil {
		return time.Time{}, err
	}
	_, ny, nm := periodBoundsTaipei(year, period)
	return time.Date(ny, nm, 16, 0, 0, 0, 0, timeutil.LocationTaipei).Add(-time.Second), nil
}

// ProviderCloseDeadlineTaipei 回傳 AMEGO 的關帳硬牆：跨期後單月 5 日 23:59:59（Asia/Taipei）。
//
// 每單月 6 日 00:00:00 起，上期發票的開立／作廢／註銷全部被鎖，要解鎖得聯繫光貿人工開權限
// （§4.5.10）。對退款流程而言，過了這道牆的正解是開折讓單而不是作廢發票。
func ProviderCloseDeadlineTaipei(taxPeriodCode string) (time.Time, error) {
	year, period, err := ParseTaxPeriodCode(taxPeriodCode)
	if err != nil {
		return time.Time{}, err
	}
	_, ny, nm := periodBoundsTaipei(year, period)
	return time.Date(ny, nm, 6, 0, 0, 0, 0, timeutil.LocationTaipei).Add(-time.Second), nil
}

// WithinVoidWindow 回報 now 是否仍在「可作廢」的窗口內（今天 <= 跨期後單月 5 日 23:59:59）。
//
// 這只是作廢／折讓決策樹的其中一個節點；決策樹還要看 provider_upload_status 與是否已有折讓
// （§4.5.10）。單獨用本函式當放行依據會漏掉那兩格。
func WithinVoidWindow(taxPeriodCode string, now time.Time) (bool, error) {
	deadline, err := ProviderCloseDeadlineTaipei(taxPeriodCode)
	if err != nil {
		return false, err
	}
	return !now.In(timeutil.LocationTaipei).After(deadline), nil
}

// ReportedStatus 的三個值。這三個值沒有 DB 落點（I-58）：全服務沒有任何一欄叫
// reported_status，它是 (taxPeriodCode, now) 的純函式。落成欄位的後果是本服務沒有任何寫入
// 路徑會產生它 ⇒ 恆為 NULL ⇒ 決策樹的 D 節點恆走 UNKNOWN ⇒ 整條 VOID 路徑永遠走不到，
// 而且不編譯錯、不測試紅。
const (
	ReportedStatusNotReported = "NOT_REPORTED"
	ReportedStatusReported    = "REPORTED"
	ReportedStatusUnknown     = "UNKNOWN"
)

// ReportedStatus 回報「該筆銷售額是否已申報」的法律界線。
//
// 三態的方向刻意不對稱：只有 NOT_REPORTED 會讓決策樹往「作廢」走，另外兩態一律走折讓——
// 折讓在任何情況下都合法，作廢不是。只吃 tax_period_code 與 now，不吃 provider_upload_status
// （那一欄答的是傳輸，不是申報）。
func ReportedStatus(taxPeriodCode string, now time.Time) (string, error) {
	end, err := PeriodEndTaipei(taxPeriodCode)
	if err != nil {
		return "", err
	}
	due, err := PeriodFilingDueTaipei(taxPeriodCode)
	if err != nil {
		return "", err
	}
	nowTaipei := now.In(timeutil.LocationTaipei)
	switch {
	case nowTaipei.Before(end):
		return ReportedStatusNotReported, nil // 期別尚未終結 ⇒ 法律上無從申報
	case nowTaipei.After(due):
		return ReportedStatusReported, nil // 法定申報期限已過
	default:
		return ReportedStatusUnknown, nil // 期已結、限未到 ⇒ fail-closed
	}
}
