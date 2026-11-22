package procurement

import (
	"sort"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/candidate"
)

// NewOpenStores 讓測試驗證 seller.ExposableStores 轉成 candidate.OpenStores 的分批與對照。
func NewOpenStores(s SellerStores) candidate.OpenStores { return openStores{sellers: s} }

// ExecutorTypes 讓測試驗證 New 登記了哪些 run 類型的 executor。
func (m *Module) ExecutorTypes() []string {
	var out []string
	for k := range m.dispatcher.Executors {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
