package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

// Admin manual seller suspension usecase tests (32 §13 / 44 §13.2).
// Covers: suspend happy path → idempotent duplicate → lift happy path →
// idempotent duplicate lift, and the four recomputeSellerStatus precedence
// branches (32 §13.3: closed > suspended > suspended_governance > active).

// ========== Suspend ==========

func TestSellerSuspension_Suspend_HappyPath(t *testing.T) {
	var insertedRow *domain.SellerSuspension
	var activeRow *domain.SellerSuspension
	suspRepo := &mockSellerSuspensionRepository{
		InsertFn: func(_ context.Context, s *domain.SellerSuspension) error {
			insertedRow = s
			activeRow = s
			return nil
		},
		HasActiveByAccountIDFn: func(_ context.Context, _ int64) (bool, error) {
			return activeRow != nil, nil
		},
		GetActiveByAccountIDFn: func(_ context.Context, _ int64) (*domain.SellerSuspension, error) {
			return activeRow, nil
		},
	}
	var statusUpdates []domain.SellerStatus
	// currentStatus models the DB read-your-own-write behavior that recompute.SyncRedis
	// relies on: its post-commit GetByAccountID call must see whatever
	// UpdateStatusCurrentByAccountID just wrote, not the pre-call snapshot.
	currentStatus := domain.SellerStatusActive
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(_ context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, StoreID: 800042, Status: currentStatus}, nil
		},
		UpdateStatusCurrentByAccountIDFn: func(_ context.Context, _ int64, status domain.SellerStatus, _ time.Time) error {
			statusUpdates = append(statusUpdates, status)
			currentStatus = status
			return nil
		},
	}
	violationRepo := &mockSellerViolationRepository{}
	outboxRepo := &fakeSellerDeletedOutboxRepo{}
	redis := &mockSellerSuspensionRedis{}
	uc := NewSellerSuspensionUseCase(suspRepo, sellerRepo, violationRepo, outboxRepo, redis, inlineTxManager{}, newCounterTSID(), nil)

	result, err := uc.Suspend(context.Background(), 42, domain.SellerSuspensionReasonPolicyViolation, "屢勸不改", 99)
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if result.AlreadySuspended {
		t.Errorf("expected a new suspension, not an idempotent hit")
	}
	if insertedRow == nil || insertedRow.AccountID != 42 || insertedRow.SuspendedByAdminID != 99 {
		t.Errorf("unexpected inserted row: %+v", insertedRow)
	}
	if len(statusUpdates) != 1 || statusUpdates[0] != domain.SellerStatusSuspended {
		t.Errorf("expected sellers.status -> suspended, got %v", statusUpdates)
	}
	if outboxRepo.insertCalled != 1 {
		t.Errorf("expected 1 outbox insert (seller.suspended), got %d", outboxRepo.insertCalled)
	}
	if outboxRepo.inserted.EventType != domain.SellerLifecycleEventSuspended {
		t.Errorf("expected outbox event_type seller.suspended, got %q", outboxRepo.inserted.EventType)
	}
	var payload domain.SellerSuspensionOutboxPayload
	if err := json.Unmarshal(outboxRepo.inserted.Payload, &payload); err != nil {
		t.Fatalf("unmarshal outbox payload: %v", err)
	}
	if payload.StoreID != 800042 {
		t.Errorf("expected outbox payload store_id = 800042 (seller.StoreID at suspend time), got %d", payload.StoreID)
	}
	if len(redis.SetCalls) != 1 || redis.SetCalls[0] != 42 {
		t.Errorf("expected Redis SetSuspended(42), got %v", redis.SetCalls)
	}
	if len(redis.SetValues) != 1 || redis.SetValues[0] != domain.SellerSuspensionReasonPolicyViolation {
		t.Errorf("expected Redis value = reason_code %q, got %v", domain.SellerSuspensionReasonPolicyViolation, redis.SetValues)
	}
}

