package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.redis.RedisPatternEvictor;
import io.github.will413028.hawkr.product.constant.CacheKeyConstants;
import io.github.will413028.hawkr.product.dto.request.ProductQueryDTO;
import io.github.will413028.hawkr.product.dto.response.ProductDetailDTO;
import io.github.will413028.hawkr.product.dto.response.ProductListResultDTO;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.dao.DataAccessException;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Service;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.time.Duration;
import java.util.Optional;

/**
 * 商品快取服務
 * 使用 Cache-Aside 模式
 */
@Service
public class ProductCacheService {

    private static final ObsLogger log = ObsLogger.forClass(ProductCacheService.class);

    /**
     * 商品詳情快取過期時間（1 小時）
     */
    private static final Duration DETAIL_TTL = Duration.ofHours(1);

    /**
     * 商品列表快取過期時間（5 分鐘）
     */
    private static final Duration LIST_TTL = Duration.ofMinutes(5);

    private final StringRedisTemplate redisTemplate;
    private final ObjectMapper objectMapper;

    public ProductCacheService(StringRedisTemplate redisTemplate, ObjectMapper objectMapper) {
        this.redisTemplate = redisTemplate;
        this.objectMapper = objectMapper;
    }


    /**
     * 取得商品詳情快取（key 含 onboarding 指紋）
     */
    public Optional<ProductDetailDTO> getProductDetail(String productId, String onboardingSig) {
        String key = CacheKeyConstants.productDetailKey(productId, onboardingSig);
        String json;
        try {
            json = redisTemplate.opsForValue().get(key);
        } catch (DataAccessException e) {
            log.warn("unavailable", "Redis 失聯，詳情快取降級走 DB: " + e.getMessage(), "product_id", productId);
            return Optional.empty();
        }
        if (json == null) {
            log.info("miss", "快取未命中", "product_id", productId);
            return Optional.empty();
        }
        try {
            ProductDetailDTO dto = objectMapper.readValue(json, ProductDetailDTO.class);
            if (dto.getPublicationState() == null || dto.getPublicationState().isBlank()
                    || dto.getPurchasable() == null) {
                safeDelete(key);
                return Optional.empty();
            }
            log.info("hit", "快取命中", "product_id", productId);
            return Optional.of(dto);
        } catch (JsonProcessingException e) {
            log.warn("parseFailed", "解析快取失敗: " + e.getMessage(), "product_id", productId);
            safeDelete(key);
            return Optional.empty();
        }
    }

    /**
     * 寫入商品詳情快取（key 含 onboarding 指紋）
     */
    public void cacheProductDetail(String productId, String onboardingSig, ProductDetailDTO dto) {
        String key = CacheKeyConstants.productDetailKey(productId, onboardingSig);
        try {
            String json = objectMapper.writeValueAsString(dto);
            redisTemplate.opsForValue().set(key, json, DETAIL_TTL);
            log.info("success", "寫入快取成功", "product_id", productId);
        } catch (JsonProcessingException e) {
            log.error("failed", "序列化失敗: " + e.getMessage(), "product_id", productId);
        } catch (DataAccessException e) {
            log.warn("unavailable", "Redis 失聯，詳情快取寫回略過: " + e.getMessage(), "product_id", productId);
        }
    }

    /**
     * 清除商品詳情快取（02 §2.7：key 含 onboarding 指紋，必須 pattern wildcard delete）
     */
    public void evictProductDetail(String productId) {
        String pattern = CacheKeyConstants.detailKeyPattern(productId);
        try {
            long removed = RedisPatternEvictor.scanUnlink(redisTemplate, pattern);
            if (removed > 0) {
                log.info("success", "清除快取成功，刪除 " + removed + " 筆", "product_id", productId);
            }
        } catch (DataAccessException e) {
            log.warn("unavailable", "Redis 失聯，清除詳情快取略過: " + e.getMessage(), "product_id", productId);
        }
    }


