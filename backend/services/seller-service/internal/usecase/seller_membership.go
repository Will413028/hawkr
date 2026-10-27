package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Will413028/hawkr/go-pkg/database"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

// AssignMembershipRequest is AssignMembership's input: an admin assigning a
// membership plan to a store. RequestID is the idempotency key — a repeated
// call with the same RequestID returns the existing assignment without
// creating a new subscription row or enqueueing a second outbox event.
type AssignMembershipRequest struct {
	StoreID            int64
	PlanCode           string
	Reason             string
	GrantedByAccountID int64
	RequestID          string
}

// SellerMembershipView is the read model returned by AssignMembership and
// GetCurrentPlan: the effective plan for a store, denormalized with the
// plan catalog's display fields so callers don't need a second lookup.
type SellerMembershipView struct {
	StoreID         int64
	PlanCode        string
	DisplayName     string
	TierRank        int
	MonthlyOrderCap int
	EffectiveAt     time.Time
}

// SellerMembershipUseCase implements admin membership plan assignment and the
// current-plan read path (子系統 1 doc 60). AssignMembership's transaction
// composition mirrors SellerSuspensionUseCase.Suspend: one txMgr.WithTx spans
// both the subscription write (via repo.AssignPlanTx, not repo.AssignPlan —
// AssignPlan owns its own internal transaction, which would only cover the
// subscription table) and the lifecycle outbox insert, so a plan assignment
// and its seller.membership_changed event either both commit or neither does.
type SellerMembershipUseCase struct {
	repo       domain.SellerMembershipRepository
	outboxRepo domain.SellerDeletedOutboxRepository
	txMgr      database.TxManager
	newID      func() int64
	now        func() time.Time
}

// NewSellerMembershipUseCase constructs the usecase. newID supplies the TSID
// for the lifecycle outbox row (the subscription row's own id is supplied by
// the repository's own newID, injected separately into
// NewSellerMembershipRepository — see AssignPlanTx). Taking newID as an
// explicit constructor parameter, rather than constructing a private
// tsid.NewGenerator() here, matches SellerSuspensionUseCase's tsid param: it
// lets callers (fx DI) share one process-wide generator instance across every
// usecase that writes to seller_deleted_outbox, avoiding the (low-probability
// but real) same-millisecond id collision two independent generator
// instances could produce against that shared table.
func NewSellerMembershipUseCase(repo domain.SellerMembershipRepository, outboxRepo domain.SellerDeletedOutboxRepository, txMgr database.TxManager, newID func() int64) *SellerMembershipUseCase {
	return &SellerMembershipUseCase{
		repo:       repo,
		outboxRepo: outboxRepo,
		txMgr:      txMgr,
		newID:      newID,
		now:        func() time.Time { return time.Now().UTC() },
	}
}

