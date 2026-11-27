package redis

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	revokedTokenPrefix = "jwt:revoked:"
	authEpochPrefix    = "auth:epoch:"
	rolesEpochPrefix   = "auth:roles-epoch:"
)

// RevokedTokenKey is the Redis key marking a single credential as revoked.
// Keyed by the token's sha256 (HashToken), never the token: the keyspace must
// not hold anything that can be replayed as a credential (same reasoning as
// SessionKey, #2049).
func RevokedTokenKey(tokenHash string) string { return revokedTokenPrefix + tokenHash }

// AuthEpochKey is the Redis key holding an account's auth epoch as Unix seconds.
func AuthEpochKey(accountID int64) string {
	return authEpochPrefix + strconv.FormatInt(accountID, 10)
}

// RolesEpochKey is the Redis key holding an account's roles epoch as Unix
// seconds. Separate key, not a second field on the auth epoch, because the two
// have different readers: the gateway consults both, the refresh path consults
// only the auth epoch (see SetRolesEpoch).
func RolesEpochKey(accountID int64) string {
	return rolesEpochPrefix + strconv.FormatInt(accountID, 10)
}

// CredentialState is what the cache was able to say about one presented JWT.
//
// Three states, not a bool, for exactly the reason AccountBlockState has three:
// collapsing "Redis is down" into "not revoked" is how a revocation silently
// stops existing during an outage, and collapsing it into "revoked" is a mass
// logout. The caller must decide, and it can only decide if it is told.
type CredentialState int

const (
	// CredentialUnknown means the cache could not answer. It is NOT "valid" —
	// rebuild the answer from the identity module, and only fail open if that
	// cannot answer either.
	CredentialUnknown CredentialState = iota
	// CredentialValid means Redis answered and holds no revocation for this
	// credential.
	CredentialValid
	// CredentialRevoked means Redis answered and this credential is void.
	CredentialRevoked
)

// SetTokenRevoked marks one credential revoked for ttl. ttl must outlive the
// credential itself (including its refresh grace window) or the entry lapses
// while the token can still be used; the identity module computes it from the row
// it just wrote.
func SetTokenRevoked(ctx context.Context, rdb *redis.Client, tokenHash string, ttl time.Duration) error {
	if rdb == nil {
		return nil
	}
	return rdb.Set(ctx, RevokedTokenKey(tokenHash), 1, ttl).Err()
}

// SetAuthEpoch publishes an account's auth epoch: every JWT with iat before it
// is void. Stored as Unix seconds because that is exactly the precision the JWT
// iat claim has — comparing at finer granularity would only invent differences
// the credential cannot express.
func SetAuthEpoch(ctx context.Context, rdb *redis.Client, accountID int64, epoch time.Time, ttl time.Duration) error {
	if rdb == nil {
		return nil
	}
	return rdb.Set(ctx, AuthEpochKey(accountID), epoch.Unix(), ttl).Err()
}

// SetRolesEpoch publishes an account's roles epoch: every JWT with iat before
// it carries a stale role set and must be exchanged before it is trusted again.
//
// This is NOT a revocation and must never be conflated with one. A role grant
// only ever ADDS capability, so an older token is under-privileged rather than
// over-privileged — safe to hold, wrong to trust. Moving the auth epoch instead
// would void the token for the refresh path too (see the identity module's
// RefreshTokenUseCase, which refuses any credential predating auth_epoch), and
// the holder would be logged out at the exact moment they gained a role.
//
// Same Unix-second precision as the auth epoch, for the same reason: that is
// all a JWT iat can express.
func SetRolesEpoch(ctx context.Context, rdb *redis.Client, accountID int64, epoch time.Time, ttl time.Duration) error {
	if rdb == nil {
		return nil
	}
	return rdb.Set(ctx, RolesEpochKey(accountID), epoch.Unix(), ttl).Err()
}

// AuthEpochTTL is how long an epoch must stay published. It is the same
// quantity as AccountBlockedTTL and derives from the same constant on purpose:
// both answer "how long can a credential minted before this event still be
// presented?", and both must outlive it. Deliberately not a second literal —
// the #1406 gap was exactly two places disagreeing about this number.
//
// The gateway auth path is what this covers: a JWT older than SessionTTL is
// already expired, so the key has no work left to do. The REFRESH path can see
// tokens older than that (it accepts expired ones inside a grace window), which
// is why refresh reads accounts.auth_epoch straight from the row rather than
// this cache.
func AuthEpochTTL() time.Duration { return AccountBlockedTTL() }

