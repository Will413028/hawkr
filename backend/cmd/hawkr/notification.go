package main

import (
	"log/slog"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/accountsuspension"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/notification"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// newNotification 組裝 notification 模組；serve 與 worker 都要建：serve 訂閱來源事件並排意圖 job，worker 執行意圖 job。
func newNotification(d *db.DB, id *identity.Module, b *buyer.Module, sl *seller.Module, jc jobs.Enqueuer, log *slog.Logger) *notification.Module {
	return notification.New(notification.Deps{DB: d, Identity: id, Buyer: b, Seller: sl, Jobs: jc, Now: time.Now, NewID: uuid.NewV7, Logger: log})
}

// subscribeNotification 讓來源事件在發布的 transaction 內排成通知意圖 job（D10），刪除與退場事件排成個資清除 job（D16）；訂閱者只排 job，不寫 notification 的表。
func subscribeNotification(bus *events.Bus, m *notification.Module) {
	events.Subscribe[accountsuspension.Suspended](bus, m.OnSuspended)
	events.Subscribe[accountsuspension.Unsuspended](bus, m.OnUnsuspended)
	events.Subscribe[identity.AccountDeleted](bus, m.OnAccountDeleted)
	events.Subscribe[buyer.BuyerDeleted](bus, m.OnBuyerDeleted)
	events.Subscribe[seller.StoreRetired](bus, m.OnStoreRetired)
}
