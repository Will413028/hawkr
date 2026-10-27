package worker

import (
	"context"
	"time"

	"github.com/Will413028/hawkr/go-pkg/timeutil"
)

// ---------------------------------------------------------------------------
// INV.trk —— 字軌餘量巡檢的排程面（§7.5.3）
//
// 排程規格逐字：「每日 `INVOICE_TRACK_BALANCE_CHECK_HOUR_UTC`（dev/prod 皆 1）；
// **單月 1–5 日改為每 4 小時** —— 期初加頻**刻意不設第二個 env**（雙月制的期初日
// 由財政部定、不隨環境變，設成可調參數只會多一個沒有人會去對的漂移點）」。
//
// 為什麼不是 pkg/outbox.NewDailyScheduler：那支只能固定每日一次，表達不了期初加頻。
// 改成 hourly tick ＋ 純函式閘門 `isTrackScanSlot`，好處是那個閘門可以單獨測
// （時間是參數不是 time.Now），而排程器本身仍是既有的 outbox.Scheduler
// ⇒ panic recovery 照樣由 scheduler.go 的 tick 給（每 tick recover）。
// ---------------------------------------------------------------------------

// trackTick 是 INV.trk 的輪詢間隔。**不是**掃描頻率——真正的頻率由
// isTrackScanSlot 決定，這裡只是「多久回頭看一次現在是不是掃描時點」。
const trackTick = time.Hour

// periodStartFastHours 是期初加頻的間隔（小時）。
const periodStartFastHours = 4

// isTrackScanSlot 回答「這個時刻該不該跑一輪字軌巡檢」。
//
// 兩條規則：
//   - 平時：UTC 小時 == baseHourUTC，每日一次
//   - 台北時間的**單月 1–5 日**（雙月制的期初，字軌不足的高峰）：每 4 小時一次，
//     相位對齊 baseHourUTC（例如 base=1 ⇒ UTC 01/05/09/13/17/21）
//
// 🔴 期初判定用**台北**日期不是 UTC：期別是 UTC+8 的概念，用 UTC 會讓每個月
// 1 日的 00:00–08:00 (UTC) 落在前一個月，而那正好是期初窗口的頭八小時
// ——加頻會整段不生效，且錯誤只在每兩個月出現一次（同 §4.5.10 對期別換算的警語）。
func isTrackScanSlot(now time.Time, baseHourUTC int) bool {
	h := now.UTC().Hour()
	lt := now.In(timeutil.LocationTaipei)
	periodStart := int(lt.Month())%2 == 1 && lt.Day() >= 1 && lt.Day() <= 5
	if periodStart {
		return ((h-baseHourUTC)%periodStartFastHours+periodStartFastHours)%periodStartFastHours == 0
	}
	return h == baseHourUTC
}

// trackRunner 是 INV.trk 的 usecase 面（窄介面，讓 worker 測試不必架 usecase）。
type trackRunner interface {
	RunOnce(ctx context.Context) error
}

// trackWorker 把閘門與 usecase 接起來。
type trackWorker struct {
	uc          trackRunner
	baseHourUTC int
	now         func() time.Time
}

// RunOnce 由排程器每小時呼叫；不是掃描時點就直接返回。
func (w *trackWorker) RunOnce(ctx context.Context) error {
	if !isTrackScanSlot(w.now(), w.baseHourUTC) {
		return nil
	}
	return w.uc.RunOnce(ctx)
}

// trackObserver 把 usecase 算出來的四個數寫進 gauge。
//
// 在 worker 這一側而不是 usecase：usecase 不 import promauto（否則每個 usecase
// 測試都會在全域 registry 註冊一次），而且這樣測試能直接斷言算出來的數字。
type trackObserver struct{}

func (trackObserver) ObserveTrackScan(remainingMin, lowSellers, exhaustedSellers, nextPeriodInsufficient int64) {
	trackRemainingMin.Set(float64(remainingMin))
	trackLowSellers.Set(float64(lowSellers))
	trackExhaustedSellers.Set(float64(exhaustedSellers))
	trackNextPeriodInsufficientSellers.Set(float64(nextPeriodInsufficient))
}
