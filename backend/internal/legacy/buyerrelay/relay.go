// Package buyerrelay 將 buyer 模組事件轉成尚未搬遷的 RabbitMQ 消費者所需訊息。
package buyerrelay

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/legacy/productservice"
	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

type eventFields struct {
	EventID  string    `json:"event_id"`
	Occurred time.Time `json:"occurred"`
	BuyerID  int64     `json:"buyer_id"`
}

type FavoriteAddedArgs struct {
	eventFields
	ProductID int64 `json:"product_id"`
}
type FavoriteRemovedArgs struct {
	eventFields
	ProductID int64 `json:"product_id"`
}
type SellerFollowedArgs struct {
	eventFields
	StoreID int64 `json:"store_id"`
}
type ProductViewedArgs struct {
	eventFields
	ProductID int64 `json:"product_id"`
}
type RestockTargetedArgs struct {
	eventFields
	ProductID   int64  `json:"product_id"`
	ProductName string `json:"product_name"`
	ImageKey    string `json:"image_key"`
}
type BuyerDeletedArgs struct {
	eventFields
	AccountID int64 `json:"account_id"`
}

func (FavoriteAddedArgs) Kind() string   { return "legacy.buyer_favorite_added" }
func (FavoriteRemovedArgs) Kind() string { return "legacy.buyer_favorite_removed" }
func (SellerFollowedArgs) Kind() string  { return "legacy.buyer_seller_followed" }
func (ProductViewedArgs) Kind() string   { return "legacy.buyer_product_viewed" }
func (RestockTargetedArgs) Kind() string { return "legacy.buyer_restock_targeted" }
func (BuyerDeletedArgs) Kind() string    { return "legacy.buyer_deleted" }

type Enqueue func(context.Context, river.JobArgs) error

func NewEnqueue(j jobs.Enqueuer) Enqueue {
	return func(ctx context.Context, args river.JobArgs) error { return j.Enqueue(ctx, args, nil) }
}

func OnFavoriteAdded(enqueue Enqueue) func(context.Context, buyer.FavoriteAdded) error {
	return func(ctx context.Context, e buyer.FavoriteAdded) error {
		return enqueue(ctx, FavoriteAddedArgs{eventFields: eventFields{e.EventID.String(), e.OccurredAt, e.BuyerID}, ProductID: e.ProductID})
	}
}
func OnFavoriteRemoved(enqueue Enqueue) func(context.Context, buyer.FavoriteRemoved) error {
	return func(ctx context.Context, e buyer.FavoriteRemoved) error {
		return enqueue(ctx, FavoriteRemovedArgs{eventFields: eventFields{e.EventID.String(), e.OccurredAt, e.BuyerID}, ProductID: e.ProductID})
	}
}
func OnSellerFollowed(enqueue Enqueue) func(context.Context, buyer.SellerFollowed) error {
	return func(ctx context.Context, e buyer.SellerFollowed) error {
		return enqueue(ctx, SellerFollowedArgs{eventFields: eventFields{e.EventID.String(), e.OccurredAt, e.BuyerID}, StoreID: e.StoreID})
	}
}
func OnProductViewed(enqueue Enqueue) func(context.Context, buyer.ProductViewed) error {
	return func(ctx context.Context, e buyer.ProductViewed) error {
		return enqueue(ctx, ProductViewedArgs{eventFields: eventFields{e.EventID.String(), e.OccurredAt, e.BuyerID}, ProductID: e.ProductID})
	}
}
func OnRestockTargeted(enqueue Enqueue) func(context.Context, buyer.RestockTargeted) error {
	return func(ctx context.Context, e buyer.RestockTargeted) error {
		return enqueue(ctx, RestockTargetedArgs{eventFields: eventFields{e.EventID.String(), e.OccurredAt, e.BuyerID}, ProductID: e.ProductID, ProductName: e.ProductName, ImageKey: e.ImageKey})
	}
}
func OnBuyerDeleted(enqueue Enqueue) func(context.Context, buyer.BuyerDeleted) error {
	return func(ctx context.Context, e buyer.BuyerDeleted) error {
		return enqueue(ctx, BuyerDeletedArgs{eventFields: eventFields{e.EventID.String(), e.OccurredAt, e.BuyerID}, AccountID: e.AccountID})
	}
}

type Buyers interface {
	AccountIDByBuyerID(context.Context, int64) (int64, error)
	ProfileByBuyerID(context.Context, int64) (buyer.Profile, error)
}
type Product = productservice.Card
type Products interface {
	Cards(context.Context, []int64, bool) (map[int64]Product, error)
}

