package redis_test

import (
	"context"
	"testing"
	"time"

	pkgredis "github.com/Will413028/hawkr/go-pkg/redis"
	"github.com/redis/go-redis/v9"
)

// CheckAuthState is the one Redis round-trip that answers both "is the account
// cut off?" and "does this credential still count?". Its tri-states are what
// let the gateway rebuild from the source of truth instead of guessing, so the
// cases that matter most here are the ones that must come back UNKNOWN.

// The two axes answer with different authority, and this is the test that says
// so. A missing blacklist key IS "not blocked" — its writers mirror `accounts`
// and a web session dies at its row regardless. A missing credential key is
// NOT "valid": a bearer JWT has no row of its own, so a key that was never
// written (MISCONF) or has been evicted (allkeys-lru) is the entire revocation.
func TestCheckAuthState_CleanCaller_BlockIsDefiniteCredentialIsNot(t *testing.T) {
	rdb := newTestClient(t)
	ctx := context.Background()

	block, cred := pkgredis.CheckAuthState(ctx, rdb, 7001, "hash-clean", time.Now())
	if block != pkgredis.AccountNotBlocked {
		t.Fatalf("block = %v, want AccountNotBlocked", block)
	}
	if cred != pkgredis.CredentialUnknown {
		t.Fatalf("cred = %v, want CredentialUnknown: absence must send the caller to the source of truth, not acquit", cred)
	}
}

func TestCheckAuthState_RevokedTokenIsRevoked(t *testing.T) {
	rdb := newTestClient(t)
	ctx := context.Background()
	if err := pkgredis.SetTokenRevoked(ctx, rdb, "hash-revoked", time.Hour); err != nil {
		t.Fatal(err)
	}

	if _, cred := pkgredis.CheckAuthState(ctx, rdb, 7002, "hash-revoked", time.Now()); cred != pkgredis.CredentialRevoked {
		t.Fatalf("cred = %v, want CredentialRevoked", cred)
	}
}

func TestCheckAuthState_EpochVoidsOlderTokensOnly(t *testing.T) {
	rdb := newTestClient(t)
	ctx := context.Background()
	epoch := time.Now()
	if err := pkgredis.SetAuthEpoch(ctx, rdb, 7003, epoch, time.Hour); err != nil {
		t.Fatal(err)
	}

	if _, cred := pkgredis.CheckAuthState(ctx, rdb, 7003, "hash-old", epoch.Add(-time.Hour)); cred != pkgredis.CredentialRevoked {
		t.Fatalf("older-than-epoch cred = %v, want CredentialRevoked", cred)
	}
	// The token minted after the revocation is the one the user just logged back
	// in with; convicting it too would make logout-all a permanent lockout. It
	// is Unknown rather than Valid because the epoch clearing it says nothing
	// about whether its own per-token key was evicted.
	if _, cred := pkgredis.CheckAuthState(ctx, rdb, 7003, "hash-new", epoch.Add(time.Hour)); cred == pkgredis.CredentialRevoked {
		t.Fatal("a token issued after the epoch must not be convicted by it")
	}
}

// The epoch is stored and compared at second precision — all a JWT `iat`
// carries. A token minted in the same second as the bump must survive rather
// than be caught by sub-second drift.
func TestCheckAuthState_EpochComparisonIsSecondPrecision(t *testing.T) {
	rdb := newTestClient(t)
	ctx := context.Background()
	epoch := time.Now().Truncate(time.Second)
	if err := pkgredis.SetAuthEpoch(ctx, rdb, 7004, epoch, time.Hour); err != nil {
		t.Fatal(err)
	}

	if _, cred := pkgredis.CheckAuthState(ctx, rdb, 7004, "hash-same-second", epoch.Add(400*time.Millisecond)); cred == pkgredis.CredentialRevoked {
		t.Fatal("sub-second drift must not void a token minted in the same second as the bump")
	}
}

// No bearer credential (a web cookie, or an anonymous caller): the credential
// lists are not queried at all, and the answer must be "nothing to refuse"
// rather than an unknown that would send the caller to the SSOT for no reason.
func TestCheckAuthState_NoTokenHash_SkipsTheCredentialLists(t *testing.T) {
	rdb := newTestClient(t)
	ctx := context.Background()
	// Even with an epoch that would void everything, a caller with no bearer
	// credential is unaffected by it.
	if err := pkgredis.SetAuthEpoch(ctx, rdb, 7005, time.Now().Add(time.Hour), time.Hour); err != nil {
		t.Fatal(err)
	}

	block, cred := pkgredis.CheckAuthState(ctx, rdb, 7005, "", time.Time{})
	if block != pkgredis.AccountNotBlocked {
		t.Fatalf("block = %v, want AccountNotBlocked", block)
	}
	if cred != pkgredis.CredentialValid {
		t.Fatalf("cred = %v, want CredentialValid (no question was asked)", cred)
	}
}

// A blocked account and a revoked credential are independent facts and must be
// reported independently — collapsing them would make the account-level and
// credential-level revocations indistinguishable.
func TestCheckAuthState_BlockAndRevocationAreIndependent(t *testing.T) {
	rdb := newTestClient(t)
	ctx := context.Background()
	if err := pkgredis.SetAccountBlocked(ctx, rdb, 7006, time.Hour); err != nil {
		t.Fatal(err)
	}

	block, cred := pkgredis.CheckAuthState(ctx, rdb, 7006, "hash-fine", time.Now())
	if block != pkgredis.AccountBlocked {
		t.Fatalf("block = %v, want AccountBlocked", block)
	}
	if cred == pkgredis.CredentialRevoked {
		t.Fatal("a blocked account must not be reported as a revoked credential: they are different facts with different writers")
	}
}

