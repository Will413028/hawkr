package main

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/backend/internal/legacy/productservice"
	"github.com/Will413028/hawkr/backend/internal/legacy/sellerrelay"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// subscribeSellerViews 讓登入買家的商品瀏覽進 seller 的原始事件：訂閱 buyer 的 ProductViewed，各插一列（只有 insert，
// 以事件的 EventID 與 OccurredAt 為準，重送不會多一列）。事件在 RecordVisit 的 transaction 內同步發布，但這裡不解析商品
// 所屬的店家、不碰彙總：解析與彙總由 seller 的 River job 在背景做（sellerProducts 是它的 port），所以商品服務不通不會讓
// 瀏覽失敗。seller 與 buyer 互不依賴，兩者的接線只在這裡。匿名的打點不經過 buyer，由入口直接呼叫 seller 的 RecordProductView。
func subscribeSellerViews(bus *events.Bus, module *seller.Module) {
	events.Subscribe(bus, func(ctx context.Context, e buyer.ProductViewed) error {
		return module.IngestProductView(ctx, seller.ProductView{EventID: e.EventID, ProductID: e.ProductID, OccurredAt: e.OccurredAt})
	})
}

// subscribeSellerRelay 把 seller 的模組事件轉給還在 RabbitMQ 上的消費者（Java product、order、coupon、cart，Go 的 im、social、
// pricebook、notification）：每個事件在發布的 transaction 內排入一個 relay job，worker 組出舊 publisher 的 payload 再送出。
// serve 與 worker 兩個角色都要訂閱：可曝光的變化與 PayUni 的審核結果也從 worker 的 job（對帳、處分到期、webhook 套用）發出。
// 不轉送的事件與原因見 sellerrelay 的套件說明。
func subscribeSellerRelay(bus *events.Bus, queue jobs.Enqueuer) {
	events.Subscribe(bus, sellerrelay.OnStoreRetired(queue))
	events.Subscribe(bus, sellerrelay.OnStoreSuspended(queue))
	events.Subscribe(bus, sellerrelay.OnStoreUnsuspended(queue))
	events.Subscribe(bus, sellerrelay.OnStoreProfileChanged(queue))
	events.Subscribe(bus, sellerrelay.OnStoreExposureChanged(queue))
	events.Subscribe(bus, sellerrelay.OnPayUniForwarded(queue))
	events.Subscribe(bus, sellerrelay.OnPayUniApproved(queue))
	events.Subscribe(bus, sellerrelay.OnPayUniPendingInfo(queue))
	events.Subscribe(bus, sellerrelay.OnPayUniUnderReview(queue))
	events.Subscribe(bus, sellerrelay.OnPayUniRejected(queue))
	events.Subscribe(bus, sellerrelay.OnKYCRejected(queue))
}

// sellerProducts 以舊的 product-service 實作 seller 的商品所屬店家 port（一次批次呼叫，含已下架的商品：直接連結仍可瀏覽）。
// 商品搬進單體之後改呼叫 catalog 的公開 API。
type sellerProducts struct{ client *productservice.Client }

func (p sellerProducts) StoreIDs(ctx context.Context, productIDs []int64) (map[int64]int64, error) {
	cards, err := p.client.Cards(ctx, productIDs, true)
	if err != nil {
		return nil, fmt.Errorf("商品：%w", err)
	}
	out := make(map[int64]int64, len(cards))
	for id, card := range cards {
		out[id] = card.StoreID
	}
	return out, nil
}
