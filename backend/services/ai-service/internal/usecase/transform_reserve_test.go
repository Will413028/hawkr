//go:build integration

package usecase

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"errors"

	"github.com/Will413028/hawkr/ai-service/internal/domain"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pkgtsid "github.com/Will413028/hawkr/go-pkg/tsid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// setupAiTestPool is defined in dbtest_test.go (same package, same directory).

// fakeGemini returns a valid single-image response after a delay wide enough to
// force the TOCTOU window if the reserve is not atomic.
func fakeGemini(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(60 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"aGVsbG8="}}]}}]}`))
	}))
}

// buildTestTransformUseCase 集中組出測試用的 TransformUseCase——跟 production 的
// NewTransformUseCase 不同，這裡繞過 config.Config（測試不需要真的 Gemini API key
// 或 URL），txManager 用同一個 pool 現場建構，跟 production 的 NewTransformUseCase
// 內部做的事完全一樣。
//
// 沒有沿用 newTestTransformUseCase 這個更順口的名字，是因為 transform_gemini_test.go
// （同 package，這波遷移沒動這個檔案）已經定義了一個同名但不相關的 helper：
// func newTestTransformUseCase(t *testing.T, handler http.HandlerFunc) *TransformUseCase，
// 簽名不同（只設 apiKey/apiURL，不碰 DB），重用這個名字在這裡會是編譯期重複宣告。
func buildTestTransformUseCase(pool *pgxpool.Pool, apiURL string) *TransformUseCase {
	return &TransformUseCase{
		pool:      pool,
		txManager: database.NewTxManager(pool),
		apiKey:    "test",
		apiURL:    apiURL,
		genID:     pkgtsid.NewGenerator(),
	}
}

func insertAiUsage(ctx context.Context, t *testing.T, pool *pgxpool.Pool, storeID identity.StoreID, weekStart string, usageCount int) {
	t.Helper()
	weekDate, err := time.Parse("2006-01-02", weekStart)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO ai_usage (store_id, week_start, usage_count) VALUES ($1, $2, $3)`,
		int64(storeID), weekDate, usageCount)
	require.NoError(t, err)
}

func insertAiQuota(ctx context.Context, t *testing.T, pool *pgxpool.Pool, storeID identity.StoreID, weeklyLimit int) {
	t.Helper()
	_, err := pool.Exec(ctx,
		`INSERT INTO ai_quotas (store_id, weekly_limit, memo) VALUES ($1, $2, $3)`,
		int64(storeID), weeklyLimit, "")
	require.NoError(t, err)
}

// insertExpiredAiUsageReservation 直接種一列已過期的 ai_usage_reservations，
// 模擬 process 在 confirmSlot/releaseSlot 之前被殺（rolling deploy SIGKILL）——
// reserveSlot 建立了列，但沒有任何後續呼叫把它刪掉，只能靠 expires_at 過期自然失效。
func insertExpiredAiUsageReservation(ctx context.Context, t *testing.T, pool *pgxpool.Pool, id int64, storeID identity.StoreID, weekStart string) {
	t.Helper()
	weekDate, err := time.Parse("2006-01-02", weekStart)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO ai_usage_reservations (id, store_id, week_start, expires_at) VALUES ($1, $2, $3, $4)`,
		id, int64(storeID), weekDate, time.Now().Add(-time.Minute))
	require.NoError(t, err)
}

func getAiUsageCount(ctx context.Context, t *testing.T, pool *pgxpool.Pool, storeID identity.StoreID, weekStart string) int {
	t.Helper()
	weekDate, err := time.Parse("2006-01-02", weekStart)
	require.NoError(t, err)
	var count int
	err = pool.QueryRow(ctx,
		`SELECT usage_count FROM ai_usage WHERE store_id = $1 AND week_start = $2`,
		int64(storeID), weekDate).Scan(&count)
	require.NoError(t, err, "ai_usage row must exist for store %d", storeID)
	return count
}

func countAiUsageReservations(ctx context.Context, t *testing.T, pool *pgxpool.Pool, storeID identity.StoreID) int64 {
	t.Helper()
	var count int64
	err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM ai_usage_reservations WHERE store_id = $1`,
		int64(storeID)).Scan(&count)
	require.NoError(t, err)
	return count
}

