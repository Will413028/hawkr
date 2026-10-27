// Package block 是店家封鎖：店家封鎖一位買家（單向，只擋買家在這家店的新互動，不擋下單，也不隱藏店家的公開內容）。
//
// 封鎖只碰 seller.store_blocks。這裡不做能力檢查：封鎖是保護動作，店家被停權時仍要能封鎖與解除；誰能代表店家操作由呼叫端的授權
// （httpapi）決定。buyer_id 屬於 buyer 模組，不建外鍵；買家刪帳時由 accountdeletion 在同一個刪帳 transaction 呼叫 ClearAgainstBuyer 刪掉以他為對象的列。不發布 seller.pub_blocks：封鎖只擋寫入，檢查是依 ID 的查詢。
package block

import (
	"context"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
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
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service { return &Service{d: d} }

// Add 店家封鎖買家；已經封鎖回 false。店家不存在回 NotFound。
func (s *Service) Add(ctx context.Context, storeID, buyerID int64) (bool, error) {
	if err := model.Positive(storeID, buyerID); err != nil {
		return false, err
	}
	changed := false
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		queries := store.New(s.d.DB.Conn(ctx))
		exists, err := queries.StoreExists(ctx, storeID)
		if err != nil {
			return err
		}
		if !exists {
			return apperr.NotFound("找不到店家")
		}
		count, err := queries.AddStoreBlock(ctx, store.AddStoreBlockParams{StoreID: storeID, BuyerID: buyerID, CreatedAt: s.d.Now().UTC()})
		changed = count > 0
		return err
	})
	return changed && err == nil, err
}

// Remove 店家解除封鎖；沒有封鎖回 false。
func (s *Service) Remove(ctx context.Context, storeID, buyerID int64) (bool, error) {
	if err := model.Positive(storeID, buyerID); err != nil {
		return false, err
	}
	count, err := store.New(s.d.DB.Conn(ctx)).RemoveStoreBlock(ctx, store.RemoveStoreBlockParams{StoreID: storeID, BuyerID: buyerID})
	return count > 0, err
}

// List 以封鎖時間與買家 ID 分頁列出某店的封鎖名單（新的在前）。
func (s *Service) List(ctx context.Context, input model.BlockedBuyerListInput) (model.BlockedBuyerPage, error) {
	if err := model.Positive(input.StoreID); err != nil {
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
	rows, err := store.New(s.d.DB.Conn(ctx)).ListStoreBlocks(ctx, store.ListStoreBlocksParams{
		StoreID:   input.StoreID,
		HasAfter:  input.After != nil,
		AfterTime: cursor.Time,
		AfterID:   cursor.BuyerID,
		PageLimit: size + 1,
	})
	if err != nil {
		return model.BlockedBuyerPage{}, err
	}
	items := make([]model.BlockedBuyer, 0, len(rows))
	for _, row := range rows {
		items = append(items, model.BlockedBuyer{BuyerID: row.BuyerID, BlockedAt: row.CreatedAt})
	}
	page := model.BlockedBuyerPage{Items: items}
	if len(items) > int(size) {
		page.Items = items[:size]
		last := page.Items[len(page.Items)-1]
		page.Next = &model.BlockedBuyerCursor{Time: last.BlockedAt, BuyerID: last.BuyerID}
	}
	return page, nil
}

// Blocks 店家是否封鎖了買家。
func (s *Service) Blocks(ctx context.Context, storeID, buyerID int64) (bool, error) {
	if err := model.Positive(storeID, buyerID); err != nil {
		return false, err
	}
	return store.New(s.d.DB.Conn(ctx)).StoreBlocksBuyer(ctx, store.StoreBlocksBuyerParams{StoreID: storeID, BuyerID: buyerID})
}

// Blocking 批次：storeIDs（最多 model.MaxBatchSize 個）中封鎖了買家的店家，回傳的 map 只含封鎖的店。
func (s *Service) Blocking(ctx context.Context, buyerID int64, storeIDs []int64) (map[int64]bool, error) {
	if err := model.Positive(buyerID); err != nil {
		return nil, err
	}
	if err := model.BatchIDs(storeIDs); err != nil {
		return nil, err
	}
	ids, err := store.New(s.d.DB.Conn(ctx)).StoresBlockingBuyer(ctx, store.StoresBlockingBuyerParams{BuyerID: buyerID, Column2: storeIDs})
	if err != nil {
		return nil, err
	}
	result := make(map[int64]bool, len(ids))
	for _, id := range ids {
		result[id] = true
	}
	return result, nil
}

// ClearAgainstBuyer 刪掉各店對這位買家的封鎖（買家刪帳）；必須在刪帳的 transaction 內呼叫。
func (s *Service) ClearAgainstBuyer(ctx context.Context, buyerID int64) error {
	if err := model.Positive(buyerID); err != nil {
		return err
	}
	return store.New(s.d.DB.Conn(ctx)).ClearStoreBlocksAgainstBuyer(ctx, buyerID)
}