func TestSellerSuspension_Suspend_IdempotentDuplicate(t *testing.T) {
	existing := &domain.SellerSuspension{ID: 1, AccountID: 42, Status: domain.SellerSuspensionActive, ReasonCode: domain.SellerSuspensionReasonOther}
	suspRepo := &mockSellerSuspensionRepository{
		InsertFn: func(_ context.Context, _ *domain.SellerSuspension) error {
			return &domain.ErrDuplicate{ConstraintName: "uq_seller_suspensions_active", Underlying: errors.New("23505")}
		},
		GetActiveByAccountIDFn: func(_ context.Context, _ int64) (*domain.SellerSuspension, error) {
			return existing, nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(_ context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, StoreID: accountID, Status: domain.SellerStatusSuspended}, nil
		},
	}
	outboxRepo := &fakeSellerDeletedOutboxRepo{}
	redis := &mockSellerSuspensionRedis{}
	uc := NewSellerSuspensionUseCase(suspRepo, sellerRepo, &mockSellerViolationRepository{}, outboxRepo, redis, inlineTxManager{}, newCounterTSID(), nil)

	result, err := uc.Suspend(context.Background(), 42, domain.SellerSuspensionReasonOther, "", 99)
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if !result.AlreadySuspended {
		t.Errorf("expected idempotent hit on duplicate active suspension")
	}
	if result.Suspension != existing {
		t.Errorf("expected the existing active row to be returned")
	}
	if outboxRepo.insertCalled != 0 {
		t.Errorf("expected no outbox insert on idempotent duplicate, got %d", outboxRepo.insertCalled)
	}
	if len(redis.SetCalls) != 1 {
		t.Errorf("expected Redis SetSuspended to still self-heal on idempotent hit, got %v", redis.SetCalls)
	}
}

func TestSellerSuspension_Suspend_InvalidReasonCode(t *testing.T) {
	uc := NewSellerSuspensionUseCase(&mockSellerSuspensionRepository{}, &mockSellerRepo{}, &mockSellerViolationRepository{},
		&fakeSellerDeletedOutboxRepo{}, &mockSellerSuspensionRedis{}, inlineTxManager{}, newCounterTSID(), nil)

	_, err := uc.Suspend(context.Background(), 42, "NOT_A_REAL_REASON", "", 99)
	if err == nil || pkgerrors.GetCode(err) != pkgerrors.CodeInvalidInput {
		t.Errorf("expected 400 invalid input for bad reason_code, got %v", err)
	}
}

// TestSellerSuspension_Suspend_StoresValueVerbatim asserts the issue #1534
// contract: the CodeMeta Value wire form ("9601") is stored verbatim —
// DB/domain/MQ/wire share the single numeric-Value representation, with no
// normalization layer in between.
func TestSellerSuspension_Suspend_StoresValueVerbatim(t *testing.T) {
	var insertedRow *domain.SellerSuspension
	suspRepo := &mockSellerSuspensionRepository{
		InsertFn: func(_ context.Context, s *domain.SellerSuspension) error {
			insertedRow = s
			return nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(_ context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, StoreID: accountID, Status: domain.SellerStatusActive}, nil
		},
	}
	outboxRepo := &fakeSellerDeletedOutboxRepo{}
	redis := &mockSellerSuspensionRedis{}
	uc := NewSellerSuspensionUseCase(suspRepo, sellerRepo, &mockSellerViolationRepository{}, outboxRepo, redis, inlineTxManager{}, newCounterTSID(), nil)

	result, err := uc.Suspend(context.Background(), 42, "9601", "", 99)
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if insertedRow == nil || insertedRow.ReasonCode != "9601" {
		t.Errorf("expected stored ReasonCode = Value form %q, got %+v", "9601", insertedRow)
	}
	if result.Suspension.ReasonCode != "9601" {
		t.Errorf("expected result ReasonCode = Value form, got %q", result.Suspension.ReasonCode)
	}
}

// TestSellerSuspension_Suspend_RejectsLegacyNameInput pins the #1534 removal
// of the #1514 dual-accept transition: the legacy Name wire form
// ("POLICY_VIOLATION") is no longer a valid input and must 400, not be
// silently normalized.
func TestSellerSuspension_Suspend_RejectsLegacyNameInput(t *testing.T) {
	uc := NewSellerSuspensionUseCase(&mockSellerSuspensionRepository{}, &mockSellerRepo{}, &mockSellerViolationRepository{},
		&fakeSellerDeletedOutboxRepo{}, &mockSellerSuspensionRedis{}, inlineTxManager{}, newCounterTSID(), nil)

	_, err := uc.Suspend(context.Background(), 42, "POLICY_VIOLATION", "", 99)
	if err == nil || pkgerrors.GetCode(err) != pkgerrors.CodeInvalidInput {
		t.Errorf("expected 400 invalid input for legacy Name reason_code, got %v", err)
	}
}

