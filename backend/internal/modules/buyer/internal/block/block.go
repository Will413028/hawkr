// Package block 是買家的個人封鎖：買家封鎖買家（blocked_buyers，對象在本模組）與買家封鎖店（blocked_stores，店在 seller，不建外鍵）。
//
// 封鎖是發起者擁有的關聯，寫入只碰這兩張表。買家刪帳時由 deletion 刪掉他發起的兩種列，以及以他為對象的買家封鎖買家的列；以店為對象的列
// 在店退場後保留。其他模組讀它經 buyer.pub_blocked_buyers()、buyer.pub_blocked_stores()（強一致、不建投影），in-process 的呼叫經根套件的
// IsBlockingStore、IsBlockedBetween。
package block

import (
	"context"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
)

// Deps 外部依賴；clock 由組裝點注入。
type Deps struct {
	DB  *db.DB
	Now func() time.Time
}

// Service 此子領域的 use case 容器。
type Service struct {
	deps Deps
}

// New 建立服務，外部依賴由組裝點提供。
func New(deps Deps) *Service { return &Service{deps: deps} }

// AddBuyer 買家封鎖買家；已經封鎖回 false。發起者必須是營運中，封鎖自己回 InvalidInput，對象不存在或已關閉回 NotFound。
func (service *Service) AddBuyer(ctx context.Context, buyerID, targetBuyerID int64) (bool, error) {
	if err := model.Positive(buyerID, targetBuyerID); err != nil {
		return false, err
	}
	if buyerID == targetBuyerID {
		return false, apperr.InvalidInput("不能封鎖自己")
	}
	changed := false
	err := service.deps.DB.InTx(ctx, func(ctx context.Context) error {
		queries := store.New(service.deps.DB.Conn(ctx))
		if err := lockPair(ctx, queries, buyerID, targetBuyerID); err != nil {
			return err
		}
		count, err := queries.AddBlockedBuyer(ctx, store.AddBlockedBuyerParams{
			BlockerBuyerID: buyerID,
			BlockedBuyerID: targetBuyerID,
			CreatedAt:      service.deps.Now().UTC(),
		})
		changed = count > 0
		return err
	})
	return changed && err == nil, err
}

// lockPair 在同一個 transaction 依 buyer_id 升冪鎖兩列：發起者排他（營運中才放行）、對象共享（已關閉回 NotFound）。固定順序讓兩人同時互相封鎖
// 不會互等；對象的共享鎖與對象刪帳的排他鎖互斥，鎖到之後才檢查狀態，所以不會留下指向已關閉買家的列。
func lockPair(ctx context.Context, queries *store.Queries, initiatorID, targetID int64) error {
	lockTarget := func() error {
		target, err := queries.LockBuyerShared(ctx, targetID)
		if err != nil {
			return model.Error(err)
		}
		if target.Status == string(model.Closed) {
			return apperr.NotFound("買家已關閉")
		}
		return nil
	}
	lockInitiator := func() error {
		_, err := stint.Require(ctx, queries, initiatorID, true)
		return err
	}
	if initiatorID < targetID {
		if err := lockInitiator(); err != nil {
			return err
		}
		return lockTarget()
	}
	if err := lockTarget(); err != nil {
		return err
	}
	return lockInitiator()
}

// RemoveBuyer 解除買家封鎖買家；沒有封鎖回 false。
func (service *Service) RemoveBuyer(ctx context.Context, buyerID, targetBuyerID int64) (bool, error) {
	if err := model.Positive(buyerID, targetBuyerID); err != nil {
		return false, err
	}
	changed := false
	err := service.deps.DB.InTx(ctx, func(ctx context.Context) error {
		queries := store.New(service.deps.DB.Conn(ctx))
		if _, err := stint.Require(ctx, queries, buyerID, false); err != nil {
			return err
		}
		count, err := queries.RemoveBlockedBuyer(ctx, store.RemoveBlockedBuyerParams{BlockerBuyerID: buyerID, BlockedBuyerID: targetBuyerID})
		changed = count > 0
		return err
	})
	return changed && err == nil, err
}

// AddStore 買家封鎖店；已經封鎖回 false。發起者必須是營運中。店是否存在不在這裡檢查（buyer 不依賴 seller，由組裝點檢查）。
func (service *Service) AddStore(ctx context.Context, buyerID, storeID int64) (bool, error) {
	if err := model.Positive(buyerID, storeID); err != nil {
		return false, err
	}
	changed := false
	err := service.deps.DB.InTx(ctx, func(ctx context.Context) error {
		queries := store.New(service.deps.DB.Conn(ctx))
		if _, err := stint.Require(ctx, queries, buyerID, true); err != nil {
			return err
		}
		count, err := queries.AddBlockedStore(ctx, store.AddBlockedStoreParams{
			BlockerBuyerID: buyerID,
			StoreID:        storeID,
			CreatedAt:      service.deps.Now().UTC(),
		})
		changed = count > 0
		return err
	})
	return changed && err == nil, err
}

