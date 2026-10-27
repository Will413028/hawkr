package dto

type ListCustomersReq struct {
	Keyword      string
	SortBy       string
	SortOrder    string
	CustomerType string // "" = 全部
	StoreID      int64
	TagID        int64
	Page         int32
	PageSize     int32
}

type UpdateCustomerTypeReq struct {
	CustomerType string
	StoreID      int64
	BuyerID      int64
}

type SearchCustomersByTagsReq struct {
	TagIDs  []int64
	StoreID int64
}
