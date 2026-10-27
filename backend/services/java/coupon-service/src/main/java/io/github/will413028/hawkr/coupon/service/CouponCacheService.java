package io.github.will413028.hawkr.coupon.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.redis.RedisPatternEvictor;
import io.github.will413028.hawkr.coupon.constant.CacheKeyConstants;
import io.github.will413028.hawkr.coupon.entity.CouponTemplateEntity;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Service;

import java.time.Duration;

@Service
public class CouponCacheService {

    private static final ObsLogger log = ObsLogger.forClass(CouponCacheService.class);
    private final StringRedisTemplate redisTemplate;
    private final ObjectMapper objectMapper;

    public CouponCacheService(StringRedisTemplate redisTemplate, ObjectMapper objectMapper) {
        this.redisTemplate = redisTemplate;
        this.objectMapper = objectMapper;
    }

    /**
     * 取得模板快取
     */
    public CouponTemplateEntity getTemplate(Long templateId) {
        try {
            String key = CacheKeyConstants.templateKey(templateId);
            String json = redisTemplate.opsForValue().get(key);
            if (json != null) {
                return objectMapper.readValue(json, CouponTemplateEntity.class);
            }
        } catch (Exception e) {
            log.error("CouponCacheService.getTemplate.failed", "讀取模板快取失敗", e);
        }
        return null;
    }

    /**
     * 寫入模板快取
     */
    public void putTemplate(CouponTemplateEntity template) {
        try {
            String key = CacheKeyConstants.templateKey(template.getTemplateId());
            String json = objectMapper.writeValueAsString(template);
            redisTemplate.opsForValue().set(key, json, Duration.ofSeconds(CacheKeyConstants.TEMPLATE_TTL_SECONDS));
        } catch (JsonProcessingException e) {
            log.error("CouponCacheService.putTemplate.failed", "寫入模板快取失敗", e);
        }
    }

    /**
     * 清除模板快取
     */
    public void evictTemplate(Long templateId) {
        try {
            redisTemplate.delete(CacheKeyConstants.templateKey(templateId));
        } catch (Exception e) {
            log.error("CouponCacheService.evictTemplate.failed", "清除模板快取失敗", e);
        }
    }

    /**
     * 清除所有可領取列表快取
     */
    public void evictAvailableList() {
        try {
            // 非阻塞 SCAN + UNLINK 取代阻塞式 KEYS + DEL（清除範圍不變，見 RedisPatternEvictor）
            RedisPatternEvictor.scanUnlink(redisTemplate, CacheKeyConstants.AVAILABLE_LIST_PATTERN);
        } catch (Exception e) {
            log.error("CouponCacheService.evictAvailableList.failed", "清除可領取列表快取失敗", e);
        }
    }
}
