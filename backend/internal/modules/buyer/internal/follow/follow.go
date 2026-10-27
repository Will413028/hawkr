// Package follow 提供追蹤關聯與粉絲統計。
package follow

import (
	"context"
	"fmt"
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

// Add 新增追蹤，pending 不允許；重複追蹤不發布事件。
func (service *Service) Add(ctx context.Context, buyerID, id int64) error {
	if err := model.Positive(buyerID, id); err != nil {
		return err
	}
	return service.deps.DB.InTx(ctx, func(ctx context.Context) error {
		queries := store.New(service.deps.DB.Conn(ctx))
		if _, err := stint.Require(ctx, queries, buyerID, true); err != nil {
			return err
		}
		count, err := queries.AddFollow(ctx, store.AddFollowParams{BuyerID: buyerID, StoreID: id, CreatedAt: service.deps.Now().UTC()})
		if err != nil {
			return err
		}
		if count == 0 {
			return nil
		}
		return events.Publish(ctx, service.deps.Events, model.SellerFollowed{
			OccurredAt: service.deps.Now().UTC(),
			EventID:    uuid.NewV7(),
			BuyerID:    buyerID,
			StoreID:    id,
		})
	})
}

// List 以時間與店家 ID 分頁列出追蹤。
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
	rows, err := store.New(service.deps.DB.Conn(ctx)).ListFollow(ctx, store.ListFollowParams{
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
		items = append(items, model.Item{ID: row.StoreID, Time: row.CreatedAt})
	}
	return model.PageOf(items, count), nil
}

// Remove 移除追蹤，重複操作沒有副作用。
func (service *Service) Remove(ctx context.Context, buyerID, id int64) error {
	if err := model.Positive(buyerID, id); err != nil {
		return err
	}
	return service.deps.DB.InTx(ctx, func(ctx context.Context) error {
		queries := store.New(service.deps.DB.Conn(ctx))
		if _, err := stint.Require(ctx, queries, buyerID, false); err != nil {
			return err
		}
		count, err := queries.RemoveFollow(ctx, store.RemoveFollowParams{BuyerID: buyerID, StoreID: id})
		if err != nil {
			return err
		}
		_ = count
		return nil
	})
}

// Check 查詢是否追蹤店家。
func (service *Service) Check(ctx context.Context, buyerID, id int64) (bool, error) {
	if err := model.Positive(buyerID, id); err != nil {
		return false, err
	}
	return store.New(service.deps.DB.Conn(ctx)).CheckFollow(ctx, store.CheckFollowParams{BuyerID: buyerID, StoreID: id})
}

// BatchCheck 批次查詢追蹤狀態，輸入最多 100 個 ID。
func (service *Service) BatchCheck(ctx context.Context, buyerID int64, ids []int64) (map[int64]bool, error) {
	if err := model.Positive(buyerID); err != nil {
		return nil, err
	}
	if err := model.BatchIDs(ids); err != nil {
		return nil, err
	}
	rows, err := store.New(service.deps.DB.Conn(ctx)).BatchFollow(ctx, store.BatchFollowParams{BuyerID: buyerID, Column2: ids})
	if err != nil {
		return nil, err
	}
	result := make(map[int64]bool, len(rows))
	for _, id := range rows {
		result[id] = true
	}
	return result, nil
}

// Count 計算買家追蹤的店家數。
func (service *Service) Count(ctx context.Context, buyerID int64) (int64, error) {
	if err := model.Positive(buyerID); err != nil {
		return 0, err
	}
	return store.New(service.deps.DB.Conn(ctx)).FollowCount(ctx, buyerID)
}

// IDs 列出全部追蹤店家 ID。
func (service *Service) IDs(ctx context.Context, buyerID int64) ([]int64, error) {
	if err := model.Positive(buyerID); err != nil {
		return nil, err
	}
	result, err := store.New(service.deps.DB.Conn(ctx)).FollowIDs(ctx, buyerID)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Subscribers 以追蹤時間與 buyer_id 分頁列出粉絲。
func (service *Service) Subscribers(ctx context.Context, input model.SubscribersInput) (model.Page, error) {
	if input.After != nil {
		if err := model.Positive(input.After.ID); err != nil {
			return model.Page{}, err
		}
	}
	if err := model.Positive(input.StoreID); err != nil {
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
	rows, err := store.New(service.deps.DB.Conn(ctx)).Subscribers(ctx, store.SubscribersParams{
		StoreID:   input.StoreID,
		HasAfter:  input.After != nil,
		AfterTime: cursor.Time,
		AfterID:   cursor.ID,
		PageLimit: count + 1,
	})
	if err != nil {
		return model.Page{}, err
	}
	result := make([]model.Item, 0, len(rows))
	for _, row := range rows {
		result = append(result, model.Item{ID: row.BuyerID, Time: row.CreatedAt})
	}
	return model.PageOf(result, count), nil
}

// SubscribersOffset 僅供 legacy social-service 的頁碼契約使用。
func (service *Service) SubscribersOffset(ctx context.Context, storeID int64, offset, limit int32) ([]model.Item, error) {
	if err := model.Positive(storeID); err != nil {
		return nil, err
	}
	if offset < 0 || offset > 100000 || limit < 1 || limit > 100 {
		return nil, apperr.InvalidInput("粉絲分頁超出範圍")
	}
	rows, err := store.New(service.deps.DB.Conn(ctx)).SubscribersOffset(ctx, store.SubscribersOffsetParams{StoreID: storeID, PageLimit: limit, PageOffset: offset})
	if err != nil {
		return nil, err
	}
	items := make([]model.Item, 0, len(rows))
	for _, row := range rows {
		items = append(items, model.Item{ID: row.BuyerID, Time: row.CreatedAt})
	}
	return items, nil
}

// SubscriberCounts 批次回傳粉絲數，沒有粉絲的店家為零。
func (service *Service) SubscriberCounts(ctx context.Context, ids []int64) (map[int64]int64, error) {
	if err := model.BatchIDs(ids); err != nil {
		return nil, err
	}
	rows, err := store.New(service.deps.DB.Conn(ctx)).SubscriberCounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := map[int64]int64{}
	for _, id := range ids {
		result[id] = 0
	}
	for _, row := range rows {
		result[row.StoreID] = row.Count
	}
	return result, err
}

// SubscriberStats 回傳粉絲總數與每個時間窗內的新增數，一句查詢。
func (service *Service) SubscriberStats(ctx context.Context, storeID int64, windows []model.TimeWindow) (model.SubscriberStats, error) {
	if err := model.Positive(storeID); err != nil {
		return model.SubscriberStats{}, err
	}
	if len(windows) < 1 || len(windows) > model.MaxStatsWindows {
		return model.SubscriberStats{}, apperr.InvalidInput("時間窗必須是 1 到 %d 個", model.MaxStatsWindows)
	}
	starts, ends := make([]time.Time, len(windows)), make([]time.Time, len(windows))
	for i, w := range windows {
		if !w.Start.Before(w.End) {
			return model.SubscriberStats{}, apperr.InvalidInput("時間窗的起點必須早於終點")
		}
		starts[i], ends[i] = w.Start, w.End
	}
	rows, err := store.New(service.deps.DB.Conn(ctx)).SubscriberStats(ctx, store.SubscriberStatsParams{StoreID: storeID, WindowStarts: starts, WindowEnds: ends})
	if err != nil {
		return model.SubscriberStats{}, err
	}
	if len(rows) != len(windows) {
		return model.SubscriberStats{}, fmt.Errorf("follow：粉絲統計回了 %d 列，請求 %d 個時間窗", len(rows), len(windows))
	}
	out := model.SubscriberStats{Total: rows[0].Total, New: make([]int64, len(rows))}
	for i, row := range rows {
		out.New[i] = row.NewCount
	}
	return out, nil
}