// THE case the tri-state exists for. A dead Redis must not be reported as a
// clean bill of health on either axis — that is what makes the gateway rebuild
// from the account SSOT instead of silently skipping both checks.
func TestCheckAuthState_RedisDown_IsUnknownOnBothAxes(t *testing.T) {
	dead := redis.NewClient(&redis.Options{Addr: "127.0.0.1:19998"})
	t.Cleanup(func() { _ = dead.Close() })

	block, cred := pkgredis.CheckAuthState(context.Background(), dead, 7007, "hash-x", time.Now())
	if block != pkgredis.AccountBlockUnknown {
		t.Fatalf("block = %v, want AccountBlockUnknown", block)
	}
	if cred != pkgredis.CredentialUnknown {
		t.Fatalf("cred = %v, want CredentialUnknown — 'Redis is down' is not 'this credential is fine'", cred)
	}
}

func TestCheckAuthState_NilClient_IsUnknown(t *testing.T) {
	block, cred := pkgredis.CheckAuthState(context.Background(), nil, 7008, "hash-y", time.Now())
	if block != pkgredis.AccountBlockUnknown || cred != pkgredis.CredentialUnknown {
		t.Fatalf("got (%v, %v), want both Unknown: a client that was never injected knows nothing", block, cred)
	}
}

// An epoch value that cannot be parsed is corruption, not evidence. Reading it
// as "valid" would silently disable the epoch; reading it as "revoked" would
// log the whole account out over a bad byte.
func TestCheckAuthState_UnparseableEpoch_IsUnknown(t *testing.T) {
	rdb := newTestClient(t)
	ctx := context.Background()
	if err := rdb.Set(ctx, pkgredis.AuthEpochKey(7009), "not-a-number", time.Hour).Err(); err != nil {
		t.Fatal(err)
	}

	if _, cred := pkgredis.CheckAuthState(ctx, rdb, 7009, "hash-z", time.Now()); cred != pkgredis.CredentialUnknown {
		t.Fatalf("cred = %v, want CredentialUnknown", cred)
	}
}

// A JWT with no iat cannot be placed relative to the epoch, so the epoch has
// nothing to say about it — the per-token list is what covers it.
func TestCheckAuthState_ZeroIssuedAt_EpochCannotSpeak(t *testing.T) {
	rdb := newTestClient(t)
	ctx := context.Background()
	if err := pkgredis.SetAuthEpoch(ctx, rdb, 7010, time.Now(), time.Hour); err != nil {
		t.Fatal(err)
	}

	if _, cred := pkgredis.CheckAuthState(ctx, rdb, 7010, "hash-noiat", time.Time{}); cred == pkgredis.CredentialRevoked {
		t.Fatal("a JWT with no iat cannot be placed against the epoch, so the epoch must not convict it")
	}
}

// The epoch must outlive any credential minted before it, or a revocation
// silently lapses while its victims are still usable.
func TestAuthEpochTTL_OutlivesASessionLifetime(t *testing.T) {
	if got, want := pkgredis.AuthEpochTTL(), pkgredis.SessionTTL(); got <= want {
		t.Fatalf("AuthEpochTTL = %v, must exceed SessionTTL %v", got, want)
	}
}

// The roles epoch is the STALENESS axis, deliberately separate from the auth
// epoch's REVOCATION axis. A role grant moves it so the gateway stops trusting
// the roles baked into an older JWT; the refresh path ignores it entirely, so
// the client trades the stale token for a fresh one instead of being logged
// out. Both epochs convict here because both mean "this bearer token must not
// be used as-is" — they differ only in what the holder can do about it.
func TestCheckAuthState_RolesEpochVoidsOlderTokens(t *testing.T) {
	rdb := newTestClient(t)
	ctx := context.Background()
	epoch := time.Now()
	if err := pkgredis.SetRolesEpoch(ctx, rdb, 7101, epoch, time.Hour); err != nil {
		t.Fatal(err)
	}

	if _, cred := pkgredis.CheckAuthState(ctx, rdb, 7101, "hash-stale", epoch.Add(-time.Hour)); cred != pkgredis.CredentialRevoked {
		t.Fatalf("token predating the roles epoch: cred = %v, want CredentialRevoked", cred)
	}
	if _, cred := pkgredis.CheckAuthState(ctx, rdb, 7101, "hash-fresh", epoch.Add(time.Hour)); cred == pkgredis.CredentialRevoked {
		t.Fatal("a token issued after the roles epoch already carries the new roles and must not be convicted")
	}
}

// A cookie caller presents no bearer credential, so neither epoch has anything
// to refuse. Web sessions carry their own role set and are refreshed in place
// at the grant point — convicting them here would log out the exact browser
// that just gained the role.
func TestCheckAuthState_NoTokenHash_RolesEpochStaysSilent(t *testing.T) {
	rdb := newTestClient(t)
	ctx := context.Background()
	epoch := time.Now()
	if err := pkgredis.SetRolesEpoch(ctx, rdb, 7102, epoch, time.Hour); err != nil {
		t.Fatal(err)
	}

	if _, cred := pkgredis.CheckAuthState(ctx, rdb, 7102, "", epoch.Add(-time.Hour)); cred != pkgredis.CredentialValid {
		t.Fatalf("cookie caller: cred = %v, want CredentialValid (no bearer credential was presented)", cred)
	}
}
