package main

import (
	"log/slog"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// newIM 組裝 im 模組並訂閱買家與店家的刪除事件（單體規則 4 的原子狀態變更：同 transaction 只排抹除 job，停用的事實在 buyer、seller）。
// serve 與 worker 都要呼叫：serve 的刪帳流程與 worker 的 CompletePending、店家退場都會發這兩個事件，worker 另外執行它的 job
// （抹除與其保底掃描）。
func newIM(d *db.DB, bus *events.Bus, jc jobs.Enqueuer, buyers im.Buyers, sellers im.Stores, med mediaPort, orders im.Orders, log *slog.Logger) *im.Module {
	m := im.New(im.Deps{
		DB: d, Events: bus, Jobs: jc, Media: med, Buyers: buyers, Stores: sellers, Orders: orders,
		NewID: uuid.NewV7, Now: time.Now, Logger: log,
	})
	events.Subscribe(bus, m.OnBuyerDeleted)
	events.Subscribe(bus, m.OnStoreRetired)
	events.Subscribe(bus, m.OnMeasureCreated)
	events.Subscribe(bus, m.OnMeasureRevoked)
	return m
}
