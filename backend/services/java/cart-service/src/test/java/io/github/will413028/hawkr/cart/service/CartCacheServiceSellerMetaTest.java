package io.github.will413028.hawkr.cart.service;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.Mockito;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.data.redis.core.ValueOperations;

import java.util.List;
import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * seller:meta 快取的「完整性」語意（backend#2098）。
 *
 * <p>背景：{@code SellerMetaCacheEntry} 這個 record 歷史上被擴充過三次
 * （name+logoUrl → +city+district → +storeCode）。每次擴充，Redis 裡都還躺著上一版寫入的
 * JSON，TTL 24h。關鍵事實是 —— Jackson 對 record 少掉的欄位是<b>以 null 補齊</b>，不是拋
 * {@code JsonProcessingException}。所以舊格式條目會被當成 Cache <b>Hit</b>，呼叫端不回源、
 * 沒有任何錯誤訊號，store_code 就這樣空到 TTL 到期為止。
 */
class CartCacheServiceSellerMetaTest {

    private static final long STORE_ID = 2685914682L;
    private static final String KEY = "seller:meta:" + STORE_ID;

    private final ObjectMapper objectMapper = new ObjectMapper();

    @SuppressWarnings("unchecked")
    private ValueOperations<String, String> stubValueOps(StringRedisTemplate redis) {
        ValueOperations<String, String> valueOps = Mockito.mock(ValueOperations.class);
        Mockito.when(redis.opsForValue()).thenReturn(valueOps);
        return valueOps;
    }

    @Test
    @DisplayName("舊 schema（沒有 storeCode 欄位）的快取條目視為 Cache Miss，讓呼叫端回源")
    void staleSchemaEntryWithoutStoreCode_isTreatedAsCacheMiss() {
        // 這串就是 586ac3ea7 之前的寫入格式，Jackson 讀得進來、storeCode 會是 null。
        String staleJson = "{\"name\":\"陳記蔬果\",\"logoUrl\":\"seller/logo/a.jpg\","
                + "\"city\":\"台北市\",\"district\":\"大安區\"}";
        StringRedisTemplate redis = Mockito.mock(StringRedisTemplate.class);
        ValueOperations<String, String> valueOps = stubValueOps(redis);
        Mockito.when(valueOps.multiGet(List.of(KEY))).thenReturn(List.of(staleJson));

        CartCacheService cache = new CartCacheService(redis, objectMapper);
        Map<Long, CartCacheService.SellerMetaCacheEntry> result = cache.getSellerMetaBatch(List.of(STORE_ID));

        assertThat(result)
                .as("缺 storeCode 的條目不是可用的快取值 —— 回報成 Hit 會讓 store_code 空到 TTL(24h) 到期")
                .isEmpty();
    }

    @Test
    @DisplayName("storeCode 是空字串同樣視為 Cache Miss（不只 null）")
    void blankStoreCode_isTreatedAsCacheMiss() {
        String blankJson = "{\"name\":\"陳記蔬果\",\"logoUrl\":\"\",\"city\":\"\",\"district\":\"\",\"storeCode\":\"\"}";
        StringRedisTemplate redis = Mockito.mock(StringRedisTemplate.class);
        ValueOperations<String, String> valueOps = stubValueOps(redis);
        Mockito.when(valueOps.multiGet(List.of(KEY))).thenReturn(List.of(blankJson));

        CartCacheService cache = new CartCacheService(redis, objectMapper);

        assertThat(cache.getSellerMetaBatch(List.of(STORE_ID))).isEmpty();
    }

    @Test
    @DisplayName("完整條目仍然是 Cache Hit，且 storeCode 原樣帶出")
    void completeEntry_isCacheHitAndCarriesStoreCode() {
        String freshJson = "{\"name\":\"陳記蔬果\",\"logoUrl\":\"seller/logo/a.jpg\","
                + "\"city\":\"台北市\",\"district\":\"大安區\",\"storeCode\":\"2612345678\"}";
        StringRedisTemplate redis = Mockito.mock(StringRedisTemplate.class);
        ValueOperations<String, String> valueOps = stubValueOps(redis);
        Mockito.when(valueOps.multiGet(List.of(KEY))).thenReturn(List.of(freshJson));

        CartCacheService cache = new CartCacheService(redis, objectMapper);
        Map<Long, CartCacheService.SellerMetaCacheEntry> result = cache.getSellerMetaBatch(List.of(STORE_ID));

        assertThat(result).containsOnlyKeys(STORE_ID);
        assertThat(result.get(STORE_ID).storeCode()).isEqualTo("2612345678");
    }

    @Test
    @DisplayName("拒絕把缺 storeCode 的 meta 寫回快取，避免殘缺值被續命 24h")
    void setSellerMeta_refusesIncompleteEntry() {
        StringRedisTemplate redis = Mockito.mock(StringRedisTemplate.class);
        ValueOperations<String, String> valueOps = stubValueOps(redis);

        CartCacheService cache = new CartCacheService(redis, objectMapper);
        cache.setSellerMeta(STORE_ID,
                new CartCacheService.SellerMetaCacheEntry("陳記蔬果", "logo.jpg", "台北市", "大安區", ""));

        Mockito.verify(valueOps, Mockito.never())
                .set(Mockito.anyString(), Mockito.anyString(), Mockito.anyLong(), Mockito.any());
    }

    @Test
    @DisplayName("完整的 meta 正常寫入快取")
    void setSellerMeta_writesCompleteEntry() {
        StringRedisTemplate redis = Mockito.mock(StringRedisTemplate.class);
        ValueOperations<String, String> valueOps = stubValueOps(redis);

        CartCacheService cache = new CartCacheService(redis, objectMapper);
        cache.setSellerMeta(STORE_ID,
                new CartCacheService.SellerMetaCacheEntry("陳記蔬果", "logo.jpg", "台北市", "大安區", "2612345678"));

        Mockito.verify(valueOps, Mockito.times(1))
                .set(Mockito.eq(KEY), Mockito.contains("2612345678"), Mockito.anyLong(), Mockito.any());
    }
}
