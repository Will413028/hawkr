package domain

import (
	"encoding/json"
	"testing"
	"time"
)

// NewLifecycleOutboxRow is the shared constructor S6 (re-onboard) and S7
// (delete) call to build+marshal a pending lifecycle outbox row without
// duplicating the marshal/build boilerplate.
func TestNewLifecycleOutboxRow_MarshalsPayloadAndSetsPendingDefaults(t *testing.T) {
	now := time.Now()
	payload := SellerLifecyclePayload{AccountID: 42, StoreID: 777}

	row, err := NewLifecycleOutboxRow(1, 42, SellerLifecycleEventOnboarded, payload, now)
	if err != nil {
		t.Fatalf("NewLifecycleOutboxRow: %v", err)
	}

	if row.ID != 1 || row.AccountID != 42 {
		t.Fatalf("row identity = {ID:%d AccountID:%d}, want {1 42}", row.ID, row.AccountID)
	}
	if row.EventType != SellerLifecycleEventOnboarded {
		t.Errorf("EventType = %q, want %q", row.EventType, SellerLifecycleEventOnboarded)
	}
	if row.Status != SellerDeletedOutboxPending {
		t.Errorf("Status = %q, want pending", row.Status)
	}
	if row.AttemptCount != 0 {
		t.Errorf("AttemptCount = %d, want 0", row.AttemptCount)
	}
	if !row.NextAttemptAt.Equal(now) || !row.CreatedAt.Equal(now) {
		t.Errorf("NextAttemptAt/CreatedAt = %v/%v, want both %v", row.NextAttemptAt, row.CreatedAt, now)
	}

	var got SellerLifecyclePayload
	if err := json.Unmarshal(row.Payload, &got); err != nil {
		t.Fatalf("unmarshal row.Payload: %v", err)
	}
	if got != payload {
		t.Errorf("round-tripped payload = %+v, want %+v", got, payload)
	}
}

func TestNewLifecycleOutboxRow_MarshalErrorReturnsError(t *testing.T) {
	// channels are not JSON-marshalable — exercises the error path.
	_, err := NewLifecycleOutboxRow(1, 42, SellerLifecycleEventDeleted, make(chan int), time.Now())
	if err == nil {
		t.Fatal("expected marshal error, got nil")
	}
}
