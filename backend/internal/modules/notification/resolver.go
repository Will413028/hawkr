package notification

import (
	"context"
	"slices"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Resolver 是 worker 端解析收件人、封鎖檢查與追蹤者展開要問的地方。
type Resolver = registry.Resolver

type resolver struct {
	buyers    *buyer.Module
	stores    *seller.Module
	reporters CaseReporters
}

// CaseReporters 是 moderation 案件的檢舉人讀取（*moderation.Module 與 worker 用的 *moderation.CaseReader 都符合）。
type CaseReporters interface {
	CaseReporters(ctx context.Context, caseID uuid.UUID, after *moderation.ReportCursor, size int32) (moderation.ReporterPage, error)
}

// NewResolver 把 buyer、seller 與 moderation 的檢舉人讀取組成 worker 端的收件人解析；reporters 只有結案的檢舉人 fan-out 用到。
func NewResolver(b *buyer.Module, s *seller.Module, reporters CaseReporters) Resolver {
	return resolver{buyers: b, stores: s, reporters: reporters}
}

func (r resolver) BuyerAccounts(ctx context.Context, buyerIDs []int64) (map[int64]int64, error) {
	return r.buyers.ResolvePrincipals(ctx, buyerIDs)
}

// StoreAccounts 逐店讀 seller 的 Store（含已關閉的 stint）：不存在與已關閉的店不在結果裡；其餘錯誤原樣回傳（讓 job 重試）。
func (r resolver) StoreAccounts(ctx context.Context, storeIDs []int64) (map[int64]int64, error) {
	out := make(map[int64]int64, len(storeIDs))
	for _, id := range slices.Compact(slices.Sorted(slices.Values(storeIDs))) {
		st, err := r.stores.Store(ctx, id)
		if apperr.KindOf(err) == apperr.KindNotFound {
			continue
		}
		if err != nil {
			return nil, err
		}
		if st.Status == seller.Closed {
			continue
		}
		out[id] = st.AccountID
	}
	return out, nil
}

func (r resolver) BuyersBlockedBetween(ctx context.Context, buyerA, buyerB int64) (bool, error) {
	return r.buyers.IsBlockedBetween(ctx, buyerA, buyerB)
}

func (r resolver) BuyerBlocksStore(ctx context.Context, buyerID, storeID int64) (bool, error) {
	return r.buyers.IsBlockingStore(ctx, buyerID, storeID)
}

func (r resolver) StoreBlocksBuyer(ctx context.Context, storeID, buyerID int64) (bool, error) {
	return r.stores.StoreBlocksBuyer(ctx, storeID, buyerID)
}

func (r resolver) StoreFollowers(ctx context.Context, storeID, afterBuyerID int64, limit int, followedBy time.Time) ([]registry.FollowerRef, error) {
	rows, err := r.buyers.ListLiveFollowerAccounts(ctx, storeID, afterBuyerID, limit, followedBy)
	if err != nil {
		return nil, err
	}
	out := make([]registry.FollowerRef, len(rows))
	for i, f := range rows {
		out[i] = registry.FollowerRef{BuyerID: f.BuyerID, AccountID: f.AccountID}
	}
	return out, nil
}

type storeCodes struct{ stores *seller.Module }

// NewStoreCodes 把 seller 組成讀取店家 store_code 的來源：只有曾經公開過的店有 store_code 可用（StoreCards 的規則），其餘不在結果裡、深連結留空。
func NewStoreCodes(s *seller.Module) StoreCodes { return storeCodes{stores: s} }

func (c storeCodes) StoreCodes(ctx context.Context, storeIDs []int64) (map[int64]int64, error) {
	ids := slices.Compact(slices.Sorted(slices.Values(storeIDs)))
	out := make(map[int64]int64, len(ids))
	for chunk := range slices.Chunk(ids, seller.MaxBatchSize) {
		cards, err := c.stores.StoreCards(ctx, chunk)
		if err != nil {
			return nil, err
		}
		for _, card := range cards {
			out[card.StoreID] = card.StoreCode
		}
	}
	return out, nil
}

// CaseReporters 以 moderation 的 keyset 從頭走到 afterRank 之後再取 limit 位：檢舉列不會被刪（只清說明），名次穩定；一個案件的檢舉人數有限，重走的成本是已走過的列數。
func (r resolver) CaseReporters(ctx context.Context, caseID uuid.UUID, afterRank int64, limit int) ([]registry.ReporterRef, error) {
	var out []registry.ReporterRef
	var after *moderation.ReportCursor
	var rank int64
	for {
		page, err := r.reporters.CaseReporters(ctx, caseID, after, registry.MaxFollowerPage)
		if err != nil {
			return nil, err
		}
		for _, it := range page.Items {
			rank++
			if rank <= afterRank {
				continue
			}
			out = append(out, registry.ReporterRef{Rank: rank, ReportID: it.ReportID, BuyerID: it.BuyerID, StoreID: it.StoreID})
			if len(out) == limit {
				return out, nil
			}
		}
		if page.Next == nil {
			return out, nil
		}
		after = page.Next
	}
}
