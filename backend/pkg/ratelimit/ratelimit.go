// Package ratelimit provides a distributed fixed-window rate limiter with a
// bounded in-memory fallback. It counts in Redis so a quota is shared across all
// pods of a horizontally-scaled service (a per-pod limiter would multiply the
// real limit by the replica count — the flaw the old Gateway limiter had, #1416).
// When Redis is absent or errors, it degrades to a bounded, per-pod in-memory
// limiter so an infra blip never hard-blocks traffic and unbounded key
// cardinality can never exhaust memory.
//
// Two decision methods trade that guarantee off against each other. Allow (and
// Count) degrade open: they exist for general-purpose throttling, where an
// infra blip letting a bit of extra traffic through for one pod's slice of a
// window is far cheaper than an outage rejecting everyone. AllowStrict is the
// fail-closed counterpart for callers where the limiter itself *is* the
// security control (e.g. anti-abuse counters gating a paid or spammable
// side-effect) — it never silently substitutes the per-pod fallback for the
// real distributed count. On any failure — no Redis client configured, or the
// Lua script call itself failing while ordinary Redis commands still work
// (e.g. a proxy that blocks EVAL/EVALSHA) — AllowStrict returns an error
// instead of a guess, so the caller can log the real cause and decide to fail
// closed itself.
package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// ErrNoRedis is returned by AllowStrict when the Limiter has no Redis client
// configured (l.rdb == nil). Unlike Allow, which silently uses the in-memory
// fallback in this case, AllowStrict has no atomic distributed counter to
// offer at all, so it fails closed immediately.
var ErrNoRedis = errors.New("ratelimit: no redis client configured")

// fixedWindowScript atomically increments a per-key counter and, on the first
// hit of a new window, sets its expiry. It returns the post-increment count so
// the caller can compare against the limit. Fixed-window (INCR+PEXPIRE) is
// chosen over a token bucket for its single-round-trip atomicity; abuse
// throttling does not need sub-window smoothing.
const fixedWindowScript = `
local c = redis.call("INCR", KEYS[1])
if c == 1 then
	redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return c
`

// Limiter is a distributed fixed-window rate limiter.
type Limiter struct {
	rdb      *redis.Client // may be nil → always use fallback
	fallback *boundedWindows
	script   *redis.Script
	logger   *zap.Logger
}

// New builds a Limiter. rdb may be nil (e.g. Redis not configured) in which case
// every decision uses the bounded in-memory fallback. capacity caps how many
// distinct keys the fallback retains (LRU eviction) to bound memory; a
// non-positive value applies a sane default.
func New(rdb *redis.Client, capacity int, logger *zap.Logger) *Limiter {
	if capacity <= 0 {
		capacity = 10000
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Limiter{
		rdb:      rdb,
		fallback: newBoundedWindows(capacity),
		script:   redis.NewScript(fixedWindowScript),
		logger:   logger,
	}
}

// Allow reports whether one more request under key is permitted given a quota of
// limit requests per window. A non-positive limit or window means "unlimited"
// (always allowed). On any Redis failure it falls back to the bounded in-memory
// limiter rather than blocking (degrade-open).
func (l *Limiter) Allow(ctx context.Context, key string, limit int, window time.Duration) bool {
	if limit <= 0 || window <= 0 {
		return true
	}
	if l.rdb != nil {
		count, err := l.script.Run(ctx, l.rdb, []string{key}, window.Milliseconds()).Int64()
		if err == nil {
			return count <= int64(limit)
		}
		l.logger.Warn("ratelimit: redis unavailable, using in-memory fallback", zap.Error(err))
	}
	return l.fallback.allow(key, limit, window)
}

// AllowStrict is the fail-closed counterpart to Allow (see the package doc for
// when to pick which). A non-positive limit or window means "unlimited"
// (always allowed), matching Allow. Otherwise it runs the same
// fixedWindowScript and, on success, returns (count <= limit, nil) — identical
// semantics to Allow. The difference is entirely in the failure path: no
// Redis client configured, or the script call failing for any reason
// (connection down, EVAL/EVALSHA rejected, timeout, ...), returns (false,
// err) instead of consulting the bounded in-memory fallback. A non-nil error
// means "no real distributed count was obtained this call" — the caller owns
// deciding what to do about that (typically: fail closed).
func (l *Limiter) AllowStrict(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	if limit <= 0 || window <= 0 {
		return true, nil
	}
	if l.rdb == nil {
		return false, ErrNoRedis
	}
	count, err := l.script.Run(ctx, l.rdb, []string{key}, window.Milliseconds()).Int64()
	if err != nil {
		return false, fmt.Errorf("ratelimit: script execution failed: %w", err)
	}
	return count <= int64(limit), nil
}

// Count reports key's current count within its window WITHOUT incrementing it.
//
// Allow both counts and decides, which is the right shape when every request
// through a path is one unit of that path's quota. It is the wrong shape when
// the thing being counted happens on a *different* path from the thing being
// gated — auth-failure throttling counts only on 401 responses, but has to be
// checked on every request. Gating with Allow there would make each check its
// own increment and the counter would trip on ordinary traffic.
//
// Returns 0 for an absent key. On Redis failure it reads the in-memory
// fallback, and a fallback miss is also 0 — degrade-open, matching Allow: an
// infra blip must not start rejecting requests.
func (l *Limiter) Count(ctx context.Context, key string, window time.Duration) int64 {
	if l.rdb != nil {
		n, err := l.rdb.Get(ctx, key).Int64()
		switch {
		case err == nil:
			return n
		case errors.Is(err, redis.Nil):
			return 0
		default:
			l.logger.Warn("ratelimit: redis unavailable for count, using in-memory fallback", zap.Error(err))
		}
	}
	return l.fallback.count(key, window)
}
