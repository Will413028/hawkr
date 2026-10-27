package dto

type CreateTagReq struct {
	Name    string
	StoreID int64
}

type UpdateTagReq struct {
	Name    string
	StoreID int64
	TagID   int64
}

type AddCustomerTagsByNameReq struct {
	TagNames []string
	StoreID  int64
	BuyerID  int64
}

type ManageCustomerTagReq struct {
	StoreID int64
	BuyerID int64
	TagID   int64
}
