package grpc

import (
	"context"

	"go.uber.org/zap"
)


// SellerNameResolver 是店名的出向面（實作在 client/seller）。
//
// 介面宣告在消費端：本層只需要「給我一批 store_id、還我店名」，
// 不需要知道 seller-service 的 SellerMeta 還有哪些欄位。
type SellerNameResolver interface {
	BatchGetSellerMeta(ctx context.Context, storeIDs []int64) (map[int64]string, error)
}

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
