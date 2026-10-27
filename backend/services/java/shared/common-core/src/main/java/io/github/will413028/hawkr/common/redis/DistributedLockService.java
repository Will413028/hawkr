package io.github.will413028.hawkr.common.redis;

import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.data.redis.core.script.DefaultRedisScript;
import org.springframework.stereotype.Service;

import java.time.Duration;
import java.util.Collections;
import java.util.UUID;
import java.util.function.Supplier;

/**
 * Distributed lock service using Redis.
 * Implements safe lock acquisition and release with Lua scripts.
 */
@Slf4j
@Service
@RequiredArgsConstructor
public class DistributedLockService {

    private static final String KEY_PREFIX = "lock";
    private static final Duration DEFAULT_TTL = Duration.ofSeconds(30);
    private static final Duration DEFAULT_RETRY_DELAY = Duration.ofMillis(100);
    private static final int DEFAULT_MAX_RETRIES = 10;

    // Lua script for safe lock release (only release if we own the lock)
    private static final String RELEASE_LOCK_SCRIPT = """
        if redis.call("GET", KEYS[1]) == ARGV[1] then
            return redis.call("DEL", KEYS[1])
        else
            return 0
        end
        """;

    // Lua script for lock extension
    private static final String EXTEND_LOCK_SCRIPT = """
        if redis.call("GET", KEYS[1]) == ARGV[1] then
            return redis.call("PEXPIRE", KEYS[1], ARGV[2])
        else
            return 0
        end
        """;

    private final StringRedisTemplate redisTemplate;

    /**
     * Build the lock key
     */
    private String buildKey(String resource, String id) {
        return String.format("%s:%s:%s", KEY_PREFIX, resource, id);
    }

    /**
     * Try to acquire a lock without retrying
     *
     * @param resource The resource type (e.g., "order", "inventory")
     * @param id The resource ID
     * @return Lock instance if acquired, null otherwise
     */
    public Lock tryAcquire(String resource, String id) {
        return tryAcquire(resource, id, DEFAULT_TTL);
    }

    /**
     * Try to acquire a lock without retrying, with custom TTL
     */
    public Lock tryAcquire(String resource, String id, Duration ttl) {
        String key = buildKey(resource, id);
        String value = UUID.randomUUID().toString();

        Boolean acquired = redisTemplate.opsForValue().setIfAbsent(key, value, ttl);
        if (Boolean.TRUE.equals(acquired)) {
            return new Lock(key, value, this);
        }
        return null;
    }

    /**
     * Acquire a lock with retrying
     *
     * @param resource The resource type
     * @param id The resource ID
     * @return Lock instance
     * @throws LockAcquisitionException if lock cannot be acquired after retries
     */
    public Lock acquire(String resource, String id) throws LockAcquisitionException {
        return acquire(resource, id, DEFAULT_TTL, DEFAULT_RETRY_DELAY, DEFAULT_MAX_RETRIES);
    }

    /**
     * Acquire a lock with custom parameters
     */
    public Lock acquire(String resource, String id, Duration ttl, Duration retryDelay, int maxRetries)
            throws LockAcquisitionException {
        String key = buildKey(resource, id);
        String value = UUID.randomUUID().toString();

        for (int i = 0; i <= maxRetries; i++) {
            Boolean acquired = redisTemplate.opsForValue().setIfAbsent(key, value, ttl);
            if (Boolean.TRUE.equals(acquired)) {
                return new Lock(key, value, this);
            }

            if (i < maxRetries) {
                try {
                    Thread.sleep(retryDelay.toMillis());
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                    throw new LockAcquisitionException("Interrupted while waiting for lock", e);
                }
            }
        }

        throw new LockAcquisitionException(
            String.format("Failed to acquire lock for %s:%s after %d retries", resource, id, maxRetries));
    }

    /**
     * Release a lock safely (only if we own it)
     */
    boolean releaseLock(String key, String value) {
        DefaultRedisScript<Long> script = new DefaultRedisScript<>(RELEASE_LOCK_SCRIPT, Long.class);
        Long result = redisTemplate.execute(script, Collections.singletonList(key), value);
        return result != null && result > 0;
    }

    /**
     * Extend a lock's TTL (only if we own it)
     */
    boolean extendLock(String key, String value, Duration ttl) {
        DefaultRedisScript<Long> script = new DefaultRedisScript<>(EXTEND_LOCK_SCRIPT, Long.class);
        Long result = redisTemplate.execute(script,
            Collections.singletonList(key), value, String.valueOf(ttl.toMillis()));
        return result != null && result > 0;
    }

    /**
     * Execute a runnable while holding a lock
     */
    public void withLock(String resource, String id, Runnable action) throws LockAcquisitionException {
        Lock lock = acquire(resource, id);
        try {
            action.run();
        } finally {
            lock.release();
        }
    }

    /**
     * Execute a supplier while holding a lock
     */
    public <T> T withLock(String resource, String id, Supplier<T> action) throws LockAcquisitionException {
        Lock lock = acquire(resource, id);
        try {
            return action.get();
        } finally {
            lock.release();
        }
    }

    /**
     * Try to execute a runnable while holding a lock (no retrying)
     *
     * @return true if lock was acquired and action executed, false if lock not acquired
     */
    public boolean tryWithLock(String resource, String id, Runnable action) {
        Lock lock = tryAcquire(resource, id);
        if (lock == null) {
            return false;
        }
        try {
            action.run();
            return true;
        } finally {
            lock.release();
        }
    }

    /**
     * Represents an acquired distributed lock
     */
    public static class Lock implements AutoCloseable {
        private final String key;
        private final String value;
        private final DistributedLockService service;
        private boolean released = false;

        Lock(String key, String value, DistributedLockService service) {
            this.key = key;
            this.value = value;
            this.service = service;
        }

        /**
         * Release the lock
         *
         * @return true if lock was released, false if we didn't own it
         */
        public boolean release() {
            if (released) {
                return false;
            }
            released = true;
            return service.releaseLock(key, value);
        }

        /**
         * Extend the lock's TTL
         *
         * @return true if extended, false if we don't own the lock anymore
         */
        public boolean extend(Duration ttl) {
            if (released) {
                return false;
            }
            return service.extendLock(key, value, ttl);
        }

        @Override
        public void close() {
            release();
        }
    }

    /**
     * Exception thrown when lock acquisition fails
     */
    public static class LockAcquisitionException extends RuntimeException {
        public LockAcquisitionException(String message) {
            super(message);
        }

        public LockAcquisitionException(String message, Throwable cause) {
            super(message, cause);
        }
    }
}