// Stores 讀店家；seller 模組滿足它。已關閉的店家與不存在一樣當成找不到（舊 seller-service 關店就是軟刪除）。
type Store = seller.Store
type Stores interface {
	Store(context.Context, int64) (Store, error)
}

type Worker struct {
	Buyers   Buyers
	Products Products
	Stores   Stores
	Jobs     jobs.Enqueuer
	Log      *slog.Logger
}

func NewWorker(b Buyers, p Products, s Stores, queue jobs.Enqueuer, log *slog.Logger) *Worker {
	return &Worker{Buyers: b, Products: p, Stores: s, Jobs: queue, Log: log}
}

// store 在 relay 當下讀店家（計畫步驟 2：legacy payload 的店家欄位由 legacy 讀，不是事件當下的快照）。店家已不存在是
// 永久狀態，重試不會改變：記 log 後回 ok=false，由呼叫方決定缺欄位照送或略過，不回錯誤讓 job 重試到上限。
func (w *Worker) store(ctx context.Context, event string, storeID int64) (Store, bool, error) {
	s, err := w.Stores.Store(ctx, storeID)
	if apperr.KindOf(err) == apperr.KindNotFound || (err == nil && s.Status == seller.Closed) {
		if w.Log != nil {
			w.Log.WarnContext(ctx, "buyerrelay：店家不存在，店家欄位留空", "event", event, "store_id", storeID)
		}
		return Store{}, false, nil
	}
	if err != nil {
		return Store{}, false, err
	}
	return s, true, nil
}

// accountRef 是舊 buyer-service 的帳號字串欄位：沒有帳號時送空字串，不送 "0"。
func accountRef(id int64) string {
	if id <= 0 {
		return ""
	}
	return strconv.FormatInt(id, 10)
}

type typedWorker[T river.JobArgs] struct {
	river.WorkerDefaults[T]
	work func(context.Context, T) error
}

func (w *typedWorker[T]) Work(ctx context.Context, job *river.Job[T]) error {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	return w.work(ctx, job.Args)
}

func (w *Worker) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &typedWorker[FavoriteAddedArgs]{work: w.favoriteAdded})
	river.AddWorker(workers, &typedWorker[FavoriteRemovedArgs]{work: w.favoriteRemoved})
	river.AddWorker(workers, &typedWorker[SellerFollowedArgs]{work: w.sellerFollowed})
	river.AddWorker(workers, &typedWorker[ProductViewedArgs]{work: w.productViewed})
	river.AddWorker(workers, &typedWorker[RestockTargetedArgs]{work: w.restockTargeted})
	river.AddWorker(workers, &typedWorker[BuyerDeletedArgs]{work: w.buyerDeleted})
}

func base(a eventFields, eventType string) map[string]any {
	return map[string]any{
		"event_id": a.EventID, "event_type": eventType, "version": "1.0",
		"timestamp": a.Occurred.UTC(), "source": "buyer-service", "metadata": map[string]string{},
	}
}

// enqueuePublish 不在 transaction 內排入（relay 要呼叫 legacy 服務）；relay job 在排入後失敗而重試時，
// PublishArgs 以 event_id 做的 unique 讓同一則事件只有一個送出的 job。
func (w *Worker) enqueuePublish(ctx context.Context, a eventFields, exchange, key string, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return w.Jobs.Enqueue(ctx, rabbitmq.PublishArgs{EventID: a.EventID, Exchange: exchange, RoutingKey: key, Body: body}, rabbitmq.InsertOpts())
}

func (w *Worker) accountID(ctx context.Context, buyerID int64) (int64, error) {
	return w.Buyers.AccountIDByBuyerID(ctx, buyerID)
}

