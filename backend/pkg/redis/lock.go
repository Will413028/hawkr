package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	// DefaultLockTTL is the default lock TTL (30 seconds)
	DefaultLockTTL = 30 * time.Second

	// DefaultRetryDelay is the default delay between retry attempts
	DefaultRetryDelay = 100 * time.Millisecond

	// DefaultMaxRetries is the default number of retry attempts
	DefaultMaxRetries = 10

	// LockKeyPrefix is the prefix for all lock keys
	LockKeyPrefix = "lock"
)

var (
	// ErrLockNotAcquired is returned when the lock could not be acquired
	ErrLockNotAcquired = errors.New("failed to acquire lock")

	// ErrLockNotOwned is returned when trying to release a lock not owned by this instance
	ErrLockNotOwned = errors.New("lock not owned by this instance")
)

// Lua script for safe lock release (only release if we own the lock)
const releaseLockScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end
`

// Lua script for lock extension
const extendLockScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("PEXPIRE", KEYS[1], ARGV[2])
else
    return 0
end
`

// LockService handles distributed locking
type LockService struct {
	rdb        *redis.Client
	ttl        time.Duration
	retryDelay time.Duration
	maxRetries int
}

// LockOption is a function that configures the LockService
type LockOption func(*LockService)

// WithLockTTL sets the lock TTL
func WithLockTTL(ttl time.Duration) LockOption {
	return func(s *LockService) {
		s.ttl = ttl
	}
}

// WithRetryDelay sets the retry delay
func WithRetryDelay(delay time.Duration) LockOption {
	return func(s *LockService) {
		s.retryDelay = delay
	}
}

// WithMaxRetries sets the maximum number of retries
func WithMaxRetries(retries int) LockOption {
	return func(s *LockService) {
		s.maxRetries = retries
	}
}

// NewLockService creates a new distributed lock service
func NewLockService(rdb *redis.Client, opts ...LockOption) *LockService {
	s := &LockService{
		rdb:        rdb,
		ttl:        DefaultLockTTL,
		retryDelay: DefaultRetryDelay,
		maxRetries: DefaultMaxRetries,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Lock represents an acquired distributed lock
type Lock struct {
	key     string
	value   string
	service *LockService
}

// buildKey constructs the lock key
func (s *LockService) buildKey(resource, id string) string {
	return fmt.Sprintf("%s:%s:%s", LockKeyPrefix, resource, id)
}

// Acquire attempts to acquire a lock with the given resource and id
func (s *LockService) Acquire(ctx context.Context, resource, id string) (*Lock, error) {
	return s.AcquireWithTTL(ctx, resource, id, s.ttl)
}

// AcquireWithTTL attempts to acquire a lock with a custom TTL
func (s *LockService) AcquireWithTTL(ctx context.Context, resource, id string, ttl time.Duration) (*Lock, error) {
	key := s.buildKey(resource, id)
	value := uuid.New().String()

	for i := 0; i <= s.maxRetries; i++ {
		// staticcheck SA1019 suggests SetArgs{Mode:"NX"} instead, but that
		// returns *StatusCmd (redis.Nil on conflict) rather than SetNX's
		// *BoolCmd (false on conflict) — swapping it would change this
		// method's bool-return contract. SetNX is only soft-deprecated
		// (still fully implemented, not removed) — deferred rather than
		// risked in an unrelated change to this lock-acquisition path.
		set, err := s.rdb.SetNX(ctx, key, value, ttl).Result() //nolint:staticcheck
		if err != nil {
			return nil, fmt.Errorf("failed to acquire lock: %w", err)
		}

		if set {
			return &Lock{
				key:     key,
				value:   value,
				service: s,
			}, nil
		}

		if i < s.maxRetries {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(s.retryDelay):
				// Continue to next retry
			}
		}
	}

	return nil, ErrLockNotAcquired
}

// TryAcquire attempts to acquire a lock without retrying
func (s *LockService) TryAcquire(ctx context.Context, resource, id string) (*Lock, error) {
	return s.TryAcquireWithTTL(ctx, resource, id, s.ttl)
}

// TryAcquireWithTTL attempts to acquire a lock with a custom TTL without retrying
func (s *LockService) TryAcquireWithTTL(ctx context.Context, resource, id string, ttl time.Duration) (*Lock, error) {
	key := s.buildKey(resource, id)
	value := uuid.New().String()

	// staticcheck SA1019 suggests SetArgs{Mode:"NX"} instead, but that
	// returns *StatusCmd (redis.Nil on conflict) rather than SetNX's
	// *BoolCmd (false on conflict) — swapping it would change this
	// method's bool-return contract. SetNX is only soft-deprecated (still
	// fully implemented, not removed) — deferred rather than risked in an
	// unrelated change to this lock-acquisition path.
	set, err := s.rdb.SetNX(ctx, key, value, ttl).Result() //nolint:staticcheck
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}

	if !set {
		return nil, ErrLockNotAcquired
	}

	return &Lock{
		key:     key,
		value:   value,
		service: s,
	}, nil
}

// Release releases the lock
func (l *Lock) Release(ctx context.Context) error {
	result, err := l.service.rdb.Eval(ctx, releaseLockScript, []string{l.key}, l.value).Int64()
	if err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}

	if result == 0 {
		return ErrLockNotOwned
	}

	return nil
}

// Extend extends the lock TTL
func (l *Lock) Extend(ctx context.Context, ttl time.Duration) error {
	result, err := l.service.rdb.Eval(ctx, extendLockScript, []string{l.key}, l.value, ttl.Milliseconds()).Int64()
	if err != nil {
		return fmt.Errorf("failed to extend lock: %w", err)
	}

	if result == 0 {
		return ErrLockNotOwned
	}

	return nil
}

// WithLock executes a function while holding a lock
func (s *LockService) WithLock(ctx context.Context, resource, id string, fn func() error) error {
	lock, err := s.Acquire(ctx, resource, id)
	if err != nil {
		return err
	}
	defer func() { _ = lock.Release(ctx) }()

	return fn()
}

// WithLockResult executes a function that returns a result while holding a lock
func WithLockResult[T any](s *LockService, ctx context.Context, resource, id string, fn func() (T, error)) (T, error) {
	var zero T

	lock, err := s.Acquire(ctx, resource, id)
	if err != nil {
		return zero, err
	}
	defer func() { _ = lock.Release(ctx) }()

	return fn()
}
