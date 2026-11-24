// Package reviewrelay 將 review 模組事件轉成還在 RabbitMQ 上的消費者（Java order 與 product 的評分聚合）所需的訊息；新通知由 notification 模組訂閱 review.ReviewCreated，不經這裡。
//
// 對應（送到 hawkr.review，source 是 review-service，wire 形狀由 wire_test 逐欄位釘住）：
//
//	review.ReviewCreated            → review.created
//	review.ReviewAggregateChanged   → review.hidden（隱藏或還原）
//
// 兩種訊息的 review_id 一律是 0：新評價的 ID 是 UUIDv7，舊 wire 是 int64（設計書 step-1-design.md §9 第 2 項）。
//
// 統計（avg_rating、total_count、product_avg_rating、product_review_count）都在 job 執行時讀現況重算，不在寫入的 transaction 內算好存進
// job：消費者（Java product）是全量覆寫，最後一個執行的 job 送的是它執行當下的最新值，任何執行順序最後都收斂到正確值（設計書 §4.2；
// 兩個 job 並行時先讀的較晚送達，是 RD8 接受的殘留，不帶版本）。刪帳者的評價在進入刪帳的 transaction 起就不計分，
// review.purge_author 每批刪除時發的 ReviewAggregateChanged 逐組送出重算後的聚合（A6-D12、A6-D13）。
//
// cmd/hawkr 在 serve 與 worker 訂閱事件、worker 登記 job（單體是 review.created、review.hidden 唯一的 producer）。
// 隨 notification、catalog、ordering 搬進單體而刪除。
package reviewrelay

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

const (
	exchangeReview = "hawkr.review"
	source         = "review-service"
)

type eventFields struct {
	EventID  string    `json:"event_id"`
	Occurred time.Time `json:"occurred"`
}

// ReviewCreatedArgs 轉成 review.created。
type ReviewCreatedArgs struct {
	eventFields
	ReviewID string `json:"review_id"`
}

// ReviewAggregateChangedArgs 轉成一則 review.hidden。
type ReviewAggregateChangedArgs struct {
	eventFields
	StoreID   int64 `json:"store_id"`
	ProductID int64 `json:"product_id"`
}

func (ReviewCreatedArgs) Kind() string          { return "legacy.review_created" }
func (ReviewAggregateChangedArgs) Kind() string { return "legacy.review_aggregate_changed" }

// InsertOpts 以整組參數（含衍生的 event_id）做 unique：父 job 重試重排同一組時只留一個。
func (ReviewAggregateChangedArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

func fields(id fmt.Stringer, at time.Time) eventFields {
	return eventFields{EventID: id.String(), Occurred: at}
}

// OnReviewCreated 等 OnX 在發布事件的 transaction 內把事件轉成 job（transactional outbox）：排入失敗時發布的寫入一起回滾。
func OnReviewCreated(q jobs.Enqueuer) func(context.Context, review.ReviewCreated) error {
	return func(ctx context.Context, e review.ReviewCreated) error {
		return q.Enqueue(ctx, ReviewCreatedArgs{fields(e.EventID, e.OccurredAt), e.ReviewID.String()}, nil)
	}
}

func OnReviewAggregateChanged(q jobs.Enqueuer) func(context.Context, review.ReviewAggregateChanged) error {
	return func(ctx context.Context, e review.ReviewAggregateChanged) error {
		return q.Enqueue(ctx, ReviewAggregateChangedArgs{fields(e.EventID, e.OccurredAt), e.StoreID, e.ProductID}, nil)
	}
}

// Reviews 是 relay 讀評價的 port；*review.Module 實作它。
type Reviews interface {
	ReviewForRelay(ctx context.Context, id uuid.UUID) (review.RelayReview, error)
	ProductStats(ctx context.Context, productID int64) (review.ProductStats, error)
	StoreStats(ctx context.Context, storeID int64) (review.StoreStats, error)
}

// Worker 組出舊 publisher 的 payload 並排入送出的 job。
type Worker struct {
	Reviews Reviews
	Jobs    jobs.Enqueuer
	Log     *slog.Logger
}

// NewWorker 建立 Worker。
func NewWorker(reviews Reviews, queue jobs.Enqueuer, log *slog.Logger) *Worker {
	return &Worker{Reviews: reviews, Jobs: queue, Log: log}
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

// AddWorkers 登記全部 relay 的 River worker。
func (w *Worker) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &typedWorker[ReviewCreatedArgs]{work: w.reviewCreated})
	river.AddWorker(workers, &typedWorker[ReviewAggregateChangedArgs]{work: w.reviewAggregateChanged})
}

// baseEvent 是舊 pkg/mq 的 BaseEvent：timestamp 是 time.Time 的 JSON（RFC3339Nano、UTC），version 固定 "1.0"；
// correlation_id 與 metadata 在舊 publisher 是空值，omitempty 之下不出現，這裡不帶。
type baseEvent struct {
	EventID   string    `json:"event_id"`
	EventType string    `json:"event_type"`
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source"`
}

