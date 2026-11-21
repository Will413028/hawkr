package procurement

import "github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/candidate"

// NewOpenStores 讓測試驗證 seller.ExposableStores 轉成 candidate.OpenStores 的分批與對照。
func NewOpenStores(s SellerStores) candidate.OpenStores { return openStores{sellers: s} }
