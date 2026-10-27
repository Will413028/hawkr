package worker

import (
	"context"
	"time"
)

// ---------------------------------------------------------------------------
// INV.hc —— provider 健康檢查的排程面（§7.5.3）
//
// 排程參數逐字取自 §7.5.3 的表：interval 60s、FetchBatch 50、ClaimLease 120s、
// **MaxAttempts 不適用**（終止條件是業務條件「帳戶恢復可用」，不是次數）。
//
// 這一層刻意只有一個轉接：巡檢的全部語意在 usecase.HealthCheck，而「多久 tick 一次」
// 是排程決策。與 INV.trk 不同，這支**沒有時點閘門**——它每一輪都真的要跑，
// 候選是否為空由 SQL 的 `health_check_next_at <= now` 決定。
// ---------------------------------------------------------------------------

// hcInterval 是 INV.hc 的 tick 間隔（§7.5.3）。
//
// ⚠️ 它**不是**巡檢週期：週期由 usecase 寫回的 `health_check_next_at = now + 15min`
// 決定。60s 是「多久回頭看一次有沒有帳戶到期」，兩者混淆會讓呼叫量變 15 倍。
const hcInterval = 60 * time.Second

// hcRunner 是 INV.hc 的 usecase 面（窄介面，讓 worker 測試不必架 usecase）。
type hcRunner interface {
	RunOnce(ctx context.Context) error
}

// healthCheckWorker 把 usecase 接上排程器。
type healthCheckWorker struct{ uc hcRunner }

// RunOnce 由排程器每 60 秒呼叫一次。
func (w *healthCheckWorker) RunOnce(ctx context.Context) error { return w.uc.RunOnce(ctx) }
