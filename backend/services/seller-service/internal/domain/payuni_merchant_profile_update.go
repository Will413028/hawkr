package domain

import (
	"context"
	"encoding/json"
	"time"
)

// PayuniMerchantProfileUpdateStatus is the lifecycle of one claimed update
// attempt (issue #2806 Phase 2).
type PayuniMerchantProfileUpdateStatus string

const (
	PayuniMerchantProfileUpdateInProgress PayuniMerchantProfileUpdateStatus = "IN_PROGRESS"
	PayuniMerchantProfileUpdateSucceeded  PayuniMerchantProfileUpdateStatus = "SUCCEEDED"
	PayuniMerchantProfileUpdateFailed     PayuniMerchantProfileUpdateStatus = "FAILED"
)

// PayuniMerchantProfileUpdate is one row of the append-only claim/audit
// ledger. ChangedFields is a REDACTED diff (field names + masked before/after
// — never full plaintext contact/address values); see the usecase layer for
// how it's built.
type PayuniMerchantProfileUpdate struct {
	CreatedAt       time.Time
	CompletedAt     *time.Time
	IdempotencyKey  string
	PayloadHash     string
	Status          PayuniMerchantProfileUpdateStatus
	Reason          string
	TraceID         string
	ProviderStatus  string
	ProviderMessage string
	ChangedFields   json.RawMessage
	ID              int64
	StoreID         int64
	AccountID       int64
}

// PayuniMerchantProfileUpdateRepository is the idempotency-key claim ledger
// for issue #2806 Phase 2. Claim-before-call: the usecase inserts an
// IN_PROGRESS row BEFORE calling PAYUni, so two concurrent requests with the
// same (store_id, idempotency_key) can never both reach the provider.
type PayuniMerchantProfileUpdateRepository interface {
	// Insert attempts to claim (store_id, idempotency_key). Returns
	// (row, true, nil) on success. Returns (nil, false, nil) — NOT an error —
	// when the key already exists; the caller must then call
	// FindByStoreAndKey to inspect what's there.
	Insert(ctx context.Context, u *PayuniMerchantProfileUpdate) (inserted bool, err error)
	FindByStoreAndKey(ctx context.Context, storeID int64, idempotencyKey string) (*PayuniMerchantProfileUpdate, error)
	MarkSucceeded(ctx context.Context, id int64, providerStatus, providerMessage string, completedAt time.Time) error
	MarkFailed(ctx context.Context, id int64, providerStatus, providerMessage string, completedAt time.Time) error
	// Delete removes a row so the same idempotency key can be re-claimed —
	// used when an existing FAILED or stale IN_PROGRESS row (same payload
	// hash) is being retried.
	Delete(ctx context.Context, id int64) error
}
