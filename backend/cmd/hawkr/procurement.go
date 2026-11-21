package main

import (
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
	events.Subscribe(bus, newProcurement(d, log).OnBuyerDeleted)
}

func newProcurement(d *db.DB, log *slog.Logger) *procurement.Module {
	return procurement.New(procurement.Deps{DB: d, Now: time.Now, Logger: log})
}

// addProcurementWorkers 登記採購 run 的 reconciler 與事件清除。執行 run 的 worker（run.Dispatcher）與送出路徑要等 Executor 與
// handler 接上才登記。
func addProcurementWorkers(workers *river.Workers, m *procurement.Module) { m.AddWorkers(workers) }
