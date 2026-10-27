package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

// payuniMerchantProfileClaimStaleAfter bounds how long an IN_PROGRESS claim
// row blocks a retry with the same idempotency key. update_merchant's 7
// allowed fields are provider-side idempotent (last write wins, no financial
// side effect — see design doc §5.3), so it is safe to let a stuck claim be
// retried rather than wait indefinitely for a crashed request to resolve.
const payuniMerchantProfileClaimStaleAfter = 60 * time.Second

// UpdateMerchantProfileRequest is the usecase-level command for issue #2806
// Phase 2. This struct's field set IS the deny-by-default allowlist:
// Payment/Fee/Open/Close are never declared, so no caller can reach them
// through this path regardless of what the HTTP request contained.
type UpdateMerchantProfileRequest struct {
	StoreID         int64
	AccountID       int64
	IdempotencyKey  string
	ExpectedVersion int64
	TraceID         string
	ContactName     string
	ContactMobile   string
	ContactTel      string
	MerEmail        string
	MerAddr         string
	MerAddrPost     string
	MerTel          string
}

type UpdateMerchantProfileResult struct {
	MerchantProfileVersion int64
}

// changedFieldNames returns the names of fields the caller actually set
// (non-empty), in a fixed order, for the redacted audit diff and the payload
// hash. Values themselves are never included — see maskedDiff.
func (r *UpdateMerchantProfileRequest) changedFieldNames() []string {
	var names []string
	for _, f := range []struct {
		name  string
		value string
	}{
		{"contactName", r.ContactName}, {"contactMobile", r.ContactMobile},
		{"contactTel", r.ContactTel}, {"merEmail", r.MerEmail},
		{"merAddr", r.MerAddr}, {"merAddrPost", r.MerAddrPost}, {"merTel", r.MerTel},
	} {
		if f.value != "" {
			names = append(names, f.name)
		}
	}
	return names
}