func TestTransform_ConcurrentReserve_NeverExceedsLimit(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupAiTestPool(ctx, t)
	defer cleanup()
	srv := fakeGemini(t)
	defer srv.Close()

	uc := buildTestTransformUseCase(pool, srv.URL)

	const storeID identity.StoreID = 12345
	const goroutines = 30
	limit := domain.DefaultWeeklyLimit // 10

	var wg sync.WaitGroup
	var success, quota int64
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := uc.Transform(ctx, storeID, []byte("img"), "image/jpeg", "make it nice")
			if err == nil {
				atomic.AddInt64(&success, 1)
				return
			}
			var qe *QuotaExceededError
			if errors.As(err, &qe) {
				atomic.AddInt64(&quota, 1)
			} else {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	require.Equal(t, int64(limit), success, "exactly limit edits succeed")
	require.Equal(t, int64(goroutines-limit), quota, "the rest are quota-exceeded")

	require.Equal(t, limit, getAiUsageCount(ctx, t, pool, storeID, currentWeekStart()), "stored usage never exceeds limit")
}

func TestTransform_CustomLimitZero_Rejects(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupAiTestPool(ctx, t)
	defer cleanup()
	srv := fakeGemini(t)
	defer srv.Close()

	const storeID identity.StoreID = 777
	insertAiQuota(ctx, t, pool, storeID, 0)

	uc := buildTestTransformUseCase(pool, srv.URL)
	_, err := uc.Transform(ctx, storeID, []byte("img"), "image/jpeg", "x")
	var qe *QuotaExceededError
	require.True(t, errors.As(err, &qe), "disabled store is rejected")
}

func TestTransform_DBError_FailsClosed(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupAiTestPool(ctx, t)
	srv := fakeGemini(t)
	defer srv.Close()
	uc := buildTestTransformUseCase(pool, srv.URL)

	cleanup() // kill the DB before the call

	_, err := uc.Transform(ctx, 999, []byte("img"), "image/jpeg", "x")
	require.Error(t, err, "a dead DB must fail closed, not silently allow")
	var qe *QuotaExceededError
	require.False(t, errors.As(err, &qe), "a DB error is an infra error, not a quota verdict")
}

func TestTransform_GeminiFailure_RefundsSlot(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupAiTestPool(ctx, t)
	defer cleanup()

	// Fake Gemini that always fails (callGemini treats an `error` body as failure).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"boom"}}`))
	}))
	defer srv.Close()

	const storeID identity.StoreID = 555
	const seeded = 4 // below the default limit of 10
	insertAiUsage(ctx, t, pool, storeID, currentWeekStart(), seeded)

	uc := buildTestTransformUseCase(pool, srv.URL)
	_, err := uc.Transform(ctx, storeID, []byte("img"), "image/jpeg", "x")
	require.Error(t, err, "a failed Gemini call surfaces an error")
	var qe *QuotaExceededError
	require.False(t, errors.As(err, &qe), "a gemini failure is not a quota verdict")

	require.Equal(t, seeded, getAiUsageCount(ctx, t, pool, storeID, currentWeekStart()), "a failed Gemini call must never confirm — ai_usage stays untouched")
	require.Zero(t, countAiUsageReservations(ctx, t, pool, storeID), "the reservation is released, not left dangling")
}

// TestTransform_AbandonedReservation_ExpiresWithoutLeakingQuota simulates a
// process killed mid-flight (e.g. rolling deploy SIGKILL) between reserveSlot
// and confirmSlot/releaseSlot — it never gets to run either. The abandoned
// reservation row is seeded directly, already past its TTL, to stand in for
// that scenario without needing an actual process kill.
func TestTransform_AbandonedReservation_ExpiresWithoutLeakingQuota(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupAiTestPool(ctx, t)
	defer cleanup()
	srv := fakeGemini(t)
	defer srv.Close()

	uc := buildTestTransformUseCase(pool, srv.URL)

	const storeID identity.StoreID = 9001
	limit := domain.DefaultWeeklyLimit // 10
	weekStart := currentWeekStart()

	insertExpiredAiUsageReservation(ctx, t, pool, uc.genID(), storeID, weekStart)

	// Full quota must still be usable — the abandoned reservation must not
	// count against the limit once it's expired, even though its row still
	// physically exists (no cleanup job required for correctness).
	for i := 0; i < limit; i++ {
		_, err := uc.Transform(ctx, storeID, []byte("img"), "image/jpeg", "x")
		require.NoError(t, err, "abandoned reservation must not eat into quota (attempt %d)", i)
	}

	require.Equal(t, limit, getAiUsageCount(ctx, t, pool, storeID, weekStart), "exactly `limit` confirmed edits, unaffected by the abandoned reservation")
}

// TestTransform_StoreScoped_NewStoreGetsFreshQuota pins the ② re-key business
// decision (see TransformUseCase.Transform's doc comment): a seller who
// deletes and re-onboards gets a new store_id, and that new store starts with
// a fresh quota unaffected by the old store's usage. Resolution itself now
// happens at the gateway (account_id 命名債正名) — this test exercises the
// two store_ids directly rather than switching what one account_id resolves
// to mid-test.
func TestTransform_StoreScoped_NewStoreGetsFreshQuota(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupAiTestPool(ctx, t)
	defer cleanup()
	srv := fakeGemini(t)
	defer srv.Close()

	const oldStoreID identity.StoreID = 8001
	const newStoreID identity.StoreID = 8002
	limit := domain.DefaultWeeklyLimit // 10

	uc := buildTestTransformUseCase(pool, srv.URL)

	for i := 0; i < limit; i++ {
		_, err := uc.Transform(ctx, oldStoreID, []byte("img"), "image/jpeg", "x")
		require.NoError(t, err, "old store attempt %d", i)
	}
	_, err := uc.Transform(ctx, oldStoreID, []byte("img"), "image/jpeg", "x")
	var qe *QuotaExceededError
	require.True(t, errors.As(err, &qe), "old store's quota is exhausted")

	// Same person re-onboards under a new store-stint surrogate.
	_, err = uc.Transform(ctx, newStoreID, []byte("img"), "image/jpeg", "x")
	require.NoError(t, err, "the new store must have its own fresh quota, unaffected by the old store's usage")

	require.Equal(t, limit, getAiUsageCount(ctx, t, pool, oldStoreID, currentWeekStart()), "old store keeps its exhausted usage")
	require.Equal(t, 1, getAiUsageCount(ctx, t, pool, newStoreID, currentWeekStart()), "new store starts from zero")
}
