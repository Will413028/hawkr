package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Will413028/hawkr/go-pkg/database"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/revalidate"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

// SellerSuspensionRedis is the port for propagating admin suspension state to
// Redis (46 §4.2, key `seller:suspended:<accountId>`). Both methods are
// best-effort by design (implementations log-and-swallow) — Redis here is an
// accelerator, not the source of truth; §22 explicitly forbids background
// reconciliation for it, so a write failure must not fail the surrounding
// suspend/lift transaction. The real correctness net is the fail-closed
// CheckoutValidate / product ACTIVE transition backstop (46 §6), which reads
// sellers.status, not this cache.
type SellerSuspensionRedis interface {
	SetSuspended(ctx context.Context, accountID int64, reasonCode string)
	ClearSuspended(ctx context.Context, accountID int64)
}

// SellerSuspensionUseCase implements admin manual suspension (32 §13 / 44 §13.2):
// suspend/lift/list/get-active. Distinct from SellerViolationUseCase's
// governance path — different trigger source, different lifecycle (manual
// lift only, no cron), same sellers.status convergence point (recompute).
type SellerSuspensionUseCase struct {
	repo       domain.SellerSuspensionRepository
	sellerRepo domain.SellerRepository
	outboxRepo domain.SellerDeletedOutboxRepository
	recompute  *sellerStatusRecomputer
	txMgr      database.TxManager
	tsid       func() int64
}

// NewSellerSuspensionUseCase constructs the usecase. redis is forwarded into
// recompute — this usecase no longer calls SellerSuspensionRedis directly
// (Suspend/Lift used to call SetSuspended/ClearSuspended inline; both now go
// through recompute.SyncRedis post-commit, the P1-fix SSOT — see
// seller_status_recompute.go).
func NewSellerSuspensionUseCase(
	repo domain.SellerSuspensionRepository,
	sellerRepo domain.SellerRepository,
	violationRepo domain.SellerViolationRepository,
	outboxRepo domain.SellerDeletedOutboxRepository,
	redis SellerSuspensionRedis,
	txMgr database.TxManager,
	tsid func() int64,
	revalidateClient *revalidate.Client,
) *SellerSuspensionUseCase {
	return &SellerSuspensionUseCase{
		repo:       repo,
		sellerRepo: sellerRepo,
		outboxRepo: outboxRepo,
		recompute:  newSellerStatusRecomputer(sellerRepo, repo, violationRepo, redis, revalidateClient),
		txMgr:      txMgr,
		tsid:       tsid,
	}
}

// SuspendResult is Suspend's return: the (new-or-existing) active suspension
// row plus whether this call actually created it (false = idempotent hit on
// an already-active suspension for this account, 32 §13.4).
type SuspendResult struct {
	Suspension       *domain.SellerSuspension
	AlreadySuspended bool
}

