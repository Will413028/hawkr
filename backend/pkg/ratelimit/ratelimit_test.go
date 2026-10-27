package ratelimit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	miniredisServer "github.com/alicebob/miniredis/v2/server"
	"github.com/redis/go-redis/v9"
)

// TestLimiter_FallbackEnforcesAndDegradesOpen covers the nil-Redis path (fallback
// only): the quota is enforced per window, and a non-positive limit is unlimited.
func TestLimiter_FallbackEnforcesAndDegradesOpen(t *testing.T) {
	l := New(nil, 100, nil)
	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		if !l.Allow(ctx, "k", 3, time.Minute) {
			t.Fatalf("request #%d denied, want allowed within quota", i)
		}
	}
	if l.Allow(ctx, "k", 3, time.Minute) {
		t.Fatal("request #4 allowed, want denied (over quota)")
	}
	// Different key has an independent counter.
	if !l.Allow(ctx, "other", 3, time.Minute) {
		t.Fatal("distinct key denied, want independent counter")
	}
	// Non-positive limit / window means unlimited.
	if !l.Allow(ctx, "k", 0, time.Minute) {
		t.Fatal("limit 0 should be treated as unlimited")
	}
}

// TestBoundedWindows_EvictsLRU proves the fallback is memory-bounded: at capacity,
// inserting a new key evicts the least-recently-used one.
func TestBoundedWindows_EvictsLRU(t *testing.T) {
	b := newBoundedWindows(2)
	b.allow("a", 5, time.Minute) // a
	b.allow("b", 5, time.Minute) // a, b
	b.allow("a", 5, time.Minute) // touch a → LRU is now b
	b.allow("c", 5, time.Minute) // insert c → evict b

	if _, ok := b.m["b"]; ok {
		t.Fatal("b should have been evicted as least-recently-used")
	}
	if len(b.m) != 2 {
		t.Fatalf("bounded size = %d, want 2", len(b.m))
	}
	if _, ok := b.m["a"]; !ok {
		t.Fatal("a should be retained (recently used)")
	}
	if _, ok := b.m["c"]; !ok {
		t.Fatal("c should be present (just inserted)")
	}
}

// TestBoundedWindows_WindowReset resets the counter after the window elapses.
func TestBoundedWindows_WindowReset(t *testing.T) {
	b := newBoundedWindows(10)
	w := 20 * time.Millisecond
	if !b.allow("k", 1, w) {
		t.Fatal("first request should pass")
	}
	if b.allow("k", 1, w) {
		t.Fatal("second request within window should be denied")
	}
	time.Sleep(w + 5*time.Millisecond)
	if !b.allow("k", 1, w) {
		t.Fatal("request after window elapsed should pass (counter reset)")
	}
}

// TestLimiter_CountDoesNotIncrement covers the nil-Redis path: Count must be a
// pure read. If it incremented, a caller that gates on it (auth-failure
// throttling) would trip its own threshold on ordinary traffic.
func TestLimiter_CountDoesNotIncrement(t *testing.T) {
	l := New(nil, 100, nil)
	ctx := context.Background()

	if n := l.Count(ctx, "absent", time.Minute); n != 0 {
		t.Fatalf("absent key: got %d, want 0", n)
	}
	// Reading repeatedly must not create or advance a counter.
	for i := 0; i < 5; i++ {
		if n := l.Count(ctx, "absent", time.Minute); n != 0 {
			t.Fatalf("read #%d of absent key: got %d, want 0 (Count must not increment)", i+1, n)
		}
	}

	for i := 0; i < 3; i++ {
		l.Allow(ctx, "k", 100, time.Minute)
	}
	for i := 0; i < 5; i++ {
		if n := l.Count(ctx, "k", time.Minute); n != 3 {
			t.Fatalf("read #%d after 3 increments: got %d, want 3 (Count must not increment)", i+1, n)
		}
	}
}

// An elapsed window reads as 0, matching Allow's reset semantics — otherwise a
// throttle keyed on Count would never release.
func TestLimiter_CountResetsWithWindow(t *testing.T) {
	l := New(nil, 100, nil)
	ctx := context.Background()

	l.Allow(ctx, "k", 100, 20*time.Millisecond)
	if n := l.Count(ctx, "k", 20*time.Millisecond); n != 1 {
		t.Fatalf("within window: got %d, want 1", n)
	}
	time.Sleep(30 * time.Millisecond)
	if n := l.Count(ctx, "k", 20*time.Millisecond); n != 0 {
		t.Fatalf("after window elapsed: got %d, want 0 (throttle must release)", n)
	}
}

// Reads must not keep an otherwise idle key alive in the bounded LRU, or a
// polled key could pin memory indefinitely.
func TestBoundedWindows_CountDoesNotRefreshLRU(t *testing.T) {
	b := newBoundedWindows(2)
	b.allow("a", 100, time.Minute)
	b.allow("b", 100, time.Minute)

	// Reading "a" must not promote it; inserting "c" should still evict "a".
	b.count("a", time.Minute)
	b.allow("c", 100, time.Minute)

	if n := b.count("a", time.Minute); n != 0 {
		t.Fatalf("key a: got %d, want 0 (reading must not protect it from LRU eviction)", n)
	}
	if n := b.count("c", time.Minute); n != 1 {
		t.Fatalf("key c: got %d, want 1", n)
	}
}

