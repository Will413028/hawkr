package redis_test

import (
	"context"
	"net"
	"testing"
	"time"

	pkgredis "github.com/Will413028/hawkr/go-pkg/redis"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// newTestClient returns a client backed by a per-test in-process miniredis, and
// still builds it through pkgredis.NewClient so the constructor stays on the
// tested path. Previously it dialled a real 127.0.0.1:6379 and skipped on
// failure, so the live-Redis blacklist tests never ran anywhere the local
// Redis had a password — see sessionTestRedis for the same fix and its history.
func newTestClient(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	host, port, err := net.SplitHostPort(mr.Addr())
	if err != nil {
		t.Fatalf("miniredis addr %q: %v", mr.Addr(), err)
	}
	rdb, err := pkgredis.NewClient(pkgredis.Config{Host: host, Port: port})
	if err != nil {
		t.Fatalf("NewClient against miniredis: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

// A live Redis must produce the two DEFINITE states, never the unknown one:
// the whole degraded path hangs off "unknown means the cache did not answer",
// so a healthy answer leaking through as unknown would send every request to
// the SSOT.
func TestCheckAccountBlocked_LiveRedis_IsDefinite(t *testing.T) {
	rdb := newTestClient(t)
	ctx := context.Background()
	const id = int64(9900000002)
	_ = rdb.Del(ctx, "account:blocked:9900000002")
	t.Cleanup(func() { _ = rdb.Del(ctx, "account:blocked:9900000002") })

	if got := pkgredis.CheckAccountBlocked(ctx, rdb, id); got != pkgredis.AccountNotBlocked {
		t.Fatalf("before block: got %v, want AccountNotBlocked", got)
	}
	if err := pkgredis.SetAccountBlocked(ctx, rdb, id, 5*time.Second); err != nil {
		t.Fatalf("SetAccountBlocked: %v", err)
	}
	if got := pkgredis.CheckAccountBlocked(ctx, rdb, id); got != pkgredis.AccountBlocked {
		t.Fatalf("after block: got %v, want AccountBlocked", got)
	}
}

// THE #2050 distinction. An unreachable Redis must report UNKNOWN, not
// NotBlocked: the old bool said "false" here, which is indistinguishable from
// a healthy Redis confirming the account is fine, and that is what left the
// degraded path with nothing to act on.
func TestCheckAccountBlocked_RedisDown_IsUnknownNotNotBlocked(t *testing.T) {
	badRdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:19999"})
	t.Cleanup(func() { _ = badRdb.Close() })
	got := pkgredis.CheckAccountBlocked(context.Background(), badRdb, 1)
	if got == pkgredis.AccountNotBlocked {
		t.Fatal("an unreachable Redis must not be reported as 'not blocked' — that is the #2050 bug")
	}
	if got != pkgredis.AccountBlockUnknown {
		t.Fatalf("got %v, want AccountBlockUnknown", got)
	}
}

// "Never injected" carries exactly as much information as "down", and must not
// panic: JWTAuth used to skip the check entirely for a nil client.
func TestCheckAccountBlocked_NilClient_IsUnknown(t *testing.T) {
	if got := pkgredis.CheckAccountBlocked(context.Background(), nil, 1); got != pkgredis.AccountBlockUnknown {
		t.Fatalf("got %v, want AccountBlockUnknown", got)
	}
}

func TestSessionTTL_Default(t *testing.T) {
	if got := pkgredis.SessionTTL(); got != 7*24*time.Hour {
		t.Errorf("SessionTTL() = %v, want %v", got, 7*24*time.Hour)
	}
}

func TestSessionTTL_EnvOverride(t *testing.T) {
	t.Setenv("SESSION_TTL_SECONDS", "3600")
	if got := pkgredis.SessionTTL(); got != time.Hour {
		t.Errorf("SessionTTL() = %v, want %v", got, time.Hour)
	}
}

func TestAccountBlockedTTL_ExceedsSessionTTL(t *testing.T) {
	if got := pkgredis.AccountBlockedTTL(); got <= pkgredis.SessionTTL() {
		t.Errorf("AccountBlockedTTL() = %v, must exceed SessionTTL() = %v", got, pkgredis.SessionTTL())
	}
}

func TestAccountBlockedTTL_TracksSessionTTLEnvOverride(t *testing.T) {
	t.Setenv("SESSION_TTL_SECONDS", "3600")
	want := time.Hour + 24*time.Hour
	if got := pkgredis.AccountBlockedTTL(); got != want {
		t.Errorf("AccountBlockedTTL() = %v, want %v", got, want)
	}
}

// ClearAccountBlocked is the cache half of lifting a suspension (#2053). It must
// delete the ONE key both the soft-delete writers and the suspension writer use,
// regardless of which TTL set it — otherwise "which block do I clear?" becomes a
// question the caller has to get right.
func TestClearAccountBlocked_RemovesTheKeyWhicheverTTLWroteIt(t *testing.T) {
	rdb := newTestClient(t)
	ctx := context.Background()
	const id = int64(9900000009)

	for _, ttl := range []time.Duration{time.Hour, 365 * 24 * time.Hour} {
		if err := pkgredis.SetAccountBlocked(ctx, rdb, id, ttl); err != nil {
			t.Fatalf("SetAccountBlocked(%v): %v", ttl, err)
		}
		if got := pkgredis.CheckAccountBlocked(ctx, rdb, id); got != pkgredis.AccountBlocked {
			t.Fatalf("precondition (ttl=%v): got %v, want AccountBlocked", ttl, got)
		}
		if err := pkgredis.ClearAccountBlocked(ctx, rdb, id); err != nil {
			t.Fatalf("ClearAccountBlocked(ttl=%v): %v", ttl, err)
		}
		if got := pkgredis.CheckAccountBlocked(ctx, rdb, id); got != pkgredis.AccountNotBlocked {
			t.Fatalf("after clear (ttl=%v): got %v, want AccountNotBlocked", ttl, got)
		}
	}
}

// Clearing an absent key is a success, not an error: an unsuspend on an account
// that was never blocked must not report failure to the operator.
func TestClearAccountBlocked_AbsentKeyIsSuccess(t *testing.T) {
	rdb := newTestClient(t)
	if err := pkgredis.ClearAccountBlocked(context.Background(), rdb, 9900000010); err != nil {
		t.Fatalf("ClearAccountBlocked on an unblocked account: %v", err)
	}
}

// The three functions are deliberately asymmetric about a missing client, and
// the asymmetry is the point: only the WRITE may not report success, because a
// block that was never written is a suspension that never happened.
func TestBlacklist_NilClientSemanticsDifferPerOperation(t *testing.T) {
	ctx := context.Background()

	if got := pkgredis.CheckAccountBlocked(ctx, nil, 1); got != pkgredis.AccountBlockUnknown {
		t.Errorf("CheckAccountBlocked(nil) = %v, want AccountBlockUnknown", got)
	}
	if err := pkgredis.ClearAccountBlocked(ctx, nil, 1); err != nil {
		t.Errorf("ClearAccountBlocked(nil) = %v, want nil (failing to lift leaves it blocked)", err)
	}
	if err := pkgredis.SetAccountBlocked(ctx, nil, 1, time.Hour); err == nil {
		t.Error("SetAccountBlocked(nil) = nil; reporting success for a block that was " +
			"never written is a silently-failed suspension")
	}
}

// Suspension's TTL must cover the bearer JWT, whose lifetime is set independently
// of SESSION_TTL_SECONDS. Pinning the floor is what stops someone shortening the
// session TTL and silently shortening every suspension below the tokens it has to
// outlive.
func TestAccountSuspendedTTL_FloorsAtBearerLifetimePlusBuffer(t *testing.T) {
	longBearer := pkgredis.AccountBlockedTTL() + 30*24*time.Hour
	if got := pkgredis.AccountSuspendedTTL(longBearer); got <= longBearer {
		t.Errorf("AccountSuspendedTTL(%v) = %v, want > the bearer lifetime", longBearer, got)
	}
	// And it never drops below what a soft-delete would get.
	if got := pkgredis.AccountSuspendedTTL(time.Minute); got < pkgredis.AccountBlockedTTL() {
		t.Errorf("AccountSuspendedTTL(1m) = %v, want >= AccountBlockedTTL (%v)",
			got, pkgredis.AccountBlockedTTL())
	}
}
