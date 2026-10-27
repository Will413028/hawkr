package dto

// SellerMeta is a lightweight seller metadata response for cross-service queries.
type SellerMeta struct {
	SellerName      string
	LogoURL         string
	ZipCode         string
	City            string
	District        string
	PayuniMerID     string
	AccountID       int64
	StoreID         int64 // 店鋪目前的 store-stint surrogate（② re-key；grandfather 期間 = AccountID）
	StoreCode       int64
	CanAcceptOrders bool
	// CanManageProducts 是店家能不能管理商品（營運中且沒有停權，不看 PayUni）。
	CanManageProducts bool
}