// blockScripting installs a pre-hook on mr that fails EVAL/EVALSHA with a
// generic (non-NOSCRIPT) error while leaving every other Redis command —
// INCR, GET, PEXPIRE, ... — working normally. This simulates a narrower
// failure than a full Redis outage: e.g. a proxy layer in front of Redis that
// disables scripting, or an EVAL call that times out while the connection
// itself is healthy. redis.Script.Run's EVALSHA-then-EVAL-on-NOSCRIPT retry
// never gets a NOSCRIPT reply here, so it returns the injected error directly
// instead of falling through to a successful EVAL.
func blockScripting(mr *miniredis.Miniredis) {
	mr.Server().SetPreHook(func(c *miniredisServer.Peer, cmd string, args ...string) bool {
		if cmd == "EVAL" || cmd == "EVALSHA" {
			c.WriteError("ERR simulated scripting failure (EVAL blocked)")
			return true
		}
		return false
	})
}

// TestLimiter_AllowStrict_MatchesAllowWhenRedisHealthy proves AllowStrict has
// the same admit/deny semantics as Allow when Redis and its scripting path
// both work: same count, same threshold comparison, same "count <= limit"
// boundary.
func TestLimiter_AllowStrict_MatchesAllowWhenRedisHealthy(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	l := New(rdb, 100, nil)
	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		ok, err := l.AllowStrict(ctx, "k", 3, time.Minute)
		if err != nil {
			t.Fatalf("request #%d: unexpected error: %v", i, err)
		}
		if !ok {
			t.Fatalf("request #%d denied, want allowed within quota (limit=3)", i)
		}
	}
	ok, err := l.AllowStrict(ctx, "k", 3, time.Minute)
	if err != nil {
		t.Fatalf("request #4: unexpected error: %v", err)
	}
	if ok {
		t.Fatal("request #4 allowed, want denied (over quota)")
	}
	// Non-positive limit / window means unlimited, matching Allow.
	if ok, err := l.AllowStrict(ctx, "k", 0, time.Minute); err != nil || !ok {
		t.Fatalf("limit 0: got (%v, %v), want (true, nil) — should be treated as unlimited", ok, err)
	}
}

// TestLimiter_AllowStrict_NilRedis_ReturnsErrNoRedis proves AllowStrict does
// NOT use the bounded in-memory fallback when there is no Redis client at
// all — unlike Allow, which would silently admit/deny via the fallback here.
// A caller that needs the real distributed count must be told there isn't
// one, not handed a fallback answer indistinguishable from a real one.
func TestLimiter_AllowStrict_NilRedis_ReturnsErrNoRedis(t *testing.T) {
	l := New(nil, 100, nil)
	ctx := context.Background()

	ok, err := l.AllowStrict(ctx, "k", 3, time.Minute)
	if !errors.Is(err, ErrNoRedis) {
		t.Fatalf("err = %v, want ErrNoRedis", err)
	}
	if ok {
		t.Fatal("got allowed=true on nil-Redis AllowStrict, want false (fail closed)")
	}
	// Confirm this is genuinely not touching the fallback: Allow on the same
	// Limiter and key must still work via the fallback, proving AllowStrict's
	// error isn't a side effect of a broken fallback.
	if !l.Allow(ctx, "k", 3, time.Minute) {
		t.Fatal("sanity check failed: Allow should still admit via fallback on this Limiter")
	}
}

// TestLimiter_AllowStrict_ScriptFailure_ReturnsErrorNotFallback is the
// regression test for the real gap AllowStrict exists to close: Redis itself
// is reachable (plain commands work) but the Lua/EVAL path alone fails — the
// scenario Allow cannot surface to its caller because it silently substitutes
// the per-pod in-memory fallback. AllowStrict must return an error instead of
// a fallback-derived bool, so the caller can detect and fail closed on this
// specific failure mode.
func TestLimiter_AllowStrict_ScriptFailure_ReturnsErrorNotFallback(t *testing.T) {
	mr := miniredis.RunT(t)
	blockScripting(mr)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	l := New(rdb, 100, nil)
	ctx := context.Background()

	// Sanity: plain Redis commands still work — this is not a full outage.
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatalf("sanity check failed: PING should succeed with scripting alone blocked: %v", err)
	}

	ok, err := l.AllowStrict(ctx, "k", 3, time.Minute)
	if err == nil {
		t.Fatal("AllowStrict returned nil error despite EVAL/EVALSHA being blocked, want an error")
	}
	if ok {
		t.Fatal("AllowStrict returned allowed=true on script failure, want false (fail closed)")
	}

	// Contrast: Allow on an identical Limiter+key silently degrades to the
	// fallback and admits the request — proving the two methods really do
	// differ only in this failure path, and that a silent bypass is exactly
	// what AllowStrict is preventing here.
	l2 := New(rdb, 100, nil)
	if allowed := l2.Allow(ctx, "k", 3, time.Minute); !allowed {
		t.Fatal("sanity check failed: Allow should degrade-open (admit) on the same script failure")
	}
}
