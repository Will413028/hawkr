package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// DefaultIdempotencyTTL is the default TTL for idempotency keys (24 hours)
	DefaultIdempotencyTTL = 24 * time.Hour

	// IdempotencyKeyPrefix is the prefix for all idempotency keys
	IdempotencyKeyPrefix = "idempotent:mq"
)

// IdempotencyService handles idempotency checks for MQ messages
type IdempotencyService struct {
	rdb *redis.Client
	ttl time.Duration
}

// NewIdempotencyService creates a new idempotency service
func NewIdempotencyService(rdb *redis.Client, ttl time.Duration) *IdempotencyService {
	if ttl == 0 {
		ttl = DefaultIdempotencyTTL
	}
	return &IdempotencyService{
		rdb: rdb,
		ttl: ttl,
	}
}

// buildKey constructs the idempotency key
func (s *IdempotencyService) buildKey(eventType, eventId string) string {
	return fmt.Sprintf("%s:%s:%s", IdempotencyKeyPrefix, eventType, eventId)
}

// IsProcessed checks if an event has already been processed
func (s *IdempotencyService) IsProcessed(ctx context.Context, eventType, eventId string) (bool, error) {
	key := s.buildKey(eventType, eventId)
	exists, err := s.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("idempotency check failed: %w", err)
	}
	return exists > 0, nil
}

// MarkProcessed marks an event as processed
func (s *IdempotencyService) MarkProcessed(ctx context.Context, eventType, eventId string) error {
	key := s.buildKey(eventType, eventId)
	return s.rdb.Set(ctx, key, "1", s.ttl).Err()
}

// TryProcess attempts to process an event with idempotency protection
// Returns true if the event should be processed, false if it was already processed
func (s *IdempotencyService) TryProcess(ctx context.Context, eventType, eventId string) (bool, error) {
	key := s.buildKey(eventType, eventId)

	// Use SETNX to atomically check and set. staticcheck SA1019 suggests
	// SetArgs{Mode:"NX"} instead, but that returns *StatusCmd (redis.Nil on
	// conflict) rather than SetNX's *BoolCmd (false on conflict) — swapping
	// it would change this method's bool-return contract for every caller.
	// SetNX itself is only soft-deprecated (still fully implemented, not
	// removed) — deferred rather than risked in an unrelated change.
	set, err := s.rdb.SetNX(ctx, key, "1", s.ttl).Result() //nolint:staticcheck
	if err != nil {
		return false, fmt.Errorf("idempotency set failed: %w", err)
	}

	return set, nil
}

// WithIdempotency wraps a handler function with idempotency protection
func (s *IdempotencyService) WithIdempotency(ctx context.Context, eventType, eventId string, handler func() error) error {
	// Try to acquire the idempotency lock
	shouldProcess, err := s.TryProcess(ctx, eventType, eventId)
	if err != nil {
		return err
	}

	if !shouldProcess {
		// Already processed, skip
		return nil
	}

	// Execute handler
	if err := handler(); err != nil {
		// On failure, remove the idempotency key to allow retry
		s.rdb.Del(ctx, s.buildKey(eventType, eventId))
		return err
	}

	return nil
}

// WithIdempotencyResult wraps a handler function with idempotency protection and returns result
func WithIdempotencyResult[T any](s *IdempotencyService, ctx context.Context, eventType, eventId string, handler func() (T, error)) (T, bool, error) {
	var zero T

	shouldProcess, err := s.TryProcess(ctx, eventType, eventId)
	if err != nil {
		return zero, false, err
	}

	if !shouldProcess {
		return zero, false, nil // Already processed
	}

	result, err := handler()
	if err != nil {
		s.rdb.Del(ctx, s.buildKey(eventType, eventId))
		return zero, true, err
	}

	return result, true, nil
}
