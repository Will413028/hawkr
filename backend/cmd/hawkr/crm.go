package main

import (
	"log/slog"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/crm"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// newCRM 組裝 crm 模組並訂閱買家刪帳與店家退場的事件（同 transaction 排清除 job，見 crm.OnBuyerDeleted、OnStoreRetired）。serve 與 worker 都要呼叫：
// serve 的店家退場、worker 的 CompletePending 都會發這兩個事件，worker 另外執行它的 job 並跑訂單投影的 consumer（crmconsumer，在 runWorker 接）。
func newCRM(d *db.DB, bus *events.Bus, jc jobs.Enqueuer, buyers *buyer.Module, sellers *seller.Module, log *slog.Logger) *crm.Module {
	m := crm.New(crm.Deps{DB: d, Jobs: jc, Buyers: buyers, Stores: sellers, NewID: uuid.NewV7, Now: time.Now, Logger: log})
	events.Subscribe(bus, m.OnBuyerDeleted)
	events.Subscribe(bus, m.OnStoreRetired)
	return m
}
