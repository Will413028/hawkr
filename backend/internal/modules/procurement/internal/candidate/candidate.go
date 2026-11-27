// Package candidate 決定哪些店可以成為候選（A9-D19 S9）：候選只收 OPEN 的店，也就是 seller 判定此刻可以公開曝光的店。
// 候選不存店名快照；沒公開過或已暫停的店名不能被讀出，所以名稱只在讀取時由 StoreCard 組裝。
package candidate

import (
	"context"
	"fmt"
	"uuid"
)

// OpenStores 回傳 storeIDs 之中此刻是 OPEN 的店（不存在、暫停、關閉的略過）；讀取失敗回錯誤（fail-closed）。
// 由根套件以 seller.ExposableStores 實作並依 seller 的批次上限分批。
type OpenStores interface {
	OpenStoreIDs(ctx context.Context, storeIDs []int64) (map[int64]struct{}, error)
}

// 候選搜尋的上限（A9-D29(6)，推論值）：最多留 MaxStores 家店，最多翻 MaxPages 頁。
const (
	MaxStores = 20
	MaxPages  = 5
)

// Query 是搜尋的輸入：分類 ID 與關鍵字二擇一；CategoryID 是 uuid.Nil() 表示用關鍵字。
type Query struct {
	CategoryID uuid.UUID
	Keyword    string
}

// Hit 是搜尋命中的一件上架商品：SourceID 是商品 ID。
type Hit struct {
	ProductID uuid.UUID
	StoreID   int64
}

// Cursor 是搜尋翻頁的不透明游標，由 Searcher 編碼與解讀；空字串是第一頁（請求）或沒有下一頁（回應）。
type Cursor string

// Page 是搜尋的一頁命中（依搜尋的順序）；Next 是下一頁的游標，空字串表示後面沒有更多頁。
type Page struct {
	Hits []Hit
	Next Cursor
}

// Searcher 依序取商品搜尋的一頁（keyset 翻頁，翻頁不重不漏）：after 是上一頁的 Next，第一頁給空字串；任一頁失敗回錯誤。
// 由根套件以 catalog 的公開商品列表實作。
type Searcher interface {
	Page(ctx context.Context, q Query, after Cursor) (Page, error)
}

// BlockedStores 回傳 storeIDs 之中封鎖了這位買家的店；讀取失敗回錯誤（fail-closed）。
// 由根套件以 seller.StoresBlockingBuyer 實作並依 seller 的批次上限分批。
type BlockedStores interface {
	StoreIDsBlockingBuyer(ctx context.Context, buyerID int64, storeIDs []int64) (map[int64]struct{}, error)
}

// Search 翻頁收集候選店：每頁先留第一次出現的店（一店一筆，留第一件商品），再經 KeepOpen 與封鎖過濾，
// 累計滿 MaxStores 家、最後一頁或翻滿 MaxPages 頁就停；順序就是排名。任一頁或任一過濾失敗，整次搜尋失敗。
func Search(ctx context.Context, s Searcher, open OpenStores, blocks BlockedStores, buyerID int64, q Query) ([]Hit, error) {
	seen := make(map[int64]struct{})
	var out []Hit
	var after Cursor
	for page := 1; page <= MaxPages; page++ {
		pg, err := s.Page(ctx, q, after)
		if err != nil {
			return nil, fmt.Errorf("candidate：搜尋第 %d 頁：%w", page, err)
		}
		var fresh []Hit
		for _, h := range pg.Hits {
			if _, dup := seen[h.StoreID]; dup {
				continue
			}
			seen[h.StoreID] = struct{}{}
			fresh = append(fresh, h)
		}
		kept, err := KeepOpen(ctx, open, fresh)
		if err != nil {
			return nil, err
		}
		if kept, err = dropBlocking(ctx, blocks, buyerID, kept); err != nil {
			return nil, err
		}
		out = append(out, kept...)
		if len(out) >= MaxStores {
			return out[:MaxStores], nil
		}
		if pg.Next == "" {
			break
		}
		after = pg.Next
	}
	return out, nil
}

func dropBlocking(ctx context.Context, blocks BlockedStores, buyerID int64, hits []Hit) ([]Hit, error) {
	if len(hits) == 0 {
		return nil, nil
	}
	ids := make([]int64, len(hits))
	for i, h := range hits {
		ids[i] = h.StoreID
	}
	blocked, err := blocks.StoreIDsBlockingBuyer(ctx, buyerID, ids)
	if err != nil {
		return nil, fmt.Errorf("candidate：確認店家有沒有封鎖買家：%w", err)
	}
	out := make([]Hit, 0, len(hits))
	for _, h := range hits {
		if _, isBlocked := blocked[h.StoreID]; !isBlocked {
			out = append(out, h)
		}
	}
	return out, nil
}

// KeepOpen 只留下 OPEN 的店，維持輸入順序；店不在 OPEN 的就不會成為候選。
// 同一家店重複出現只留第一筆（candidates 的 (candidate_set_id, store_id) 唯一）。
func KeepOpen(ctx context.Context, open OpenStores, found []Hit) ([]Hit, error) {
	if len(found) == 0 {
		return nil, nil
	}
	seen := make(map[int64]struct{}, len(found))
	unique := make([]Hit, 0, len(found))
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
	out := make([]Hit, 0, len(unique))
	for _, f := range unique {
		if _, isOpen := ok[f.StoreID]; isOpen {
			out = append(out, f)
		}
	}
	return out, nil
}
