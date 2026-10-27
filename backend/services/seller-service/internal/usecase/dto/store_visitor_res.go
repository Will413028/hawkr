package dto

// StoreVisitorItem 單筆訪客資料
type StoreVisitorItem struct {
	LastVisitedAt  int64 // Unix timestamp
	VisitorBuyerID int64
	VisitCount     int32
}

// StoreVisitorList 訪客列表（分頁）
type StoreVisitorList struct {
	Records  []StoreVisitorItem
	Total    int64
	Page     int32
	PageSize int32
}
