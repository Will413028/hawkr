// services/seller-service/internal/domain/merchant_verification_outbox_test.go
package domain_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

func TestNewMerchantVerificationOutboxRow_BuildsPendingRowWithEventIDPayload(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)

	row, err := domain.NewMerchantVerificationOutboxRow(12345, 100, 200, "approved", "", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if row.ID != 12345 || row.AccountID != 100 || row.EventType != "approved" {
		t.Fatalf("row = %+v, want ID=12345 AccountID=100 EventType=approved", row)
	}
	if row.Status != domain.MerchantVerificationOutboxPending {
		t.Fatalf("Status = %v, want pending", row.Status)
	}
	if !row.NextAttemptAt.Equal(now) || !row.CreatedAt.Equal(now) {
		t.Fatalf("NextAttemptAt/CreatedAt = %v/%v, want both %v", row.NextAttemptAt, row.CreatedAt, now)
	}

	var event domain.MerchantVerificationCompletedEvent
	if err := json.Unmarshal(row.Payload, &event); err != nil {
		t.Fatalf("payload did not unmarshal: %v", err)
	}
	if event.EventID != "12345" {
		t.Errorf("EventID = %q, want %q (decimal string of the outbox row's own id)", event.EventID, "12345")
	}
	if event.Status != "approved" || event.MerchantID != 100 {
		t.Errorf("event = %+v, want Status=approved MerchantID=100", event)
	}
	// accountID (100) and storeID (200) are deliberately distinct values here
	// so a StoreID assertion of 200 can only pass if the constructor threaded
	// the right positional argument into the right field — see
	// TestNewMerchantVerificationOutboxRow_AccountIDAndStoreIDDoNotSwap for the
	// dedicated argument-order-swap guard.
	if event.StoreID != 200 {
		t.Errorf("event.StoreID = %d, want 200", event.StoreID)
	}
}

// TestNewMerchantVerificationOutboxRow_AccountIDAndStoreIDDoNotSwap locks in
// that accountID and storeID (issue #2707 followup, Phase A) land in the
// correct wire fields (merchant_id vs store_id respectively) even though both
// are adjacent int64 positional parameters — an accidental swap at any of the
// six producer call sites would silently publish the wrong id in the wrong
// identity space with no compile-time or runtime signal.
func TestNewMerchantVerificationOutboxRow_AccountIDAndStoreIDDoNotSwap(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)

	const accountID, storeID int64 = 111, 222
	row, err := domain.NewMerchantVerificationOutboxRow(1, accountID, storeID, "approved", "", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var event domain.MerchantVerificationCompletedEvent
	if err := json.Unmarshal(row.Payload, &event); err != nil {
		t.Fatalf("payload did not unmarshal: %v", err)
	}
	if event.MerchantID != accountID {
		t.Errorf("event.MerchantID = %d, want accountID=%d", event.MerchantID, accountID)
	}
	if event.StoreID != storeID {
		t.Errorf("event.StoreID = %d, want storeID=%d", event.StoreID, storeID)
	}
}

func TestMerchantVerificationOutbox_SatisfiesOutboxRowInterface(t *testing.T) {
	row := &domain.MerchantVerificationOutbox{ID: 7, AttemptCount: 3}
	if row.OutboxID() != 7 {
		t.Errorf("OutboxID() = %d, want 7", row.OutboxID())
	}
	if row.OutboxAttemptCount() != 3 {
		t.Errorf("OutboxAttemptCount() = %d, want 3", row.OutboxAttemptCount())
	}
}