func TestSellerSuspension_Suspend_ClosedSeller_NotFound(t *testing.T) {
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(_ context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, Status: domain.SellerStatusClosed}, nil
		},
	}
	outboxRepo := &fakeSellerDeletedOutboxRepo{}
	uc := NewSellerSuspensionUseCase(&mockSellerSuspensionRepository{}, sellerRepo, &mockSellerViolationRepository{},
		outboxRepo, &mockSellerSuspensionRedis{}, inlineTxManager{}, newCounterTSID(), nil)

	_, err := uc.Suspend(context.Background(), 42, domain.SellerSuspensionReasonOther, "", 99)
	if err == nil || pkgerrors.GetCode(err) != pkgerrors.CodeNotFound {
		t.Errorf("expected 404 for a closed seller, got %v", err)
	}
	if outboxRepo.insertCalled != 0 {
		t.Errorf("expected no outbox insert when suspend is rejected")
	}
}

// ========== Lift ==========

func TestSellerSuspension_Lift_HappyPath(t *testing.T) {
	existing := &domain.SellerSuspension{ID: 1, AccountID: 42, Status: domain.SellerSuspensionActive}
	lifted := &domain.SellerSuspension{ID: 1, AccountID: 42, Status: domain.SellerSuspensionLifted}
	suspRepo := &mockSellerSuspensionRepository{
		GetByIDFn: func(_ context.Context, id int64) (*domain.SellerSuspension, error) {
			if id != 1 {
				t.Errorf("expected GetByID(1), got %d", id)
			}
			return existing, nil
		},
		LiftFn: func(_ context.Context, id, liftedByAdminID int64, _ time.Time) (*domain.SellerSuspension, error) {
			if id != 1 || liftedByAdminID != 99 {
				t.Errorf("unexpected Lift args id=%d adminID=%d", id, liftedByAdminID)
			}
			return lifted, nil
		},
		HasActiveByAccountIDFn: func(_ context.Context, _ int64) (bool, error) { return false, nil },
	}
	var statusUpdates []domain.SellerStatus
	// currentStatus models read-your-own-write so recompute.SyncRedis's post-commit
	// GetByAccountID call observes the lift, not the pre-call "suspended" snapshot.
	currentStatus := domain.SellerStatusSuspended
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(_ context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, StoreID: 800042, Status: currentStatus}, nil
		},
		UpdateStatusCurrentByAccountIDFn: func(_ context.Context, _ int64, status domain.SellerStatus, _ time.Time) error {
			statusUpdates = append(statusUpdates, status)
			currentStatus = status
			return nil
		},
	}
	outboxRepo := &fakeSellerDeletedOutboxRepo{}
	redis := &mockSellerSuspensionRedis{}
	uc := NewSellerSuspensionUseCase(suspRepo, sellerRepo, &mockSellerViolationRepository{}, outboxRepo, redis, inlineTxManager{}, newCounterTSID(), nil)

	result, err := uc.Lift(context.Background(), 42, 1, 99)
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if result.AlreadyLifted {
		t.Errorf("expected a real lift, not an idempotent hit")
	}
	if len(statusUpdates) != 1 || statusUpdates[0] != domain.SellerStatusActive {
		t.Errorf("expected sellers.status -> active, got %v", statusUpdates)
	}
	if outboxRepo.insertCalled != 1 || outboxRepo.inserted.EventType != domain.SellerLifecycleEventUnsuspended {
		t.Errorf("expected 1 outbox insert (seller.unsuspended), got %d type=%q", outboxRepo.insertCalled, outboxRepo.inserted.EventType)
	}
	var liftPayload domain.SellerSuspensionOutboxPayload
	if err := json.Unmarshal(outboxRepo.inserted.Payload, &liftPayload); err != nil {
		t.Fatalf("unmarshal outbox payload: %v", err)
	}
	if liftPayload.StoreID != 800042 {
		t.Errorf("expected outbox payload store_id = 800042 (seller.StoreID at lift time), got %d", liftPayload.StoreID)
	}
	if len(redis.ClearCalls) != 1 || redis.ClearCalls[0] != 42 {
		t.Errorf("expected Redis ClearSuspended(42), got %v", redis.ClearCalls)
	}
}

