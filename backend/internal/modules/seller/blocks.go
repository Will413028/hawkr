package seller

import (
	"context"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
)

// 店家封鎖（遷移計畫 A7-1b）：店家封鎖一位買家。單向：被封鎖的買家仍看得到店家的公開內容，只是不能在這家店留言、討論區發言與按讚
// （擋寫入的檢查在 social）；不擋下單。這些方法不做能力檢查：封鎖是保護動作，店家被停權時仍要能封鎖與解除；誰能代表店家操作由呼叫端
// 的授權（httpapi 的 ActingStore）決定。買家刪帳時由 accountdeletion 在同一個刪帳 transaction 刪掉以他為對象的列（ClearBlocksAgainstBuyer）。不發布 seller.pub_blocks，檢查是依 ID 的查詢。

// BlockedBuyerCursor 是店家封鎖名單的 keyset 位置。
type BlockedBuyerCursor = model.BlockedBuyerCursor

// BlockedBuyerListInput 是店家封鎖名單的分頁輸入，預設 20、最多 100 筆。
type BlockedBuyerListInput = model.BlockedBuyerListInput

// BlockedBuyer 是店家封鎖名單的一列。
type BlockedBuyer = model.BlockedBuyer

// BlockedBuyerPage 是一頁店家封鎖名單及下一頁位置。
type BlockedBuyerPage = model.BlockedBuyerPage

// BlockBuyer 店家封鎖買家；已經封鎖回 false，店家不存在回 NotFound。冪等，不發布事件。
func (m *Module) BlockBuyer(ctx context.Context, storeID, buyerID int64) (bool, error) {
	return m.block.Add(ctx, storeID, buyerID)
}

// UnblockBuyer 店家解除封鎖；沒有封鎖回 false。冪等。
func (m *Module) UnblockBuyer(ctx context.Context, storeID, buyerID int64) (bool, error) {
	return m.block.Remove(ctx, storeID, buyerID)
}

// ListBlockedBuyers 以封鎖時間與買家 ID 分頁列出某店封鎖的買家（新的在前）。買家刪帳時列就消失。
func (m *Module) ListBlockedBuyers(ctx context.Context, input BlockedBuyerListInput) (BlockedBuyerPage, error) {
	return m.block.List(ctx, input)
}

// StoreBlocksBuyer 店家是否封鎖了買家。給 social 的寫入檢查與 A7-3 的通知過濾使用；加入呼叫方的 transaction。
func (m *Module) StoreBlocksBuyer(ctx context.Context, storeID, buyerID int64) (bool, error) {
	return m.block.Blocks(ctx, storeID, buyerID)
}

// StoresBlockingBuyer 批次：storeIDs（最多 MaxBatchSize 個）中封鎖了買家的店家，回傳的 map 只含封鎖的店。給列表每列的
// viewer_can_comment 使用。
func (m *Module) StoresBlockingBuyer(ctx context.Context, buyerID int64, storeIDs []int64) (map[int64]bool, error) {
	return m.block.Blocking(ctx, buyerID, storeIDs)
}

// ClearBlocksAgainstBuyer 刪掉各店對這位買家的封鎖。只由 accountdeletion 在買家刪帳的 transaction 內呼叫（鎖順序：帳號列、買家列、店家列之後，
// 買家的刪除事件派發之前）。
func (m *Module) ClearBlocksAgainstBuyer(ctx context.Context, buyerID int64) error {
	return m.block.ClearAgainstBuyer(ctx, buyerID)
}
