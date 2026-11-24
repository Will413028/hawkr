package intake

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
)

// Blocked 回報收件人（self）與任何一位 actor 之間是否有封鎖，任一方向都算：買家與買家（IsBlockedBetween 本身雙向）、買家與店（買家封鎖店，或店封鎖買家）。
// 店與店之間沒有封鎖。沒有 actor 或 self 是零值（參數裡已有帳號、沒有 actor 的型別）時不問任何人。呼叫端在已鎖定收件人的 transaction 內呼叫。
func Blocked(ctx context.Context, r registry.Resolver, self registry.Principal, actors []registry.Actor) (bool, error) {
	if self.ID <= 0 {
		return false, nil
	}
	for _, a := range actors {
		blocked, err := blockedBetween(ctx, r, self, a.Principal)
		if err != nil || blocked {
			return blocked, err
		}
	}
	return false, nil
}

func blockedBetween(ctx context.Context, r registry.Resolver, self, other registry.Principal) (bool, error) {
	switch {
	case self.Side == model.AudienceBuyer && other.Side == model.AudienceBuyer:
		return r.BuyersBlockedBetween(ctx, self.ID, other.ID)
	case self.Side == model.AudienceBuyer && other.Side == model.AudienceStore:
		return eitherBlocks(ctx, r, self.ID, other.ID)
	case self.Side == model.AudienceStore && other.Side == model.AudienceBuyer:
		return eitherBlocks(ctx, r, other.ID, self.ID)
	case self.Side == model.AudienceStore && other.Side == model.AudienceStore:
		return false, nil
	}
	return false, fmt.Errorf("封鎖檢查的身分面 %q、%q 不合法", self.Side, other.Side)
}

func eitherBlocks(ctx context.Context, r registry.Resolver, buyerID, storeID int64) (bool, error) {
	byBuyer, err := r.BuyerBlocksStore(ctx, buyerID, storeID)
	if err != nil || byBuyer {
		return byBuyer, err
	}
	return r.StoreBlocksBuyer(ctx, storeID, buyerID)
}