func TestSellerSuspension_Lift_IdempotentAlreadyLifted(t *testing.T) {
	existing := &domain.SellerSuspension{ID: 1, AccountID: 42, Status: domain.SellerSuspensionLifted}
	suspRepo := &mockSellerSuspensionRepository{
		GetByIDFn: func(_ context.Context, _ int64) (*domain.SellerSuspension, error) { return existing, nil },
		LiftFn: func(_ context.Context, _, _ int64, _ time.Time) (*domain.SellerSuspension, error) {
			t.Fatal("Lift should not be called when already lifted")
			return nil, nil
		},
	}
	outboxRepo := &fakeSellerDeletedOutboxRepo{}
	redis := &mockSellerSuspensionRedis{}
	// This suspension row is already lifted and no other admin suspension is
	// active for the account, so the SyncRedis self-heal read must observe
	// 'active' — matching what a real recomputed sellers.status would show.
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(_ context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, StoreID: accountID, Status: domain.SellerStatusActive}, nil
		},
	}
	uc := NewSellerSuspensionUseCase(suspRepo, sellerRepo, &mockSellerViolationRepository{}, outboxRepo, redis, inlineTxManager{}, newCounterTSID(), nil)

	result, err := uc.Lift(context.Background(), 42, 1, 99)
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if !result.AlreadyLifted {
		t.Errorf("expected idempotent hit")
	}
	if outboxRepo.insertCalled != 0 {
		t.Errorf("expected no outbox insert on idempotent lift, got %d", outboxRepo.insertCalled)
	}
	if len(redis.ClearCalls) != 1 {
		t.Errorf("expected Redis ClearSuspended to still self-heal, got %v", redis.ClearCalls)
	}
}

// TestSellerSuspension_Lift_ConcurrentRace_RefetchAlreadyLifted covers the
// race window at seller_suspension.go:180-189: GetByID sees the suspension
// still active, but a concurrent Lift call wins the repo-level Lift() first,
// so this call's Lift() returns (nil, nil) — the losing side must refetch
// and report AlreadyLifted instead of erroring or double-writing recompute
// / outbox.
func TestSellerSuspension_Lift_ConcurrentRace_RefetchAlreadyLifted(t *testing.T) {
	activeSnapshot := &domain.SellerSuspension{ID: 1, AccountID: 42, Status: domain.SellerSuspensionActive}
	winnerAdminID := int64(7)
	liftedByWinner := &domain.SellerSuspension{ID: 1, AccountID: 42, Status: domain.SellerSuspensionLifted, LiftedByAdminID: &winnerAdminID}
	getByIDCalls := 0
	suspRepo := &mockSellerSuspensionRepository{
		GetByIDFn: func(_ context.Context, id int64) (*domain.SellerSuspension, error) {
			getByIDCalls++
			if id != 1 {
				t.Errorf("expected GetByID(1), got %d", id)
			}
			if getByIDCalls == 1 {
				// First read: still active (this goroutine hasn't lost the race yet).
				return activeSnapshot, nil
			}
			// Refetch after the raced Lift() returned nil: the winner's row.
			return liftedByWinner, nil
		},
		LiftFn: func(_ context.Context, _, _ int64, _ time.Time) (*domain.SellerSuspension, error) {
			// Simulate losing the race: another goroutine's Lift already
			// flipped the row, so this UPDATE affects 0 rows.
			return nil, nil
		},
	}
	// statusWritten tracks whether Recompute actually wrote sellers.status —
	// the true "did recompute run" signal. GetByAccountID alone can no longer
	// serve as that proxy: the post-commit SyncRedis self-heal (P1 fix) also
	// calls it on the losing side, by design, to refresh the Redis marker.
	var statusWritten bool
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(_ context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, StoreID: accountID, Status: domain.SellerStatusActive}, nil
		},
		UpdateStatusCurrentByAccountIDFn: func(_ context.Context, _ int64, _ domain.SellerStatus, _ time.Time) error {
			statusWritten = true
			return nil
		},
	}
	outboxRepo := &fakeSellerDeletedOutboxRepo{}
	redis := &mockSellerSuspensionRedis{}
	uc := NewSellerSuspensionUseCase(suspRepo, sellerRepo, &mockSellerViolationRepository{}, outboxRepo, redis, inlineTxManager{}, newCounterTSID(), nil)

	result, err := uc.Lift(context.Background(), 42, 1, 99)
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if !result.AlreadyLifted {
		t.Errorf("expected AlreadyLifted on the losing side of the race")
	}
	if result.Suspension != liftedByWinner {
		t.Errorf("expected the refetched (winner's) row to be returned, got %+v", result.Suspension)
	}
	if getByIDCalls != 2 {
		t.Errorf("expected GetByID to be called twice (initial read + post-race refetch), got %d", getByIDCalls)
	}
	if statusWritten {
		t.Errorf("expected no recompute write on the losing side (the winner already recomputed)")
	}
	if outboxRepo.insertCalled != 0 {
		t.Errorf("expected no outbox insert on the losing side, got %d", outboxRepo.insertCalled)
	}
	if len(redis.ClearCalls) != 1 || redis.ClearCalls[0] != 42 {
		t.Errorf("expected Redis ClearSuspended(42) to still self-heal on the losing side, got %v", redis.ClearCalls)
	}
}

