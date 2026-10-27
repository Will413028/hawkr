//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
)

// TestUpsertPayuniRegistration_RefusesMerIDChange proves the write-once
// invariant: MerIDFromAccountID(storeID) is a pure function of store_id, so
// every legitimate bind_merchant call for the same store_id computes the
// same MerID — a second Upsert carrying a DIFFERENT, non-empty MerID for an
// already-bound store_id can only be a bug (or manual DB tampering), never a
// real retry. Upsert must refuse it and leave the original value intact.
func TestUpsertPayuniRegistration_RefusesMerIDChange(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	repo := postgres.NewPayuniRepository(pool)

	const storeID int64 = 2614000100

	first := &domain.PayuniRegistration{
		ID: 1, AccountID: storeID, StoreID: storeID, ApplicationID: 1,
		PayuniMerID: "HAWK100", Status: domain.PayuniStatusPending,
	}
	require.NoError(t, repo.Upsert(ctx, first), "first Upsert (establishes the MerID)")

	second := &domain.PayuniRegistration{
		ID: 2, AccountID: storeID, StoreID: storeID, ApplicationID: 2,
		PayuniMerID: "HAWK999", Status: domain.PayuniStatusPending,
	}
	err := repo.Upsert(ctx, second)
	require.Error(t, err, "Upsert must refuse changing an already-set MerID")

	var got string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT payuni_mer_id FROM payuni_registrations WHERE store_id = $1`, storeID).
		Scan(&got))
	require.Equal(t, "HAWK100", got, "original MerID must be untouched after the refused Upsert")
}

// TestUpsertPayuniRegistration_AllowsSameMerIDReupsert proves the guard
// doesn't break the real production shape: a retry mints a fresh reg.ID but
// recomputes the SAME MerID (deterministic per store_id) — this must still
// succeed and update the other fields normally, not be treated as a change.
func TestUpsertPayuniRegistration_AllowsSameMerIDReupsert(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	repo := postgres.NewPayuniRepository(pool)

	const storeID int64 = 2614000200

	first := &domain.PayuniRegistration{
		ID: 10, AccountID: storeID, StoreID: storeID, ApplicationID: 1,
		PayuniMerID: "HAWK200", Status: domain.PayuniStatusPending,
	}
	require.NoError(t, repo.Upsert(ctx, first))

	retry := &domain.PayuniRegistration{
		ID: 20, AccountID: storeID, StoreID: storeID, ApplicationID: 2,
		PayuniMerID: "HAWK200", Status: domain.PayuniStatusActive,
	}
	require.NoError(t, repo.Upsert(ctx, retry), "same-MerID re-upsert must succeed")

	var gotMerID, gotStatus string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT payuni_mer_id, registration_status FROM payuni_registrations WHERE store_id = $1`, storeID).
		Scan(&gotMerID, &gotStatus))
	require.Equal(t, "HAWK200", gotMerID)
	require.Equal(t, domain.PayuniStatusActive.String(), gotStatus,
		"other fields must still update normally on a same-MerID re-upsert")
}

// TestUpsertPayuniRegistration_AllowsFirstMerIDAfterEmptyFailure proves the
// guard doesn't block the real failure-then-retry flow: a failed
// bind_merchant attempt persists a row with NO MerID at all (see
// usecase/certification.go's transport/business failure branches), and a
// later successful retry sets the real MerID for the first time — going
// from empty to a value is not a "change" and must be allowed.
func TestUpsertPayuniRegistration_AllowsFirstMerIDAfterEmptyFailure(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	repo := postgres.NewPayuniRepository(pool)

	const storeID int64 = 2614000300

	failed := &domain.PayuniRegistration{
		ID: 30, AccountID: storeID, StoreID: storeID, ApplicationID: 1,
	}
	failed.SetFailed("", "bind_merchant call failed: timeout", failed.UpdatedAt)
	require.NoError(t, repo.Upsert(ctx, failed), "failed attempt (no MerID yet) must persist")

	retry := &domain.PayuniRegistration{
		ID: 40, AccountID: storeID, StoreID: storeID, ApplicationID: 2,
		PayuniMerID: "HAWK300", Status: domain.PayuniStatusActive,
	}
	require.NoError(t, repo.Upsert(ctx, retry), "first successful MerID after an empty attempt must be allowed")

	var got string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT payuni_mer_id FROM payuni_registrations WHERE store_id = $1`, storeID).
		Scan(&got))
	require.Equal(t, "HAWK300", got)
}

// TestUpsertPayuniRegistration_AllowsFailedRetryAfterMerIDSet proves the
// reverse direction of the same forgiving-bookkeeping rule: if a store_id
// already has a real MerID and a LATER attempt fails before even calling
// PayUni (so its Upsert carries no MerID at all), that failure bookkeeping
// must still be allowed to write through — it doesn't collide with the
// write-once guard, which only fires when BOTH sides have a value and they
// genuinely differ. The pre-existing MerID must be left untouched (a
// failure attempt with no MerID has nothing to contribute there anyway).
func TestUpsertPayuniRegistration_AllowsFailedRetryAfterMerIDSet(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	repo := postgres.NewPayuniRepository(pool)

	const storeID int64 = 2614000400

	success := &domain.PayuniRegistration{
		ID: 50, AccountID: storeID, StoreID: storeID, ApplicationID: 1,
		PayuniMerID: "HAWK400", Status: domain.PayuniStatusActive,
	}
	require.NoError(t, repo.Upsert(ctx, success))

	laterFailure := &domain.PayuniRegistration{
		ID: 60, AccountID: storeID, StoreID: storeID, ApplicationID: 2,
	}
	laterFailure.SetFailed("", "bind_merchant call failed: timeout", laterFailure.UpdatedAt)
	require.NoError(t, repo.Upsert(ctx, laterFailure),
		"a no-MerID failure upsert must not be treated as a write-once violation")

	var gotMerID, gotStatus string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT payuni_mer_id, registration_status FROM payuni_registrations WHERE store_id = $1`, storeID).
		Scan(&gotMerID, &gotStatus))
	require.Equal(t, "HAWK400", gotMerID, "existing MerID must survive a no-MerID failure upsert")
	require.Equal(t, domain.PayuniStatusFailed.String(), gotStatus,
		"other fields (status) must still update even though MerID didn't change")
}
