// Package buyerconsumer bridges order and product events to the buyer module.
package buyerconsumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

type Profiles interface {
	AccountIDByBuyerID(context.Context, int64) (int64, error)
	FanOut(context.Context, buyer.FanOutInput) (bool, error)
}

// CompletePending 隔離 accountdeletion 的結果型別。
type CompletePending func(context.Context, int64) error

// StoreAccount 由 store_id 解析店家所屬的帳號；不在單體的店家（還在舊 seller-service）回 NotFound。
type StoreAccount func(context.Context, int64) (int64, error)

// AdvanceStore 加速店家所屬帳號生效中的終止合作關店（店家的訂單結案了，終止生效後等著未結訂單完成的店家可以關了）。
type AdvanceStore func(ctx context.Context, storeID int64) error

type Consumer struct {
	Buyer    Profiles
	Complete CompletePending
	// StoreAccount 讓店家那一側的訂單結案也能加速完成等待中的刪帳（店家是商家的訂單，與買家的訂單一樣阻擋刪帳）。
	StoreAccount StoreAccount
	// AdvanceStore 讓店家那一側的訂單結案也能加速終止合作的關店；沒有設定時不加速（每小時的對帳保底）。
	AdvanceStore AdvanceStore
	Jobs         jobs.Enqueuer
}

type orderCompleted struct {
	EventID string `json:"event_id"`
	BuyerID string `json:"buyer_id"`
	StoreID string `json:"store_id"`
}

type permanentError struct{ error }

func invalidEvent(format string, args ...any) error {
	return permanentError{fmt.Errorf(format, args...)}
}

type OrderCompletedArgs struct {
	EventID string `json:"event_id" river:"unique"`
	BuyerID int64  `json:"buyer_id"`
	// StoreID 是訂單的店家（store_id，不是帳號）；事件沒有帶時是 0，只加速買家那一側。
	StoreID int64 `json:"store_id,omitempty"`
}