func TestSellerSuspension_Lift_AccountMismatch_NotFound(t *testing.T) {
	existing := &domain.SellerSuspension{ID: 1, AccountID: 42, Status: domain.SellerSuspensionActive}
	suspRepo := &mockSellerSuspensionRepository{
		GetByIDFn: func(_ context.Context, _ int64) (*domain.SellerSuspension, error) { return existing, nil },
	}
	uc := NewSellerSuspensionUseCase(suspRepo, &mockSellerRepo{}, &mockSellerViolationRepository{},
		&fakeSellerDeletedOutboxRepo{}, &mockSellerSuspensionRedis{}, inlineTxManager{}, newCounterTSID(), nil)

	// Caller passes accountID=7, but the suspension row belongs to account 42.
	_, err := uc.Lift(context.Background(), 7, 1, 99)
	if err == nil || pkgerrors.GetCode(err) != pkgerrors.CodeNotFound {
		t.Errorf("expected 404 on (id, accountID) mismatch, got %v", err)
	}
}

// ========== recomputeSellerStatus precedence (32 §13.3) ==========

func TestSellerStatusRecompute_Precedence(t *testing.T) {
	tests := []struct {
		name          string
		sellerStatus  domain.SellerStatus
		hasSuspension bool
		hasGovernance bool
		wantWrite     bool
		wantStatus    domain.SellerStatus
	}{
		{"closed is terminal, never overwritten", domain.SellerStatusClosed, true, true, false, ""},
		{"suspended wins over governance", domain.SellerStatusActive, true, true, true, domain.SellerStatusSuspended},
		{"governance when no admin suspension", domain.SellerStatusActive, false, true, true, domain.SellerStatusSuspendedGovernance},
		{"active when neither", domain.SellerStatusSuspended, false, false, true, domain.SellerStatusActive},
		{"no-op when already correct", domain.SellerStatusActive, false, false, false, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var wrote *domain.SellerStatus
			sellerRepo := &mockSellerRepo{
				GetByAccountIDFn: func(_ context.Context, accountID int64) (*domain.Seller, error) {
					return &domain.Seller{AccountID: accountID, StoreID: accountID, Status: tc.sellerStatus}, nil
				},
				UpdateStatusCurrentByAccountIDFn: func(_ context.Context, _ int64, status domain.SellerStatus, _ time.Time) error {
					s := status
					wrote = &s
					return nil
				},
			}
			suspRepo := &mockSellerSuspensionRepository{
				HasActiveByAccountIDFn: func(_ context.Context, _ int64) (bool, error) { return tc.hasSuspension, nil },
			}
			violationRepo := &mockSellerViolationRepository{
				HasActiveFullSuspensionBySellerFn: func(_ context.Context, _ int64) (bool, error) { return tc.hasGovernance, nil },
			}
			r := newSellerStatusRecomputer(sellerRepo, suspRepo, violationRepo, &mockSellerSuspensionRedis{}, nil)

			if err := r.Recompute(context.Background(), 42); err != nil {
				t.Fatalf("got %v", err)
			}
			if tc.wantWrite {
				if wrote == nil {
					t.Fatalf("expected a status write, got none")
				}
				if *wrote != tc.wantStatus {
					t.Errorf("wrote status = %s, want %s", *wrote, tc.wantStatus)
				}
			} else if wrote != nil {
				t.Errorf("expected no status write, got %s", *wrote)
			}
		})
	}
}

