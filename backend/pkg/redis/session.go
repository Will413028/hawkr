package redis

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const sessionPrefix = "session:"

// getSessionWithTTLScript reads a session AND its remaining TTL in one
// round-trip, mutating nothing. Returns a 3-tuple {found, ttl, value}: found=0
// when the key is absent/expired (ttl 0, value empty).
//
// Read and slide are deliberately NOT fused (they were, until #2014's
// follow-up): the caller has to be able to run its own checks — is this account
// blocked? does the source of truth still hold this session? — in between, and
// only then decide to extend the session. A request that is about to be refused
// must not have already renewed the session it is refused on.
const getSessionWithTTLScript = `
local v = redis.call("GET", KEYS[1])
if not v then
    return {0, 0, ""}
end
return {1, redis.call("TTL", KEYS[1]), v}
`

// HashToken returns the lowercase hex SHA-256 of an opaque session token.
//
// It is the single derivation shared by everything that has to NAME a session
// without holding the credential: the Redis key (SessionKey), the gateway's L1
// cache key, and web_sessions.token_hash — the only form the identity module has
// been given since #1592. Because all three agree, a party holding just the
// hash can address the session in any of them, which is what makes account-level
// revocation able to evict the cache (see SessionKeyForHash).
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// SessionKey is the Redis key holding the session for token.
//
// Keyed by the token's HASH, not the token itself (#2049). Two reasons:
//
//   - Account-level revocation has to be able to COMPUTE this key from what the
//     source of truth stores. The identity module holds token_hash and deliberately
//     nothing else, so under the old "session:"+rawToken format it could not name
//     the cache entry it needed to evict; revocation had to fall back on the
//     blacklist key, whose only precision is "this whole account", and whose only
//     alternative was SCAN session:* — O(keyspace) and racy.
//   - The keyspace stops being a credential store. `KEYS session:*` used to print
//     usable bearer tokens on a Redis sharing a VM with other workloads, so read
//     access to the keyspace was account takeover. The VALUE is only
//     {accountId, accountType, loginRole}, so behind the hash there is nothing
//     left worth stealing.
//
// Cost is nil: HashToken is already computed once per request for the L1 cache.
func SessionKey(token string) string { return SessionKeyForHash(HashToken(token)) }

// SessionKeyForHash builds the same key from an ALREADY-hashed token, for the
// holders of web_sessions.token_hash (the identity module's account-level
// revocation). Keeping it next to SessionKey is the point: the two must not be
// able to drift, or a revocation would delete a key nobody reads.
func SessionKeyForHash(tokenHash string) string { return sessionPrefix + tokenHash }

// Session is the server-side web-session record. Web clients hold only the
// opaque token (in an httpOnly cookie); the gateway resolves it to this on
// every request. AccountType is the role SET the account holds (bitmask:
// 1=buyer, 2=seller, 3=both).
//
// LoginRole is the persisted active role (1=buyer, 2=seller) the user last
// selected — the SERVER's record of "which workspace am I in". It exists because
// a dual-role account (accountType=3) sending a bare cookie with no X-Active-Role
// hint is otherwise unresolvable (the server can't guess which role the user
// chose). For single-role accounts it is redundant (the role is derivable from
// accountType). An X-Active-Role request hint still takes precedence when present;
// this is the fallback that survives a bare RSC call. (The monolith entry now
// reads X-Active-Role itself and synthesizes X-Login-Role for the legacy gateway,
// internal/legacy/accounttype.OfLoginRole; Connect does not read it at all, ADR
// 2026-10-07-connect-without-perspective.) Old sessions written before
// this field deserialize LoginRole=0 ("no active role"), which is the same
// behaviour as before — backward-compatible.
type Session struct {
	AccountID   int64 `json:"accountId"`
	AccountType int32 `json:"accountType"`
	LoginRole   int32 `json:"loginRole,omitempty"`
}