    /**
     * 取得商品列表快取
     */
    public Optional<ProductListResultDTO> getProductList(String cacheKey) {
        String json;
        try {
            json = redisTemplate.opsForValue().get(cacheKey);
        } catch (DataAccessException e) {
            log.warn("unavailable", "Redis 失聯，列表快取降級走 DB: " + e.getMessage(), "cache_key", cacheKey);
            return Optional.empty();
        }
        if (json == null) {
            log.info("miss", "列表快取未命中", "cache_key", cacheKey);
            return Optional.empty();
        }
        try {
            ProductListResultDTO dto = objectMapper.readValue(json, ProductListResultDTO.class);
            log.info("hit", "列表快取命中", "cache_key", cacheKey);
            return Optional.of(dto);
        } catch (JsonProcessingException e) {
            log.warn("parseFailed", "解析快取失敗: " + e.getMessage(), "cache_key", cacheKey);
            safeDelete(cacheKey);
            return Optional.empty();
        }
    }

    /**
     * 寫入商品列表快取
     */
    public void cacheProductList(String cacheKey, ProductListResultDTO dto) {
        try {
            String json = objectMapper.writeValueAsString(dto);
            redisTemplate.opsForValue().set(cacheKey, json, LIST_TTL);
            log.info("success", "寫入列表快取成功", "cache_key", cacheKey);
        } catch (JsonProcessingException e) {
            log.error("failed", "序列化失敗: " + e.getMessage(), "cache_key", cacheKey);
        } catch (DataAccessException e) {
            log.warn("unavailable", "Redis 失聯，列表快取寫回略過: " + e.getMessage(), "cache_key", cacheKey);
        }
    }

    /**
     * 清除特定擁有者的所有列表快取
     */
    public void evictProductList(Long storeId) {
        String pattern = CacheKeyConstants.productListPattern(storeId);
        try {
            long removed = RedisPatternEvictor.scanUnlink(redisTemplate, pattern);
            if (removed > 0) {
                log.info("success", "清除列表快取成功，刪除 " + removed + " 筆", "store_id", String.valueOf(storeId));
            }
        } catch (DataAccessException e) {
            log.warn("unavailable", "Redis 失聯，清除列表快取略過: " + e.getMessage(), "store_id", String.valueOf(storeId));
        }
    }

    /**
     * 商品寫入路徑統一列表失效入口：同時清除 owner bucket 與公開聚合 bucket 0。
     *
     * <p>公開主列表 {@code /public/products?store_ids=...} 的 cache key ownerId 恆為 0
     * （Gateway 帶複數 store_ids → query.storeId 為 null → ownerId 落到 0），
     * 與賣家自身 bucket（{@code prod:list:{ownerId}:*}）分屬不同分桶。任何改變商品「公開可見集合」
     * 的寫入都必須同時清這兩桶，否則公開列表會陳舊至 LIST_TTL，並被 frontend 回灌進 24h SSR 快取。
     * 寫入路徑一律走本方法，禁止各自手拼 evictProductList(owner) + evictProductList(0L)。
     */
    public void evictProductListForWrite(Long ownerId) {
        evictProductList(ownerId);
        if (ownerId != null && ownerId != 0L) {
            evictProductList(0L);
        }
    }

    /**
     * 批次列表失效：多商品可能橫跨多賣家 owner bucket（庫存 / 銷量 cascade 一次波及多 product）。
     *
     * <p>語意等同對每個 distinct owner 呼叫 {@link #evictProductListForWrite(Long)}，但去重 owner、
     * 且公開聚合 bucket 0 僅清一次——避免 N 個 owner 各觸發一次 bucket-0 pattern delete。
     * 寫入路徑同樣禁止各自手拼 owner + bucket 0，多 owner 場景一律走本方法。
     */
    public void evictProductListForWriteBatch(java.util.Collection<Long> ownerIds) {
        if (ownerIds == null || ownerIds.isEmpty()) {
            return;
        }
        java.util.Set<Long> distinctOwners = new java.util.LinkedHashSet<>();
        boolean needPublicBucket = false;
        for (Long owner : ownerIds) {
            if (owner == null) continue;
            distinctOwners.add(owner);
            if (owner != 0L) needPublicBucket = true;
        }
        for (Long owner : distinctOwners) {
            evictProductList(owner);
        }
        if (needPublicBucket && !distinctOwners.contains(0L)) {
            evictProductList(0L);
        }
    }