func base(eventID string, at time.Time, eventType string) baseEvent {
	return baseEvent{EventID: eventID, EventType: eventType, Version: "1.0", Timestamp: at.UTC(), Source: source}
}

// reviewCreated 是舊 ReviewCreatedEvent：id 與計數是 JSON 數字，review_id 是 0。
type reviewCreated struct {
	baseEvent
	ReviewID           int64    `json:"review_id"`
	OrderNo            string   `json:"order_no"`
	OrderItemID        int64    `json:"order_item_id"`
	StoreID            int64    `json:"store_id"`
	BuyerID            int64    `json:"buyer_id"`
	Rating             int32    `json:"rating"`
	Content            string   `json:"content,omitempty"`
	ImageKeys          []string `json:"image_keys,omitempty"`
	CreatedAt          string   `json:"created_at,omitempty"`
	AvgRating          float64  `json:"avg_rating"`
	TotalCount         int64    `json:"total_count"`
	ProductID          int64    `json:"product_id"`
	ProductAvgRating   float64  `json:"product_avg_rating"`
	ProductReviewCount int64    `json:"product_review_count"`
}

// reviewHidden 是舊 ReviewHiddenEvent：帶重算後的店與商品聚合，review_id 是 0。
type reviewHidden struct {
	baseEvent
	ReviewID           int64   `json:"review_id"`
	StoreID            int64   `json:"store_id"`
	ProductID          int64   `json:"product_id"`
	AvgRating          float64 `json:"avg_rating"`
	TotalCount         int64   `json:"total_count"`
	ProductAvgRating   float64 `json:"product_avg_rating"`
	ProductReviewCount int64   `json:"product_review_count"`
}

// enqueuePublish 不在 transaction 內排入（relay 要讀 review、buyer、seller）；relay job 在排入後失敗而重試時，PublishArgs 以 event_id 做的
// unique 讓同一則事件只有一個送出的 job。
func (w *Worker) enqueuePublish(ctx context.Context, eventID, exchange, key string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return w.Jobs.Enqueue(ctx, rabbitmq.PublishArgs{EventID: eventID, Exchange: exchange, RoutingKey: key, Body: body}, rabbitmq.InsertOpts())
}

// hidden 在 job 執行當下重算 (店, 商品) 的聚合，組成一則 review.hidden。
func (w *Worker) hidden(ctx context.Context, eventID string, at time.Time, storeID, productID int64) (reviewHidden, error) {
	store, err := w.Reviews.StoreStats(ctx, storeID)
	if err != nil {
		return reviewHidden{}, err
	}
	product, err := w.Reviews.ProductStats(ctx, productID)
	if err != nil {
		return reviewHidden{}, err
	}
	return reviewHidden{
		baseEvent: base(eventID, at, "REVIEW_HIDDEN"), StoreID: storeID, ProductID: productID,
		AvgRating: store.Average, TotalCount: store.Count, ProductAvgRating: product.Average, ProductReviewCount: product.Count,
	}, nil
}

func (w *Worker) reviewAggregateChanged(ctx context.Context, a ReviewAggregateChangedArgs) error {
	p, err := w.hidden(ctx, a.EventID, a.Occurred, a.StoreID, a.ProductID)
	if err != nil {
		return err
	}
	return w.enqueuePublish(ctx, a.EventID, exchangeReview, "review.hidden", p)
}

func (w *Worker) reviewCreated(ctx context.Context, a ReviewCreatedArgs) error {
	id, err := uuid.Parse(a.ReviewID)
	if err != nil {
		return fmt.Errorf("reviewrelay：評價 ID %q 不是 UUID：%w", a.ReviewID, err)
	}
	r, err := w.Reviews.ReviewForRelay(ctx, id)
	if apperr.KindOf(err) == apperr.KindNotFound {
		return nil // 評價已經清除（作者刪帳）：沒有東西可通知
	}
	if err != nil {
		return err
	}
	if !r.Public {
		return nil // 評價在 job 執行時已被隱藏或作者在刪帳中：不通知賣家，也不外送內容
	}
	h, err := w.hidden(ctx, a.EventID, a.Occurred, r.StoreID, r.ProductID)
	if err != nil {
		return err
	}
	keys := make([]string, len(r.Images))
	for i, im := range r.Images {
		keys[i] = im.ObjectKey
	}
	return w.enqueuePublish(ctx, a.EventID, exchangeReview, "review.created", reviewCreated{
		baseEvent: base(a.EventID, a.Occurred, "REVIEW_CREATED"), OrderNo: r.OrderNo, OrderItemID: r.OrderItemID, StoreID: r.StoreID,
		BuyerID: r.BuyerID, Rating: int32(r.Rating), //nolint:gosec // rating 由 reviews_rating_check 保證 1 到 5
		Content: r.Content, ImageKeys: keys,
		CreatedAt: r.CreatedAt.UTC().Format(time.RFC3339), AvgRating: h.AvgRating, TotalCount: h.TotalCount, ProductID: r.ProductID,
		ProductAvgRating: h.ProductAvgRating, ProductReviewCount: h.ProductReviewCount,
	})
}
