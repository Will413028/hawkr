package main

import (
	"context"
	"errors"
	"log/slog"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/legacy/orderservice"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// newReview 組裝 review 模組並訂閱買家的刪除事件（單體規則 4 的原子狀態變更：同 transaction 記 inactive_authors、排清除 job）。serve 與 worker 都要呼叫：serve 的刪帳申請與 worker 的 CompletePending 都會發買家事件，worker 另外執行
// 它的 job（媒體引用同步、刪帳清除與其保底掃描）。review 的 relay（reviewrelay）與事件訂閱不在這裡接：舊服務在 A6-6e 前仍是評價唯一的寫入者，
// 並行期只能有一個生效的 producer。
func newReview(d *db.DB, bus *events.Bus, jc jobs.Enqueuer, buyers *buyer.Module, sellers *seller.Module, med media.Media, orders reviewOrderClient, log *slog.Logger) *review.Module {
	m := review.New(review.Deps{
		DB: d, Events: bus, Media: med, Jobs: jc, Buyers: buyers, Stores: sellers, Orders: reviewOrders{orders},
		NewID: uuid.NewV7, Now: time.Now, Logger: log,
	})
	events.Subscribe(bus, m.OnBuyerDeletionRequested)
	events.Subscribe(bus, m.OnBuyerDeleted)
	return m
}

// reviewOrders 以還在舊服務的 order-service 實作 review 的訂單 port（ordering 搬進單體後換成 in-process）：訂單不存在回 review.ErrOrderNotFound，
// 其他錯誤原樣回傳（review 轉成 Unavailable，不當成不存在）。
type reviewOrders struct{ client reviewOrderClient }

// reviewOrderClient 是 reviewOrders 用到的 orderservice.Client 的方法。
type reviewOrderClient interface {
	OrderForReview(ctx context.Context, orderNo string) (orderservice.ReviewOrder, error)
}

func (a reviewOrders) OrderForReview(ctx context.Context, orderNo string) (review.OrderForReview, error) {
	o, err := a.client.OrderForReview(ctx, orderNo)
	if errors.Is(err, orderservice.ErrOrderNotFound) {
		return review.OrderForReview{}, review.ErrOrderNotFound
	}
	if err != nil {
		return review.OrderForReview{}, err
	}
	return mapReviewOrder(o), nil
}

func mapReviewOrder(o orderservice.ReviewOrder) review.OrderForReview {
	out := review.OrderForReview{OrderNo: o.OrderNo, BuyerID: o.BuyerID, StoreID: o.StoreID, Status: o.Status, CompletedAt: o.CompletedAt, Items: make([]review.OrderItem, len(o.Items))}
	for i, it := range o.Items {
		out.Items[i] = review.OrderItem{OrderItemID: it.OrderItemID, ProductID: it.ProductID, SkuName: it.SkuName, ImageKey: it.ImageKey, Price: it.Price}
	}
	return out
}
