package kyc

import (
	"context"
	"fmt"
	"strings"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/seal"
)

// protector 計算統編的 blind index（防詐保存與處分比對；平台不加密保存個資，Stripe 計畫 D22）。
type protector struct {
	mac *seal.MAC
}

// hashes 以金鑰環裡的每一把金鑰計算 value 的 blind index，查詢時比對全部。
func (p protector) hashes(input []byte) [][]byte {
	all := p.mac.SumAll(input)
	out := make([][]byte, 0, len(all))
	for _, h := range all {
		out = append(out, h.Sum)
	}
	return out
}

func taxInput(id string) []byte { return model.IdentityInput(model.IdentityTaxID, id) }

// CheckKeys 檢查資料庫參照的每一把金鑰 ID 都在設定的金鑰環裡；缺的 ID 一併列在錯誤裡（金鑰 ID 不是機密，值不會出現）。
func (s *Service) CheckKeys(ctx context.Context) error {
	rows, err := store.New(s.d.DB.Conn(ctx)).ReferencedKeyIDs(ctx)
	if err != nil {
		return fmt.Errorf("seller：讀取資料庫參照的金鑰：%w", err)
	}
	var missing []string
	for _, r := range rows {
		if !s.p.mac.Has(r.KeyID) {
			missing = append(missing, r.Kind+":"+r.KeyID)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("seller：資料庫參照的金鑰不在設定的金鑰環裡（KYC_HASH_KEYS）：%s", strings.Join(missing, ", "))
	}
	return nil
}
