package worker

import (
	"context"
	"time"
)

// ---------------------------------------------------------------------------
// INV.rc —— 結果未明對帳 ＋ 上傳狀態覆查的排程面（§7.5.3）
//
// interval 60s、FetchBatch 50、兩組租約 60s／120s、**MaxAttempts 不適用**
// （自寫 claim 迴圈、無 MarkFailed；終止條件是業務條件不是次數）。
//
// 🔴 這支**不走 pkg/outbox.Worker**（§4.3.3 line 825 的判準：週期性重查同一列、
// 次數無上界）。但它仍由 pkg/outbox.Scheduler 驅動 ⇒ panic recovery 照樣有
// （出自 scheduler.go 的逐 tick recover，不是 Worker 給的）。
// ---------------------------------------------------------------------------

// rcInterval 是 INV.rc 的 tick 間隔（§7.5.3）。
//
// ⚠️ 它不是重查頻率：頻率由 usecase 寫回的 next_attempt_at 決定，而那個值走
// 退避階梯（30s→1m→2m→5m→10m→30m，位置由 unknown_since 推導）。
const rcInterval = 60 * time.Second

// rcRunner 是 INV.rc 的 usecase 面。
type rcRunner interface {
	RunOnce(ctx context.Context) error
}

// recheckWorker 把 usecase 接上排程器。
type recheckWorker struct{ uc rcRunner }

// RunOnce 由排程器每 60 秒呼叫一次。
func (w *recheckWorker) RunOnce(ctx context.Context) error { return w.uc.RunOnce(ctx) }
