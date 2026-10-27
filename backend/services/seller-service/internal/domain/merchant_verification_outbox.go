// services/seller-service/internal/domain/merchant_verification_outbox.go
package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

type MerchantVerificationOutboxStatus string

const (
	MerchantVerificationOutboxPending MerchantVerificationOutboxStatus = "pending"
	MerchantVerificationOutboxDone    MerchantVerificationOutboxStatus = "done"
	MerchantVerificationOutboxFailed  MerchantVerificationOutboxStatus = "failed"
)

// MerchantVerificationCompletedEvent is the MQ payload published when a
// certification review outcome changes. 36 §7.4.1 L1 wire is snake_case
// (issue #1176): event_id / merchant_id. EventID is the owning outbox row's
// own TSID as a decimal string (issue #2806) — it doubles as the MQ event
// identifier and the notification-service reference_id_template ingredient
// (see 18 §5.7.1), replacing the previous uuid.NewString() value.
//
// StoreID is additive (issue #2707 followup, Phase A): the seller's current
// store_id, threaded through alongside the pre-existing account_id-space
// MerchantID so notification-service can eventually retarget these five
// templates (identity_migrated=true) without another producer-side change.
// No `omitempty` — matches the sibling identity-space field convention (e.g.
// pkg/mq/events.go ReviewCreatedEvent.StoreID).
type MerchantVerificationCompletedEvent struct {
	EventID    string `json:"event_id"`
	Status     string `json:"status"`
	Reason     string `json:"reason,omitempty"`
	MerchantID int64  `json:"merchant_id"`
	StoreID    int64  `json:"store_id"`
}

// MerchantVerificationOutbox is one guaranteed-delivery notification task for
// a certification/PayUni verification outcome (event_type: approved |
// pending_info | rejected | forwarded_to_payuni | under_review). Independent
// of SellerDeletedOutbox: different bounded context (certification/PayUni vs.
// seller lifecycle), different exchange (hawkr.merchant vs.
// seller.events), different publisher type.
type MerchantVerificationOutbox struct {
	ID            int64
	AccountID     int64
	EventType     string
	Reason        string
	Payload       json.RawMessage
	Status        MerchantVerificationOutboxStatus
	AttemptCount  int
	NextAttemptAt time.Time
	LastError     string
	CreatedAt     time.Time
	CompletedAt   *time.Time
}

// OutboxID and OutboxAttemptCount satisfy github.com/Will413028/hawkr/go-pkg/outbox.Row.
func (o *MerchantVerificationOutbox) OutboxID() int64         { return o.ID }
func (o *MerchantVerificationOutbox) OutboxAttemptCount() int { return o.AttemptCount }

// MerchantVerificationOutboxRepository is implemented by the postgres package
// (Task 2) against pkg/outbox.Repository[*MerchantVerificationOutbox] plus the
// monitoring methods pkg/outbox.MonitorRepo needs.
type MerchantVerificationOutboxRepository interface {
	Insert(ctx context.Context, row *MerchantVerificationOutbox) error
	FetchPendingDue(ctx context.Context, now time.Time, limit int) ([]*MerchantVerificationOutbox, error)
	Claim(ctx context.Context, id int64, leaseUntil, claimableBefore time.Time) (int64, error)
	UpdateAttempt(ctx context.Context, id int64, attempt int, nextAt time.Time, lastErr string) error
	MarkDone(ctx context.Context, id int64, now time.Time) error
	MarkFailed(ctx context.Context, id int64, lastErr string, now time.Time) error
	CountPending(ctx context.Context) (int, error)
	OldestPendingAge(ctx context.Context, now time.Time) (age time.Duration, ok bool, err error)
	DeleteDoneBefore(ctx context.Context, cutoff time.Time, limit int) (deleted int, err error)
}

// NewMerchantVerificationOutboxRow builds a pending outbox row for the given
// verification outcome. id/now are supplied by the caller (tsid + tx
// timestamp) so this stays a pure constructor with no side effects, mirroring
// NewLifecycleOutboxRow in seller_deleted_outbox.go. eventType is one of
// "approved" | "pending_info" | "rejected" | "forwarded_to_payuni" |
// "under_review"; reason may be empty. storeID is the seller's current
// store_id (issue #2707 followup, Phase A) — a separate parameter from
// accountID, not derived from it, so callers must resolve it explicitly
// rather than accidentally reusing the account id.
func NewMerchantVerificationOutboxRow(id, accountID, storeID int64, eventType, reason string, now time.Time) (*MerchantVerificationOutbox, error) {
	event := MerchantVerificationCompletedEvent{
		EventID:    strconv.FormatInt(id, 10),
		Status:     eventType,
		Reason:     reason,
		MerchantID: accountID,
		StoreID:    storeID,
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("marshal merchant verification payload: %w", err)
	}
	return &MerchantVerificationOutbox{
		ID:            id,
		AccountID:     accountID,
		EventType:     eventType,
		Reason:        reason,
		Payload:       payload,
		Status:        MerchantVerificationOutboxPending,
		NextAttemptAt: now,
		CreatedAt:     now,
	}, nil
}
