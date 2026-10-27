package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"go.uber.org/zap"
)

// notifyRunWakeTimeout 是 notifyRunWake 這個 fire-and-forget NOTIFY 的最長
// 等待時間（等 pool 給連線、送出 NOTIFY）。這是本地／同區網路對 Postgres
// 的一次 best-effort 呼叫，正常情況遠低於這個值；設一個上限只是為了在
// pool 沒有可用連線時（呼叫端最需要優雅降級的時刻）避免無限期卡住這條
// 同步呼叫的 caller（AppendRunEvent／TerminalizeRun／CancelRun commit 之後
// 的 hot path）——見本函式文件承諾的「不阻塞呼叫端」。
const notifyRunWakeTimeout = 2 * time.Second

// notifyRunWake 在 event 已經 commit 之後呼叫（doc57 事件不變式 3：live
// broadcaster 只在 DB commit 之後送 wake hint）。用一個獨立、短命的連線發
// NOTIFY，不佔用交易連線、不阻塞呼叫端——失敗只記 log，不回錯誤：live wake
// 只是「加速」，WatchRun 訂閱端本來就會定期重查 DB（見 Step 2），漏一次
// NOTIFY 不會漏事件，只會讓那次 replay 慢一點被下一次 poll 或下一個 NOTIFY
// 補上。用 context.WithTimeout（而非 context.Background()）界定上限：逾時
// 一樣走下面的 log-and-continue 路徑，不回錯誤、不改變呼叫端行為。
func (r *Repo) notifyRunWake(runID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), notifyRunWakeTimeout)
	defer cancel()
	channel := runWakeChannel(runID)
	if _, err := r.pool.Exec(ctx, fmt.Sprintf("NOTIFY %s", channel)); err != nil {
		pkglogger.Error(ctx, "procurement: NOTIFY run wake failed, live subscriber falls back to poll",
			zap.Error(err), zap.Int64("run_id", runID))
	}
}

// runWakeChannel 把 run_id 編碼成合法的 Postgres NOTIFY channel 名稱
// （只能是 identifier，不能直接用數字開頭）。
func runWakeChannel(runID int64) string {
	return fmt.Sprintf("procurement_agent_run_%d", runID)
}

// WatchRun 落地 doc57 §2.5 replay 演算法第 4 點：「先註冊 live wake channel
// 再重查一次」。回傳的 channel 每次收到值代表「這個 run 可能有新事件，去
// DB 查」——channel 本身不帶事件內容，DB 永遠是 SSOT（同一小節第 5 點）。
//
// 內部用一個獨立於 pool 一般查詢的專屬連線做 LISTEN（LISTEN 是連線層級的
// session 狀態，不能用一般的 pool.Query 短連線）。呼叫端必須呼叫回傳的
// cancel() 釋放這個連線，否則會洩漏 pool 裡的一個連線。
//
// 逾時／連線斷線只關閉這條訂閱（回傳的 channel 會被 close），呼叫端
// （Task 11 的 gRPC handler）收到 channel close 後改回退到定期 poll，不是
// 直接把整個 gRPC stream 判定失敗——理由同 doc57 §2.5-7「slow consumer 或
// channel overflow 只中斷該 stream，不阻塞 worker」的精神，這裡對稱地
// 保護 watch 端不因為 LISTEN 連線的暫時性問題就整條 stream 掛掉。
//
// ⚠️ **執行順序不可調換**：`listenAll`（真正的 `LISTEN`）必須在啟動下面那個
// 讀 `WaitForNotification` 的 goroutine 之前完成，不能反過來。Task 5
// 診斷測試時發現的競態：`*pgx.Conn` 不是併發安全的——如果 goroutine 的
// read-loop 跟 `listenAll` 的 `Exec` 同時對同一條連線操作，pgx 會把連線判定
// 為壞掉並關閉（"conn closed"）。listenAll 在 Task 5 是 no-op，這個競態當時
// 不會真的觸發；Task 11 把 listenAll 接上真正的 `LISTEN` 之後就會，所以這裡
// 把原本「先起 goroutine 再 listenAll」的順序倒過來：先同步呼叫 listenAll
// 拿到「LISTEN 已經生效」的確認，再啟動 goroutine 開始讀通知。
func (r *Repo) WatchRun(ctx context.Context, runID int64) (chan int64, func(), error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return nil, nil, err
	}

	watchCtx, cancel := context.WithCancel(ctx)

	if err := listenAll(watchCtx, conn, runID); err != nil {
		cancel()
		conn.Release()
		return nil, nil, err
	}

	wake := make(chan int64, 16)

	go func() {
		defer conn.Release()
		defer close(wake)
		for {
			notification, err := conn.Conn().WaitForNotification(watchCtx)
			if err != nil {
				if watchCtx.Err() != nil {
					return // 正常關閉（呼叫端呼叫了 cancel()）
				}
				pkglogger.Error(context.Background(), "procurement: WaitForNotification failed, closing watch",
					zap.Error(err))
				return
			}
			var notifiedRunID int64
			if _, scanErr := fmt.Sscanf(notification.Channel, "procurement_agent_run_%d", &notifiedRunID); scanErr != nil {
				continue
			}
			select {
			case wake <- notifiedRunID:
			default:
				// buffer 滿了——訂閱端本來就會定期重查最新狀態（見 Task 11
				// gRPC handler 的 sendEventsAfter），丟掉這次 wake 不會漏
				// 事件，只會延遲到下一次 wake 或下一次重查。
			}
		}
	}()

	return wake, cancel, nil
}

// listenAll 對這個連線執行 LISTEN，訂閱指定 run 的 wake channel。v1 用單一
// wildcard 不可行（Postgres LISTEN 不支援 pattern matching），因此每個 run
// 有自己的 channel（runWakeChannel），呼叫端（Task 11 的 gRPC handler）在
// 需要訂閱特定 run 時才呼叫這裡。
//
// 呼叫時機見 WatchRun 上方的順序警告：必須在啟動讀通知的 goroutine之前，
// 在同一個 connection 上同步執行完成。
func listenAll(ctx context.Context, conn *pgxpool.Conn, runID int64) error {
	_, err := conn.Exec(ctx, "LISTEN "+runWakeChannel(runID))
	return err
}
