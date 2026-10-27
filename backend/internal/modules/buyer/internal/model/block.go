package model

import "time"

// BlockedBuyerCursor 買家封鎖買家名單的降冪分頁位置（建立時間與被封鎖的買家 ID）。
type BlockedBuyerCursor struct {
	Time    time.Time
	BuyerID int64
}

// Valid 是 cursor 的內容合法（解自 client 回傳的 page token，不可信）。
func (c BlockedBuyerCursor) Valid() bool { return c.BuyerID > 0 && !c.Time.IsZero() }

// BlockedBuyerListInput 買家封鎖買家名單的 keyset 輸入，預設 20、最多 100 筆。
type BlockedBuyerListInput struct {
	BuyerID  int64
	PageSize int32
	After    *BlockedBuyerCursor
}

// BlockedBuyerItem 買家封鎖買家名單的一列。
type BlockedBuyerItem struct {
	BuyerID int64
	Time    time.Time
}

// BlockedBuyerPage 一頁買家封鎖買家名單及下一頁位置。
type BlockedBuyerPage struct {
	Items []BlockedBuyerItem
	Next  *BlockedBuyerCursor
}

// BlockedStoreCursor 買家封鎖店名單的降冪分頁位置（建立時間與店家 ID）。
type BlockedStoreCursor struct {
	Time    time.Time
	StoreID int64
}

// Valid 是 cursor 的內容合法（解自 client 回傳的 page token，不可信）。
func (c BlockedStoreCursor) Valid() bool { return c.StoreID > 0 && !c.Time.IsZero() }

// BlockedStoreListInput 買家封鎖店名單的 keyset 輸入，預設 20、最多 100 筆。
type BlockedStoreListInput struct {
	BuyerID  int64
	PageSize int32
	After    *BlockedStoreCursor
}

// BlockedStoreItem 買家封鎖店名單的一列。
type BlockedStoreItem struct {
	StoreID int64
	Time    time.Time
}

// BlockedStorePage 一頁買家封鎖店名單及下一頁位置。
type BlockedStorePage struct {
	Items []BlockedStoreItem
	Next  *BlockedStoreCursor
}
