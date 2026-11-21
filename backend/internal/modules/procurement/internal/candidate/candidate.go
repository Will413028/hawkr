// Package candidate 決定哪些店可以成為候選（A9-D19 S9）：候選只收 OPEN 的店，也就是 seller 判定此刻可以公開曝光的店。
// 候選不存店名快照；沒公開過或已暫停的店名不能被讀出，所以名稱只在讀取時由 StoreCard 組裝。
package candidate

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
)

// OpenStores 回傳 storeIDs 之中此刻是 OPEN 的店（不存在、暫停、關閉的略過）；讀取失敗回錯誤（fail-closed）。
// 由根套件以 seller.ExposableStores 實作並依 seller 的批次上限分批。
type OpenStores interface {
	OpenStoreIDs(ctx context.Context, storeIDs []int64) (map[int64]struct{}, error)
}

// Found 是搜尋找到的一家店與給 LLM 看的資料。
type Found struct {
	model.LLMCandidate
}

// KeepOpen 只留下 OPEN 的店，維持輸入順序；店不在 OPEN 的就不會成為候選。
// 同一家店重複出現只留第一筆（candidates 的 (candidate_set_id, store_id) 唯一）。
func KeepOpen(ctx context.Context, open OpenStores, found []Found) ([]Found, error) {
	if len(found) == 0 {
		return nil, nil
	}
	seen := make(map[int64]struct{}, len(found))
	unique := make([]Found, 0, len(found))
	ids := make([]int64, 0, len(found))
	for _, f := range found {
		if _, dup := seen[f.StoreID]; dup {
			continue
		}
		seen[f.StoreID] = struct{}{}
		unique = append(unique, f)
		ids = append(ids, f.StoreID)
	}
	ok, err := open.OpenStoreIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("candidate：確認店家是否營業中：%w", err)
	}
	out := make([]Found, 0, len(unique))
	for _, f := range unique {
		if _, isOpen := ok[f.StoreID]; isOpen {
			out = append(out, f)
		}
	}
	return out, nil
}
