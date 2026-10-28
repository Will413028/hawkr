package redis

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const accountBlockedPrefix = "account:blocked:"

func accountBlockedKey(accountID int64) string {
	return fmt.Sprintf("%s%d", accountBlockedPrefix, accountID)
}

// ErrNoRedisClient is returned when a blacklist WRITE is asked for without a
// client. It is an error rather than a nil-panic or a silent success because the
// three functions here are not symmetric about a missing client:
//
//   - reading (CheckAccountBlocked) answers AccountBlockUnknown — "cannot tell"
//     is a real, useful answer that callers already degrade on;
//   - clearing (ClearAccountBlocked) is a no-op success — failing to remove a
//     refusal leaves the account refused, which is the conservative residual;
//   - setting is neither. Reporting success for a block that was never written
//     is how a suspension silently fails to reach the one surface it exists for.
var ErrNoRedisClient = stderrors.New("redis client not configured")

// SetAccountBlocked writes a blacklist key for accountID with the given TTL.
func SetAccountBlocked(ctx context.Context, rdb *redis.Client, accountID int64, ttl time.Duration) error {
	if rdb == nil {
		return ErrNoRedisClient
	}
	return rdb.Set(ctx, accountBlockedKey(accountID), 1, ttl).Err()
}

// ClearAccountBlocked removes accountID's blacklist key — the cache half of
// lifting a suspension (#2053).
//
// There is exactly ONE key, whichever writer set it and whatever TTL they chose:
// the soft-delete paths and the suspension path all write accountBlockedKey. So
// this is TTL-agnostic by construction, and "which of the two blocks do I
// clear?" is not a question the caller has to answer.
//
// It is emphatically NOT the whole of an unsuspend. The key is a DERIVED cache
// of accounts.status (see gateway's blacklist_fallback.go): deleting it while
// the status still says suspended just means the gateway keeps refusing from the
// source of truth instead — which is the safe residual, and why the unsuspend
// use case does this half FIRST. The reverse order would leave the account
// looking active everywhere while every request still 401s.
//
// A nil client is a no-op success, mirroring how the read side treats "Redis was
// never injected" as an information deficit rather than a panic. Callers that
// need the delete to be durable must therefore inject a real client — the use
// case's own tests pin that.
func ClearAccountBlocked(ctx context.Context, rdb *redis.Client, accountID int64) error {
	if rdb == nil {
		return nil
	}
	return rdb.Del(ctx, accountBlockedKey(accountID)).Err()
}

// AccountBlockState is what this cache was able to say about an account.
//
// The third case is the one a bool cannot express, and it is the whole of
// issue #2050: "Redis could not answer" is not the same fact as "this account
// is fine". Collapsing them is what made the blacklist behave like a second,
// unrebuildable state — the moment Redis went away the information went with
// it, and callers had no way to tell that they were guessing.
//
// The key is strictly derived from the account SSOT (accounts.deleted_at /
// accounts.status; see the three writers in delete_account.go, remove_role.go
// and social's dispatcher.go), so a caller that gets AccountBlockUnknown can
// go and rebuild the answer from that SSOT instead of assuming "not blocked".
type AccountBlockState int

const (
	// AccountBlockUnknown means the cache could not answer (Redis unreachable,
	// erroring, or not injected). It is NOT "not blocked".
	AccountBlockUnknown AccountBlockState = iota
	// AccountNotBlocked means Redis answered and holds no block key.
	AccountNotBlocked
	// AccountBlocked means Redis answered and holds a block key.
	AccountBlocked
)

// CheckAccountBlocked reports the blacklist cache's answer for accountID,
// keeping "cannot tell" distinct from "not blocked" so the caller can choose
// its own degraded behaviour (gateway JWTAuth rebuilds the answer from the
// account SSOT; see gateway/internal/middleware/blacklist_fallback.go).
//
// A nil client is AccountBlockUnknown, not a panic: "Redis was never injected"
// is the same information deficit as "Redis is down".
func CheckAccountBlocked(ctx context.Context, rdb *redis.Client, accountID int64) AccountBlockState {
	if rdb == nil {
		return AccountBlockUnknown
	}
	val, err := rdb.Exists(ctx, accountBlockedKey(accountID)).Result()
	if err != nil {
		return AccountBlockUnknown
	}
	if val > 0 {
		return AccountBlocked
	}
	return AccountNotBlocked
}

// SessionTTL returns the configured web session idle TTL (SESSION_TTL_SECONDS
// env, default 7 days). Single source of truth: gateway (different Go module)
// and account-service's AccountBlockedTTL both derive from this instead of
// each hardcoding their own literal, which is exactly how the #1406 TTL gap
// happened (a 25h blacklist literal never updated when the session's own TTL
// grew to 7 days elsewhere).
func SessionTTL() time.Duration {
	return time.Duration(envIntDefault("SESSION_TTL_SECONDS", 7*24*3600)) * time.Second
}

// AccountBlockedTTL returns the TTL to use when blacklisting an account for
// soft-delete (delete_account / remove_role's auto-soft-delete path). It must
// exceed SessionTTL() to prevent an idle session from silently reactivating once
// the block key expires. However, an actively-polled blocked session can persist
// longer, since each authenticated request slides the session's TTL before the
// block check runs. The 24h buffer mirrors the original 24h-JWT+1h-buffer
// convention, transplanted onto the session-TTL basis.
//
// Suspension uses AccountSuspendedTTL instead, which takes this as a floor but
// derives from the bearer-JWT lifetime rather than the session's.
func AccountBlockedTTL() time.Duration {
	return SessionTTL() + 24*time.Hour
}

// AccountSuspendedTTL returns the TTL to use when blacklisting a SUSPENDED
// account (#2053). bearerTTL is the caller's access-token lifetime.
//
// Why not just reuse AccountBlockedTTL: the two answer different questions, and
// collapsing them would wire an unrelated knob to suspension's correctness.
// AccountBlockedTTL is derived from SESSION_TTL_SECONDS because soft-delete's
// job is to outlive the web sessions that existed at deletion time. Suspension's
// job is to outlive the STATELESS BEARER JWT — the only credential that has no
// server-side record to delete, and hence the only one this key still exists for. Those lifetimes are configured independently, so tying
// suspension to the session knob means lowering SESSION_TTL_SECONDS silently
// shortens suspensions below the tokens they must cover.
//
// Taking the max rather than only bearerTTL+24h keeps this monotonic with the
// other block writers: whatever coverage a soft-delete would get, a suspension
// gets at least as much. The +24h buffer is the same convention as
// AccountBlockedTTL's.
//
// It does NOT need to encode a moderation policy duration. A suspended account
// cannot log in again or refresh (both paths gate on Account.IsActive), so the
// only credential the key still has to stop is one issued BEFORE the suspension
// — bounded by bearerTTL. Past that the key changes no verdict and only spends
// a share of an allkeys-lru budget shared with all site caching (§28).
func AccountSuspendedTTL(bearerTTL time.Duration) time.Duration {
	if floor := bearerTTL + 24*time.Hour; floor > AccountBlockedTTL() {
		return floor
	}
	return AccountBlockedTTL()
}

// envIntDefault reads a positive integer env var, falling back to def.
func envIntDefault(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}