// SellerMembershipPlanCatalogEntry is the admin read model for one
// seller_membership_plans row (the catalog itself, not a per-store
// subscription). Mirrors buyer's dto.MembershipPlan shape.
type SellerMembershipPlanCatalogEntry struct {
	PlanCode        string
	DisplayName     string
	TierRank        int
	MonthlyOrderCap int
	MonthlyPriceTWD int
	IsPurchasable   bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// UpdateMembershipPlanCatalogRequest is admin's partial update on the plan
// catalog itself (monthly_order_cap／monthly_price_twd／is_purchasable only
// — tier_rank and display_name have no field here, so this request can never
// carry a change to them, regardless of what a caller upstream attempts).
type UpdateMembershipPlanCatalogRequest struct {
	PlanCode        string
	MonthlyOrderCap *int
	MonthlyPriceTWD *int
	IsPurchasable   *bool
}

func toSellerMembershipPlanCatalogEntry(p domain.SellerMembershipPlan) SellerMembershipPlanCatalogEntry {
	return SellerMembershipPlanCatalogEntry{
		PlanCode: p.PlanCode, DisplayName: p.DisplayName, TierRank: p.TierRank,
		MonthlyOrderCap: p.MonthlyOrderCap, MonthlyPriceTWD: p.MonthlyPriceTWD,
		IsPurchasable: p.IsPurchasable, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

// ListMembershipPlanCatalog returns the full plan catalog for admin display
// (doc gap this closes: no other read path exposes the full catalog with
// cap/price/is_purchasable together).
func (u *SellerMembershipUseCase) ListMembershipPlanCatalog(ctx context.Context) ([]SellerMembershipPlanCatalogEntry, error) {
	plans, err := u.repo.ListPlans(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]SellerMembershipPlanCatalogEntry, len(plans))
	for i, p := range plans {
		out[i] = toSellerMembershipPlanCatalogEntry(p)
	}
	return out, nil
}

// UpdateMembershipPlanCatalog applies an admin partial update to a
// seller_membership_plans catalog row. At least one field must be provided;
// MonthlyOrderCap must be positive and MonthlyPriceTWD non-negative,
// mirroring the table's CHECK constraints so the admin gets a clear business
// error instead of a raw DB constraint failure. An unknown PlanCode surfaces
// the same bizcode.SellerInvalidPlanCode AssignMembership already uses for
// an unknown plan_code (one owner, same meaning).
func (u *SellerMembershipUseCase) UpdateMembershipPlanCatalog(ctx context.Context, in UpdateMembershipPlanCatalogRequest) (SellerMembershipPlanCatalogEntry, error) {
	if in.MonthlyOrderCap == nil && in.MonthlyPriceTWD == nil && in.IsPurchasable == nil {
		return SellerMembershipPlanCatalogEntry{}, pkgerrors.NewInvalidInput("至少需提供一個要更新的欄位")
	}
	if in.MonthlyOrderCap != nil && *in.MonthlyOrderCap <= 0 {
		return SellerMembershipPlanCatalogEntry{}, pkgerrors.NewInvalidInput("monthlyOrderCap 必須為正整數")
	}
	if in.MonthlyPriceTWD != nil && *in.MonthlyPriceTWD < 0 {
		return SellerMembershipPlanCatalogEntry{}, pkgerrors.NewInvalidInput("monthlyPriceTwd 不可為負數")
	}

	updated, err := u.repo.UpdatePlan(ctx, domain.UpdateSellerMembershipPlanInput{
		PlanCode: in.PlanCode, MonthlyOrderCap: in.MonthlyOrderCap,
		MonthlyPriceTWD: in.MonthlyPriceTWD, IsPurchasable: in.IsPurchasable,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SellerMembershipPlanCatalogEntry{}, pkgerrors.NewBusiness(bizcode.SellerInvalidPlanCode, "無此賣家方案代碼")
		}
		return SellerMembershipPlanCatalogEntry{}, err
	}
	return toSellerMembershipPlanCatalogEntry(updated), nil
}

// GetCurrentPlan implements the store's effective membership plan read path.
// A store with no active subscription row is on the implicit "free" plan.
func (u *SellerMembershipUseCase) GetCurrentPlan(ctx context.Context, storeID int64) (SellerMembershipView, error) {
	sub, err := u.repo.GetActiveSubscription(ctx, storeID)
	planCode := "free"
	effectiveAt := time.Time{}
	if err == nil {
		planCode = sub.PlanCode
		effectiveAt = sub.EffectiveAt
	} else if !errors.Is(err, domain.ErrNoActiveSellerMembership) {
		return SellerMembershipView{}, err
	}

	plan, err := u.repo.GetPlanByCode(ctx, planCode)
	if err != nil {
		return SellerMembershipView{}, err
	}
	return SellerMembershipView{
		StoreID: storeID, PlanCode: plan.PlanCode, DisplayName: plan.DisplayName,
		TierRank: plan.TierRank, MonthlyOrderCap: plan.MonthlyOrderCap, EffectiveAt: effectiveAt,
	}, nil
}

// AssignMembership implements admin-only plan assignment (idempotent on
// RequestID). A genuinely new assignment supersedes any existing active
// subscription for the store, inserts the new one, and enqueues a
// seller.membership_changed lifecycle outbox row — all inside one
// txMgr.WithTx, so the write and the outbox enqueue are atomic.
func (u *SellerMembershipUseCase) AssignMembership(ctx context.Context, in AssignMembershipRequest) (SellerMembershipView, error) {
	if existing, found, err := u.repo.FindByRequestID(ctx, in.RequestID); err != nil {
		return SellerMembershipView{}, err
	} else if found {
		plan, err := u.repo.GetPlanByCode(ctx, existing.PlanCode)
		if err != nil {
			return SellerMembershipView{}, err
		}
		return SellerMembershipView{
			StoreID: existing.StoreID, PlanCode: plan.PlanCode, DisplayName: plan.DisplayName,
			TierRank: plan.TierRank, MonthlyOrderCap: plan.MonthlyOrderCap, EffectiveAt: existing.EffectiveAt,
		}, nil
	}

	plan, err := u.repo.GetPlanByCode(ctx, in.PlanCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SellerMembershipView{}, pkgerrors.NewBusiness(bizcode.SellerInvalidPlanCode, "無此賣家方案代碼")
		}
		return SellerMembershipView{}, err
	}

	now := u.now()
	var grantedBy *int64
	if in.GrantedByAccountID > 0 {
		grantedBy = &in.GrantedByAccountID
	}

	var sub domain.SellerMembershipSubscription
	err = u.txMgr.WithTx(ctx, func(txCtx context.Context) error {
		var txErr error
		sub, txErr = u.repo.AssignPlanTx(txCtx, domain.AssignSellerMembershipInput{
			StoreID: in.StoreID, PlanCode: in.PlanCode, EffectiveAt: now,
			Source: "admin", GrantedBy: grantedBy, Reason: in.Reason, RequestID: in.RequestID,
		})
		if txErr != nil {
			return txErr
		}

		row, txErr := domain.NewLifecycleOutboxRow(u.newID(), in.GrantedByAccountID, domain.SellerLifecycleEventMembershipChanged,
			domain.SellerMembershipOutboxPayload{
				StoreID: in.StoreID, PlanCode: plan.PlanCode, TierRank: plan.TierRank,
				MonthlyOrderCap: plan.MonthlyOrderCap, EffectiveAt: now,
			}, now)
		if txErr != nil {
			return txErr
		}
		return u.outboxRepo.Insert(txCtx, row)
	})
	if err != nil {
		return SellerMembershipView{}, err
	}

	return SellerMembershipView{
		StoreID: sub.StoreID, PlanCode: plan.PlanCode, DisplayName: plan.DisplayName,
		TierRank: plan.TierRank, MonthlyOrderCap: plan.MonthlyOrderCap, EffectiveAt: sub.EffectiveAt,
	}, nil
}
