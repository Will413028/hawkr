package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type SellerDeletedOutboxStatus string

const (
	SellerDeletedOutboxPending SellerDeletedOutboxStatus = "pending"
	SellerDeletedOutboxDone    SellerDeletedOutboxStatus = "done"
	SellerDeletedOutboxFailed  SellerDeletedOutboxStatus = "failed"
)

// Seller-lifecycle outbox event types (seller_deleted_outbox.event_type).
const (
	SellerLifecycleEventDeleted           = "seller.deleted"
	SellerLifecycleEventOnboarded         = "seller.onboarded"
	SellerLifecycleEventSuspended         = "seller.suspended"          // 32 §13 / 44 §13.2 admin manual suspension
	SellerLifecycleEventUnsuspended       = "seller.unsuspended"        // 32 §13 / 44 §13.2 admin manual lift
	SellerLifecycleEventMembershipChanged = "seller.membership_changed" // 賣家會員等級異動（子系統 1）
)

// SellerLifecyclePayload is the JSONB snapshot written at enqueue time inside
// the same transaction as the state change. The surrogate is snapshotted HERE —
// a consume-time "current store" lookup would race a re-onboard (spec §3.1).
//
// StoreID was SellerAccountID (json sellerAccountId) until 2026-08-02 — pure
// naming debt (the value has always been the store surrogate), renamed to
// match the sibling SellerSuspensionOutboxPayload.StoreID below. This is a
// private DB-internal JSONB blob (never crosses the wire to another service),
// so the only risk is an in-flight outbox row enqueued before this deploy with
// the old JSON key — that already degrades gracefully via the worker's
// `if p.StoreID == 0 { p.StoreID = row.AccountID }` grandfather fallback.
type SellerLifecyclePayload struct {
	AccountID int64 `json:"accountId"`
	StoreID   int64 `json:"storeId"`
}

// SellerSuspensionOutboxPayload is the JSONB snapshot for seller.suspended /
// seller.unsuspended lifecycle rows. ReasonCode is only meaningful for the
// suspended event (unsuspended consumers don't need it — 36 §3.6f).
type SellerSuspensionOutboxPayload struct {
	AccountID    int64  `json:"accountId"`
	SuspensionID int64  `json:"suspensionId"`
	ReasonCode   string `json:"reasonCode,omitempty"`
	// StoreID snapshots the store surrogate at the moment of suspend/lift
	// (Plan C Phase 0) — captured here, not re-resolved by the outbox worker
	// at dispatch time, so a delayed retry can't pick up a re-onboarded
	// store's new store_id.
	StoreID int64 `json:"storeId"`
}

// SellerMembershipOutboxPayload is the JSONB snapshot for seller.membership_changed
// lifecycle rows. Snapshotted at enqueue time (same rationale as
// SellerLifecyclePayload above) so a delayed retry publishes the plan that was
// actually assigned, not whatever the store's current plan happens to be by
// the time the outbox worker gets to it.
type SellerMembershipOutboxPayload struct {
	StoreID         int64     `json:"storeId"`
	PlanCode        string    `json:"planCode"`
	TierRank        int       `json:"tierRank"`
	MonthlyOrderCap int       `json:"monthlyOrderCap"`
	EffectiveAt     time.Time `json:"effectiveAt"`
}

// SellerDeletedOutbox 一筆賣家 lifecycle 事件可靠投遞任務（對應 seller_deleted_outbox 表；
// 2026-07-11 起一般化：EventType 分派 seller.deleted / seller.onboarded）。
// EventPublishedAt / RoleRemovedAt 是兩個獨立子任務各自的完成時間戳；
// RoleRemovedAt 只對 deletion 事件有意義（onboarded 不移除角色）。
type SellerDeletedOutbox struct {
	ID               int64
	AccountID        int64
	EventType        string
	Payload          json.RawMessage
	Status           SellerDeletedOutboxStatus
	AttemptCount     int
	NextAttemptAt    time.Time
	LastError        string
	CreatedAt        time.Time
	CompletedAt      *time.Time
	EventPublishedAt *time.Time
	RoleRemovedAt    *time.Time
}

// OutboxID and OutboxAttemptCount satisfy github.com/Will413028/hawkr/go-pkg/outbox.Row.
func (s *SellerDeletedOutbox) OutboxID() int64         { return s.ID }
func (s *SellerDeletedOutbox) OutboxAttemptCount() int { return s.AttemptCount }

type SellerDeletedOutboxRepository interface {
	Insert(ctx context.Context, row *SellerDeletedOutbox) error
	FetchPendingDue(ctx context.Context, now time.Time, limit int) ([]*SellerDeletedOutbox, error)
	// Claim is the multi-replica preemption gate (issue #2294; see
	// docs/開發規範/多副本排程搶佔規範.md). It pushes next_attempt_at to leaseUntil
	// with a CAS whose predicate mirrors the candidate SQL
	// (status='pending' AND next_attempt_at <= claimableBefore), and returns the
	// number of rows affected: 1 = this replica owns the row, 0 = another replica
	// already claimed it (or the row left the candidate set).
	//
	// It is a LEASE, not a done-flag: if the owning pod disappears between the
	// claim and the side effects (publish + account-service RemoveSellerRole),
	// the lease expires and the row returns to the candidate set on its own.
	// FetchPendingDue deliberately no longer carries FOR UPDATE SKIP LOCKED —
	// that lock is released the moment the SELECT returns under autocommit, so
	// it protected nothing.
	Claim(ctx context.Context, id int64, leaseUntil, claimableBefore time.Time) (int64, error)
	UpdateAttempt(ctx context.Context, id int64, attempt int, nextAt time.Time, lastErr string) error
	MarkDone(ctx context.Context, id int64, now time.Time) error
	MarkFailed(ctx context.Context, id int64, lastErr string, now time.Time) error
	MarkEventPublished(ctx context.Context, id int64, now time.Time) error
	MarkRoleRemoved(ctx context.Context, id int64, now time.Time) error
	// CountPending, OldestPendingAge and DeleteDoneBefore satisfy
	// github.com/Will413028/hawkr/go-pkg/outbox.MonitorRepo, consumed by
	// outbox.Metrics.Refresh / outbox.CleanupDone (Task 6).
	CountPending(ctx context.Context) (int, error)
	// OldestPendingAge returns the age of the oldest pending row relative to
	// now. ok is false when there are no pending rows.
	OldestPendingAge(ctx context.Context, now time.Time) (age time.Duration, ok bool, err error)
	// DeleteDoneBefore deletes up to limit 'done' rows completed before
	// cutoff, returning the number deleted.
	DeleteDoneBefore(ctx context.Context, cutoff time.Time, limit int) (deleted int, err error)
}

// NewLifecycleOutboxRow builds a pending seller-lifecycle outbox row for the
// given event type, marshaling payload into the row's JSONB snapshot. Callers
// (S6 re-onboard, S7 delete) pass the result straight to
// SellerDeletedOutboxRepository.Insert inside the same transaction as the state
// change that produced it. id/now are supplied by the caller (tsid + tx
// timestamp) so this stays a pure constructor with no side effects.
func NewLifecycleOutboxRow(id, accountID int64, eventType string, payload any, now time.Time) (*SellerDeletedOutbox, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal lifecycle payload: %w", err)
	}
	return &SellerDeletedOutbox{
		ID:            id,
		AccountID:     accountID,
		EventType:     eventType,
		Payload:       b,
		Status:        SellerDeletedOutboxPending,
		AttemptCount:  0,
		NextAttemptAt: now,
		CreatedAt:     now,
	}, nil
}
