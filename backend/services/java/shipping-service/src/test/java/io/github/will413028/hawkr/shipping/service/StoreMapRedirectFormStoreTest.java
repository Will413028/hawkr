package io.github.will413028.hawkr.shipping.service;

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.data.redis.core.ValueOperations;

import java.time.Duration;
import java.util.HashMap;
import java.util.Map;
import java.util.Optional;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

/**
 * 單元測試：驗證 StoreMapRedirectFormStore 的 store / 一次性 consume 語意（#1394）。
 * <p>
 * 以 Map 回填 StringRedisTemplate.opsForValue() 的 set / getAndDelete，
 * 真實驗證序列化往返與 GETDEL 一次性取用行為（過期以清空 backing map 模擬）。
 */
class StoreMapRedirectFormStoreTest {

    private Map<String, String> backing;
    private StoreMapRedirectFormStore store;

    @BeforeEach
    @SuppressWarnings("unchecked")
    void setUp() {
        backing = new HashMap<>();
        StringRedisTemplate redisTemplate = mock(StringRedisTemplate.class);
        ValueOperations<String, String> ops = mock(ValueOperations.class);
        when(redisTemplate.opsForValue()).thenReturn(ops);

        org.mockito.Mockito.doAnswer(inv -> {
            backing.put(inv.getArgument(0), inv.getArgument(1));
            return null;
        }).when(ops).set(anyString(), anyString(), any(Duration.class));

        when(ops.getAndDelete(anyString())).thenAnswer(inv -> backing.remove(inv.getArgument(0)));

        store = new StoreMapRedirectFormStore(redisTemplate);
    }

    private StoreMapRedirectFormStore.RedirectForm sampleForm() {
        return new StoreMapRedirectFormStore.RedirectForm(
                "https://sandbox-api.payuni.com.tw/api/storeMap",
                "HAWK456", "1.0", "ENC_INFO", "HASH_INFO");
    }

    @Test
    void store_thenConsume_returnsAllFields() throws Exception {
        String id = store.store(sampleForm());
        assertFalse(id.isEmpty(), "id 不可為空");

        Optional<StoreMapRedirectFormStore.RedirectForm> got = store.consume(id);
        assertTrue(got.isPresent(), "store 後 consume 應取回表單");
        StoreMapRedirectFormStore.RedirectForm form = got.get();
        assertEquals("https://sandbox-api.payuni.com.tw/api/storeMap", form.formAction());
        assertEquals("HAWK456", form.merId());
        assertEquals("1.0", form.version());
        assertEquals("ENC_INFO", form.encryptInfo());
        assertEquals("HASH_INFO", form.hashInfo());
    }

    @Test
    void consume_isOneTime_secondReadIsEmpty() throws Exception {
        String id = store.store(sampleForm());
        assertTrue(store.consume(id).isPresent(), "首次 consume 應成功");
        assertTrue(store.consume(id).isEmpty(), "一次性：第二次 consume 應為空");
    }

    @Test
    void consume_afterExpiry_isEmpty() throws Exception {
        String id = store.store(sampleForm());
        backing.clear();
        assertTrue(store.consume(id).isEmpty(), "過期後 consume 應為空");
    }

    @Test
    void consume_unknownId_isEmpty() {
        assertTrue(store.consume("non-existent-id").isEmpty(), "不存在的 id 應為空");
    }

    @Test
    void consume_nullOrBlankId_isEmpty() {
        assertTrue(store.consume(null).isEmpty(), "null id 應為空");
        assertTrue(store.consume("").isEmpty(), "空字串 id 應為空");
    }

    @Test
    @SuppressWarnings("unchecked")
    void store_usesConfiguredKeyPrefixAndTtl() throws Exception {
        StringRedisTemplate redisTemplate = mock(StringRedisTemplate.class);
        ValueOperations<String, String> ops = mock(ValueOperations.class);
        when(redisTemplate.opsForValue()).thenReturn(ops);
        StoreMapRedirectFormStore s = new StoreMapRedirectFormStore(redisTemplate);

        String id = s.store(sampleForm());

        org.mockito.Mockito.verify(ops).set(
                eq(StoreMapRedirectFormStore.KEY_PREFIX + id),
                anyString(),
                eq(StoreMapRedirectFormStore.TTL));
    }
}
