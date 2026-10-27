// Package restock 提供補貨訂閱與通知目標。
package restock

import (
	"context"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
)

// Deps 外部依賴；clock 由組裝點注入。
type Deps struct {
	DB     *db.DB
	Events *events.Bus
	Now    func() time.Time
}

// Service 此子領域的 use case 容器。
type Service struct {
	deps Deps
}

// New 建立服務，外部依賴由組裝點提供。
func New(deps Deps) *Service { return &Service{deps: deps} }

// Subscribe 建立 待通知訂閱，同組重複訂閱冪等。
func (service *Service) Subscribe(ctx context.Context, buyerID, productID int64) error {
	if err := model.Positive(buyerID, productID); err != nil {
		return err
	}
	return service.deps.DB.InTx(ctx, func(ctx context.Context) error {
		queries := store.New(service.deps.DB.Conn(ctx))
		if _, err := stint.Require(ctx, queries, buyerID, false); err != nil {
			return err
		}
		return queries.SubscribeRestock(ctx, store.SubscribeRestockParams{
			BuyerID:   buyerID,
			ProductID: productID,
			CreatedAt: service.deps.Now().UTC(),
		})
	})
}

// Unsubscribe 取消指定商品的 待通知訂閱。
func (service *Service) Unsubscribe(ctx context.Context, buyerID, productID int64) error {
	if err := model.Positive(buyerID, productID); err != nil {
		return err
	}
	return service.deps.DB.InTx(ctx, func(ctx context.Context) error {
		queries := store.New(service.deps.DB.Conn(ctx))
		if _, err := stint.Require(ctx, queries, buyerID, false); err != nil {
			return err
		}
		return queries.UnsubscribeRestock(ctx, store.UnsubscribeRestockParams{BuyerID: buyerID, ProductID: productID})
	})
}

// FanOut 在獨立 transaction 內處理最多 Limit 筆（1..100），回傳是否仍有待通知訂閱。事件的發生時間與顯示快照
// 一律來自輸入（上游事件），不在這裡補當下時間。
func (service *Service) FanOut(ctx context.Context, input model.FanOutInput) (bool, error) {
	if err := model.Positive(input.ProductID); err != nil {
		return false, err
	}
	if input.Limit < 1 || input.Limit > model.MaxBatchSize {
		return false, apperr.InvalidInput("補貨批次大小必須為 1 到 100")
	}
	if input.OccurredAt.IsZero() {
		return false, apperr.InvalidInput("補貨事件的發生時間必填")
	}
	if _, ok := db.TxFrom(ctx); ok {
		return false, apperr.FailedPrecondition("補貨分批必須在 transaction 外開始")
	}
	productID, limit := input.ProductID, input.Limit
	var more bool
	err := service.deps.DB.InTx(ctx, func(ctx context.Context) error {
		queries := store.New(service.deps.DB.Conn(ctx))
		buyerIDs, err := queries.FanOut(ctx, store.FanOutParams{ProductID: productID, BatchLimit: limit})
		if err != nil {
			return err
		}
		for _, buyerID := range buyerIDs {
			if err := events.Publish(ctx, service.deps.Events, model.RestockTargeted{
				OccurredAt:  input.OccurredAt.UTC(),
				EventID:     uuid.NewV7(),
				BuyerID:     buyerID,
				ProductID:   productID,
				ProductName: input.ProductName,
				ImageKey:    input.ImageKey,
			}); err != nil {
				return err
			}
		}
		more, err = queries.HasRestock(ctx, productID)
		return err
	})
	if err != nil {
		return false, err
	}
	return more, nil
}

// BatchCheck 批次查詢 待通知訂閱，輸入最多 100 個 ID。
func (service *Service) BatchCheck(ctx context.Context, buyerID int64, ids []int64) (map[int64]bool, error) {
	if err := model.Positive(buyerID); err != nil {
		return nil, err
	}
	if err := model.BatchIDs(ids); err != nil {
		return nil, err
	}
	rows, err := store.New(service.deps.DB.Conn(ctx)).BatchRestock(ctx, store.BatchRestockParams{BuyerID: buyerID, Column2: ids})
	if err != nil {
		return nil, err
	}
	result := map[int64]bool{}
	for _, id := range ids {
		result[id] = false
	}
	for _, id := range rows {
		result[id] = true
	}
	return result, err
}

// List 分頁列出 待通知補貨訂閱。
func (service *Service) List(ctx context.Context, input model.RestockListInput) (model.RestockPage, error) {
	if err := model.Positive(input.BuyerID); err != nil {
		return model.RestockPage{}, err
	}
	count, err := pagination.PageSize(input.PageSize, 20, 100)
	if err != nil {
		return model.RestockPage{}, err
	}
	cursor := model.RestockCursor{}
	if input.After != nil {
		if err := model.Positive(input.After.ID); err != nil {
			return model.RestockPage{}, err
		}
		cursor = *input.After
	}
	rows, err := store.New(service.deps.DB.Conn(ctx)).ListRestock(ctx, store.ListRestockParams{
		BuyerID:   input.BuyerID,
		HasAfter:  input.After != nil,
		AfterTime: cursor.Time,
		AfterID:   cursor.ID,
		PageLimit: count + 1,
	})
	if err != nil {
		return model.RestockPage{}, err
	}
	result := model.RestockPage{Items: make([]model.Restock, 0, len(rows))}
	for _, row := range rows {
		result.Items = append(result.Items, model.Restock{
			BuyerID:   row.BuyerID,
			ProductID: row.ProductID,
			CreatedAt: row.CreatedAt,
		})
	}
	if len(result.Items) > int(count) {
		result.Items = result.Items[:count]
		last := result.Items[len(result.Items)-1]
		result.Next = &model.RestockCursor{Time: last.CreatedAt, ID: last.ProductID}
	}
	return result, nil
}
