package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres/gen"
)

type sellerMembershipRepository struct {
	pool      *pgxpool.Pool
	txManager database.TxManager
	newID     func() int64
}

func NewSellerMembershipRepository(pool *pgxpool.Pool, txManager database.TxManager, newID func() int64) domain.SellerMembershipRepository {
	return &sellerMembershipRepository{pool: pool, txManager: txManager, newID: newID}
}

func (r *sellerMembershipRepository) q(ctx context.Context) *gen.Queries {
	return gen.New(database.GetDBTX(ctx, r.pool))
}

func (r *sellerMembershipRepository) GetPlanByCode(ctx context.Context, planCode string) (domain.SellerMembershipPlan, error) {
	row, err := r.q(ctx).GetSellerMembershipPlanByCode(ctx, planCode)
	if err != nil {
		return domain.SellerMembershipPlan{}, err
	}
	return toSellerMembershipPlanEntity(row), nil
}

func (r *sellerMembershipRepository) ListPlans(ctx context.Context) ([]domain.SellerMembershipPlan, error) {
	rows, err := r.q(ctx).ListSellerMembershipPlans(ctx)
	if err != nil {
		return nil, err
	}
	plans := make([]domain.SellerMembershipPlan, len(rows))
	for i, row := range rows {
		plans[i] = toSellerMembershipPlanEntity(row)
	}
	return plans, nil
}

func (r *sellerMembershipRepository) UpdatePlan(ctx context.Context, in domain.UpdateSellerMembershipPlanInput) (domain.SellerMembershipPlan, error) {
	row, err := r.q(ctx).UpdateSellerMembershipPlan(ctx, gen.UpdateSellerMembershipPlanParams{
		PlanCode:        in.PlanCode,
		MonthlyOrderCap: intPtrToInt32Ptr(in.MonthlyOrderCap),
		MonthlyPriceTwd: intPtrToInt32Ptr(in.MonthlyPriceTWD),
		IsPurchasable:   in.IsPurchasable,
	})
	if err != nil {
		return domain.SellerMembershipPlan{}, err
	}
	return toSellerMembershipPlanEntity(row), nil
}

func intPtrToInt32Ptr(v *int) *int32 {
	if v == nil {
		return nil
	}
	out := int32(*v)
	return &out
}

func (r *sellerMembershipRepository) GetActiveSubscription(ctx context.Context, storeID int64) (domain.SellerMembershipSubscription, error) {
	row, err := r.q(ctx).GetActiveSellerMembershipSubscription(ctx, storeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SellerMembershipSubscription{}, domain.ErrNoActiveSellerMembership
		}
		return domain.SellerMembershipSubscription{}, err
	}
	return toSellerMembershipSubscriptionEntity(row), nil
}

func (r *sellerMembershipRepository) FindByRequestID(ctx context.Context, requestID string) (domain.SellerMembershipSubscription, bool, error) {
	if requestID == "" {
		return domain.SellerMembershipSubscription{}, false, nil
	}
	row, err := r.q(ctx).FindSellerMembershipSubscriptionByRequestID(ctx, requestID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SellerMembershipSubscription{}, false, nil
		}
		return domain.SellerMembershipSubscription{}, false, err
	}
	return toSellerMembershipSubscriptionEntity(row), true, nil
}

func (r *sellerMembershipRepository) AssignPlan(ctx context.Context, in domain.AssignSellerMembershipInput) (domain.SellerMembershipSubscription, error) {
	var out domain.SellerMembershipSubscription
	err := r.txManager.WithTx(ctx, func(txCtx context.Context) error {
		var txErr error
		out, txErr = r.AssignPlanTx(txCtx, in)
		return txErr
	})
	if err != nil {
		return domain.SellerMembershipSubscription{}, err
	}
	return out, nil
}

// AssignPlanTx is AssignPlan's steps (supersede + create), but runs against an
// already-open transaction context supplied by the caller, so a caller (like
// SellerMembershipUseCase.AssignMembership) can compose it with another
// repository's write (the lifecycle outbox insert) in one atomic transaction.
// AssignPlan remains for callers that don't need this composition.
func (r *sellerMembershipRepository) AssignPlanTx(ctx context.Context, in domain.AssignSellerMembershipInput) (domain.SellerMembershipSubscription, error) {
	q := r.q(ctx)
	if err := q.SupersedeActiveSellerMembershipSubscription(ctx, in.StoreID); err != nil {
		return domain.SellerMembershipSubscription{}, err
	}
	row, err := q.CreateSellerMembershipSubscription(ctx, gen.CreateSellerMembershipSubscriptionParams{
		ID: r.newID(), StoreID: in.StoreID, PlanCode: in.PlanCode,
		EffectiveAt: timestamptzParam(in.EffectiveAt), Source: in.Source,
		GrantedBy: in.GrantedBy, Reason: in.Reason, RequestID: in.RequestID,
	})
	if err != nil {
		return domain.SellerMembershipSubscription{}, err
	}
	return toSellerMembershipSubscriptionEntity(row), nil
}

func toSellerMembershipPlanEntity(row gen.SellerMembershipPlan) domain.SellerMembershipPlan {
	return domain.SellerMembershipPlan{
		ID: row.ID, PlanCode: row.PlanCode, DisplayName: row.DisplayName,
		TierRank: int(row.TierRank), MonthlyPriceTWD: int(row.MonthlyPriceTwd),
		MonthlyOrderCap: int(row.MonthlyOrderCap), IsPurchasable: row.IsPurchasable,
		CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
	}
}

func toSellerMembershipSubscriptionEntity(row gen.SellerMembershipSubscription) domain.SellerMembershipSubscription {
	return domain.SellerMembershipSubscription{
		ID: row.ID, StoreID: row.StoreID, PlanCode: row.PlanCode, Status: row.Status,
		EffectiveAt: row.EffectiveAt.Time, Source: row.Source,
		GrantedBy: row.GrantedBy, Reason: row.Reason,
		RequestID: row.RequestID, CreatedAt: row.CreatedAt.Time,
	}
}
