package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/legacy/orderservice"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// newIM 組裝 im 模組並訂閱買家與店家的刪除事件（單體規則 4 的原子狀態變更：同 transaction 記 inactive_parties、排抹除 job）。
// serve 與 worker 都要呼叫：serve 的刪帳流程與 worker 的 CompletePending、店家退場都會發這兩個事件，worker 另外執行它的 job
// （媒體引用同步、抹除與其保底掃描）。im 的 relay（imrelay）與其他事件訂閱不在這裡接：舊 im-service 在 A6-6e 前仍是聊天唯一的寫入者，
// 並行期只能有一個生效的 producer；沒有對外入口，所以 im 的表在那之前沒有寫入者，訂閱接上沒有副作用。
func newIM(d *db.DB, bus *events.Bus, jc jobs.Enqueuer, buyers im.Buyers, sellers im.Stores, med media.Media, orders imOrderClient, log *slog.Logger) *im.Module {
	m := im.New(im.Deps{
		DB: d, Events: bus, Jobs: jc, Media: med, Buyers: buyers, Stores: sellers, Orders: imOrders{orders},
		NewID: uuid.NewV7, Now: time.Now, Logger: log,
	})
	events.Subscribe(bus, m.OnBuyerDeleted)
	events.Subscribe(bus, m.OnStoreRetired)
	return m
}

// imOrderClient 是 imOrders 用到的 orderservice.Client 的方法。
type imOrderClient interface {
	HasOrderBetween(ctx context.Context, buyerID, storeID int64) (bool, string, error)
	OrderParties(ctx context.Context, orderNo string) (orderservice.OrderParties, error)
}

// imOrders 以還在舊服務的 order-service 實作 im 的訂單 port（ordering 搬進單體後換成 in-process 的精確查詢）。
type imOrders struct{ client imOrderClient }

// imOpenStatuses 是「進行中」的訂單狀態：訂單未完成。有活躍售後的訂單不會離開 40（自動完成反查售後、確認收貨被進行中的 case 擋下，
// 50 不可再發起售後），所以售後不必另查。
var imOpenStatuses = map[string]bool{"10": true, "20": true, "30": true, "32": true, "35": true, "40": true}

// OpenOrderBetween 取雙方最近一筆有關聯的訂單，看它的狀態是否進行中。已知看不到「最近一筆已結案、較舊的一筆還在配送」，
// 錯的方向是多擋；任何查詢錯誤（含訂單查無、買家或店家與訂單不符）都回錯誤，呼叫端一律擋（fail-closed）。
func (a imOrders) OpenOrderBetween(ctx context.Context, buyerID, storeID int64) (bool, error) {
	has, orderNo, err := a.client.HasOrderBetween(ctx, buyerID, storeID)
	if err != nil || !has {
		return false, err
	}
	p, err := a.client.OrderParties(ctx, orderNo)
	if err != nil {
		return false, err
	}
	if p.BuyerID != buyerID || p.StoreID != storeID {
		return false, fmt.Errorf("im：訂單 %s 的買家 %d、店家 %d 不是查詢的 %d、%d", orderNo, p.BuyerID, p.StoreID, buyerID, storeID)
	}
	return imOpenStatuses[p.Status], nil
}
