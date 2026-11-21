package main

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// subscribeProcurement 讓買家關閉在同一個 transaction 刪除買家的採購資料、匿名保留賣家看得到的紀錄（A9-D19 S10；單體規則 4 的原子狀態變更）。
// serve 與 worker 都要訂：BuyerDeleted 在哪個程序的 transaction 內發布，訂閱者就要在那個程序裡。
func subscribeProcurement(bus *events.Bus, d *db.DB, log *slog.Logger) {
	events.Subscribe(bus, newProcurement(d, nil, log).OnBuyerDeleted)
}

// newProcurement 組出採購模組；jobs 是送出與取消 run 的 River client（只訂閱事件的呼叫端給 nil）。
// 執行 run 的 Executor 依 run 類型登記在 Executors，現在沒有登記：領到 run 的 worker 把它標為該類型的 pipeline 失敗並退額度。
func newProcurement(d *db.DB, jobs procurement.JobQueue, log *slog.Logger) *procurement.Module {
	return procurement.New(procurement.Deps{DB: d, Now: time.Now, Logger: log, Jobs: jobs})
}

// addProcurementWorkers 登記採購 run 的執行 worker、reconciler 與事件清除。
func addProcurementWorkers(workers *river.Workers, m *procurement.Module) { m.AddWorkers(workers) }

// startProcurementHub 啟動 WatchRun 共用的 LISTEN 連線，回傳的函式停止它並等它結束。只有 serve 角色需要（串流在那裡）。
func startProcurementHub(ctx context.Context, m *procurement.Module, log *slog.Logger) (stop func()) {
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := m.RunHub(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("procurement：run 的 LISTEN 連線結束", "error", err)
		}
	}()
	return func() {
		cancel()
		<-done
	}
}