func (OrderCompletedArgs) Kind() string { return "legacy.buyer_order_completed" }
func (OrderCompletedArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

// OrderCompleted 先把事件持久排入 River，RabbitMQ ack 後由 worker 加速完成刪帳。
func (c Consumer) OrderCompleted(ctx context.Context, body []byte) error {
	var event orderCompleted
	if err := json.Unmarshal(body, &event); err != nil {
		return permanentError{fmt.Errorf("buyerconsumer：order.completed payload：%w", err)}
	}
	id, err := strconv.ParseInt(event.BuyerID, 10, 64)
	if event.EventID == "" || err != nil || id <= 0 {
		return invalidEvent("buyerconsumer：order.completed event_id/buyer_id 不合法")
	}
	var storeID int64
	if event.StoreID != "" {
		if storeID, err = strconv.ParseInt(event.StoreID, 10, 64); err != nil || storeID <= 0 {
			return invalidEvent("buyerconsumer：order.completed store_id 不合法")
		}
	}
	args := OrderCompletedArgs{EventID: event.EventID, BuyerID: id, StoreID: storeID}
	opts := args.InsertOpts()
	return c.Jobs.Enqueue(ctx, args, &opts)
}

type orderCompletedWorker struct {
	river.WorkerDefaults[OrderCompletedArgs]
	buyer        Profiles
	complete     CompletePending
	storeAccount StoreAccount
	advanceStore AdvanceStore
}

// Work 完成買家與店家兩側的等待中刪帳：訂單結案時，買家的帳號與店家的帳號都可能正在等這筆訂單。兩側各自重新查
// 未結訂單（CompletePending 本身會查），一側失敗不擋另一側，錯誤合併後回傳讓 River 重試。
func (w *orderCompletedWorker) Work(ctx context.Context, job *river.Job[OrderCompletedArgs]) error {
	err := w.completeAccountOf(ctx, func(ctx context.Context) (int64, error) { return w.buyer.AccountIDByBuyerID(ctx, job.Args.BuyerID) })
	if job.Args.StoreID > 0 {
		storeErr := errors.New("buyerconsumer：沒有設定 store_id 的解析器")
		if w.storeAccount != nil {
			storeErr = w.completeAccountOf(ctx, func(ctx context.Context) (int64, error) { return w.storeAccount(ctx, job.Args.StoreID) })
		}
		err = errors.Join(err, storeErr)
		if w.advanceStore != nil {
			err = errors.Join(err, w.advanceStore(ctx, job.Args.StoreID))
		}
	}
	return err
}

// completeAccountOf 解析帳號後完成它等待中的刪除；找不到帳號、沒有待完成的刪除都不是錯誤。
func (w *orderCompletedWorker) completeAccountOf(ctx context.Context, resolve func(context.Context) (int64, error)) error {
	accountID, err := resolve(ctx)
	if apperr.KindOf(err) == apperr.KindNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	if err := w.complete(ctx, accountID); apperr.KindOf(err) == apperr.KindFailedPrecondition || apperr.KindOf(err) == apperr.KindNotFound {
		return nil
	} else {
		return err
	}
}

type restockEvent struct {
	EventID   string `json:"event_id"`
	ProductID string `json:"product_id"`
	Name      string `json:"product_name"`
	ImageKey  string `json:"image_key"`
	Occurred  string `json:"occurred_at"`
}

// RestockArgs 的唯一鍵只包含上游 event_id，重送的 payload 其他欄位不會另排工作。
type RestockArgs struct {
	EventID   string `json:"event_id" river:"unique"`
	ProductID int64  `json:"product_id"`
	Name      string `json:"product_name"`
	ImageKey  string `json:"image_key"`
	Occurred  string `json:"occurred_at"`
}

func (RestockArgs) Kind() string { return "legacy.buyer_restock" }

func (RestockArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

// ProductRestock 先排持久、以 event_id 去重的 River 工作，再確認 RabbitMQ 訊息。
func (c Consumer) ProductRestock(ctx context.Context, body []byte) error {
	var event restockEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return permanentError{fmt.Errorf("buyerconsumer：product.restock payload：%w", err)}
	}
	id, err := strconv.ParseInt(event.ProductID, 10, 64)
	if event.EventID == "" || err != nil || id <= 0 {
		return invalidEvent("buyerconsumer：product.restock event_id/product_id 不合法")
	}
	if _, err := time.Parse(time.RFC3339Nano, event.Occurred); err != nil {
		return invalidEvent("buyerconsumer：product.restock occurred_at 不合法：%v", err)
	}
	args := RestockArgs{EventID: event.EventID, ProductID: id, Name: event.Name, ImageKey: event.ImageKey, Occurred: event.Occurred}
	opts := args.InsertOpts()
	return c.Jobs.Enqueue(ctx, args, &opts)
}

type restockWorker struct {
	river.WorkerDefaults[RestockArgs]
	buyer Profiles
}

func (w *restockWorker) Work(ctx context.Context, job *river.Job[RestockArgs]) error {
	occurred, err := time.Parse(time.RFC3339Nano, job.Args.Occurred)
	if err != nil {
		return err
	}
	for {
		more, err := w.buyer.FanOut(ctx, buyer.FanOutInput{
			ProductID: job.Args.ProductID, Limit: 100,
			ProductName: job.Args.Name, ImageKey: job.Args.ImageKey, OccurredAt: occurred,
		})
		if err != nil || !more {
			return err
		}
	}
}

func (c Consumer) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &restockWorker{buyer: c.Buyer})
	river.AddWorker(workers, &orderCompletedWorker{buyer: c.Buyer, complete: c.Complete, storeAccount: c.StoreAccount, advanceStore: c.AdvanceStore})
}

func isPermanent(err error) bool {
	var invalid permanentError
	return errors.As(err, &invalid)
}