// GenerateSessionToken returns a 256-bit cryptographically-random, URL-safe
// opaque token. Deliberately NOT a TSID — TSIDs are time-ordered and guessable,
// which is unacceptable for a bearer secret (TSID is for primary keys, not
// credentials).
func GenerateSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// PutSession stores sess under token with the given TTL. It sets the ABSOLUTE
// expiry (login mints, role-change re-Puts with redis.KeepTTL). Idle-timeout
// sliding on read is handled separately by RefreshSessionTTL (#1290).
func PutSession(ctx context.Context, rdb *redis.Client, token string, sess *Session, ttl time.Duration) error {
	payload, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	return rdb.Set(ctx, SessionKey(token), payload, ttl).Err()
}

// GetSession returns the session for token, or (nil, nil) if absent/expired.
func GetSession(ctx context.Context, rdb *redis.Client, token string) (*Session, error) {
	val, err := rdb.Get(ctx, SessionKey(token)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var sess Session
	if err := json.Unmarshal(val, &sess); err != nil {
		return nil, err
	}
	return &sess, nil
}

// GetSessionWithTTL resolves the session for token and reports how much of its
// idle timeout is left, without extending anything. The caller compares
// remaining against its refresh threshold to decide whether a slide is due, and
// performs it with RefreshSessionTTL once its own gates have passed.
//
// remaining is negative when the key carries no expiry at all (Redis TTL -1), so
// the "is a slide due" comparison self-heals a stray persistent session key.
// Absent/expired returns (nil, 0, nil) — mirrors GetSession's (nil, nil).
func GetSessionWithTTL(ctx context.Context, rdb *redis.Client, token string) (sess *Session, remaining time.Duration, err error) {
	res, err := rdb.Eval(ctx, getSessionWithTTLScript, []string{SessionKey(token)}).Result()
	if err != nil {
		return nil, 0, err
	}
	arr, ok := res.([]interface{})
	if !ok || len(arr) != 3 {
		return nil, 0, fmt.Errorf("session: unexpected eval result %T (%v)", res, res)
	}
	found, _ := arr[0].(int64)
	if found == 0 {
		return nil, 0, nil
	}
	ttlSecs, _ := arr[1].(int64)
	payload, _ := arr[2].(string)
	var s Session
	if err := json.Unmarshal([]byte(payload), &s); err != nil {
		return nil, 0, err
	}
	return &s, time.Duration(ttlSecs) * time.Second, nil
}

// RefreshSessionTTL resets a session's idle timeout to ttl (sliding expiry,
// #1290). The caller re-issues the browser cookie's Max-Age alongside it so the
// client-side and server-side expiries stay in lockstep.
//
// EXPIRE, never DEL+SET: the key is never momentarily absent, so a concurrent
// reader (e.g. a long-lived SSE stream) can never observe a false "session gone"
// and hard-log-out the user. For the same reason it cannot resurrect anything —
// EXPIRE on a key deleted by a concurrent logout is a no-op.
func RefreshSessionTTL(ctx context.Context, rdb *redis.Client, token string, ttl time.Duration) error {
	return rdb.Expire(ctx, SessionKey(token), ttl).Err()
}

// DeleteSession removes a session token (logout / revoke).
func DeleteSession(ctx context.Context, rdb *redis.Client, token string) error {
	return rdb.Del(ctx, SessionKey(token)).Err()
}

// DeleteSessionsByTokenHash evicts the cached copies of sessions named by their
// token hashes — the form account-level revocation has to work in, since the
// rows it just deleted carried token_hash and never the token (#2049).
//
// One DEL for the whole set: the caller is deleting the sessions of ONE account,
// so the count is bounded by that account's device list, and a partial eviction
// would leave exactly the sessions this call exists to kill still resolvable.
//
// It is an eviction, not the revocation: the web_sessions rows are what end the
// sessions, and they are already gone by the time this runs. That is why a
// caller may treat a failure here as best-effort — the consequence is delay
// (until the cached copy's next slide or TTL), not survival.
//
// A nil client or an empty set is a no-op, so callers need no guard of their own.
func DeleteSessionsByTokenHash(ctx context.Context, rdb *redis.Client, tokenHashes []string) error {
	if rdb == nil || len(tokenHashes) == 0 {
		return nil
	}
	keys := make([]string, 0, len(tokenHashes))
	for _, h := range tokenHashes {
		if h == "" {
			continue
		}
		keys = append(keys, SessionKeyForHash(h))
	}
	if len(keys) == 0 {
		return nil
	}
	return rdb.Del(ctx, keys...).Err()
}