// Suspend implements POST /admin/sellers/{sellerId}/suspensions
// (`SuspendSellerForAdmin`): create the suspension row, recompute
// sellers.status, enqueue the must-deliver seller.suspended outbox event, all
// inside one transaction; propagate to Redis after commit.
func (u *SellerSuspensionUseCase) Suspend(ctx context.Context, accountID int64, reasonCode, reasonDetail string, adminID int64) (*SuspendResult, error) {
	// issue #1534：reason_code 全鏈路統一 CodeMeta Value 數字碼（"9601"/"9602"/
	// "9603"），DB/domain/MQ/wire 同一形態直存直傳；#1514 的 Name 輸入相容
	// （雙認正規化）已移除，只認 Value。
	if !domain.IsValidSellerSuspensionReason(reasonCode) {
		return nil, pkgerrors.NewInvalidInput("reason_code '" + reasonCode + "' is not a valid SellerSuspensionReason")
	}

	now := time.Now()
	result := &SuspendResult{}
	err := u.txMgr.WithTx(ctx, func(txCtx context.Context) error {
		seller, err := u.sellerRepo.GetByAccountIDForUpdate(txCtx, accountID)
		if err != nil {
			return err
		}
		if seller.IsClosed() {
			return pkgerrors.NewNotFound("seller not found")
		}

		s := &domain.SellerSuspension{
			ID:                 u.tsid(),
			AccountID:          accountID,
			ReasonCode:         reasonCode,
			ReasonDetail:       reasonDetail,
			Status:             domain.SellerSuspensionActive,
			SuspendedByAdminID: adminID,
			SuspendedAt:        now,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		// uq_seller_suspensions_active 衝突時直接回傳錯誤讓 WithTx rollback — Postgres
		// 在 unique_violation 後會把整個 transaction 標成 aborted（25P02），同一 txCtx
		// 內任何後續 query 都會被拒絕，不能在這裡 catch 了繼續查。改為 tx 外用全新
		// connection 查既有 active row（見下方 errors.As 分支）。
		if err := u.repo.Insert(txCtx, s); err != nil {
			return err
		}

		if err := u.recompute.Recompute(txCtx, accountID); err != nil {
			return err
		}

		row, err := domain.NewLifecycleOutboxRow(u.tsid(), accountID, domain.SellerLifecycleEventSuspended,
			domain.SellerSuspensionOutboxPayload{AccountID: accountID, SuspensionID: s.ID, ReasonCode: reasonCode, StoreID: seller.StoreID}, now)
		if err != nil {
			return err
		}
		if err := u.outboxRepo.Insert(txCtx, row); err != nil {
			return err
		}

		result.Suspension = s
		return nil
	})
	if err != nil {
		var dup *domain.ErrDuplicate
		if errors.As(err, &dup) {
			// Tx already rolled back by WithTx — query on a fresh connection (ctx,
			// not txCtx) for the idempotent "already suspended" response.
			existing, gerr := u.repo.GetActiveByAccountID(ctx, accountID)
			if gerr != nil {
				return nil, gerr
			}
			result.Suspension = existing
			result.AlreadySuspended = true
			u.recompute.SyncRedis(ctx, accountID)
			return result, nil
		}
		return nil, err
	}

	// Post-commit, best-effort: always self-heal the marker regardless of
	// whether this call created the row or hit the idempotent branch.
	u.recompute.SyncRedis(ctx, accountID)

	return result, nil
}

// LiftResult is Lift's return.
type LiftResult struct {
	Suspension    *domain.SellerSuspension
	AlreadyLifted bool
}

// Lift implements POST /admin/sellers/{sellerId}/suspensions/{id}/lift
// (`LiftSellerSuspensionForAdmin`). accountID is the path {sellerId} —
// verified against the suspension's own AccountID to reject a mismatched
// (id, sellerId) pair (defense in depth against an admin fat-fingering the
// wrong pair) rather than silently lifting someone else's suspension.
func (u *SellerSuspensionUseCase) Lift(ctx context.Context, accountID, suspensionID, adminID int64) (*LiftResult, error) {
	now := time.Now()
	result := &LiftResult{}
	err := u.txMgr.WithTx(ctx, func(txCtx context.Context) error {
		existing, err := u.repo.GetByID(txCtx, suspensionID)
		if err != nil {
			return err
		}
		if existing.AccountID != accountID {
			return pkgerrors.NewNotFound("seller suspension not found")
		}
		if !existing.IsActive() {
			result.Suspension = existing
			result.AlreadyLifted = true
			return nil
		}

		lifted, err := u.repo.Lift(txCtx, suspensionID, adminID, now)
		if err != nil {
			return err
		}
		if lifted == nil {
			// Raced with a concurrent lift between GetByID and Lift.
			refetched, gerr := u.repo.GetByID(txCtx, suspensionID)
			if gerr != nil {
				return gerr
			}
			result.Suspension = refetched
			result.AlreadyLifted = true
			return nil
		}

		if err := u.recompute.Recompute(txCtx, accountID); err != nil {
			return err
		}

		// Only needed to snapshot store_id into the outbox event (Plan C Phase 0)
		// — Lift() has no other reason to read the seller row; no FOR UPDATE lock
		// needed since we're not writing to sellers ourselves here.
		seller, err := u.sellerRepo.GetByAccountID(txCtx, accountID)
		if err != nil {
			return err
		}

		row, err := domain.NewLifecycleOutboxRow(u.tsid(), accountID, domain.SellerLifecycleEventUnsuspended,
			domain.SellerSuspensionOutboxPayload{AccountID: accountID, SuspensionID: suspensionID, StoreID: seller.StoreID}, now)
		if err != nil {
			return err
		}
		if err := u.outboxRepo.Insert(txCtx, row); err != nil {
			return err
		}

		result.Suspension = lifted
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Post-commit, best-effort: always self-heal regardless of whether this
	// call actually lifted the row or hit the idempotent/raced branch.
	u.recompute.SyncRedis(ctx, accountID)

	return result, nil
}

// GetActive implements GET /admin/sellers/{sellerId}/suspension. Returns nil
// (no error) when the account has no active suspension.
func (u *SellerSuspensionUseCase) GetActive(ctx context.Context, accountID int64) (*domain.SellerSuspension, error) {
	return u.repo.GetActiveByAccountID(ctx, accountID)
}

// List implements GET /admin/sellers/{sellerId}/suspensions (audit history).
func (u *SellerSuspensionUseCase) List(ctx context.Context, accountID int64, limit, offset int32) ([]*domain.SellerSuspension, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return u.repo.ListByAccountID(ctx, accountID, limit, offset)
}
