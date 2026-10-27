package infrastructure

import (
	"context"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	pkgredis "github.com/Will413028/hawkr/go-pkg/redis"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
)

// 32 §13 / 46 §4.2: admin suspension state propagation via Redis
// (seller:suspended:<accountId>). Write authority = seller-service — this is
// the post-commit writer invoked right after the admin suspend/lift
// transaction commits. Best-effort: a write failure only logs (the
// CheckoutValidate / product ACTIVE transition backstops at 46 §6 are the
// fail-closed correctness net, not this cache write — §22 no reconciliation).
type sellerSuspensionRedis struct {
	rdb *redis.Client
}

// NewSellerSuspensionRedis constructs the adapter. nil rdb is tolerated
// (Redis unavailable at startup) — writes become no-ops, matching the
// existing fail-graceful posture of this service's other Redis usages.
func NewSellerSuspensionRedis(rdb *redis.Client) usecase.SellerSuspensionRedis {
	return &sellerSuspensionRedis{rdb: rdb}
}

func (c *sellerSuspensionRedis) SetSuspended(ctx context.Context, accountID int64, reasonCode string) {
	if c.rdb == nil {
		return
	}
	if err := pkgredis.SetSellerSuspended(ctx, c.rdb, accountID, reasonCode); err != nil {
		pkglogger.Warn(ctx, "seller_suspension_redis: SetSellerSuspended failed",
			zap.Int64("account_id", accountID), zap.Error(err))
	}
}

func (c *sellerSuspensionRedis) ClearSuspended(ctx context.Context, accountID int64) {
	if c.rdb == nil {
		return
	}
	if err := pkgredis.ClearSellerSuspended(ctx, c.rdb, accountID); err != nil {
		pkglogger.Warn(ctx, "seller_suspension_redis: ClearSellerSuspended failed",
			zap.Int64("account_id", accountID), zap.Error(err))
	}
}
