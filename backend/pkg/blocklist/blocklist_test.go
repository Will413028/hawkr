package blocklist_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/blocklist"
	"github.com/alicebob/miniredis/v2"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/redis/go-redis/v9"
)

// newTestClient returns a client backed by a per-test in-process miniredis.
//
// It used to dial a real 127.0.0.1:6379 and t.Skip when that failed, which on
// any machine whose Redis has a password — the normal state here and in CI —
// meant the five tests hanging off it reported success while asserting nothing.
// Same fix as the gateway middleware suite (commit 1128811d7) and pkg/redis
// (commit 47f4a4061): remove the skip by removing its cause, never by loosening
// its condition. Per-test isolation is a bonus over the old shared real server:
// the block IDs no longer have to be unique across the package.
func newTestClient(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

func cleanup(t *testing.T, rdb *redis.Client, ids ...int64) {
	t.Helper()
	ctx := context.Background()
	for _, id := range ids {
		s := strconv.FormatInt(id, 10)
		_ = rdb.Del(ctx, "block:out:"+s, "block:in:"+s).Err()
	}
}

func TestIsBlocked_DirectAfterAdd(t *testing.T) {
	rdb := newTestClient(t)
	const a, b = int64(9900000001), int64(9900000002)
	cleanup(t, rdb, a, b)
	defer cleanup(t, rdb, a, b)

	ctx := context.Background()
	if err := blocklist.Add(ctx, rdb, a, b); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if !blocklist.IsBlocked(ctx, rdb, a, b) {
		t.Fatal("expected IsBlocked(a, b) = true after Add(a, b)")
	}
}

func TestIsBlocked_ReverseDirection(t *testing.T) {
	rdb := newTestClient(t)
	const a, b = int64(9900000003), int64(9900000004)
	cleanup(t, rdb, a, b)
	defer cleanup(t, rdb, a, b)

	ctx := context.Background()
	if err := blocklist.Add(ctx, rdb, a, b); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if !blocklist.IsBlocked(ctx, rdb, b, a) {
		t.Fatal("expected IsBlocked(b, a) = true after Add(a, b) (mutual block via block:in reverse index)")
	}
}

func TestIsBlocked_FailOpenWhenRedisDown(t *testing.T) {
	badRdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:19999"})
	if blocklist.IsBlocked(context.Background(), badRdb, 1, 2) {
		t.Fatal("expected fail-open (false) when redis is unavailable")
	}
}

func TestIsBlocked_FailOpenWhenRedisIsNil(t *testing.T) {
	// nil client treated as unavailable — same fail-open as connection error.
	if blocklist.IsBlocked(context.Background(), nil, 1, 2) {
		t.Fatal("expected fail-open (false) when rdb is nil")
	}
}

func TestIsBlocked_IncrementsHitMetricWhenBlocked(t *testing.T) {
	rdb := newTestClient(t)
	const a, b = int64(9900000007), int64(9900000008)
	cleanup(t, rdb, a, b)
	defer cleanup(t, rdb, a, b)

	ctx := context.Background()
	if err := blocklist.Add(ctx, rdb, a, b); err != nil {
		t.Fatalf("Add: %v", err)
	}

	before := testutil.ToFloat64(blocklist.MetricCacheHit())
	_ = blocklist.IsBlocked(ctx, rdb, a, b)
	after := testutil.ToFloat64(blocklist.MetricCacheHit())

	if after-before != 1 {
		t.Fatalf("expected hit counter +1, got +%v", after-before)
	}
}

func TestIsBlocked_IncrementsMissMetricWhenNotBlocked(t *testing.T) {
	rdb := newTestClient(t)
	const a, b = int64(9900000009), int64(9900000010)
	cleanup(t, rdb, a, b)
	defer cleanup(t, rdb, a, b)

	before := testutil.ToFloat64(blocklist.MetricCacheMiss())
	_ = blocklist.IsBlocked(context.Background(), rdb, a, b)
	after := testutil.ToFloat64(blocklist.MetricCacheMiss())

	if after-before != 1 {
		t.Fatalf("expected miss counter +1, got +%v", after-before)
	}
}

func TestIsBlocked_IncrementsUnavailableMetricOnRedisError(t *testing.T) {
	badRdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:19999"})
	before := testutil.ToFloat64(blocklist.MetricCacheUnavailable())
	_ = blocklist.IsBlocked(context.Background(), badRdb, 1, 2)
	after := testutil.ToFloat64(blocklist.MetricCacheUnavailable())

	if after-before != 1 {
		t.Fatalf("expected unavailable counter +1, got +%v", after-before)
	}
}

func TestRemove_ClearsBothDirections(t *testing.T) {
	rdb := newTestClient(t)
	const a, b = int64(9900000005), int64(9900000006)
	cleanup(t, rdb, a, b)
	defer cleanup(t, rdb, a, b)

	ctx := context.Background()
	if err := blocklist.Add(ctx, rdb, a, b); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := blocklist.Remove(ctx, rdb, a, b); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if blocklist.IsBlocked(ctx, rdb, a, b) {
		t.Fatal("expected IsBlocked(a, b) = false after Remove(a, b)")
	}
	if blocklist.IsBlocked(ctx, rdb, b, a) {
		t.Fatal("expected IsBlocked(b, a) = false after Remove(a, b) (reverse index also cleared)")
	}
}
