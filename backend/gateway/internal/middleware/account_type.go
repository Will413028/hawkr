package middleware

// Account Type Code 值定義（對應 API 設計規範 §4）
const (
	AccountTypeBuyer          = 1 // 買家
	AccountTypeSeller         = 2 // 賣家
	AccountTypeSellerAndBuyer = 3 // 同時擁有買家與賣家權限
	AccountTypeAdmin          = 4 // 管理員
)

// IsBuyer 檢查帳號類型是否涵蓋買家權限（BUYER 或 SELLER_AND_BUYER）
func IsBuyer(code int) bool {
	return code == AccountTypeBuyer || code == AccountTypeSellerAndBuyer
}

// IsSeller 檢查帳號類型是否涵蓋賣家權限（SELLER 或 SELLER_AND_BUYER）
func IsSeller(code int) bool {
	return code == AccountTypeSeller || code == AccountTypeSellerAndBuyer
}

// IsAdmin 檢查帳號類型是否為管理員
func IsAdmin(code int) bool {
	return code == AccountTypeAdmin
}
