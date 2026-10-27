package io.github.will413028.hawkr.common.redis;

import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Service;

import java.time.Duration;
import java.util.function.Supplier;

/**
 * Idempotency service for MQ message processing.
 * Prevents duplicate processing of the same event.
 */
@Slf4j
@Service
@RequiredArgsConstructor
public class IdempotencyService {

    private static final String KEY_PREFIX = "idempotent:mq";
    private static final Duration DEFAULT_TTL = Duration.ofHours(24);

    private final StringRedisTemplate redisTemplate;

    /**
     * Build the idempotency key
     */
    private String buildKey(String eventType, String eventId) {
        return String.format("%s:%s:%s", KEY_PREFIX, eventType, eventId);
    }

    /**
     * Check if an event has already been processed
     */
    public boolean isProcessed(String eventType, String eventId) {
        String key = buildKey(eventType, eventId);
        return Boolean.TRUE.equals(redisTemplate.hasKey(key));
    }

    /**
     * Mark an event as processed with default TTL
     */
    public void markProcessed(String eventType, String eventId) {
        markProcessed(eventType, eventId, DEFAULT_TTL);
    }

    /**
     * Mark an event as processed with custom TTL
     */
    public void markProcessed(String eventType, String eventId, Duration ttl) {
        String key = buildKey(eventType, eventId);
        redisTemplate.opsForValue().set(key, "1", ttl);
    }

    /**
     * Try to acquire processing right for an event.
     * Uses SETNX for atomic check-and-set.
     *
     * @return true if this instance should process the event, false if already processed
     */
    public boolean tryProcess(String eventType, String eventId) {
        return tryProcess(eventType, eventId, DEFAULT_TTL);
    }

    /**
     * Try to acquire processing right for an event with custom TTL
     */
    public boolean tryProcess(String eventType, String eventId, Duration ttl) {
        String key = buildKey(eventType, eventId);
        Boolean result = redisTemplate.opsForValue().setIfAbsent(key, "1", ttl);
        return Boolean.TRUE.equals(result);
    }

    /**
     * Remove idempotency key (for retry scenarios)
     */
    public void removeKey(String eventType, String eventId) {
        String key = buildKey(eventType, eventId);
        redisTemplate.delete(key);
    }

    /**
     * Execute a handler with idempotency protection.
     * If the event was already processed, the handler is skipped.
     * If the handler throws an exception, the idempotency key is removed to allow retry.
     *
     * @param eventType The type of event
     * @param eventId The unique event ID
     * @param handler The handler to execute
     * @return true if handler was executed, false if skipped (already processed)
     */
    public boolean withIdempotency(String eventType, String eventId, Runnable handler) {
        return withIdempotency(eventType, eventId, DEFAULT_TTL, handler);
    }

    /**
     * Execute a handler with idempotency protection and custom TTL
     */
    public boolean withIdempotency(String eventType, String eventId, Duration ttl, Runnable handler) {
        if (!tryProcess(eventType, eventId, ttl)) {
            log.debug("Event already processed, skipping: type={}, id={}", eventType, eventId);
            return false;
        }

        try {
            handler.run();
            return true;
        } catch (Exception e) {
            // On failure, remove the key to allow retry
            removeKey(eventType, eventId);
            throw e;
        }
    }

    /**
     * Execute a handler that returns a result with idempotency protection
     *
     * @param eventType The type of event
     * @param eventId The unique event ID
     * @param handler The handler to execute
     * @return IdempotencyResult containing the result and whether handler was executed
     */
    public <T> IdempotencyResult<T> withIdempotencyResult(String eventType, String eventId, Supplier<T> handler) {
        return withIdempotencyResult(eventType, eventId, DEFAULT_TTL, handler);
    }

    /**
     * Execute a handler that returns a result with idempotency protection and custom TTL
     */
    public <T> IdempotencyResult<T> withIdempotencyResult(String eventType, String eventId, Duration ttl, Supplier<T> handler) {
        if (!tryProcess(eventType, eventId, ttl)) {
            log.debug("Event already processed, skipping: type={}, id={}", eventType, eventId);
            return IdempotencyResult.skipped();
        }

        try {
            T result = handler.get();
            return IdempotencyResult.executed(result);
        } catch (Exception e) {
            removeKey(eventType, eventId);
            throw e;
        }
    }

    /**
     * Result wrapper for idempotency operations
     */
    public static class IdempotencyResult<T> {
        private final boolean executed;
        private final T result;

        private IdempotencyResult(boolean executed, T result) {
            this.executed = executed;
            this.result = result;
        }

        public static <T> IdempotencyResult<T> executed(T result) {
            return new IdempotencyResult<>(true, result);
        }

        public static <T> IdempotencyResult<T> skipped() {
            return new IdempotencyResult<>(false, null);
        }

        public boolean wasExecuted() {
            return executed;
        }

        public boolean wasSkipped() {
            return !executed;
        }

        public T getResult() {
            return result;
        }
    }
}
