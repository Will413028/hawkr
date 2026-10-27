package usecase

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/revalidate"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

// governanceSuspendedRedisValue is the seller:suspended:<accountId> Redis
// value written for sellers.status='suspended_governance' (46 §4.2). It is
// deliberately NOT a member of the CodeMeta SellerSuspensionReason enum
// (業務狀態碼.md) — that enum is scoped to seller_suspensions.reason_code,
// and the governance path never writes that table (it derives from
// seller_violations instead, see Recompute below). This is a Redis-only
// sentinel so downstream (Gateway header) can tell the suspension source
// apart from an admin reason code; gating itself only checks key existence.
const governanceSuspendedRedisValue = "SUSPENDED_GOVERNANCE"

// sellerStatusRecomputer is the 32 §13.3 / 44 §13.2 SSOT convergence point:
// sellers.status is a single column with multiple write sources (admin manual
// suspension, governance full_suspension, self/admin deletion). Every path
// that used to write the column directly must instead call Recompute, which
// re-derives the value from precedence closed > suspended >
// suspended_governance > active and writes it only if it changed. This is
// what stops the governance cron (§6.2) from clobbering an active admin
// suspension when the governance penalty separately expires.
//
// It is also the SSOT for propagating that status to the Redis
// seller:suspended:<accountId> marker (SyncRedis, below) — P1 fix
// (2026-07-13): governance full_suspension recomputed sellers.status but
// never touched Redis (that logic previously lived only in
// seller_suspension.go's admin Suspend/Lift), so the Gateway capability layer
// never BLOCKed governance-suspended sellers. Folding both concerns into this
// one type means every current and future caller of Recompute gets Redis
// propagation "for free" by also calling SyncRedis post-commit — see doc
// comment on SyncRedis for why that call must stay outside the transaction.
type sellerStatusRecomputer struct {
	sellerRepo     domain.SellerRepository
	suspensionRepo domain.SellerSuspensionRepository
	violationRepo  domain.SellerViolationRepository
	redis          SellerSuspensionRedis
	revalidate     *revalidate.Client
}

func newSellerStatusRecomputer(
	sellerRepo domain.SellerRepository,
	suspensionRepo domain.SellerSuspensionRepository,
	violationRepo domain.SellerViolationRepository,
	redis SellerSuspensionRedis,
	revalidateClient *revalidate.Client,
) *sellerStatusRecomputer {
	return &sellerStatusRecomputer{
		sellerRepo:     sellerRepo,
		suspensionRepo: suspensionRepo,
		violationRepo:  violationRepo,
		redis:          redis,
		revalidate:     revalidateClient,
	}
}

// Recompute derives and (if changed) writes sellers.status for accountID's
// current stint. Must run inside the caller's transaction — all three repo
// dependencies resolve their DBTX from ctx (pkg/database.GetDBTX), so the read
// of suspension/violation state is consistent with whatever the caller just
// wrote earlier in the same tx.
//
// closed is terminal and is never overwritten here (no un-closing path) —
// callers that need to close a stint use SellerRepository.UpdateStatusByStoreID
// directly (see DeleteSellerAccountUseCase), not this helper.
func (r *sellerStatusRecomputer) Recompute(ctx context.Context, accountID int64) error {
	seller, err := r.sellerRepo.GetByAccountID(ctx, accountID)
	if err != nil {
		return err
	}
	if seller.IsClosed() {
		return nil
	}

	newStatus := domain.SellerStatusActive

	hasSuspension, err := r.suspensionRepo.HasActiveByAccountID(ctx, accountID)
	if err != nil {
		return err
	}
	switch {
	case hasSuspension:
		newStatus = domain.SellerStatusSuspended
	default:
		hasGovernance, err := r.violationRepo.HasActiveFullSuspensionBySeller(ctx, seller.StoreID)
		if err != nil {
			return err
		}
		if hasGovernance {
			newStatus = domain.SellerStatusSuspendedGovernance
		}
	}

	if seller.Status == newStatus {
		return nil
	}
	return r.sellerRepo.UpdateStatusCurrentByAccountID(ctx, accountID, newStatus, time.Now())
}