    /**
     * 清除所有商品列表快取
     */
    public void evictAllProductLists() {
        String pattern = CacheKeyConstants.PRODUCT_LIST_PREFIX + "*";
        try {
            long removed = RedisPatternEvictor.scanUnlink(redisTemplate, pattern);
            if (removed > 0) {
                log.info("success", "清除所有列表快取成功，刪除 " + removed + " 筆");
            }
        } catch (DataAccessException e) {
            log.warn("unavailable", "Redis 失聯，清除所有列表快取略過: " + e.getMessage());
        }
    }


    private static final Duration ORIGINS_SUMMARY_TTL = Duration.ofMinutes(5);

    public String generateOriginsSummaryCacheKey(Long categoryId, int limit) {
        return CacheKeyConstants.productOriginsSummaryKey(categoryId, limit);
    }

    public Optional<java.util.List<io.github.will413028.hawkr.product.repository.ProductRepository.OriginSummaryRow>>
            getOriginsSummary(String cacheKey) {
        String json;
        try {
            json = redisTemplate.opsForValue().get(cacheKey);
        } catch (DataAccessException e) {
            log.warn("originsSummary.unavailable", "Redis 失聯，產地聚合快取降級走 DB: " + e.getMessage(), "key", cacheKey);
            return Optional.empty();
        }
        if (json == null) {
            return Optional.empty();
        }
        try {
            OriginSummaryCacheEntry[] entries = objectMapper.readValue(json, OriginSummaryCacheEntry[].class);
            java.util.List<io.github.will413028.hawkr.product.repository.ProductRepository.OriginSummaryRow> rows =
                    new java.util.ArrayList<>(entries.length);
            for (OriginSummaryCacheEntry e : entries) {
                rows.add(e);
            }
            return Optional.of(rows);
        } catch (JsonProcessingException e) {
            log.warn("originsSummary.parseFailed", "解析產地聚合快取失敗: " + e.getMessage(), "key", cacheKey);
            safeDelete(cacheKey);
            return Optional.empty();
        }
    }

    public void cacheOriginsSummary(String cacheKey,
            java.util.List<io.github.will413028.hawkr.product.repository.ProductRepository.OriginSummaryRow> rows) {
        try {
            OriginSummaryCacheEntry[] entries = new OriginSummaryCacheEntry[rows.size()];
            for (int i = 0; i < rows.size(); i++) {
                entries[i] = new OriginSummaryCacheEntry(rows.get(i).getOrigin(), rows.get(i).getProductCount());
            }
            String json = objectMapper.writeValueAsString(entries);
            redisTemplate.opsForValue().set(cacheKey, json, ORIGINS_SUMMARY_TTL);
        } catch (JsonProcessingException e) {
            log.warn("originsSummary.cacheWriteFailed", "寫入產地聚合快取失敗: " + e.getMessage(), "key", cacheKey);
        } catch (DataAccessException e) {
            log.warn("originsSummary.unavailable", "Redis 失聯，產地聚合快取寫回略過: " + e.getMessage(), "key", cacheKey);
        }
    }

    /**
     * 產地聚合快取項目（直接是 POJO，Jackson 與 MyBatis 共用）。
     */
    public static class OriginSummaryCacheEntry
            extends io.github.will413028.hawkr.product.repository.ProductRepository.OriginSummaryRow {
        public OriginSummaryCacheEntry() { super(); }

        public OriginSummaryCacheEntry(String origin, String productCount) {
            super(origin, productCount);
        }
    }


    /**
     * best-effort 刪除壞快取：Redis 失聯時 log.warn 不拋（02 §2.7.3）。
     * 用於解析失敗（JsonProcessingException）後清除無效 key，避免與 redis 失聯疊加二次拋。
     */
    private void safeDelete(String key) {
        try {
            redisTemplate.delete(key);
        } catch (DataAccessException e) {
            log.warn("unavailable", "Redis 失聯，刪除壞快取略過: " + e.getMessage(), "key", key);
        }
    }

