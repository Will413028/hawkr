package amego

import (
	"context"
	"sync/atomic"
	"time"
)

// Clock 是打 AMEGO 時 `time` 欄位的來源，帶持續偏移補償。
//
// AMEGO 的 time 窗口是 ±60 秒（含），pod 偏移超過就全線 code 15。採**持續補償**
// 而非啟動時 fail-closed：後者會讓一次上游校時抖動變成整個服務起不來，而法定
// 存證期限的時鐘還在跑。
//
// offset 正常時恆為 0 ⇒ 行為與沒有這一層完全相同。
type Clock struct {
	offsetSec atomic.Int64
	now       func() time.Time
}

// NewClock 回一個以系統時間為基準、偏移為 0 的 Clock。
func NewClock() *Clock { return &Clock{now: time.Now} }

// newClockWithSource 供測試注入時間來源。
func newClockWithSource(now func() time.Time) *Clock { return &Clock{now: now} }

// Now 回補償後的時間。
func (c *Clock) Now() time.Time {
	return c.now().Add(time.Duration(c.offsetSec.Load()) * time.Second)
}

// Unix 回補償後的 Unix 秒（送給 AMEGO 的 time 欄位一律整數秒，不可讓浮點混進來：
// 帶小數的 time 會讓伺服器轉整數後算 sign，回 code 16 而不是 15）。
func (c *Clock) Unix() int64 { return c.Now().Unix() }

// OffsetSeconds 回目前偏移，供 |offset| > 30s 持續 10 分鐘的 NTP 告警使用。
func (c *Clock) OffsetSeconds() int64 { return c.offsetSec.Load() }

// applyServerTime 依一次成功的校時結果更新偏移。
func (c *Clock) applyServerTime(serverUnix int64) {
	c.offsetSec.Store(serverUnix - c.now().Unix())
}

// Resync 打一次 GET /json/time 並更新偏移。
//
// 校時來源刻意用免簽章、免 Credential 的那支 ⇒ 校時本身不會因憑證問題失敗。
//
// 🔴 失敗時**保留舊 offset，不歸零**：舊值再壞也比在無資訊下歸零好
// （歸零等於宣稱「本機時間是對的」，而我們剛剛才失去確認它的能力）。
//
// 呼叫時機由呼叫端決定：啟動、每 15 分鐘、任何一次 code 15 之後立刻一次。
func (c *Clock) Resync(ctx context.Context, src ServerTimeSource) error {
	t, err := src.ServerTime(ctx)
	if err != nil {
		return err
	}
	c.applyServerTime(t.Unix())
	return nil
}

// ServerTimeSource 是 Resync 的依賴，讓 Clock 可在無網路的單元測試中被驗證。
type ServerTimeSource interface {
	ServerTime(ctx context.Context) (time.Time, error)
}
