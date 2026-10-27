package usecase

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

func TestUpdateMerchantProfile_HappyPath(t *testing.T) {
	payuniRepo := &mockPayuniRepo{
		GetByStoreIDFn: func(ctx context.Context, storeID int64) (*domain.PayuniRegistration, error) {
			return &domain.PayuniRegistration{ID: 1, StoreID: 999, PayuniMerID: "HAWK999", Status: domain.PayuniStatusActive, MerchantProfileVersion: 3}, nil
		},
		UpdateMerchantProfileFn: func(ctx context.Context, storeID, expectedVersion int64, contactName, contactMobile, contactTel, merEmail, merAddr, merAddrPost, merTel string) (*domain.PayuniRegistration, error) {
			if expectedVersion != 3 {
				t.Fatalf("expectedVersion = %d, want 3", expectedVersion)
			}
			return &domain.PayuniRegistration{StoreID: 999, MerchantProfileVersion: 4, ContactName: contactName}, nil
		},
	}
	updateRepo := &mockPayuniMerchantProfileUpdateRepo{
		InsertFn: func(ctx context.Context, u *domain.PayuniMerchantProfileUpdate) (bool, error) { return true, nil },
	}
	pc := &mockPayuniClient{
		UpdateMerchantFn: func(ctx context.Context, req *UpdateMerchantRequest) (*PayuniResponse, error) {
			if req.MerID != "HAWK999" {
				t.Fatalf("MerID = %q, want HAWK999 (must be server-resolved, not client-supplied)", req.MerID)
			}
			return &PayuniResponse{Success: true, Message: "ok"}, nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc, tsid: func() int64 { return 1 }, txManager: &mockTxManager{}}
	uc.payuniProfileUpdateRepo = updateRepo

	result, err := uc.UpdateMerchantProfile(context.Background(), &UpdateMerchantProfileRequest{
		StoreID: 999, AccountID: 100, IdempotencyKey: "key-happy0001", ExpectedVersion: 3,
		ContactName: "王小明",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.MerchantProfileVersion != 4 {
		t.Fatalf("MerchantProfileVersion = %d, want 4", result.MerchantProfileVersion)
	}
}

func TestUpdateMerchantProfile_IdempotentReplay_SameHash(t *testing.T) {
	callCount := 0
	payuniRepo := &mockPayuniRepo{
		GetByStoreIDFn: func(ctx context.Context, storeID int64) (*domain.PayuniRegistration, error) {
			return &domain.PayuniRegistration{ID: 1, StoreID: 999, PayuniMerID: "HAWK999", Status: domain.PayuniStatusActive, MerchantProfileVersion: 3}, nil
		},
	}
	req := &UpdateMerchantProfileRequest{StoreID: 999, AccountID: 100, IdempotencyKey: "key-replay0001", ExpectedVersion: 3, ContactName: "王小明"}
	hash := hashMerchantProfileChange(req)
	updateRepo := &mockPayuniMerchantProfileUpdateRepo{
		InsertFn: func(ctx context.Context, u *domain.PayuniMerchantProfileUpdate) (bool, error) { return false, nil },
		FindByStoreAndKeyFn: func(ctx context.Context, storeID int64, key string) (*domain.PayuniMerchantProfileUpdate, error) {
			return &domain.PayuniMerchantProfileUpdate{
				ID: 5, StoreID: 999, PayloadHash: hash,
				Status:    domain.PayuniMerchantProfileUpdateSucceeded,
				CreatedAt: time.Now().UTC(),
			}, nil
		},
	}
	pc := &mockPayuniClient{
		UpdateMerchantFn: func(ctx context.Context, req *UpdateMerchantRequest) (*PayuniResponse, error) {
			callCount++
			return &PayuniResponse{Success: true}, nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc, tsid: func() int64 { return 99 }, txManager: &mockTxManager{}}
	uc.payuniProfileUpdateRepo = updateRepo

	_, err := uc.UpdateMerchantProfile(context.Background(), req)
	if err != nil {
		t.Fatalf("replay of a SUCCEEDED same-hash claim must not error: %v", err)
	}
	if callCount != 0 {
		t.Fatalf("PAYUni must NOT be called again on a same-key-same-hash replay of a SUCCEEDED attempt, got %d calls", callCount)
	}
}

func TestUpdateMerchantProfile_IdempotencyKeyReused_DifferentHash(t *testing.T) {
	payuniRepo := &mockPayuniRepo{
		GetByStoreIDFn: func(ctx context.Context, storeID int64) (*domain.PayuniRegistration, error) {
			return &domain.PayuniRegistration{ID: 1, StoreID: 999, PayuniMerID: "HAWK999", Status: domain.PayuniStatusActive, MerchantProfileVersion: 3}, nil
		},
	}
	updateRepo := &mockPayuniMerchantProfileUpdateRepo{
		InsertFn: func(ctx context.Context, u *domain.PayuniMerchantProfileUpdate) (bool, error) { return false, nil },
		FindByStoreAndKeyFn: func(ctx context.Context, storeID int64, key string) (*domain.PayuniMerchantProfileUpdate, error) {
			return &domain.PayuniMerchantProfileUpdate{
				ID: 5, StoreID: 999, PayloadHash: "a-completely-different-hash",
				Status: domain.PayuniMerchantProfileUpdateSucceeded, CreatedAt: time.Now().UTC(),
			}, nil
		},
	}
	pc := &mockPayuniClient{
		UpdateMerchantFn: func(ctx context.Context, req *UpdateMerchantRequest) (*PayuniResponse, error) {
			t.Fatal("PAYUni must not be called when the idempotency key is reused with a different payload")
			return nil, nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc, tsid: func() int64 { return 99 }, txManager: &mockTxManager{}}
	uc.payuniProfileUpdateRepo = updateRepo

	_, err := uc.UpdateMerchantProfile(context.Background(), &UpdateMerchantProfileRequest{
		StoreID: 999, AccountID: 100, IdempotencyKey: "key-reused0001", ExpectedVersion: 3, ContactName: "王小明",
	})
	if !errors.Is(err, domain.ErrMerchantProfileIdempotencyKeyReused) {
		t.Fatalf("err = %v, want domain.ErrMerchantProfileIdempotencyKeyReused", err)
	}
}

func TestUpdateMerchantProfile_NotActive_Rejected(t *testing.T) {
	payuniRepo := &mockPayuniRepo{
		GetByStoreIDFn: func(ctx context.Context, storeID int64) (*domain.PayuniRegistration, error) {
			return &domain.PayuniRegistration{ID: 1, StoreID: 999, PayuniMerID: "HAWK999", Status: domain.PayuniStatusPending, MerchantProfileVersion: 3}, nil
		},
	}
	pc := &mockPayuniClient{
		UpdateMerchantFn: func(ctx context.Context, req *UpdateMerchantRequest) (*PayuniResponse, error) {
			t.Fatal("PAYUni must not be called when the registration is not active")
			return nil, nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc, tsid: func() int64 { return 1 }}
	uc.payuniProfileUpdateRepo = &mockPayuniMerchantProfileUpdateRepo{}

	_, err := uc.UpdateMerchantProfile(context.Background(), &UpdateMerchantProfileRequest{
		StoreID: 999, AccountID: 100, IdempotencyKey: "key-notactv0001", ExpectedVersion: 3, ContactName: "王小明",
	})
	if !errors.Is(err, domain.ErrMerchantProfileNotActive) {
		t.Fatalf("err = %v, want domain.ErrMerchantProfileNotActive (registration Status=Pending, not Active)", err)
	}
}

// TestUpdateMerchantProfile_ReclaimRace_SecondInsertLoses proves the re-claim
// path (delete a stale/failed claim, then re-Insert) checks the re-Insert's
// `inserted` bool instead of treating any non-error Insert as success. Two
// concurrent requests can both reach this branch for the same stale/failed
// claim (both Delete succeed — the second a no-op — and both Insert), but
// only one Insert actually inserts; the other must be rejected, not proceed
// to call PAYUni.
func TestUpdateMerchantProfile_ReclaimRace_SecondInsertLoses(t *testing.T) {
	payuniRepo := &mockPayuniRepo{
		GetByStoreIDFn: func(ctx context.Context, storeID int64) (*domain.PayuniRegistration, error) {
			return &domain.PayuniRegistration{ID: 1, StoreID: 999, PayuniMerID: "HAWK999", Status: domain.PayuniStatusActive, MerchantProfileVersion: 3}, nil
		},
		UpdateMerchantProfileFn: func(ctx context.Context, storeID, expectedVersion int64, contactName, contactMobile, contactTel, merEmail, merAddr, merAddrPost, merTel string) (*domain.PayuniRegistration, error) {
			return &domain.PayuniRegistration{StoreID: 999, MerchantProfileVersion: 4, ContactName: contactName}, nil
		},
	}
	req := &UpdateMerchantProfileRequest{StoreID: 999, AccountID: 100, IdempotencyKey: "key-race00001", ExpectedVersion: 3, ContactName: "王小明"}
	hash := hashMerchantProfileChange(req)

	// The initial claim Insert (top of UpdateMerchantProfile) always finds
	// the key already occupied by the stale/failed claim FindByStoreAndKeyFn
	// returns below, so it always reports inserted=false — that is what
	// routes both calls into the delete-then-re-claim branch under test.
	// The RE-CLAIM Insert is the interesting one: the first
	// UpdateMerchantProfile call's re-claim wins the race (true), the
	// second's loses it (false), exactly as two concurrent requests racing
	// on the same stale/failed claim would.
	insertResults := []bool{false, true, false, false}
	insertCall := 0
	updateRepo := &mockPayuniMerchantProfileUpdateRepo{
		InsertFn: func(ctx context.Context, u *domain.PayuniMerchantProfileUpdate) (bool, error) {
			result := insertResults[insertCall]
			insertCall++
			return result, nil
		},
		FindByStoreAndKeyFn: func(ctx context.Context, storeID int64, key string) (*domain.PayuniMerchantProfileUpdate, error) {
			return &domain.PayuniMerchantProfileUpdate{
				ID: 5, StoreID: 999, PayloadHash: hash,
				Status: domain.PayuniMerchantProfileUpdateFailed, CreatedAt: time.Now().UTC(),
			}, nil
		},
	}
	payuniCallCount := 0
	pc := &mockPayuniClient{
		UpdateMerchantFn: func(ctx context.Context, req *UpdateMerchantRequest) (*PayuniResponse, error) {
			payuniCallCount++
			return &PayuniResponse{Success: true, Message: "ok"}, nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc, tsid: func() int64 { return 1 }, txManager: &mockTxManager{}}
	uc.payuniProfileUpdateRepo = updateRepo

	if _, err := uc.UpdateMerchantProfile(context.Background(), req); err != nil {
		t.Fatalf("first call (wins the re-claim race): unexpected error: %v", err)
	}
	if payuniCallCount != 1 {
		t.Fatalf("first call: PAYUni call count = %d, want 1", payuniCallCount)
	}

	_, err := uc.UpdateMerchantProfile(context.Background(), req)
	if !errors.Is(err, domain.ErrMerchantProfileIdempotencyKeyReused) {
		t.Fatalf("second call (loses the re-claim race): err = %v, want domain.ErrMerchantProfileIdempotencyKeyReused", err)
	}
	if payuniCallCount != 1 {
		t.Fatalf("second call: PAYUni call count = %d, want still 1 (must NOT call PAYUni after losing the re-claim race)", payuniCallCount)
	}
}

func TestUpdateMerchantProfile_VersionConflict(t *testing.T) {
	payuniRepo := &mockPayuniRepo{
		GetByStoreIDFn: func(ctx context.Context, storeID int64) (*domain.PayuniRegistration, error) {
			return &domain.PayuniRegistration{ID: 1, StoreID: 999, PayuniMerID: "HAWK999", Status: domain.PayuniStatusActive, MerchantProfileVersion: 5}, nil
		},
	}
	uc := &PayuniUseCase{payuniRepo: payuniRepo, tsid: func() int64 { return 1 }}
	uc.payuniProfileUpdateRepo = &mockPayuniMerchantProfileUpdateRepo{}

	_, err := uc.UpdateMerchantProfile(context.Background(), &UpdateMerchantProfileRequest{
		StoreID: 999, AccountID: 100, IdempotencyKey: "key-stale00001", ExpectedVersion: 3, ContactName: "王小明",
	})
	if !errors.Is(err, domain.ErrMerchantProfileVersionConflict) {
		t.Fatalf("err = %v, want domain.ErrMerchantProfileVersionConflict (client's ExpectedVersion=3 vs actual 5)", err)
	}
}

// blockingTxManager models pg_advisory_xact_lock's "released only at
// transaction commit/rollback" semantics for tests: the closure (transaction
// body) is expected to acquire the mutex as its first action (via the mock
// repo's AcquireMerchantProfileUpdateLockFn calling mu.Lock()); WithTx
// releases it once the closure returns, regardless of outcome — exactly like
// a real advisory lock releasing on either commit or rollback. The shared
// mockTxManager (mock_test.go) is a bare passthrough and cannot model real
// mutual exclusion, so concurrency tests need this instead.
type blockingTxManager struct{ mu sync.Mutex }

func (b *blockingTxManager) WithTx(ctx context.Context, fn func(context.Context) error) error {
	err := fn(ctx)
	b.mu.Unlock()
	return err
}

// TestUpdateMerchantProfile_ConcurrentRequests_LoserNeverCallsPayuni proves
// the per-store advisory lock actually prevents a losing concurrent request
// from ever reaching PAYUni — not just serializes when the conflict is
// detected. Two goroutines race with the same starting version, different
// Idempotency-Keys, different fields; exactly one must succeed and PAYUni
// must be called exactly once total (issue #2806 Phase 2 follow-up — see
// seller-flow-verify/reference/102's live-verified "併發下的 PayUni／本地
// 分歧窗口" finding: before this fix, both concurrent requests reached
// PAYUni and only the local CAS decided the winner afterward).
func TestUpdateMerchantProfile_ConcurrentRequests_LoserNeverCallsPayuni(t *testing.T) {
	var version atomic.Int64
	version.Store(3)
	var lockMgr blockingTxManager

	var claimsMu sync.Mutex
	claims := map[string]bool{}

	payuniRepo := &mockPayuniRepo{
		GetByStoreIDFn: func(ctx context.Context, storeID int64) (*domain.PayuniRegistration, error) {
			return &domain.PayuniRegistration{ID: 1, StoreID: 999, PayuniMerID: "HAWK999", Status: domain.PayuniStatusActive, MerchantProfileVersion: version.Load()}, nil
		},
		UpdateMerchantProfileFn: func(ctx context.Context, storeID, expectedVersion int64, contactName, contactMobile, contactTel, merEmail, merAddr, merAddrPost, merTel string) (*domain.PayuniRegistration, error) {
			if version.Load() != expectedVersion {
				return nil, domain.ErrMerchantProfileVersionConflict
			}
			version.Add(1)
			return &domain.PayuniRegistration{StoreID: 999, MerchantProfileVersion: version.Load()}, nil
		},
	}
	// AcquireMerchantProfileUpdateLockFn is the mock repo's side of the real
	// pg_advisory_xact_lock call — it blocks (via lockMgr.mu) until the other
	// goroutine's transaction (WithTx closure) has returned and released it.
	payuniRepo.AcquireMerchantProfileUpdateLockFn = func(ctx context.Context, storeID int64) error {
		lockMgr.mu.Lock()
		return nil
	}
	updateRepo := &mockPayuniMerchantProfileUpdateRepo{
		InsertFn: func(ctx context.Context, u *domain.PayuniMerchantProfileUpdate) (bool, error) {
			claimsMu.Lock()
			defer claimsMu.Unlock()
			if claims[u.IdempotencyKey] {
				return false, nil
			}
			claims[u.IdempotencyKey] = true
			return true, nil
		},
	}
	var payuniCalls atomic.Int32
	pc := &mockPayuniClient{
		UpdateMerchantFn: func(ctx context.Context, req *UpdateMerchantRequest) (*PayuniResponse, error) {
			payuniCalls.Add(1)
			return &PayuniResponse{Success: true, Message: "ok"}, nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc, tsid: func() int64 { return time.Now().UnixNano() }, txManager: &lockMgr}
	uc.payuniProfileUpdateRepo = updateRepo

	var wg sync.WaitGroup
	results := make([]error, 2)
	reqs := []*UpdateMerchantProfileRequest{
		{StoreID: 999, AccountID: 100, IdempotencyKey: "key-race-A", ExpectedVersion: 3, ContactMobile: "0911111111"},
		{StoreID: 999, AccountID: 100, IdempotencyKey: "key-race-B", ExpectedVersion: 3, MerTel: "02-11111111"},
	}
	wg.Add(2)
	for i := range reqs {
		go func(i int) {
			defer wg.Done()
			_, err := uc.UpdateMerchantProfile(context.Background(), reqs[i])
			results[i] = err
		}(i)
	}
	wg.Wait()

	if got := payuniCalls.Load(); got != 1 {
		t.Fatalf("PAYUni was called %d times, want exactly 1 — a losing concurrent request must never reach PAYUni", got)
	}
	successCount, conflictCount := 0, 0
	for _, err := range results {
		switch {
		case err == nil:
			successCount++
		case errors.Is(err, domain.ErrMerchantProfileVersionConflict):
			conflictCount++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if successCount != 1 || conflictCount != 1 {
		t.Fatalf("successCount=%d conflictCount=%d, want exactly one of each", successCount, conflictCount)
	}
}
