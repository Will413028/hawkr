package main

import (
	"context"
	"log/slog"
	"time"

	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	"github.com/Will413028/hawkr/backend/internal/legacy/orderservice"
	"github.com/Will413028/hawkr/backend/internal/legacy/productservice"
	"github.com/Will413028/hawkr/backend/internal/legacy/reviewservice"
	"github.com/Will413028/hawkr/backend/internal/modules/discovery"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// newDiscovery 組裝 discovery 模組。worker 用它重算分數（AddWorkers、PeriodicJobs），serve 用它讀排行（對外的
// DiscoveryPublicService）；只有重算會呼叫三個上游，讀取只用 seller。三個上游還在舊服務，由 internal/legacy 的 client 實作 port：order 的各店統計、
// product 的「有公開商品的店家」、review 的批次評價統計（ordering、catalog、review 搬進單體後改呼叫它們的公開 API）；店家的
// 曝光資格是 seller 模組（*seller.Module 直接滿足 discovery.Stores）。
func newDiscovery(d *db.DB, orders *orderservice.Client, products *productservice.Client, reviews *reviewservice.Client, sellers *seller.Module, log *slog.Logger) *discovery.Module {
	return discovery.New(discovery.Deps{
		DB: d, Orders: discoveryOrders{orders}, Products: products, Reviews: discoveryReviews{reviews}, Stores: sellers,
		Now: time.Now, Logger: log,
	})
}

// discoveryOrders 以舊的 order-service 實作 discovery 的訂單統計 port（成立訂單的口徑由 order-service 定義，
// 見 orderservice.StoreOrderStats）。
type discoveryOrders struct{ client *orderservice.Client }

func (o discoveryOrders) StoreOrderStats(ctx context.Context, days int) ([]discovery.OrderStats, error) {
	stats, err := o.client.StoreOrderStats(ctx, days)
	if err != nil {
		return nil, err
	}
	out := make([]discovery.OrderStats, len(stats))
	for i, s := range stats {
		out[i] = discovery.OrderStats(s)
	}
	return out, nil
}

// ratingStats 是舊 review-service 的批次評價統計（沒有評價的店不在回傳裡，讀取失敗回錯誤），轉成消費端自己的型別：discovery
// 與店面頁各自擁有 port 的形狀（兩個型別欄位相同），轉換只在這裡寫一次。
func ratingStats[T ~struct {
	Count   int64
	Average float64
}](ctx context.Context, client *reviewservice.Client, storeIDs []int64) (map[int64]T, error) {
	stats, err := client.StoreRatingStats(ctx, storeIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]T, len(stats))
	for id, s := range stats {
		out[id] = T(s)
	}
	return out, nil
}

// discoveryReviews 以舊的 review-service 實作 discovery 的批次評價統計 port。
type discoveryReviews struct{ client *reviewservice.Client }

func (r discoveryReviews) StoreRatingStats(ctx context.Context, storeIDs []int64) (map[int64]discovery.RatingStats, error) {
	return ratingStats[discovery.RatingStats](ctx, r.client, storeIDs)
}

// sellerRatings 以舊的 review-service 實作店面頁的評價統計 port（httpseller.Ratings）。
type sellerRatings struct{ client *reviewservice.Client }

func (r sellerRatings) StoreRatingStats(ctx context.Context, storeIDs []int64) (map[int64]httpseller.RatingStats, error) {
	return ratingStats[httpseller.RatingStats](ctx, r.client, storeIDs)
}
