package redis_test

import (
	"context"
	"strings"
	"testing"
	"time"

	pkgredis "github.com/Will413028/hawkr/go-pkg/redis"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// sessionTestRedis returns a client backed by a per-test in-process miniredis.
//
// It used to dial a real 127.0.0.1:6379 and t.Skip when that failed, which on
// any machine whose Redis has a password — the normal state here and in CI —
// meant every session test in this file reported success while asserting
// nothing. Same fix as the gateway middleware suite (commit 1128811d7): remove
// the skip by removing its cause, never by loosening its condition. Per-test
// isolation is a bonus: tokens no longer have to be unique across the package.
func sessionTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

func TestSessionRoundTrip(t *testing.T) {
	rdb := sessionTestRedis(t)
	ctx := context.Background()
	const token = "unit-test-session-token"
	want := &pkgredis.Session{AccountID: 123, AccountType: 3}

	if err := pkgredis.PutSession(ctx, rdb, token, want, time.Minute); err != nil {
		t.Fatalf("PutSession: %v", err)
	}
	t.Cleanup(func() { _ = pkgredis.DeleteSession(ctx, rdb, token) })

	got, err := pkgredis.GetSession(ctx, rdb, token)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got == nil || got.AccountID != 123 || got.AccountType != 3 {
		t.Fatalf("GetSession = %+v, want %+v", got, want)
	}

	if err := pkgredis.DeleteSession(ctx, rdb, token); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	gone, err := pkgredis.GetSession(ctx, rdb, token)
	if err != nil {
		t.Fatalf("GetSession after delete: %v", err)
	}
	if gone != nil {
		t.Fatalf("expected nil after delete, got %+v", gone)
	}
}

func TestGetSession_Missing_ReturnsNilNil(t *testing.T) {
	rdb := sessionTestRedis(t)
	got, err := pkgredis.GetSession(context.Background(), rdb, "definitely-absent-token")
	if err != nil {
		t.Fatalf("err = %v, want nil for missing key", err)
	}
	if got != nil {
		t.Fatalf("got = %+v, want nil for missing key", got)
	}
}

// The read half must report the remaining TTL without touching it: the caller
// needs to run its own gates (account blocked? session still in the source of
// truth?) before deciding to extend anything, so a request that ends in a 401
// cannot have already renewed the session it is refused on (#2014 follow-up).
func TestGetSessionWithTTL_ReportsRemainingWithoutSliding(t *testing.T) {
	rdb := sessionTestRedis(t)
	ctx := context.Background()
	const token = "unit-test-read-ttl"
	want := &pkgredis.Session{AccountID: 777, AccountType: 3, LoginRole: 2}

	if err := pkgredis.PutSession(ctx, rdb, token, want, 10*time.Second); err != nil {
		t.Fatalf("PutSession: %v", err)
	}
	t.Cleanup(func() { _ = pkgredis.DeleteSession(ctx, rdb, token) })

	got, remaining, err := pkgredis.GetSessionWithTTL(ctx, rdb, token)
	if err != nil {
		t.Fatalf("GetSessionWithTTL: %v", err)
	}
	if got == nil || got.AccountID != 777 || got.AccountType != 3 || got.LoginRole != 2 {
		t.Fatalf("session = %+v, want %+v", got, want)
	}
	if remaining <= 0 || remaining > 10*time.Second {
		t.Fatalf("remaining = %v, want (0, 10s]", remaining)
	}
	// Untouched: the read must not extend the key.
	if ttl := rdb.TTL(ctx, pkgredis.SessionKey(token)).Val(); ttl > 10*time.Second {
		t.Fatalf("read extended the TTL to %v — the read half must not slide", ttl)
	}
}

// A key with no expiry at all (TTL -1) must read as "a slide is overdue", not as
// "plenty of time left" — otherwise a stray persistent session key never heals.
func TestGetSessionWithTTL_NoExpiryReportsNegative(t *testing.T) {
	rdb := sessionTestRedis(t)
	ctx := context.Background()
	const token = "unit-test-no-expiry"
	if err := pkgredis.PutSession(ctx, rdb, token,
		&pkgredis.Session{AccountID: 999, AccountType: 1}, 0); err != nil {
		t.Fatalf("PutSession: %v", err)
	}
	t.Cleanup(func() { _ = pkgredis.DeleteSession(ctx, rdb, token) })

	got, remaining, err := pkgredis.GetSessionWithTTL(ctx, rdb, token)
	if err != nil {
		t.Fatalf("GetSessionWithTTL: %v", err)
	}
	if got == nil || got.AccountID != 999 {
		t.Fatalf("session = %+v, want AccountID 999", got)
	}
	if remaining >= 0 {
		t.Fatalf("remaining = %v, want negative so the slide threshold treats it as due", remaining)
	}
}

func TestGetSessionWithTTL_Absent_ReturnsNilNil(t *testing.T) {
	rdb := sessionTestRedis(t)
	got, remaining, err := pkgredis.GetSessionWithTTL(context.Background(), rdb, "definitely-absent-token")
	if err != nil {
		t.Fatalf("err = %v, want nil for absent key", err)
	}
	if got != nil || remaining != 0 {
		t.Fatalf("got (%+v, remaining=%v), want (nil, 0) for absent key", got, remaining)
	}
}

// The slide extends the live key with EXPIRE (never DEL+SET), so no reader ever
// sees a transient absence.
func TestRefreshSessionTTL_ExtendsLiveKey(t *testing.T) {
	rdb := sessionTestRedis(t)
	ctx := context.Background()
	const token = "unit-test-slide"
	if err := pkgredis.PutSession(ctx, rdb, token,
		&pkgredis.Session{AccountID: 888, AccountType: 1}, 10*time.Second); err != nil {
		t.Fatalf("PutSession: %v", err)
	}
	t.Cleanup(func() { _ = pkgredis.DeleteSession(ctx, rdb, token) })

	if err := pkgredis.RefreshSessionTTL(ctx, rdb, token, time.Minute); err != nil {
		t.Fatalf("RefreshSessionTTL: %v", err)
	}
	if ttl := rdb.TTL(ctx, pkgredis.SessionKey(token)).Val(); ttl <= 50*time.Second {
		t.Fatalf("TTL not slid to ~60s, got %v", ttl)
	}
	// The value must still be there — a slide is not a re-write.
	if sess, err := pkgredis.GetSession(ctx, rdb, token); err != nil || sess == nil || sess.AccountID != 888 {
		t.Fatalf("session after slide = %+v (err %v), want AccountID 888", sess, err)
	}
}

// A slide racing a logout must not bring the session back: EXPIRE on a deleted
// key is a no-op, so the key stays gone.
func TestRefreshSessionTTL_AbsentKeyIsNotResurrected(t *testing.T) {
	rdb := sessionTestRedis(t)
	ctx := context.Background()
	if err := pkgredis.RefreshSessionTTL(ctx, rdb, "already-logged-out-token", time.Minute); err != nil {
		t.Fatalf("RefreshSessionTTL on absent key: %v", err)
	}
	if sess, err := pkgredis.GetSession(ctx, rdb, "already-logged-out-token"); err != nil || sess != nil {
		t.Fatalf("absent key resurrected: sess=%+v err=%v", sess, err)
	}
}

// #2049, half one: the keyspace must stop being a credential store. Anyone who
// can read the keys of a Redis that shares a VM with other workloads used to be
// able to lift a usable bearer token straight out of `KEYS session:*`.
func TestSessionKey_DoesNotExposeTheRawToken(t *testing.T) {
	rdb := sessionTestRedis(t)
	ctx := context.Background()
	const token = "a-perfectly-usable-bearer-secret"

	if err := pkgredis.PutSession(ctx, rdb, token,
		&pkgredis.Session{AccountID: 1, AccountType: 1}, time.Minute); err != nil {
		t.Fatalf("PutSession: %v", err)
	}

	keys, err := rdb.Keys(ctx, "session:*").Result()
	if err != nil {
		t.Fatalf("KEYS: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("keys = %v, want exactly one session key", keys)
	}
	if strings.Contains(keys[0], token) {
		t.Fatalf("key %q still carries the raw token — reading the keyspace is account takeover", keys[0])
	}
	if want := "session:" + pkgredis.HashToken(token); keys[0] != want {
		t.Fatalf("key = %q, want %q (sha256 hex of the token)", keys[0], want)
	}
}

// #2049, half two — and the reason the ticket exists. The identity module holds
// web_sessions.token_hash and deliberately never the token (#1592), so the key
// has to be COMPUTABLE from the hash alone. Without this equality, account-level
// revocation cannot name the cache entries it needs to evict and is back to
// SCAN session:* or waiting out the blacklist.
func TestSessionKeyForHash_MatchesTheKeyWrittenByPutSession(t *testing.T) {
	rdb := sessionTestRedis(t)
	ctx := context.Background()
	const token = "token-known-only-to-the-gateway"

	if err := pkgredis.PutSession(ctx, rdb, token,
		&pkgredis.Session{AccountID: 7, AccountType: 1}, time.Minute); err != nil {
		t.Fatalf("PutSession: %v", err)
	}

	// Everything the identity module has is the hash — this is that path exactly.
	tokenHash := pkgredis.HashToken(token)
	if got := rdb.Exists(ctx, pkgredis.SessionKeyForHash(tokenHash)).Val(); got != 1 {
		t.Fatalf("the key derived from token_hash alone does not exist (Exists=%d); "+
			"revocation would delete a key nobody reads", got)
	}
	if pkgredis.SessionKeyForHash(tokenHash) != pkgredis.SessionKey(token) {
		t.Fatalf("SessionKeyForHash(%q) = %q, SessionKey(token) = %q — the two derivations have drifted",
			tokenHash, pkgredis.SessionKeyForHash(tokenHash), pkgredis.SessionKey(token))
	}
}

// The revocation act itself, in the form the identity module performs it: it has a
// set of token hashes (the rows it just deleted) and nothing else. Exactly those
// sessions must go, and no other account's may be touched.
func TestDeleteSessionsByTokenHash_EvictsExactlyTheNamedSessions(t *testing.T) {
	rdb := sessionTestRedis(t)
	ctx := context.Background()
	revoked := []string{"revoked-device-1", "revoked-device-2"}
	const bystander = "another-accounts-session"

	for _, tok := range append(append([]string{}, revoked...), bystander) {
		if err := pkgredis.PutSession(ctx, rdb, tok,
			&pkgredis.Session{AccountID: 1, AccountType: 1}, time.Minute); err != nil {
			t.Fatalf("seed %q: %v", tok, err)
		}
	}

	hashes := make([]string, 0, len(revoked))
	for _, tok := range revoked {
		hashes = append(hashes, pkgredis.HashToken(tok))
	}
	if err := pkgredis.DeleteSessionsByTokenHash(ctx, rdb, hashes); err != nil {
		t.Fatalf("DeleteSessionsByTokenHash: %v", err)
	}

	for _, tok := range revoked {
		if sess, err := pkgredis.GetSession(ctx, rdb, tok); err != nil || sess != nil {
			t.Fatalf("revoked session %q survived the eviction: sess=%+v err=%v", tok, sess, err)
		}
	}
	if sess, err := pkgredis.GetSession(ctx, rdb, bystander); err != nil || sess == nil {
		t.Fatalf("eviction must be precise: bystander session gone (sess=%+v err=%v)", sess, err)
	}
}

// Callers pass whatever the DELETE returned, so the empty and nil-client cases
// are ordinary, not exceptional: an account with no live session must not turn
// into a DEL with zero keys (a Redis error) or a nil-pointer panic.
func TestDeleteSessionsByTokenHash_EmptyAndNilAreNoOps(t *testing.T) {
	rdb := sessionTestRedis(t)
	ctx := context.Background()
	if err := pkgredis.DeleteSessionsByTokenHash(ctx, rdb, nil); err != nil {
		t.Fatalf("nil hashes: %v", err)
	}
	if err := pkgredis.DeleteSessionsByTokenHash(ctx, rdb, []string{}); err != nil {
		t.Fatalf("empty hashes: %v", err)
	}
	if err := pkgredis.DeleteSessionsByTokenHash(ctx, rdb, []string{""}); err != nil {
		t.Fatalf("blank hash: %v", err)
	}
	if err := pkgredis.DeleteSessionsByTokenHash(ctx, nil, []string{"deadbeef"}); err != nil {
		t.Fatalf("nil client must be a no-op, got %v", err)
	}
}

// HashToken is the one derivation the Redis key, the gateway's L1 key and
// web_sessions.token_hash all share. Pin the value, not just the shape: a change
// here silently orphans every live session AND every stored token_hash at once.
func TestHashToken_IsStableSHA256Hex(t *testing.T) {
	// echo -n "abc" | shasum -a 256
	const want = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got := pkgredis.HashToken("abc"); got != want {
		t.Fatalf("HashToken(\"abc\") = %q, want %q", got, want)
	}
	if got := len(pkgredis.HashToken("")); got != 64 {
		t.Fatalf("hash length = %d, want 64 hex chars (web_sessions.token_hash is varchar(64))", got)
	}
}

func TestGenerateSessionToken_UniqueAndLong(t *testing.T) {
	a, err := pkgredis.GenerateSessionToken()
	if err != nil {
		t.Fatalf("GenerateSessionToken: %v", err)
	}
	b, _ := pkgredis.GenerateSessionToken()
	if a == b {
		t.Fatal("tokens must be unique")
	}
	if len(a) < 40 { // 32 random bytes, base64url (no padding) ≈ 43 chars
		t.Fatalf("token too short (%d chars) — insufficient entropy", len(a))
	}
}