// ========== SyncRedis value mapping (46 §4.2 P1 fix) ==========
//
// SyncRedis is now the single point deciding the seller:suspended:<id> Redis
// value for every sellers.status outcome. This directly covers the P1 gap:
// before this fix, governance full_suspension (suspended_governance) never
// wrote this key at all — only seller_suspension.go's admin Suspend/Lift did
// — so the Gateway capability layer never BLOCKed governance-suspended
// sellers. These cases exercise SyncRedis independently of Recompute, since
// production callers always invoke it as a separate post-commit step (see
// SyncRedis's doc comment for why it must not run inside the same tx).
func TestSellerStatusRecompute_SyncRedis(t *testing.T) {
	tests := []struct {
		name         string
		sellerStatus domain.SellerStatus
		activeReason string // stubbed seller_suspensions.reason_code; only read for 'suspended'
		wantSetValue string // expected SetSuspended value; ignored when wantClear
		wantClear    bool
	}{
		{"admin suspended uses seller_suspensions.reason_code", domain.SellerStatusSuspended, domain.SellerSuspensionReasonPolicyViolation, domain.SellerSuspensionReasonPolicyViolation, false},
		{"governance suspended uses fixed sentinel", domain.SellerStatusSuspendedGovernance, "", governanceSuspendedRedisValue, false},
		{"active clears the marker", domain.SellerStatusActive, "", "", true},
		{"closed clears the marker", domain.SellerStatusClosed, "", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sellerRepo := &mockSellerRepo{
				GetByAccountIDFn: func(_ context.Context, accountID int64) (*domain.Seller, error) {
					return &domain.Seller{AccountID: accountID, StoreID: accountID, Status: tc.sellerStatus}, nil
				},
			}
			suspRepo := &mockSellerSuspensionRepository{
				GetActiveByAccountIDFn: func(_ context.Context, _ int64) (*domain.SellerSuspension, error) {
					if tc.activeReason == "" {
						return nil, nil
					}
					return &domain.SellerSuspension{AccountID: 42, ReasonCode: tc.activeReason, Status: domain.SellerSuspensionActive}, nil
				},
			}
			redis := &mockSellerSuspensionRedis{}
			r := newSellerStatusRecomputer(sellerRepo, suspRepo, &mockSellerViolationRepository{}, redis, nil)

			r.SyncRedis(context.Background(), 42)

			if tc.wantClear {
				if len(redis.ClearCalls) != 1 || redis.ClearCalls[0] != 42 {
					t.Errorf("expected ClearSuspended(42), got Set=%v Clear=%v", redis.SetCalls, redis.ClearCalls)
				}
				if len(redis.SetCalls) != 0 {
					t.Errorf("expected no SetSuspended, got %v", redis.SetCalls)
				}
				return
			}
			if len(redis.SetCalls) != 1 || redis.SetCalls[0] != 42 {
				t.Errorf("expected SetSuspended(42), got %v", redis.SetCalls)
			}
			if len(redis.SetValues) != 1 || redis.SetValues[0] != tc.wantSetValue {
				t.Errorf("expected redis value %q, got %v", tc.wantSetValue, redis.SetValues)
			}
			if len(redis.ClearCalls) != 0 {
				t.Errorf("expected no ClearSuspended, got %v", redis.ClearCalls)
			}
		})
	}
}

// TestSellerStatusRecompute_SyncRedis_NilRedisNoop covers the
// NewSellerSuspensionRedis(nil rdb) startup posture (infrastructure/
// seller_suspension_redis.go tolerates a nil *redis.Client) — SyncRedis must
// short-circuit before touching sellerRepo, not panic.
func TestSellerStatusRecompute_SyncRedis_NilRedisNoop(t *testing.T) {
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(_ context.Context, _ int64) (*domain.Seller, error) {
			t.Fatal("SyncRedis must short-circuit on nil redis before reading seller state")
			return nil, nil
		},
	}
	r := newSellerStatusRecomputer(sellerRepo, &mockSellerSuspensionRepository{}, &mockSellerViolationRepository{}, nil, nil)
	r.SyncRedis(context.Background(), 42) // must not panic
}
