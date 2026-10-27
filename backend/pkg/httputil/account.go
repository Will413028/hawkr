package httputil

import (
	"strconv"

	"github.com/Will413028/hawkr/go-pkg/identity"
)

// ParseAccountID 解析 X-Account-Id header 值為 int64。
// 空字串、零值、非法格式皆回傳 (0, false)。
//
// 只用於真正的登入身份（X-Account-Id）。store/buyer 語意的 header 請用
// ParseStoreID / ParseBuyerID——它們回傳具名型別，接錯編譯不過。
func ParseAccountID(headerVal string) (int64, bool) {
	id, err := strconv.ParseInt(headerVal, 10, 64)
	if err != nil || id == 0 {
		return 0, false
	}
	return id, true
}

// ParseAccountType 解析 X-Account-Type header 值為 int32。
// 空字串或非法格式回傳 0。
func ParseAccountType(headerVal string) int32 {
	v, _ := strconv.ParseInt(headerVal, 10, 32)
	return int32(v)
}

// ParseStoreID 解析 X-Store-Id header 值為 identity.StoreID。
// Gateway 已把登入 account_id 解析成現任 store_id 才注入這個 header，
// 所以這裡拿到的是店舖代理鍵，不是登入身份——回傳具名型別，讓下游接錯編譯不過。
// 空字串、零值、非法格式皆回傳 (0, false)。
func ParseStoreID(headerVal string) (identity.StoreID, bool) {
	id, err := strconv.ParseInt(headerVal, 10, 64)
	if err != nil || id == 0 {
		return 0, false
	}
	return identity.StoreID(id), true
}

// ParseBuyerID 解析 X-Buyer-Id header 值為 identity.BuyerID。
// 同 ParseStoreID：gateway 已解析成現任 buyer_id，不是登入 account_id。
// 空字串、零值、非法格式皆回傳 (0, false)。
func ParseBuyerID(headerVal string) (identity.BuyerID, bool) {
	id, err := strconv.ParseInt(headerVal, 10, 64)
	if err != nil || id == 0 {
		return 0, false
	}
	return identity.BuyerID(id), true
}
