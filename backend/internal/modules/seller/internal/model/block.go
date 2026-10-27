package model

import "time"

// BlockedBuyerCursor 店家封鎖名單的降冪分頁位置（封鎖時間與買家 ID）。
type BlockedBuyerCursor struct {
	Time    time.Time
	BuyerID int64
}

// Valid 是 cursor 的內容合法（解自 client 回傳的 page token，不可信）。
func (c BlockedBuyerCursor) Valid() bool { return c.BuyerID > 0 && !c.Time.IsZero() }

// BlockedBuyerListInput 店家封鎖名單的 keyset 輸入，預設 20、最多 100 筆。
type BlockedBuyerListInput struct {
	StoreID  int64
	PageSize int32
	After    *BlockedBuyerCursor
}

// BlockedBuyer 店家封鎖名單的一列。
type BlockedBuyer struct {
	BuyerID   int64
	BlockedAt time.Time
}

// BlockedBuyerPage 一頁店家封鎖名單及下一頁位置。
type BlockedBuyerPage struct {
	Items []BlockedBuyer
	Next  *BlockedBuyerCursor
}
