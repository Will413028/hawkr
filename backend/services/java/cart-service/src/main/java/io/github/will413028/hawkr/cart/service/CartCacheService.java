package io.github.will413028.hawkr.cart.service;

import io.github.will413028.hawkr.cart.constant.CacheKeyConstants;
import io.github.will413028.hawkr.cart.entity.CartItemEntity;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Service;

import java.util.*;
import java.util.concurrent.TimeUnit;

/**
 * 購物車 Redis 快取操作（設計書 §3.2）
 *
 * - 僅快取 NORMAL items（LOCKED items 不進 Redis）
 * - 所有寫操作後需 DEL cache key（Cache-Aside invalidation）
 */
@Service
public class CartCacheService {

    private static final ObsLogger log = ObsLogger.forClass(CartCacheService.class);
    private final StringRedisTemplate redisTemplate;
    private final ObjectMapper objectMapper;

    public CartCacheService(StringRedisTemplate redisTemplate, ObjectMapper objectMapper) {
        this.redisTemplate = redisTemplate;
        this.objectMapper = objectMapper;
    }

    public List<CartItemEntity> getCartCache(Long buyerId, String guestId) {
        try {
            String key = CacheKeyConstants.cartKey(buyerId, guestId);
            Map<Object, Object> entries = redisTemplate.opsForHash().entries(key);

            if (entries.isEmpty()) {
                return null; // Cache Miss
            }

            List<CartItemEntity> items = new ArrayList<>();
            for (Object value : entries.values()) {
                try {
                    items.add(objectMapper.readValue(value.toString(), CartItemEntity.class));
                } catch (JsonProcessingException e) {
                    log.error("CartCacheService.deserialize.fail",
                            "反序列化快取失敗 " + (buyerId != null ? "buyerId=" + buyerId : "guestId=" + guestId), e);
                }
            }
            return items;
        } catch (Exception e) {
            log.error("CartCacheService.getCartCache.redisFail",
                    "Redis 不可用，降級至 DB " + (buyerId != null ? "buyerId=" + buyerId : "guestId=" + guestId), e);
            return null;
        }
    }

    public void fillCartCache(Long buyerId, String guestId, List<CartItemEntity> normalItems) {
        try {
            String key = CacheKeyConstants.cartKey(buyerId, guestId);

            if (normalItems.isEmpty()) {
                redisTemplate.delete(key);
                return;
            }

            Map<String, String> hashEntries = new HashMap<>();
            for (CartItemEntity item : normalItems) {
                try {
                    hashEntries.put(String.valueOf(item.getSkuId()), objectMapper.writeValueAsString(item));
                } catch (JsonProcessingException e) {
                    log.error("CartCacheService.serialize.fail",
                            "序列化快取失敗 " + (buyerId != null ? "buyerId=" + buyerId : "guestId=" + guestId), e);
                }
            }

            redisTemplate.delete(key);
            redisTemplate.opsForHash().putAll(key, hashEntries);
            redisTemplate.expire(key, CacheKeyConstants.CART_TTL_SECONDS, TimeUnit.SECONDS);
        } catch (Exception e) {
            log.error("CartCacheService.fillCartCache.redisFail",
                    "Redis 回填失敗，靜默跳過 " + (buyerId != null ? "buyerId=" + buyerId : "guestId=" + guestId), e);
        }
    }

    public void invalidateCartCache(Long buyerId, String guestId) {
        try {
            String key = CacheKeyConstants.cartKey(buyerId, guestId);
            redisTemplate.delete(key);
        } catch (Exception e) {
            log.error("CartCacheService.invalidateCartCache.redisFail",
                    "Redis 失效失敗，靜默跳過 " + (buyerId != null ? "buyerId=" + buyerId : "guestId=" + guestId), e);
        }
    }