// SyncRedis mirrors sellers.status to the seller:suspended:<accountId> Redis
// marker (46 §4.2) for accountID's current stint.
//
// Callers must invoke this AFTER the transaction that ran Recompute has
// committed, using a non-tx ctx — NOT from inside the same WithTx closure as
// Recompute. Redis here is a best-effort accelerator (§22: no background
// reconciliation), so writing it before the surrounding transaction is
// guaranteed to land would let a later rollback in that same transaction
// leave a stale marker with no self-healing path. This mirrors the pre-P1
// pattern in seller_suspension.go, which always called SetSuspended/
// ClearSuspended only after txMgr.WithTx returned nil.
//
// Always self-heals regardless of whether Recompute actually changed the
// column — an idempotent hit (e.g. duplicate Suspend call, already-lifted
// Lift call) still needs the marker refreshed. Re-reads sellers.status
// (rather than taking a status parameter) precisely so it can be called this
// way, decoupled from whatever Recompute call (if any) preceded it in the
// same request.
//
// This is the single point that decides the Redis value for every status:
//   - 'suspended' (admin manual)  → the active seller_suspensions.reason_code
//     (front-end-visible reason; §5.1 CodeMeta SellerSuspensionReason)
//   - 'suspended_governance'      → the fixed sentinel governanceSuspendedRedisValue
//     (no seller_suspensions row exists for this path, so there is no
//     reason_code to read)
//   - 'active' / 'deleted' / anything else → Clear
//
// Best-effort: failures to read seller/suspension state are logged and
// swallowed (matches SellerSuspensionRedis.SetSuspended/ClearSuspended, which
// already log-and-swallow their own Redis errors) — this must never fail the
// caller's already-committed business operation.
func (r *sellerStatusRecomputer) SyncRedis(ctx context.Context, accountID int64) {
	// 兩個副作用都不需要時才略過查詢。原本的守衛只看 redis，那在這支還只做一件事
	// 的時候是對的；現在「沒有 Redis」不再蘊含「不必通知前端」，只看 redis 會把
	// 失效整個吞掉。
	if r.redis == nil && !r.revalidate.Enabled() {
		return
	}
	seller, err := r.sellerRepo.GetByAccountID(ctx, accountID)
	if err != nil {
		logger.Warn(ctx, "seller_status_recompute: SyncRedis GetByAccountID failed",
			zap.Int64("account_id", accountID), zap.Error(err))
		return
	}
	// 前端快取失效與下面的 Redis 同步是同一件事的兩面：status 變了，我們自己的
	// blacklist 要更新，website 依 seller tag 快取的商家頁與推薦列表也要失效。
	// 掛在這裡而不是各呼叫端，是因為這支就是 status 變更的收攏點——Recompute 的
	// 6 個呼叫點全部經過它。散在呼叫端就會漏，第一版正是只接到 3 個。
	// 用已查到的 seller 組 tag，省一次往返。
	r.bustSellerCache(ctx, accountID, seller.StoreID, seller.StoreCode)
	if r.redis == nil {
		return
	}
	switch seller.Status {
	case domain.SellerStatusSuspended:
		reasonCode := ""
		active, err := r.suspensionRepo.GetActiveByAccountID(ctx, accountID)
		if err != nil {
			logger.Warn(ctx, "seller_status_recompute: SyncRedis GetActiveByAccountID failed",
				zap.Int64("account_id", accountID), zap.Error(err))
		} else if active != nil {
			reasonCode = active.ReasonCode
		}
		r.redis.SetSuspended(ctx, accountID, reasonCode)
	case domain.SellerStatusSuspendedGovernance:
		r.redis.SetSuspended(ctx, accountID, governanceSuspendedRedisValue)
	default:
		r.redis.ClearSuspended(ctx, accountID)
	}
}

// bustSellerCache 通知 website 失效這個賣家的快取。best-effort，與 Redis 同步
// 同層：送不出去不該讓已完成的狀態變更變成失敗。
func (r *sellerStatusRecomputer) bustSellerCache(ctx context.Context, accountID, storeID, storeCode int64) {
	if accountID == 0 || !r.revalidate.Enabled() {
		return
	}
	r.revalidate.BustAsync(ctx, sellerCacheTagsFrom(ctx, accountID, storeID, storeCode))
}
