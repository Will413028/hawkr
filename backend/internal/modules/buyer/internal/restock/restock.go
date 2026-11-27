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

// Subscribe 建立 SKU 粒度的待通知訂閱（一次性，不設到期），同組重複訂閱冪等。productID 是 SKU 所屬的商品，供清單顯示；
// SKU 存在與否、是否有貨由呼叫端（httpapi）經 catalog 確認，這裡不看 catalog。
func (service *Service) Subscribe(ctx context.Context, buyerID int64, productID, skuID uuid.UUID) error {
	if err := model.Positive(buyerID); err != nil {
		return err
	}
	if err := model.NonNil(productID, skuID); err != nil {
		return err
	}
	return service.deps.DB.InTx(ctx, func(ctx context.Context) error {
		queries := store.New(service.deps.DB.Conn(ctx))
		if _, err := stint.Require(ctx, queries, buyerID, false); err != nil {
			return err
		}
		return queries.SubscribeRestock(ctx, store.SubscribeRestockParams{
			BuyerID:   buyerID,
			SkuID:     skuID,
			ProductID: productID,
			CreatedAt: service.deps.Now().UTC(),
		})
	})
}

// Unsubscribe 取消指定 SKU 的待通知訂閱。
func (service *Service) Unsubscribe(ctx context.Context, buyerID int64, skuID uuid.UUID) error {
	if err := model.Positive(buyerID); err != nil {
		return err
	}
	if err := model.NonNil(skuID); err != nil {
		return err
	}
	return service.deps.DB.InTx(ctx, func(ctx context.Context) error {
		queries := store.New(service.deps.DB.Conn(ctx))
		if _, err := stint.Require(ctx, queries, buyerID, false); err != nil {
			return err
		}
		return queries.UnsubscribeRestock(ctx, store.UnsubscribeRestockParams{BuyerID: buyerID, SkuID: skuID})
	})
}

// PurgeSKUs 刪除這些 SKU 的全部待通知訂閱（商品或 SKU 刪除後，由 catalog 的清理 job 在 transaction 外呼叫）；沒有訂閱的 SKU 是 no-op，
// 可重複呼叫，空清單回 nil。
func (service *Service) PurgeSKUs(ctx context.Context, skuIDs []uuid.UUID) error {
	if len(skuIDs) == 0 {
		return nil
	}
	if err := model.NonNil(skuIDs...); err != nil {
		return err
	}
	return store.New(service.deps.DB.Conn(ctx)).PurgeRestockSKUs(ctx, skuIDs)
}

// FanOut 在獨立 transaction 內處理某個 SKU 最多 Limit 筆（1..100）訂閱：通知並刪除（一次性），回傳這一批通知了幾筆與該 SKU 是否仍有待通知訂閱。
// 事件的發生時間與顯示快照一律來自輸入，不在這裡補當下時間。其他 SKU 的訂閱（含同商品的）不被通知也不被刪除。
func (service *Service) FanOut(ctx context.Context, input model.FanOutInput) (int, bool, error) {
	if err := model.NonNil(input.ProductID, input.SKUID); err != nil {
		return 0, false, err
	}
	if input.Limit < 1 || input.Limit > model.MaxBatchSize {
		return 0, false, apperr.InvalidInput("補貨批次大小必須為 1 到 100")
	}
	if input.OccurredAt.IsZero() {
		return 0, false, apperr.InvalidInput("補貨事件的發生時間必填")
	}
	if _, ok := db.TxFrom(ctx); ok {
		return 0, false, apperr.FailedPrecondition("補貨分批必須在 transaction 外開始")
	}
	skuID, limit := input.SKUID, input.Limit
	var (
		notified int
		more     bool
	)
	err := service.deps.DB.InTx(ctx, func(ctx context.Context) error {
		queries := store.New(service.deps.DB.Conn(ctx))
		buyerIDs, err := queries.FanOut(ctx, store.FanOutParams{SkuID: skuID, BatchLimit: limit})
		if err != nil {
			return err
		}
		notified = len(buyerIDs)
		for _, buyerID := range buyerIDs {
			if err := events.Publish(ctx, service.deps.Events, model.RestockTargeted{
				OccurredAt:  input.OccurredAt.UTC(),
				EventID:     uuid.NewV7(),
				BuyerID:     buyerID,
				ProductID:   input.ProductID,
				SKUID:       skuID,
				ProductName: input.ProductName,
				ImageKey:    input.ImageKey,
			}); err != nil {
				return err
			}
		}
		more, err = queries.HasRestock(ctx, skuID)
		return err
	})
	if err != nil {
		return 0, false, err
	}
	return notified, more, nil
}

// BatchCheck 批次查詢商品是否有 SKU 在待通知訂閱，輸入最多 100 個商品 ID。
func (service *Service) BatchCheck(ctx context.Context, buyerID int64, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	if err := model.Positive(buyerID); err != nil {
		return nil, err
	}
	if err := model.BatchUUIDs(ids); err != nil {
		return nil, err
	}
	rows, err := store.New(service.deps.DB.Conn(ctx)).BatchRestock(ctx, store.BatchRestockParams{BuyerID: buyerID, Column2: ids})
	if err != nil {
		return nil, err
	}
	result := map[uuid.UUID]bool{}
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
		if err := model.NonNil(input.After.ID); err != nil {
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
			SKUID:     row.SkuID,
			CreatedAt: row.CreatedAt,
		})
	}
	if len(result.Items) > int(count) {
		result.Items = result.Items[:count]
		last := result.Items[len(result.Items)-1]
		result.Next = &model.RestockCursor{Time: last.CreatedAt, ID: last.SKUID}
	}
	return result, nil
}