    public Map<Long, SellerMetaCacheEntry> getSellerMetaBatch(List<Long> storeIds) {
        if (storeIds == null || storeIds.isEmpty()) {
            return Collections.emptyMap();
        }

        try {
            List<String> keys = storeIds.stream()
                    .map(CacheKeyConstants::sellerMetaKey)
                    .toList();

            List<String> values = redisTemplate.opsForValue().multiGet(keys);
            Map<Long, SellerMetaCacheEntry> result = new HashMap<>();
            if (values != null) {
                for (int i = 0; i < storeIds.size(); i++) {
                    String val = values.get(i);
                    if (val != null) {
                        try {
                            SellerMetaCacheEntry entry = objectMapper.readValue(val, SellerMetaCacheEntry.class);
                            if (!entry.hasStoreCode()) {
                                // 舊版本寫入的 JSON 欄位較少時，Jackson 是「以 null 補齊」而非拋
                                // JsonProcessingException（record canonical constructor 的預設行為），
                                // 所以上面那個 catch 攔不到它。當成 Cache Hit 的話 store_code 會一路
                                // 空到 TTL（24h）到期，而呼叫端不會回源、也不會有任何錯誤訊號。
                                // 缺必要欄位的條目不是可用的快取值，直接算 Miss 讓它回源自癒（#2098）。
                                log.info("CartCacheService.getSellerMetaBatch.staleSchema",
                                        "賣家快取缺 storeCode（舊格式），視為 Cache Miss storeId=" + storeIds.get(i));
                                continue;
                            }
                            result.put(storeIds.get(i), entry);
                        } catch (JsonProcessingException e) {
                            log.info("CartCacheService.getSellerMetaBatch.oldFormat",
                                    "賣家快取格式不相容，視為 Cache Miss storeId=" + storeIds.get(i));
                        }
                    }
                }
            }
            return result;
        } catch (Exception e) {
            log.error("CartCacheService.getSellerMetaBatch.redisFail",
                    "Redis 批次查詢賣家失敗，降級為空", e);
            return new HashMap<>();
        }
    }

    public void setSellerMeta(Long storeId, SellerMetaCacheEntry entry) {
        if (entry == null || !entry.hasStoreCode()) {
            // 讀取端已把缺 storeCode 的條目當 Miss，寫入端就不能再把它塞回去 ——
            // 否則同一個殘缺值會被寫成 24h TTL，每次讀都 Miss、每次都回源寫回同一份殘缺值。
            log.error("CartCacheService.setSellerMeta.incomplete",
                    "賣家 meta 缺 storeCode，拒絕寫入快取 storeId=" + storeId);
            return;
        }
        try {
            String key = CacheKeyConstants.sellerMetaKey(storeId);
            String json = objectMapper.writeValueAsString(entry);
            redisTemplate.opsForValue().set(key, json,
                    CacheKeyConstants.SELLER_META_TTL_SECONDS, TimeUnit.SECONDS);
        } catch (Exception e) {
            log.error("CartCacheService.setSellerMeta.redisFail",
                    "Redis 回填賣家失敗，靜默跳過 storeId=" + storeId, e);
        }
    }

    /**
     * 清除單一賣家的 seller:meta 快取（#1426 方案 A）。
     * <p>由 {@code seller.profile.changed} consumer 呼叫：賣家改店名 / logo / 封面後主動失效，
     * 下次 GetCart miss 經 gRPC lazy 回填新值，取代等 24h TTL safety net。delete 天然冪等，
     * 重放同 key 無副作用；Redis 不可用時靜默跳過（不阻斷 consumer ack）。
     */
    public void evictSellerMeta(Long storeId) {
        try {
            redisTemplate.delete(CacheKeyConstants.sellerMetaKey(storeId));
        } catch (Exception e) {
            log.error("CartCacheService.evictSellerMeta.redisFail",
                    "Redis 清除賣家快取失敗，靜默跳過 storeId=" + storeId, e);
        }
    }

    /** 賣家資訊快取條目（name + logoUrl + city + district + storeCode） */
    public record SellerMetaCacheEntry(String name, String logoUrl, String city, String district, String storeCode) {

        /**
         * storeCode 是唯一「一定有值」的欄位：{@code sellers.store_code} 是 bigint NOT NULL +
         * unique（db/schema/schema.sql），seller-service 也是無條件 FormatInt 出來的。所以空值只
         * 代表這份快取殘缺（舊 schema 或寫入時上游降級），不代表這個賣家沒有 store_code。
         * <p>name / city / district 相反 —— 它們業務上就可以是空字串，不能拿來判斷完整性。
         */
        public boolean hasStoreCode() {
            return storeCode != null && !storeCode.isBlank();
        }
    }

    public boolean tryMergeLock(String guestId, Long buyerId) {
        try {
            String key = CacheKeyConstants.mergeLockKey(guestId, buyerId);
            Boolean result = redisTemplate.opsForValue()
                    .setIfAbsent(key, "1", CacheKeyConstants.MERGE_LOCK_TTL_SECONDS, TimeUnit.SECONDS);
            return Boolean.TRUE.equals(result);
        } catch (Exception e) {
            log.error("CartCacheService.tryMergeLock.redisFail",
                    "Redis 互斥鎖失敗，允許合併（DB 層冪等） guestId=" + guestId, e);
            return true;
        }
    }
}
