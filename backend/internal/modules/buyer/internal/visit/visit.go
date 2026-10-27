// Package visit 提供商品瀏覽紀錄。
package visit

import (
	"context"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/store"
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

// Add 更新最近瀏覽時間並發布 ProductViewed。
func (service *Service) Add(ctx context.Context, buyerID, id int64) error {
	if err := model.Positive(buyerID, id); err != nil {
		return err
	}
	return service.deps.DB.InTx(ctx, func(ctx context.Context) error {
		queries := store.New(service.deps.DB.Conn(ctx))
		if _, err := stint.Require(ctx, queries, buyerID, false); err != nil {
			return err
		}
		_, err := queries.AddVisit(ctx, store.AddVisitParams{
			BuyerID:   buyerID,
			ProductID: id,
			VisitedAt: service.deps.Now().UTC(),
		})
		if err != nil {
			return err
		}
		return events.Publish(ctx, service.deps.Events, model.ProductViewed{
			OccurredAt: service.deps.Now().UTC(),
			EventID:    uuid.NewV7(),
			BuyerID:    buyerID,
			ProductID:  id,
		})
	})
}

// List 以瀏覽時間與商品 ID 分頁。
func (service *Service) List(ctx context.Context, input model.ListInput) (model.Page, error) {
	if input.After != nil {
		if err := model.Positive(input.After.ID); err != nil {
			return model.Page{}, err
		}
	}
	if err := model.Positive(input.BuyerID); err != nil {
		return model.Page{}, err
	}
	count, err := pagination.PageSize(input.PageSize, 20, 100)
	if err != nil {
		return model.Page{}, err
	}
	cursor := model.Cursor{}
	if input.After != nil {
		cursor = *input.After
	}
	rows, err := store.New(service.deps.DB.Conn(ctx)).ListVisit(ctx, store.ListVisitParams{
		BuyerID:   input.BuyerID,
		HasAfter:  input.After != nil,
		AfterTime: cursor.Time,
		AfterID:   cursor.ID,
		PageLimit: count + 1,
	})
	if err != nil {
		return model.Page{}, err
	}
	items := make([]model.Item, 0, len(rows))
	for _, row := range rows {
		items = append(items, model.Item{ID: row.ProductID, Time: row.VisitedAt})
	}
	return model.PageOf(items, count), nil
}

// Clear 清空買家的瀏覽紀錄。
func (service *Service) Clear(ctx context.Context, buyerID int64) error {
	if err := model.Positive(buyerID); err != nil {
		return err
	}
	return service.deps.DB.InTx(ctx, func(ctx context.Context) error {
		queries := store.New(service.deps.DB.Conn(ctx))
		if _, err := stint.Require(ctx, queries, buyerID, false); err != nil {
			return err
		}
		_, err := queries.ClearVisit(ctx, buyerID)
		return err
	})
}