func (w *Worker) name(ctx context.Context, buyerID int64) (string, error) {
	profile, err := w.Buyers.ProfileByBuyerID(ctx, buyerID)
	if apperr.KindOf(err) == apperr.KindNotFound {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return profile.DisplayName, nil
}

func (w *Worker) product(ctx context.Context, id int64) (Product, error) {
	cards, err := w.Products.Cards(ctx, []int64{id}, true)
	if err != nil {
		return Product{}, err
	}
	return cards[id], nil
}

func (w *Worker) favoriteAdded(ctx context.Context, a FavoriteAddedArgs) error {
	accountID, err := w.accountID(ctx, a.BuyerID)
	if err != nil {
		return err
	}
	product, err := w.product(ctx, a.ProductID)
	if err != nil {
		return err
	}
	name, err := w.name(ctx, a.BuyerID)
	if err != nil {
		return err
	}
	payload := base(a.eventFields, "BUYER_FAVORITE_ADDED")
	payload["product_id"], payload["account_id"] = strconv.FormatInt(a.ProductID, 10), strconv.FormatInt(accountID, 10)
	payload["occurred_at"], payload["buyer_name"], payload["product_name"] = a.Occurred.UTC().Format(time.RFC3339Nano), name, product.Name
	payload["seller_account_id"], payload["store_id"] = "", ""
	if product.StoreID > 0 {
		// 店家不存在時仍送：product-service 的收藏數依 product_id 計，只缺賣家通知的欄位。
		store, _, err := w.store(ctx, "buyer.favorite.added", product.StoreID)
		if err != nil {
			return err
		}
		payload["seller_account_id"], payload["store_id"] = accountRef(store.AccountID), strconv.FormatInt(product.StoreID, 10)
	}
	return w.enqueuePublish(ctx, a.eventFields, "hawkr.buyer", "buyer.favorite.added", payload)
}

func (w *Worker) favoriteRemoved(ctx context.Context, a FavoriteRemovedArgs) error {
	accountID, err := w.accountID(ctx, a.BuyerID)
	if err != nil {
		return err
	}
	payload := base(a.eventFields, "BUYER_FAVORITE_REMOVED")
	payload["product_id"], payload["account_id"] = strconv.FormatInt(a.ProductID, 10), strconv.FormatInt(accountID, 10)
	payload["occurred_at"] = a.Occurred.UTC().Format(time.RFC3339Nano)
	payload["seller_account_id"], payload["store_id"] = "", ""
	payload["buyer_name"], payload["product_name"] = "", ""
	return w.enqueuePublish(ctx, a.eventFields, "hawkr.buyer", "buyer.favorite.removed", payload)
}

func (w *Worker) sellerFollowed(ctx context.Context, a SellerFollowedArgs) error {
	accountID, err := w.accountID(ctx, a.BuyerID)
	if err != nil {
		return err
	}
	store, ok, err := w.store(ctx, "follow.added", a.StoreID)
	if err != nil {
		return err
	}
	if !ok {
		// follow.added 唯一的用途是通知被追蹤的賣家（merchant_id）；店家已不存在就沒有收件人。舊 buyer-service 在這裡
		// 把 store_id 當成帳號送出（#2789，通知送錯人），不沿用。
		return nil
	}
	name, err := w.name(ctx, a.BuyerID)
	if err != nil {
		return err
	}
	payload := base(a.eventFields, "FOLLOW_ADDED")
	payload["follower_id"], payload["follower_name"] = accountID, name
	payload["merchant_id"], payload["store_id"] = store.AccountID, a.StoreID
	payload["occurred_at"] = a.Occurred.UTC().Format(time.RFC3339Nano)
	return w.enqueuePublish(ctx, a.eventFields, "hawkr.content", "follow.added", payload)
}

func (w *Worker) productViewed(ctx context.Context, a ProductViewedArgs) error {
	product, err := w.product(ctx, a.ProductID)
	if err != nil {
		return err
	}
	var store Store
	if product.StoreID > 0 {
		// 店家不存在時仍送：瀏覽數依 product_id 計，seller_id 與沒有店家的商品一樣是 0。
		store, _, err = w.store(ctx, "product.viewed", product.StoreID)
		if err != nil {
			return err
		}
	}
	payload := base(a.eventFields, "PRODUCT_VIEWED")
	payload["seller_id"], payload["store_id"] = store.AccountID, product.StoreID
	payload["product_name"], payload["product_id"] = product.Name, a.ProductID
	return w.enqueuePublish(ctx, a.eventFields, "hawkr.content", "product.viewed", payload)
}

func (w *Worker) restockTargeted(ctx context.Context, a RestockTargetedArgs) error {
	payload := base(a.eventFields, "PRODUCT_RESTOCK_TARGET")
	payload["subscription_id"], payload["buyer_id"] = a.EventID, a.BuyerID
	payload["product_id"] = strconv.FormatInt(a.ProductID, 10)
	payload["product_name"], payload["image_key"] = a.ProductName, a.ImageKey
	payload["deep_link"] = "hawkr://products/" + strconv.FormatInt(a.ProductID, 10)
	payload["occurred_at"] = a.Occurred.UTC().Format(time.RFC3339Nano)
	return w.enqueuePublish(ctx, a.eventFields, "hawkr.product", "product.restock.target", payload)
}

func (w *Worker) buyerDeleted(ctx context.Context, a BuyerDeletedArgs) error {
	payload := base(a.eventFields, "BUYER_DELETED")
	payload["accountId"], payload["buyerId"] = a.AccountID, a.BuyerID
	return w.enqueuePublish(ctx, a.eventFields, "hawkr.buyer", "buyer.deleted", payload)
}
