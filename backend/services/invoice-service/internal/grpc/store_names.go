package grpc

import (
	"context"

	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// ADM 工作台的「識別對象」欄（#3017）
//
// 🔴 這一欄在 2026-09-02 之前**全服務零賦值**：proto 有 `store_name`、gateway 照實
// 透傳、只有生產者那一端從來沒填 ⇒ A02／A04 的每一列都只有 18 位數字 store_id，
// 而 A04 的搜尋框 placeholder 寫著「搜尋店名 / store_id」——依店名搜尋是死的。
//
// 為什麼測試沒抓到：gateway 的契約測試 fixture 是**手寫**的
// （`invoice_preservedata_e2e_test.go` 自己填了 `StoreName: "測試商店"`），
// 於是驗的是「gateway 會不會透傳」而不是「這個欄位會不會有值」。生產者永遠回空、
// 測試永遠有值，兩邊都綠。
// ---------------------------------------------------------------------------

// SellerNameResolver 是店名的出向面（實作在 client/seller）。
//
// 介面宣告在消費端：本層只需要「給我一批 store_id、還我店名」，
// 不需要知道 seller-service 的 SellerMeta 還有哪些欄位。
type SellerNameResolver interface {
	BatchGetSellerMeta(ctx context.Context, storeIDs []int64) (map[int64]string, error)
}

// resolveStoreNames 批次取店名。
//
// 🔴 **失敗一律降級成空 map，不中止列表**：店名是識別欄不是資料本身，
// 取不到就退回 store_id——那正是修這件事之前的畫面，不會更差。
// 做成 fail-loud 會讓 seller-service 一次抖動就把整個稅籍審核佇列打不開，
// 而佇列本身的資料完全健康。
//
// ⚠️ 降級要**留痕**：靜默降級會讓「店名又全空了」在下一次發生時無從診斷，
// 而那正是本票的原始症狀。
func (s *InvoiceServer) resolveStoreNames(ctx context.Context, storeIDs []int64) map[int64]string {
	if s.sellerNames == nil || len(storeIDs) == 0 {
		return map[int64]string{}
	}
	names, err := s.sellerNames.BatchGetSellerMeta(ctx, dedupeIDs(storeIDs))
	if err != nil {
		if s.log == nil {
			return map[int64]string{}
		}
		s.log.Warn("invoice.query.store_names_unavailable",
			zap.Int("store_count", len(storeIDs)),
			zap.String("degrade", "store_name 退回空字串，ADM 只看得到 store_id"),
			zap.Error(err))
		return map[int64]string{}
	}
	return names
}

// dedupeIDs 去重。同一個賣家在一頁裡出現多列是常態（A04 一個賣家一列、
// A02 一個賣家一列，但文件列表不是），多送幾個 id 不會錯，只是白費。
func dedupeIDs(in []int64) []int64 {
	seen := make(map[int64]struct{}, len(in))
	out := make([]int64, 0, len(in))
	for _, id := range in {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
