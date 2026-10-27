package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.product.constant.CacheKeyConstants;
import io.github.will413028.hawkr.product.entity.CategoryEntity;
import io.github.will413028.hawkr.product.repository.CategoryRepository;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.dao.DataAccessException;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Service;

import java.time.Duration;
import java.util.*;
import java.util.stream.Collectors;

/**
 * 分類表快取服務
 *
 * 整表快取策略：分類表資料量小（幾十～幾百筆），
 * 以 cat:all → Map&lt;Long, CategoryEntity&gt; 的形式快取整張表，
 * 所有讀取操作從快取取，寫入操作清除快取。
 */
@Service
public class CategoryCacheService {

    private static final ObsLogger log = ObsLogger.forClass(CategoryCacheService.class);

    /**
     * 分類快取過期時間（2 小時）
     */
    private static final Duration CATEGORY_TTL = Duration.ofHours(2);

    private final StringRedisTemplate redisTemplate;
    private final ObjectMapper objectMapper;
    private final CategoryRepository categoryRepository;

    public CategoryCacheService(StringRedisTemplate redisTemplate,
                                ObjectMapper objectMapper,
                                CategoryRepository categoryRepository) {
        this.redisTemplate = redisTemplate;
        this.objectMapper = objectMapper;
        this.categoryRepository = categoryRepository;
    }

    /**
     * 取得所有分類的 Map（快取優先）
     *
     * @return Map&lt;categoryId, CategoryEntity&gt;
     */
    public Map<Long, CategoryEntity> getAllCategoryMap() {
        // 1. 查快取
        try {
            String json = redisTemplate.opsForValue().get(CacheKeyConstants.CATEGORY_ALL);
            if (json != null) {
                Map<Long, CategoryEntity> map = objectMapper.readValue(json,
                        new TypeReference<Map<Long, CategoryEntity>>() {});
                log.info("hit", "分類快取命中", "size", String.valueOf(map.size()));
                return map;
            }
        } catch (JsonProcessingException e) {
            log.warn("parseFailed", "分類快取解析失敗: " + e.getMessage());
            safeDelete(CacheKeyConstants.CATEGORY_ALL);
        } catch (DataAccessException e) {
            // Redis 失聯 → 降級走 DB（02 §2.7.3 graceful degradation），落到 step 2
            log.warn("unavailable", "Redis 失聯，分類快取降級走 DB: " + e.getMessage());
        }

        // 2. 快取未命中（或 Redis 失聯），查 DB
        log.info("miss", "分類快取未命中，查詢 DB");
        List<CategoryEntity> all = categoryRepository.findAll();
        Map<Long, CategoryEntity> map = all.stream()
                .collect(Collectors.toMap(CategoryEntity::getId, c -> c));

        // 3. 寫入快取
        try {
            String json = objectMapper.writeValueAsString(map);
            redisTemplate.opsForValue().set(CacheKeyConstants.CATEGORY_ALL, json, CATEGORY_TTL);
            log.info("cached", "分類快取已寫入", "size", String.valueOf(map.size()));
        } catch (JsonProcessingException e) {
            log.error("serializeFailed", "分類快取序列化失敗: " + e.getMessage());
        } catch (DataAccessException e) {
            // Redis 失聯 → log.warn 不影響當次回應（資料已從 DB 取得，02 §2.7.3）
            log.warn("unavailable", "Redis 失聯，分類快取寫回略過: " + e.getMessage());
        }

        return map;
    }

    /**
     * 根據 ID 取得分類（從快取）
     */
    public Optional<CategoryEntity> findById(Long id) {
        if (id == null) {
            return Optional.empty();
        }
        return Optional.ofNullable(getAllCategoryMap().get(id));
    }

    /**
     * 根據狀態取得分類列表（從快取過濾）
     */
    public List<CategoryEntity> findByStatus(String status) {
        return getAllCategoryMap().values().stream()
                .filter(c -> status.equals(c.getStatus()))
                .sorted(Comparator.comparing(CategoryEntity::getId))
                .collect(Collectors.toList());
    }

    /**
     * 根據 parent_id 取得子分類（從快取過濾）
     */
    public List<CategoryEntity> findByParentId(Long parentId) {
        return getAllCategoryMap().values().stream()
                .filter(c -> parentId.equals(c.getParentId()))
                .sorted(Comparator.comparing(CategoryEntity::getId))
                .collect(Collectors.toList());
    }

    /**
     * 取得頂層分類（parent_id IS NULL）
     */
    public List<CategoryEntity> findTopCategories() {
        return getAllCategoryMap().values().stream()
                .filter(c -> c.getParentId() == null)
                .sorted(Comparator.comparing(CategoryEntity::getId))
                .collect(Collectors.toList());
    }

    /**
     * 取得所有分類列表（從快取）
     */
    public List<CategoryEntity> findAll() {
        return getAllCategoryMap().values().stream()
                .sorted(Comparator.comparing(CategoryEntity::getId))
                .collect(Collectors.toList());
    }

    /**
     * 清除分類快取
     */
    public void evict() {
        safeDelete(CacheKeyConstants.CATEGORY_ALL);
        log.info("evicted", "分類快取已清除");
    }

    /**
     * best-effort 刪除：Redis 失聯時 log.warn 不拋（02 §2.7.3），不阻斷觸發清除的寫入業務。
     */
    private void safeDelete(String key) {
        try {
            redisTemplate.delete(key);
        } catch (DataAccessException e) {
            log.warn("unavailable", "Redis 失聯，清除分類快取略過: " + e.getMessage());
        }
    }
}
