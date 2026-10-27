package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/blocklist"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var ErrCannotBlockSelf = errors.New("cannot block self")

// ErrUnresolvedBlockPrincipal is the fail-closed answer when at least one side
// of a block request is not a live buyer_id and not a live store_id (#2789).
//
// The self-block guard below compares two login principals. Both role services
// answer "I do not know this id" by echoing the input straight back, so before
// this error existed the guard silently degraded into comparing two
// *unverified* raw ids — two different ids belonging to the same human compare
// as different, and the self-block went through. Refusing is the only answer
// that keeps the guard meaning what it says.
var ErrUnresolvedBlockPrincipal = pkgerrors.NewInvalidInput("無法辨識的使用者 ID")

type BlockUseCase struct {
	blockRepo    domain.BlockRepository
	sellerClient SellerClient
	buyerClient  BuyerClient
	rdb          *redis.Client
	tsid         func() int64
}

func NewBlockUseCase(blockRepo domain.BlockRepository, sellerClient SellerClient, buyerClient BuyerClient, rdb *redis.Client, tsid func() int64) *BlockUseCase {
	return &BlockUseCase{blockRepo: blockRepo, sellerClient: sellerClient, buyerClient: buyerClient, rdb: rdb, tsid: tsid}
}

func (uc *BlockUseCase) BlockUser(ctx context.Context, blockerID, blockedID int64) error {
	// 同一個原始 id 不需要任何反解析就知道是自己封鎖自己。放在最前面，讓這個
	// 最常見的情形不受下面身分反解析成敗的影響。
	if blockerID == blockedID {
		return ErrCannotBlockSelf
	}
	blockerPrincipal, blockerResolved, err := uc.resolvePrincipal(ctx, blockerID)
	if err != nil {
		return fmt.Errorf("resolve blocker principal: %w", err)
	}
	blockedPrincipal, blockedResolved, err := uc.resolvePrincipal(ctx, blockedID)
	if err != nil {
		return fmt.Errorf("resolve blocked principal: %w", err)
	}
	if !blockerResolved || !blockedResolved {
		pkglogger.Warn(ctx, "block principal unresolved; refusing to block (fail-closed)",
			zap.Int64("blockerID", blockerID), zap.Bool("blockerResolved", blockerResolved),
			zap.Int64("blockedID", blockedID), zap.Bool("blockedResolved", blockedResolved))
		return ErrUnresolvedBlockPrincipal
	}
	if blockerPrincipal == blockedPrincipal {
		return ErrCannotBlockSelf
	}
	if _, err := uc.blockRepo.Create(ctx, uc.tsid(), blockerID, blockedID); err != nil {
		return err
	}
	// Best-effort cache write. Errors logged via blocklist metrics.
	_ = blocklist.Add(ctx, uc.rdb, blockerID, blockedID)
	return nil
}

// resolvePrincipal maps one social user id to its login principal, trying the
// seller axis first and then the buyer axis — the same order, and the same
// chaining, the inline code used before #2789.
//
// The chaining is load-bearing and is preserved exactly: a social user id is
// either a store_id or a buyer_id and the caller does not know which, so the
// seller axis is asked first and whatever comes out is fed to the buyer axis.
// Under the old ResolvePrincipal that worked because an id the seller axis did
// not recognise was echoed back unchanged and therefore reached the buyer axis
// intact; ResolvePrincipalStrict makes the same fall-through explicit via
// Found=false, so the value only advances when an axis actually claims it.
//
// resolved reports whether ANY axis claimed the id (sticky across both axes):
// a store_id that the seller axis resolves stays resolved even though the
// buyer axis then answers "not mine" about the resulting account_id. resolved
// == false means neither role service has a live row for this id — the state
// that used to be indistinguishable from a successful resolution.
//
// A returned error means an axis could not answer at all (transport failure or
// a non-zero business code). That is deliberately not folded into
// resolved=false: "not mine" is a fact, "I am down" is not.
func (uc *BlockUseCase) resolvePrincipal(ctx context.Context, id int64) (principal int64, resolved bool, err error) {
	principal = id
	if uc.sellerClient != nil {
		res, rerr := uc.sellerClient.ResolvePrincipalStrict(ctx, principal)
		if rerr != nil {
			return 0, false, fmt.Errorf("seller axis: %w", rerr)
		}
		if res.Found {
			principal, resolved = res.AccountID, true
		}
	}
	if uc.buyerClient != nil {
		res, rerr := uc.buyerClient.ResolvePrincipalStrict(ctx, principal)
		if rerr != nil {
			return 0, false, fmt.Errorf("buyer axis: %w", rerr)
		}
		if res.Found {
			principal, resolved = res.AccountID, true
		}
	}
	return principal, resolved, nil
}

// UnblockUser deletes the row (idempotent) and removes from Redis cache.
func (uc *BlockUseCase) UnblockUser(ctx context.Context, blockerID, blockedID int64) error {
	if err := uc.blockRepo.Delete(ctx, blockerID, blockedID); err != nil {
		return err
	}
	_ = blocklist.Remove(ctx, uc.rdb, blockerID, blockedID)
	return nil
}

// ListBlocks returns the blocker's block list (DB read; Redis is
// hot-path-only and not used here).
func (uc *BlockUseCase) ListBlocks(ctx context.Context, blockerID int64, limit, offset int64) ([]*gen.UserBlock, error) {
	if limit <= 0 {
		limit = 20
	}
	return uc.blockRepo.ListByBlocker(ctx, blockerID, limit, offset)
}
