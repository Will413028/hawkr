package main

import (
	"context"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// newCatalog 組出 catalog 模組給唯讀消費端（procurement、discovery、content）用。它的訂閱、週期 job 與補貨 worker 不在這裡登記：
// 舊服務處理商品流量到切換，補貨通知接 buyer 前由 noRestock 替身接住。
func newCatalog(d *db.DB, med catalog.Media) *catalog.Module {
	return catalog.New(catalog.Deps{DB: d, NewID: uuid.NewV7, Now: time.Now, Media: med, Restock: noRestock{}})
}

// noRestock 是補貨通知下游接線前的替身：不通知也不清訂閱（worker 沒登記，不會被呼叫）。
type noRestock struct{}

func (noRestock) NotifyRestocked(context.Context, catalog.RestockedSKU) error { return nil }

func (noRestock) PurgeSKUSubscriptions(context.Context, []uuid.UUID) error { return nil }
