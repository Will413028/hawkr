package io.github.will413028.hawkr.productimport.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Service;

import java.time.Duration;
import java.util.UUID;

/**
 * GetCategoryConfirmationToken / BatchUpdateItemsCategory Redis token 管理（40 號 §4.2.13a）。
 *
 * <p>key 格式：{@code product-import:cat-token:{jobId}:{token}}，TTL = 300s。
 */
@Service
public class CategoryTokenService {

    private static final ObsLogger log = ObsLogger.forClass(CategoryTokenService.class);
    private static final String KEY_PREFIX = "product-import:cat-token:";

    @Value("${hawkr.product-import.confirmation-token.ttl-seconds:300}")
    private int ttlSeconds;

    private final StringRedisTemplate redis;

    public CategoryTokenService(StringRedisTemplate redis) {
        this.redis = redis;
    }

    public IssuedToken issue(long jobId, String categoryId) {
        String token = UUID.randomUUID().toString();
        String key = key(jobId, token);
        redis.opsForValue().set(key, categoryId, Duration.ofSeconds(ttlSeconds));
        return new IssuedToken(token, ttlSeconds);
    }

    /** GETDEL — 單次使用；回傳 null 表示 token 不存在或已過期。 */
    public String consume(long jobId, String token) {
        if (token == null || token.isBlank()) return null;
        String key = key(jobId, token);
        try {
            return redis.opsForValue().getAndDelete(key);
        } catch (Exception e) {
            log.warn("CategoryTokenService.consume.error", "Failed redis GETDEL: " + e.getMessage());
            return null;
        }
    }

    private String key(long jobId, String token) {
        return KEY_PREFIX + jobId + ":" + token;
    }

    public record IssuedToken(String token, int ttlSeconds) {}
}