// RolesEpochTTL is how long a roles epoch must stay published. Same quantity as
// AuthEpochTTL and derived from it rather than restated, because it answers the
// identical question — "how long can a token minted before this event still be
// presented?" — about a different event.
func RolesEpochTTL() time.Duration { return AuthEpochTTL() }

// CheckAuthState asks every question Redis can answer about a caller's right to
// make this request, in ONE round-trip: is the ACCOUNT cut off (the #2050
// blacklist), and does the presented CREDENTIAL still count (#2035).
//
// One call rather than two on purpose. Each probe carries its own timeout, and
// a hanging Redis spends that timeout in full — so two sequential probes double
// the stall a degraded Redis inflicts on every authenticated request, which is
// the regression TestJWTAuth_RedisHanging_BlockCheckDoesNotStall exists to
// catch. Pipelining also keeps the healthy path at exactly the cost it had
// before revocation existed.
//
// tokenHash == "" means no bearer credential was presented (a web cookie, or an
// anonymous caller). There is then nothing for the credential lists to refuse,
// so they are not queried at all and the answer is CredentialValid — "no
// question asked", not "asked and cleared".
//
// The two axes answer with different authority, deliberately:
//
//   - the BLOCK axis treats a missing key as "not blocked". Its three writers
//     all mirror `accounts`, and a web session additionally dies at its row, so
//     the cache is a second line rather than the only one.
//   - the CREDENTIAL axis treats a missing key as UNKNOWN and never as valid
//     (see credentialStateFrom). A bearer JWT has no row of its own to fall
//     back on, so a lost or evicted key there is the entire revocation.
//
// A nil client is Unknown on both axes, not a panic: "Redis was never injected"
// is the same information deficit as "Redis is down".
func CheckAuthState(ctx context.Context, rdb *redis.Client, accountID int64, tokenHash string, issuedAt time.Time) (AccountBlockState, CredentialState) {
	if rdb == nil {
		return AccountBlockUnknown, CredentialUnknown
	}

	pipe := rdb.Pipeline()
	blocked := pipe.Exists(ctx, accountBlockedKey(accountID))
	var (
		revoked    *redis.IntCmd
		epoch      *redis.StringCmd
		rolesEpoch *redis.StringCmd
	)
	if tokenHash != "" {
		revoked = pipe.Exists(ctx, RevokedTokenKey(tokenHash))
		epoch = pipe.Get(ctx, AuthEpochKey(accountID))
		rolesEpoch = pipe.Get(ctx, RolesEpochKey(accountID))
	}
	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return AccountBlockUnknown, CredentialUnknown
	}

	return blockStateFromExists(blocked), credentialStateFrom(revoked, epoch, rolesEpoch, issuedAt)
}

func blockStateFromExists(cmd *redis.IntCmd) AccountBlockState {
	n, err := cmd.Result()
	if err != nil {
		return AccountBlockUnknown
	}
	if n > 0 {
		return AccountBlocked
	}
	return AccountNotBlocked
}

func credentialStateFrom(revoked *redis.IntCmd, epoch, rolesEpoch *redis.StringCmd, issuedAt time.Time) CredentialState {
	if revoked == nil || epoch == nil {
		return CredentialValid
	}

	if n, err := revoked.Result(); err == nil && n > 0 {
		return CredentialRevoked
	}

	if epochConvicts(epoch, issuedAt) || epochConvicts(rolesEpoch, issuedAt) {
		return CredentialRevoked
	}

	return CredentialUnknown
}

func epochConvicts(epoch *redis.StringCmd, issuedAt time.Time) bool {
	if epoch == nil || issuedAt.IsZero() {
		return false
	}
	raw, err := epoch.Result()
	if err != nil {
		return false
	}
	secs, convErr := strconv.ParseInt(raw, 10, 64)
	return convErr == nil && issuedAt.Unix() < secs
}
