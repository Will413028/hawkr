package notification

import (
	"context"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// Resolver 是 worker 端解析收件人、封鎖檢查與追蹤者展開要問的地方。
type Resolver = registry.Resolver

type resolver struct {
	buyers *buyer.Module
	stores *seller.Module
}

// NewResolver 把 buyer 與 seller 組成 worker 端的收件人解析。
func NewResolver(b *buyer.Module, s *seller.Module) Resolver { return resolver{buyers: b, stores: s} }

func (r resolver) BuyerAccounts(ctx context.Context, buyerIDs []int64) (map[int64]int64, error) {
	return r.buyers.ResolvePrincipals(ctx, buyerIDs)
}

func (r resolver) StoreAccounts(ctx context.Context, storeIDs []int64) (map[int64]int64, error) {
	return r.stores.ResolvePrincipals(ctx, storeIDs)
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

func (r resolver) StoreFollowers(ctx context.Context, storeID, afterBuyerID int64, limit int) ([]registry.FollowerRef, error) {
	rows, err := r.buyers.ListLiveFollowerAccounts(ctx, storeID, afterBuyerID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]registry.FollowerRef, len(rows))
	for i, f := range rows {
		out[i] = registry.FollowerRef{BuyerID: f.BuyerID, AccountID: f.AccountID}
	}
	return out, nil
}
