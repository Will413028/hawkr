// Package blocklist provides a Redis-backed bidirectional user block check
// for hot query paths in chat, post, and review services.
package blocklist

import (
	"context"
	"errors"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
)

// ErrRedisUnavailable is returned by Add/Remove when rdb is nil, mirroring
// IsBlocked's fail-open treatment of a nil client. Callers (e.g. block.go)
// already treat these as best-effort and discard the error.
var ErrRedisUnavailable = errors.New("blocklist: redis client unavailable")

func keyOut(userID int64) string { return "block:out:" + strconv.FormatInt(userID, 10) }
func keyIn(userID int64) string  { return "block:in:" + strconv.FormatInt(userID, 10) }

var (
	metricCacheHit = promauto.NewCounter(prometheus.CounterOpts{
		Name: "blocklist_cache_hit_total",
		Help: "Number of IsBlocked checks where the viewer is blocked from seeing the other user.",
	})
	metricCacheMiss = promauto.NewCounter(prometheus.CounterOpts{
		Name: "blocklist_cache_miss_total",
		Help: "Number of IsBlocked checks where no block exists between viewer and other.",
	})
	metricCacheUnavailable = promauto.NewCounter(prometheus.CounterOpts{
		Name: "blocklist_cache_unavailable_total",
		Help: "Number of IsBlocked checks that failed open because Redis was unavailable.",
	})
)

// MetricCacheHit returns the counter incremented when IsBlocked finds a block.
// Exposed for test verification and ops introspection.
func MetricCacheHit() prometheus.Counter { return metricCacheHit }

// MetricCacheMiss returns the counter incremented when IsBlocked finds no block.
func MetricCacheMiss() prometheus.Counter { return metricCacheMiss }

// MetricCacheUnavailable returns the counter incremented when IsBlocked fails open.
func MetricCacheUnavailable() prometheus.Counter { return metricCacheUnavailable }

// Add records that blockerID has blocked blockedID. Writes both forward
// (block:out:blocker) and reverse (block:in:blocked) indices in a single
// transaction so IsBlocked can answer with one user's keys.
func Add(ctx context.Context, rdb *redis.Client, blockerID, blockedID int64) error {
	if rdb == nil {
		metricCacheUnavailable.Inc()
		return ErrRedisUnavailable
	}
	pipe := rdb.TxPipeline()
	pipe.SAdd(ctx, keyOut(blockerID), blockedID)
	pipe.SAdd(ctx, keyIn(blockedID), blockerID)
	_, err := pipe.Exec(ctx)
	return err
}

// Remove undoes Add: removes the entry from both block:out:blocker and
// block:in:blocked. Idempotent — safe to call when entry does not exist.
func Remove(ctx context.Context, rdb *redis.Client, blockerID, blockedID int64) error {
	if rdb == nil {
		metricCacheUnavailable.Inc()
		return ErrRedisUnavailable
	}
	pipe := rdb.TxPipeline()
	pipe.SRem(ctx, keyOut(blockerID), blockedID)
	pipe.SRem(ctx, keyIn(blockedID), blockerID)
	_, err := pipe.Exec(ctx)
	return err
}

// IsBlocked reports whether viewerID should be denied access to otherID's content.
// Detects mutual block: returns true if viewer blocked other OR other blocked viewer.
// Returns false (fail-open) if Redis is unavailable or rdb is nil.
//
// This is a SOCIAL block (who may see whose posts), not an authentication gate,
// and that is why fail-open here needs no source-of-truth fallback: the worst
// case is that one piece of content is visible to someone who muted its author.
// Do not read it as endorsement of the pattern for anything that decides
// ACCESS: collapsing "Redis could not answer" into a definite "no" on an auth
// path silently disables the check for the whole of an outage, which is why the
// account blacklist gained the tri-state pkgredis.CheckAccountBlocked (#2050)
// and why its fail-open bool no longer exists.
func IsBlocked(ctx context.Context, rdb *redis.Client, viewerID, otherID int64) bool {
	if rdb == nil {
		metricCacheUnavailable.Inc()
		return false
	}
	out, err := rdb.SIsMember(ctx, keyOut(viewerID), otherID).Result()
	if err != nil {
		metricCacheUnavailable.Inc()
		return false
	}
	if out {
		metricCacheHit.Inc()
		return true
	}
	in, err := rdb.SIsMember(ctx, keyIn(viewerID), otherID).Result()
	if err != nil {
		metricCacheUnavailable.Inc()
		return false
	}
	if in {
		metricCacheHit.Inc()
		return true
	}
	metricCacheMiss.Inc()
	return false
}