// hashMerchantProfileChange computes the idempotency payload hash over the
// caller's requested changes only (not the store/account/key envelope) — two
// requests with the same StoreID+IdempotencyKey but different field values
// must hash differently.
func hashMerchantProfileChange(r *UpdateMerchantProfileRequest) string {
	payload, _ := json.Marshal(struct {
		ContactName, ContactMobile, ContactTel, MerEmail, MerAddr, MerAddrPost, MerTel string
	}{r.ContactName, r.ContactMobile, r.ContactTel, r.MerEmail, r.MerAddr, r.MerAddrPost, r.MerTel})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

// maskedDiff builds the redacted audit record: field names that changed plus
// a masked marker, never the raw value.
func maskedDiff(names []string) json.RawMessage {
	m := make(map[string]string, len(names))
	for _, n := range names {
		m[n] = "***"
	}
	out, _ := json.Marshal(m)
	return out
}

// UpdateMerchantProfile applies a limited, deny-by-default edit to the
// seller's PAYUni merchant contact/address fields (issue #2806 Phase 2).
//
// Flow: resolve registration by StoreID (never trust a client-supplied MerID)
// -> fast-path reject an obviously-stale ExpectedVersion before taking any
// lock -> acquire a per-store advisory lock (issue #2806 Phase 2 follow-up)
// and re-verify the version freshly under it, so a losing concurrent request
// is rejected here and never reaches PAYUni at all -> claim the idempotency
// key -> call PAYUni -> persist the new snapshot + advance
// merchant_profile_version, marking the claim SUCCEEDED, all only after
// PAYUni confirms success.
//
// The advisory-lock transaction almost always COMMITS, even on a business
// failure — MarkFailed audit rows must survive, not be rolled back with the
// rest of the transaction. It only ROLLS BACK for the two paths that bail
// out before writing anything: the fresh version-conflict re-check, and an
// unexpected infra error acquiring the lock or claiming the idempotency key.
// Every other branch (PAYUni rejection, the final CAS write failing, or
// success) commits and reports its outcome via the outcomeErr/result
// closures below.
func (u *PayuniUseCase) UpdateMerchantProfile(ctx context.Context, req *UpdateMerchantProfileRequest) (*UpdateMerchantProfileResult, error) {
	info, err := u.payuniRepo.GetByStoreID(ctx, req.StoreID)
	if err != nil {
		return nil, fmt.Errorf("get payuni registration: %w", err)
	}
	if info.PayuniMerID == "" {
		return nil, fmt.Errorf("store_id %d has no PAYUni MerID", req.StoreID)
	}
	if info.Status != domain.PayuniStatusActive {
		return nil, domain.ErrMerchantProfileNotActive
	}
	if req.ExpectedVersion != info.MerchantProfileVersion {
		// Fast-path only — saves taking the lock for an obviously-stale
		// request. The authoritative check happens again below, under the
		// lock, since this read can itself be stale by the time we get there.
		return nil, domain.ErrMerchantProfileVersionConflict
	}

	var (
		result     *UpdateMerchantProfileResult
		outcomeErr error
	)
	err = u.txManager.WithTx(ctx, func(ctx context.Context) error {
		if err := u.payuniRepo.AcquireMerchantProfileUpdateLock(ctx, req.StoreID); err != nil {
			return fmt.Errorf("acquire merchant profile update lock: %w", err)
		}

		// Re-read the authoritative version now that we hold the per-store
		// lock — the outer check above can be stale by the time we get here.
		// A losing concurrent request MUST be rejected here, before it ever
		// reaches the idempotency claim or PAYUni — that is the entire point
		// of this lock (see the function doc and issue #2806 follow-up).
		fresh, err := u.payuniRepo.GetByStoreID(ctx, req.StoreID)
		if err != nil {
			return fmt.Errorf("re-read payuni registration: %w", err)
		}
		if req.ExpectedVersion != fresh.MerchantProfileVersion {
			return domain.ErrMerchantProfileVersionConflict
		}
		info = fresh

		payloadHash := hashMerchantProfileChange(req)
		now := time.Now().UTC()
		claimID := u.tsid()

		inserted, err := u.payuniProfileUpdateRepo.Insert(ctx, &domain.PayuniMerchantProfileUpdate{
			ID: claimID, StoreID: req.StoreID, AccountID: req.AccountID,
			IdempotencyKey: req.IdempotencyKey, PayloadHash: payloadHash,
			Status:        domain.PayuniMerchantProfileUpdateInProgress,
			ChangedFields: maskedDiff(req.changedFieldNames()), TraceID: req.TraceID,
		})
		if err != nil {
			return fmt.Errorf("claim merchant profile update: %w", err)
		}

		claimIDToUse := claimID
		if !inserted {
			existing, err := u.payuniProfileUpdateRepo.FindByStoreAndKey(ctx, req.StoreID, req.IdempotencyKey)
			if err != nil {
				return fmt.Errorf("find existing merchant profile update claim: %w", err)
			}
			if existing.PayloadHash != payloadHash {
				return domain.ErrMerchantProfileIdempotencyKeyReused
			}
			switch existing.Status {
			case domain.PayuniMerchantProfileUpdateSucceeded:
				// Same key, same payload, already succeeded: replay, do not
				// call PAYUni again.
				result = &UpdateMerchantProfileResult{MerchantProfileVersion: info.MerchantProfileVersion}
				return nil
			case domain.PayuniMerchantProfileUpdateInProgress:
				if now.Sub(existing.CreatedAt) < payuniMerchantProfileClaimStaleAfter {
					return domain.ErrMerchantProfileIdempotencyKeyReused
				}
				// Stale claim (crashed mid-flight, or PAYUni never
				// responded) — safe to retry because these 7 fields are
				// provider-idempotent.
				if err := u.payuniProfileUpdateRepo.Delete(ctx, existing.ID); err != nil {
					return fmt.Errorf("delete stale merchant profile update claim: %w", err)
				}
			case domain.PayuniMerchantProfileUpdateFailed:
				if err := u.payuniProfileUpdateRepo.Delete(ctx, existing.ID); err != nil {
					return fmt.Errorf("delete failed merchant profile update claim: %w", err)
				}
			}
			// Re-claim after deleting the stale/failed row. The per-store
			// lock now means a second concurrent request never reaches this
			// branch for the same store at the same time — kept as
			// defense-in-depth (e.g. a second request for the SAME
			// idempotency key, which isn't what this lock scopes) rather
			// than relied upon.
			reinserted, err := u.payuniProfileUpdateRepo.Insert(ctx, &domain.PayuniMerchantProfileUpdate{
				ID: claimID, StoreID: req.StoreID, AccountID: req.AccountID,
				IdempotencyKey: req.IdempotencyKey, PayloadHash: payloadHash,
				Status:        domain.PayuniMerchantProfileUpdateInProgress,
				ChangedFields: maskedDiff(req.changedFieldNames()), TraceID: req.TraceID,
			})
			if err != nil {
				return fmt.Errorf("re-claim merchant profile update: %w", err)
			}
			if !reinserted {
				return domain.ErrMerchantProfileIdempotencyKeyReused
			}
		}

		resp, err := u.payuniClient.UpdateMerchant(ctx, &UpdateMerchantRequest{
			MerID: info.PayuniMerID, ContactName: req.ContactName, ContactMobile: req.ContactMobile,
			ContactTel: req.ContactTel, MerEmail: req.MerEmail, MerAddr: req.MerAddr,
			MerAddrPost: req.MerAddrPost, MerTel: req.MerTel,
		})
		if err != nil {
			_ = u.payuniProfileUpdateRepo.MarkFailed(ctx, claimIDToUse, "", err.Error(), time.Now().UTC())
			outcomeErr = fmt.Errorf("call payuni update_merchant: %w", err)
			return nil
		}
		if !resp.Success {
			// A transport failure surfaces here too (the adapter converts it
			// into a non-nil, !Success response instead of a non-nil err),
			// where ErrorCode/Message are both empty and the real text lives
			// in ErrorMsg — same resolution certification.go uses for the
			// sibling bind_merchant flow.
			errMsg := resp.Message
			if resp.ErrorMsg != "" {
				errMsg = resp.ErrorMsg
			}
			_ = u.payuniProfileUpdateRepo.MarkFailed(ctx, claimIDToUse, resp.ErrorCode, errMsg, time.Now().UTC())
			outcomeErr = fmt.Errorf("payuni update_merchant rejected: %s", errMsg)
			return nil
		}

		updated, err := u.payuniRepo.UpdateMerchantProfile(ctx, req.StoreID, req.ExpectedVersion,
			req.ContactName, req.ContactMobile, req.ContactTel, req.MerEmail, req.MerAddr, req.MerAddrPost, req.MerTel)
		if err != nil {
			if errors.Is(err, domain.ErrMerchantProfileVersionConflict) {
				// The fresh re-check above should make this unreachable
				// under the lock — kept as defense-in-depth. PAYUni already
				// accepted the change but our own optimistic lock lost a
				// race after the fact. Mark the claim FAILED (not
				// SUCCEEDED — our local snapshot did not actually advance)
				// so a retry with the same key can be safely reattempted
				// once the caller re-reads the current version.
				_ = u.payuniProfileUpdateRepo.MarkFailed(ctx, claimIDToUse, resp.ErrorCode, "local version conflict after provider success", time.Now().UTC())
				outcomeErr = err
				return nil
			}
			// Every other failure branch in this function marks the claim
			// FAILED before returning — do the same here so a claim row
			// never gets stuck permanently IN_PROGRESS on this path.
			_ = u.payuniProfileUpdateRepo.MarkFailed(ctx, claimIDToUse, resp.ErrorCode, fmt.Sprintf("persist merchant profile: %v", err), time.Now().UTC())
			outcomeErr = fmt.Errorf("persist merchant profile: %w", err)
			return nil
		}

		if err := u.payuniProfileUpdateRepo.MarkSucceeded(ctx, claimIDToUse, resp.ErrorCode, resp.Message, time.Now().UTC()); err != nil {
			return fmt.Errorf("mark merchant profile update succeeded: %w", err)
		}
		result = &UpdateMerchantProfileResult{MerchantProfileVersion: updated.MerchantProfileVersion}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	return result, nil
}
