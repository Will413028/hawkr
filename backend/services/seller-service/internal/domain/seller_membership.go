package domain

import (
	"context"
	"errors"
	"time"
)

var ErrNoActiveSellerMembership = errors.New("store has no active membership subscription")

type SellerMembershipPlan struct {
	ID              int64
	PlanCode        string
	DisplayName     string
	TierRank        int
	MonthlyPriceTWD int
	MonthlyOrderCap int
	IsPurchasable   bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// UpdateSellerMembershipPlanInput is a partial update on the plan catalog
// itself. Nil fields keep their stored value (repository translates this to
// a SQL COALESCE, not a caller-side merge) — tier_rank/display_name are
// deliberately absent: this struct has no field for them, so an update can
// never touch them regardless of what a caller passes upstream.
type UpdateSellerMembershipPlanInput struct {
	PlanCode        string
	MonthlyOrderCap *int
	MonthlyPriceTWD *int
	IsPurchasable   *bool
}

type SellerMembershipSubscription struct {
	ID          int64
	StoreID     int64
	PlanCode    string
	Status      string
	EffectiveAt time.Time
	Source      string
	GrantedBy   *int64
	Reason      string
	RequestID   string
	CreatedAt   time.Time
}

// AssignSellerMembershipInput is the full input for creating a new active
// SellerMembershipSubscription row (superseding any existing active row for
// the same store in the same transaction). See SellerMembershipRepository.AssignPlan.
type AssignSellerMembershipInput struct {
	StoreID     int64
	PlanCode    string
	EffectiveAt time.Time
	Source      string
	GrantedBy   *int64
	Reason      string
	RequestID   string
}

// SellerMembershipRepository defines the persistence interface for the plan
// catalog and per-store subscription assignment.
type SellerMembershipRepository interface {
	GetPlanByCode(ctx context.Context, planCode string) (SellerMembershipPlan, error)
	// ListPlans returns the full plan catalog, ordered by tier_rank ascending
	// (admin display — doc convention matches buyer's ListMembershipPlans).
	ListPlans(ctx context.Context) ([]SellerMembershipPlan, error)
	// UpdatePlan applies an admin partial update to a catalog row. Returns
	// pgx.ErrNoRows (unwrapped) if in.PlanCode doesn't exist — callers map
	// that to bizcode.SellerInvalidPlanCode, the same code AssignMembership
	// already uses for an unknown plan_code (one owner, same meaning).
	UpdatePlan(ctx context.Context, in UpdateSellerMembershipPlanInput) (SellerMembershipPlan, error)
	GetActiveSubscription(ctx context.Context, storeID int64) (SellerMembershipSubscription, error)
	FindByRequestID(ctx context.Context, requestID string) (SellerMembershipSubscription, bool, error)
	// AssignPlan supersedes any existing active row and inserts the new one in
	// one transaction. Callers must call FindByRequestID first for idempotency;
	// AssignPlan itself does not check requestID uniqueness beyond the DB's
	// partial unique index as a last-resort guard.
	AssignPlan(ctx context.Context, in AssignSellerMembershipInput) (SellerMembershipSubscription, error)
	// AssignPlanTx is AssignPlan's steps (supersede + create), but runs against an
	// already-open transaction context supplied by the caller, so a caller (like
	// SellerMembershipUseCase.AssignMembership) can compose it with another
	// repository's write (the lifecycle outbox insert) in one atomic transaction.
	// AssignPlan remains for callers that don't need this composition.
	AssignPlanTx(ctx context.Context, in AssignSellerMembershipInput) (SellerMembershipSubscription, error)
}
