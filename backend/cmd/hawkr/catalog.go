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

// newCatalog 組出 catalog 模組給消費端（procurement、discovery、content、buyer 與 seller 的商品 port）用。它的事件訂閱、週期 job 與補貨 worker 不在這裡登記：
// 舊服務處理商品流量到切換（B4-6）。補貨通知的下游是 buyer（catalogRestock）；收藏數的訂閱見 subscribeCatalogFavorites。
func newCatalog(d *db.DB, med catalog.Media, buyers *buyer.Module) *catalog.Module {
	return catalog.New(catalog.Deps{DB: d, NewID: uuid.NewV7, Now: time.Now, Media: med, Restock: catalogRestock{buyer: buyers}})
}

// restockBuyers 是補貨通知用到的 buyer 方法（*buyer.Module 滿足它）。
type restockBuyers interface {
	FanOut(ctx context.Context, input buyer.FanOutInput) (int, bool, error)
	PurgeRestockSKUs(ctx context.Context, skuIDs []uuid.UUID) error
}

// errNoRestockProgress 表示補貨分批還有待通知的訂閱，這一批卻一筆也沒處理到。
var errNoRestockProgress = errors.New("補貨通知這一批沒有進展")

// catalogRestock 以 buyer 模組實作 catalog 的補貨通知 port（catalog 在 buyer 之上，buyer 不能 import catalog，接線在這裡）。
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
			// 剩下的訂閱被別的 transaction 鎖著（SKIP LOCKED）而這一批一筆也沒處理：回錯誤讓 catalog worker 退避重試，不空轉。
			return errNoRestockProgress
		}
	}
}

// PurgeSKUSubscriptions 刪除這些 SKU 的訂閱（商品或 SKU 刪除後）。
func (r catalogRestock) PurgeSKUSubscriptions(ctx context.Context, skuIDs []uuid.UUID) error {
	return r.buyer.PurgeRestockSKUs(ctx, skuIDs)
}

// subscribeCatalogFavorites 讓 catalog 的收藏數與收藏列在同一個 transaction 內進退（D3）：buyer 在 InTx 內發布
// FavoriteAdded／FavoriteRemoved，同步執行這裡的訂閱者，訂閱者失敗收藏就回滾。serve 與 worker 都要訂閱：刪帳的 job 在 worker 清收藏。
func subscribeCatalogFavorites(bus *events.Bus, module *catalog.Module) {
	events.Subscribe(bus, func(ctx context.Context, e buyer.FavoriteAdded) error {
		err := module.AddFavorite(ctx, e.ProductID)
		if errors.Is(err, catalog.ErrProductNotFound) {
			return apperr.NotFound("找不到商品")
		}
		return err
	})
	events.Subscribe(bus, func(ctx context.Context, e buyer.FavoriteRemoved) error {
		// 商品不存在時沒有收藏數可減：不讓它擋住取消收藏與刪帳。
		if err := module.RemoveFavorite(ctx, e.ProductID); err != nil && !errors.Is(err, catalog.ErrProductNotFound) {
			return err
		}
		return nil
	})
}
