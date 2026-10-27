// Package favorite 提供收藏關聯與事件。
package favorite

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

// Add 新增收藏，pending 不允許；重複新增不發布事件。
func (service *Service) Add(ctx context.Context, buyerID, id int64) error {
	if err := model.Positive(buyerID, id); err != nil {
		return err
	}
	return service.deps.DB.InTx(ctx, func(ctx context.Context) error {
		queries := store.New(service.deps.DB.Conn(ctx))
		if _, err := stint.Require(ctx, queries, buyerID, true); err != nil {
			return err
		}
		count, err := queries.AddFavorite(ctx, store.AddFavoriteParams{
			BuyerID:   buyerID,
			ProductID: id,
			CreatedAt: service.deps.Now().UTC(),
		})
		if err != nil {
			return err
		}
		if count == 0 {
			return nil
		}
		return events.Publish(ctx, service.deps.Events, model.FavoriteAdded{
			OccurredAt: service.deps.Now().UTC(),
			EventID:    uuid.NewV7(),
			BuyerID:    buyerID,
			ProductID:  id,
		})
	})
}

// List 以時間與商品 ID 分頁列出收藏。
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
	rows, err := store.New(service.deps.DB.Conn(ctx)).ListFavorite(ctx, store.ListFavoriteParams{
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
		items = append(items, model.Item{ID: row.ProductID, Time: row.CreatedAt})
	}
	return model.PageOf(items, count), nil
}

// Remove 移除收藏，只有實際刪除才發布事件。
func (service *Service) Remove(ctx context.Context, buyerID, id int64) error {
	if err := model.Positive(buyerID, id); err != nil {
		return err
	}
	return service.deps.DB.InTx(ctx, func(ctx context.Context) error {
		queries := store.New(service.deps.DB.Conn(ctx))
		if _, err := stint.Require(ctx, queries, buyerID, false); err != nil {
			return err
		}
		count, err := queries.RemoveFavorite(ctx, store.RemoveFavoriteParams{BuyerID: buyerID, ProductID: id})
		if err != nil {
			return err
		}
		if count > 0 {
			return events.Publish(ctx, service.deps.Events, model.FavoriteRemoved{
				OccurredAt: service.deps.Now().UTC(),
				EventID:    uuid.NewV7(),
				BuyerID:    buyerID,
				ProductID:  id,
			})
		}
		return nil
	})
}

// Check 查詢商品是否已收藏。
func (service *Service) Check(ctx context.Context, buyerID, id int64) (bool, error) {
	if err := model.Positive(buyerID, id); err != nil {
		return false, err
	}
	return store.New(service.deps.DB.Conn(ctx)).CheckFavorite(ctx, store.CheckFavoriteParams{BuyerID: buyerID, ProductID: id})
}

// IDs 列出全部收藏商品 ID。
func (service *Service) IDs(ctx context.Context, buyerID int64) ([]int64, error) {
	if err := model.Positive(buyerID); err != nil {
		return nil, err
	}
	result, err := store.New(service.deps.DB.Conn(ctx)).FavoriteIDs(ctx, buyerID)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Count 計算買家收藏總數。
func (service *Service) Count(ctx context.Context, buyerID int64) (int64, error) {
	if err := model.Positive(buyerID); err != nil {
		return 0, err
	}
	return store.New(service.deps.DB.Conn(ctx)).CountFavorite(ctx, buyerID)
}

// BatchCheck 批次查詢收藏狀態，輸入最多 100 個 ID。
func (service *Service) BatchCheck(ctx context.Context, buyerID int64, ids []int64) (map[int64]bool, error) {
	if err := model.Positive(buyerID); err != nil {
		return nil, err
	}
	if err := model.BatchIDs(ids); err != nil {
		return nil, err
	}
	rows, err := store.New(service.deps.DB.Conn(ctx)).BatchFavorite(ctx, store.BatchFavoriteParams{BuyerID: buyerID, Column2: ids})
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