    /**
     * 生成列表快取鍵
     * 格式：prod:list:{ownerId}:{queryHash}（02 §2.7.2：list 不在 key 加 onboarding sig，改為 cache 存「pre-filter snapshot」，
     * 由 ProductService 在 cache 取出後逐項套 onboarding filter — 避免 cache lookup 與 sig 計算雞生蛋問題）
     */
    public String generateListCacheKey(ProductQueryDTO query) {
        Long ownerId = query.getStoreId() != null ? query.getStoreId()
                : (query.getOwnerStoreId() != null ? query.getOwnerStoreId() : 0L);
        String queryHash = hashQuery(query);
        return CacheKeyConstants.productListKey(ownerId, queryHash, "all");
    }

    /**
     * 計算查詢條件的雜湊值
     */
    private String hashQuery(ProductQueryDTO query) {
        StringBuilder sb = new StringBuilder();
        sb.append("cat:");
        if (query.getCategoryIds() != null && !query.getCategoryIds().isEmpty()) {
            java.util.List<Long> sortedCats = new java.util.ArrayList<>(query.getCategoryIds());
            java.util.Collections.sort(sortedCats);
            for (int i = 0; i < sortedCats.size(); i++) {
                if (i > 0) sb.append('|');
                sb.append(sortedCats.get(i));
            }
        }
        sb.append(",kw:").append(query.getKeyword() != null ? query.getKeyword() : "");
        sb.append(",page:").append(query.getPage());
        sb.append(",size:").append(query.getPageSize());
        sb.append(",sort:").append(query.getSort() != null ? query.getSort() : "");
        sb.append(",status:").append(query.getStatus() != null ? query.getStatus() : "");
        sb.append(",pub:").append(query.isPublicApi() ? 1 : 0);
        if (query.getHashtags() != null && !query.getHashtags().isEmpty()) {
            java.util.List<String> sorted = new java.util.ArrayList<>(query.getHashtags());
            java.util.Collections.sort(sorted);
            sb.append(",ht:").append(String.join("|", sorted));
            sb.append(",htm:").append(query.isHashtagMatchAll() ? "all" : "any");
        }
        if (query.getDeliveryTempFilter() != null && !query.getDeliveryTempFilter().isEmpty()) {
            java.util.List<String> sortedTemps = new java.util.ArrayList<>(query.getDeliveryTempFilter());
            java.util.Collections.sort(sortedTemps);
            sb.append(",dt:").append(String.join("|", sortedTemps));
        }
        if (query.getCertificationTypes() != null && !query.getCertificationTypes().isEmpty()) {
            java.util.List<String> sortedCerts = new java.util.ArrayList<>(query.getCertificationTypes());
            java.util.Collections.sort(sortedCerts);
            sb.append(",ct:").append(String.join("|", sortedCerts));
        }
        if (query.getPriceMinFilter() != null) {
            sb.append(",pmin:").append(query.getPriceMinFilter());
        }
        if (query.getPriceMaxFilter() != null) {
            sb.append(",pmax:").append(query.getPriceMaxFilter());
        }
        if (query.getStoreIds() != null && !query.getStoreIds().isEmpty()) {
            java.util.List<Long> sortedSellers = new java.util.ArrayList<>(query.getStoreIds());
            java.util.Collections.sort(sortedSellers);
            sb.append(",sids:");
            for (int i = 0; i < sortedSellers.size(); i++) {
                if (i > 0) sb.append('|');
                sb.append(sortedSellers.get(i));
            }
        }
        if (query.isFollowedProductOnly()) {
            sb.append(",fp:").append(query.getViewerBuyerId() != null ? query.getViewerBuyerId() : 0);
        }
        if (query.isFollowedSellerOnly()) {
            sb.append(",fs:").append(query.getViewerBuyerId() != null ? query.getViewerBuyerId() : 0);
        }

        try {
            MessageDigest md = MessageDigest.getInstance("MD5");
            byte[] digest = md.digest(sb.toString().getBytes(StandardCharsets.UTF_8));
            StringBuilder hexString = new StringBuilder();
            for (byte b : digest) {
                String hex = Integer.toHexString(0xff & b);
                if (hex.length() == 1) {
                    hexString.append('0');
                }
                hexString.append(hex);
            }
            return hexString.substring(0, 8);
        } catch (NoSuchAlgorithmException e) {
            return String.valueOf(sb.toString().hashCode());
        }
    }
}
