package main

import (
	"context"
	"errors"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

func newCatalog(d *db.DB, bus *events.Bus, med catalog.Media, buyers *buyer.Module) *catalog.Module {
	return catalog.New(catalog.Deps{DB: d, Events: bus, NewID: uuid.NewV7, Now: time.Now, Media: med, Restock: catalogRestock{buyer: buyers}})
}

type restockBuyers interface {
	FanOut(ctx context.Context, input buyer.FanOutInput) (int, bool, error)
	PurgeRestockSKUs(ctx context.Context, skuIDs []uuid.UUID) error
}

var errNoRestockProgress = errors.New("補貨通知這一批沒有進展")

type catalogRestock struct{ buyer restockBuyers }

// NotifyRestocked 依 SKU 通知並刪除訂閱：一批批呼叫 buyer 的 FanOut 直到該 SKU 沒有待通知的訂閱（一次性訂閱，重送是 no-op）。
// 同商品其他 SKU 的訂閱不受影響。
func (r catalogRestock) NotifyRestocked(ctx context.Context, sku catalog.RestockedSKU) error {
	for {
		notified, more, err := r.buyer.FanOut(ctx, buyer.FanOutInput{
			ProductID: sku.ProductID, SKUID: sku.SKUID, Limit: buyer.MaxBatchSize,
			ProductName: sku.Name, ImageKey: sku.ImageKey, OccurredAt: sku.OccurredAt,
		})
		if err != nil || !more {
			return err
		}
		if notified == 0 {
			return errNoRestockProgress
		}
	}
}

// PurgeSKUSubscriptions 刪除這些 SKU 的訂閱（商品或 SKU 刪除後）。
func (r catalogRestock) PurgeSKUSubscriptions(ctx context.Context, skuIDs []uuid.UUID) error {
	return r.buyer.PurgeRestockSKUs(ctx, skuIDs)
}

func subscribeCatalogFavorites(bus *events.Bus, module *catalog.Module) {
	events.Subscribe(bus, func(ctx context.Context, e buyer.FavoriteAdded) error {
		err := module.AddFavorite(ctx, e.ProductID)
		if errors.Is(err, catalog.ErrProductNotFound) {
			return apperr.NotFound("找不到商品")
		}
		return err
	})
	events.Subscribe(bus, func(ctx context.Context, e buyer.FavoriteRemoved) error {
		if err := module.RemoveFavorite(ctx, e.ProductID); err != nil && !errors.Is(err, catalog.ErrProductNotFound) {
			return err
		}
		return nil
	})
}