// RemoveStore 解除買家封鎖店；沒有封鎖回 false。
func (service *Service) RemoveStore(ctx context.Context, buyerID, storeID int64) (bool, error) {
	if err := model.Positive(buyerID, storeID); err != nil {
		return false, err
	}
	changed := false
	err := service.deps.DB.InTx(ctx, func(ctx context.Context) error {
		queries := store.New(service.deps.DB.Conn(ctx))
		if _, err := stint.Require(ctx, queries, buyerID, false); err != nil {
			return err
		}
		count, err := queries.RemoveBlockedStore(ctx, store.RemoveBlockedStoreParams{BlockerBuyerID: buyerID, StoreID: storeID})
		changed = count > 0
		return err
	})
	return changed && err == nil, err
}

// ListBuyers 以建立時間與被封鎖的買家 ID 分頁列出買家封鎖的買家。
func (service *Service) ListBuyers(ctx context.Context, input model.BlockedBuyerListInput) (model.BlockedBuyerPage, error) {
	if err := model.Positive(input.BuyerID); err != nil {
		return model.BlockedBuyerPage{}, err
	}
	if input.After != nil && !input.After.Valid() {
		return model.BlockedBuyerPage{}, apperr.InvalidInput("分頁位置不合法")
	}
	size, err := pagination.PageSize(input.PageSize, 20, 100)
	if err != nil {
		return model.BlockedBuyerPage{}, err
	}
	cursor := model.BlockedBuyerCursor{}
	if input.After != nil {
		cursor = *input.After
	}
	rows, err := store.New(service.deps.DB.Conn(ctx)).ListBlockedBuyers(ctx, store.ListBlockedBuyersParams{
		BlockerBuyerID: input.BuyerID,
		HasAfter:       input.After != nil,
		AfterTime:      cursor.Time,
		AfterID:        cursor.BuyerID,
		PageLimit:      size + 1,
	})
	if err != nil {
		return model.BlockedBuyerPage{}, err
	}
	items := make([]model.BlockedBuyerItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, model.BlockedBuyerItem{BuyerID: row.BlockedBuyerID, Time: row.CreatedAt})
	}
	page := model.BlockedBuyerPage{Items: items}
	if len(items) > int(size) {
		page.Items = items[:size]
		last := page.Items[len(page.Items)-1]
		page.Next = &model.BlockedBuyerCursor{Time: last.Time, BuyerID: last.BuyerID}
	}
	return page, nil
}

// ListStores 以建立時間與店家 ID 分頁列出買家封鎖的店。
func (service *Service) ListStores(ctx context.Context, input model.BlockedStoreListInput) (model.BlockedStorePage, error) {
	if err := model.Positive(input.BuyerID); err != nil {
		return model.BlockedStorePage{}, err
	}
	if input.After != nil && !input.After.Valid() {
		return model.BlockedStorePage{}, apperr.InvalidInput("分頁位置不合法")
	}
	size, err := pagination.PageSize(input.PageSize, 20, 100)
	if err != nil {
		return model.BlockedStorePage{}, err
	}
	cursor := model.BlockedStoreCursor{}
	if input.After != nil {
		cursor = *input.After
	}
	rows, err := store.New(service.deps.DB.Conn(ctx)).ListBlockedStores(ctx, store.ListBlockedStoresParams{
		BlockerBuyerID: input.BuyerID,
		HasAfter:       input.After != nil,
		AfterTime:      cursor.Time,
		AfterID:        cursor.StoreID,
		PageLimit:      size + 1,
	})
	if err != nil {
		return model.BlockedStorePage{}, err
	}
	items := make([]model.BlockedStoreItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, model.BlockedStoreItem{StoreID: row.StoreID, Time: row.CreatedAt})
	}
	page := model.BlockedStorePage{Items: items}
	if len(items) > int(size) {
		page.Items = items[:size]
		last := page.Items[len(page.Items)-1]
		page.Next = &model.BlockedStoreCursor{Time: last.Time, StoreID: last.StoreID}
	}
	return page, nil
}

// IsBlockingStore 查詢買家是否封鎖了店。
func (service *Service) IsBlockingStore(ctx context.Context, buyerID, storeID int64) (bool, error) {
	if err := model.Positive(buyerID, storeID); err != nil {
		return false, err
	}
	return store.New(service.deps.DB.Conn(ctx)).IsBlockingStore(ctx, store.IsBlockingStoreParams{BlockerBuyerID: buyerID, StoreID: storeID})
}

// IsBlockedBetween 查詢兩位買家之間任一方向是否有封鎖。
func (service *Service) IsBlockedBetween(ctx context.Context, buyerA, buyerB int64) (bool, error) {
	if err := model.Positive(buyerA, buyerB); err != nil {
		return false, err
	}
	return store.New(service.deps.DB.Conn(ctx)).IsBlockedBetweenBuyers(ctx, store.IsBlockedBetweenBuyersParams{BuyerA: buyerA, BuyerB: buyerB})
}
