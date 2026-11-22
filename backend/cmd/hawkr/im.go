package main

import (
	"log/slog"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/legacy/imrelay"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// newIM 組裝 im 模組並訂閱買家與店家的刪除事件（單體規則 4 的原子狀態變更：同 transaction 只排抹除 job，停用的事實在 buyer、seller）。
// serve 與 worker 都要呼叫：serve 的刪帳流程與 worker 的 CompletePending、店家退場都會發這兩個事件，worker 另外執行它的 job
// （媒體引用同步、抹除與其保底掃描）。relay（imrelay）在這裡接上：單體的 im 是 chat.message.sent 唯一的 producer。
func newIM(d *db.DB, bus *events.Bus, jc jobs.Enqueuer, buyers im.Buyers, sellers im.Stores, med media.Media, orders im.Orders, log *slog.Logger) *im.Module {
	m := im.New(im.Deps{
		DB: d, Events: bus, Jobs: jc, Media: med, Buyers: buyers, Stores: sellers, Orders: orders,
		NewID: uuid.NewV7, Now: time.Now, Logger: log,
	})
	events.Subscribe(bus, m.OnBuyerDeleted)
	events.Subscribe(bus, m.OnStoreRetired)
	events.Subscribe(bus, m.OnMeasureCreated)
	events.Subscribe(bus, m.OnMeasureRevoked)
	subscribeIMRelay(bus, jc)
	return m
}

// subscribeIMRelay 把 im.MessageSent 在發布的 transaction 內排成 relay job，由 worker 組成 chat.message.sent 送給 notification-service。
// serve 與 worker 都要訂閱。
func subscribeIMRelay(bus *events.Bus, queue jobs.Enqueuer) {
	events.Subscribe(bus, imrelay.OnMessageSent(queue))
}
