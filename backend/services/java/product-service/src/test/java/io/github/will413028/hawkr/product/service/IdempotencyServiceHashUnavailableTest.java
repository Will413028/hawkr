package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.product.entity.IdempotencyRequestEntity;
import io.github.will413028.hawkr.product.exception.IdempotencyConflictException;
import io.github.will413028.hawkr.product.repository.IdempotencyRepository;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import java.util.Map;
import java.util.Optional;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * 冪等鍵撞鍵時，hash 不可用的處理（#2295 ②）。
 *
 * <p>釘的是 fail-closed：這一關的作用是「確認撞到的確實是同一個請求」，確認不了
 * 時的正解是拒絕。原本寫成「hash 為空就跳過比對」，配上冪等鍵<b>沒有主體維度</b>
 * （PK 只有 key、{@code findByKey} 不帶 storeId），效果是另一個賣家用同一把 key
 * 進來會拿到 HTTP 200 加上前一位賣家的完整商品資料。
 *
 * <p>空 hash 不是理論情況：{@code calculateHash} 序列化失敗時回 {@code ""}，
 * {@code request_hash} 欄位本身也是 nullable。
 *
 * <p>⚠️ 本測試擋住的是「拿到別人的資料」，不是「撞鍵」本身——鍵少了 store_id 這件
 * 事要改主鍵，另案處理。修好之後這個測試仍然必要：同一個賣家自撞也會拿到錯的快取。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
@DisplayName("IdempotencyService: hash 不可用時必須拒絕，不得重用既有記錄")
class IdempotencyServiceHashUnavailableTest {

    private static final String KEY = "b5-072009-9"; // dev 實測 132 筆中 127 筆是這種短鍵
    private static final Long STORE_A = 4L;
    private static final Long STORE_B = 9L;

    @Mock
    private IdempotencyRepository repository;

    private IdempotencyService service;

    @BeforeEach
    void setUp() {
        service = new IdempotencyService(repository, new ObjectMapper());
        // 撞鍵：插入回 0 列
        when(repository.tryInsert(anyString(), any(), anyString(), any())).thenReturn(0);
    }

    private void existingRecord(String storedHash) {
        IdempotencyRequestEntity e = new IdempotencyRequestEntity();
        e.setKey(KEY);
        e.setStoreId(STORE_A);
        e.setEndpoint("CreateProduct");
        e.setRequestHash(storedHash);
        e.setStatus("8002"); // SUCCEEDED — 這正是會把別人的 response 交出去的狀態
        e.setResponseBody("{\"productId\":\"873483808124207639\",\"name\":\"賣家A的商品\"}");
        when(repository.findByKeyAndStoreId(KEY, STORE_A)).thenReturn(Optional.of(e));
    }

    @Test
    @DisplayName("既有記錄的 hash 為空字串（calculateHash 降級的回傳值）→ 拒絕")
    void storedHashEmpty() {
        existingRecord("");
        IdempotencyConflictException ex = assertThrows(IdempotencyConflictException.class,
                () -> service.acquireLock(KEY, STORE_A, "CreateProduct", "abc123", String.class));
        assertTrue(ex.getMessage().contains("無法確認"),
                "訊息要說明為什麼被拒，實際：" + ex.getMessage());
    }

    @Test
    @DisplayName("既有記錄的 hash 為 null（欄位 nullable）→ 拒絕")
    void storedHashNull() {
        existingRecord(null);
        assertThrows(IdempotencyConflictException.class,
                () -> service.acquireLock(KEY, STORE_A, "CreateProduct", "abc123", String.class));
    }

    @Test
    @DisplayName("本次請求算不出 hash → 拒絕（不能拿一個空值去比對別人的記錄）")
    void incomingHashEmpty() {
        existingRecord("deadbeef");
        assertThrows(IdempotencyConflictException.class,
                () -> service.acquireLock(KEY, STORE_A, "CreateProduct", "", String.class));
    }

    @Test
    @DisplayName("hash 兩邊都有且不同 → 拒絕（既有行為）")
    void hashMismatch() {
        existingRecord("deadbeef");
        assertThrows(IdempotencyConflictException.class,
                () -> service.acquireLock(KEY, STORE_A, "CreateProduct", "abc123", String.class));
    }

    @Test
    @DisplayName("hash 相同 → 正常回傳先前結果（不得過度收緊，冪等本身要還能用）")
    void hashMatchStillReturnsCachedResult() {
        existingRecord("samehash");
        // responseType 用 Map：responseBody 是 JSON 物件，拿 String.class 去接會在
        // 反序列化就失敗、走進 catch 回 empty，測到的就不是這裡要釘的分支了。
        Optional<Map> got = service.acquireLock(KEY, STORE_A, "CreateProduct", "samehash", Map.class);
        assertTrue(got.isPresent(), "hash 一致時仍應回傳既有結果，否則冪等失效");
        assertEquals("873483808124207639", got.get().get("productId"));
    }

    @Test
    @DisplayName("撈既有記錄必須帶 storeId：撞鍵時不得撈到另一個賣家的記錄")
    void lookupIsScopedToStore() {
        existingRecord("deadbeef"); // 記錄屬於 STORE_A

        // 賣家 B 用同一把 key。撈不到「B 自己的」記錄，於是走「記錄狀態不一致」——
        // 這是階段 1 的過渡行為（舊的單欄 key 主鍵還在，插入仍被它擋下）。
        // 階段 2 移除舊 PK 之後，這裡會變成正常插入成功，兩個賣家互不干擾。
        assertThrows(IdempotencyConflictException.class,
                () -> service.acquireLock(KEY, STORE_B, "CreateProduct", "abc123", String.class));

        // 關鍵斷言：查詢帶的是呼叫者自己的 storeId，不是裸 key。
        verify(repository).findByKeyAndStoreId(KEY, STORE_B);
        verify(repository, never()).findByKey(anyString());
    }

    @Test
    @DisplayName("衝突例外的 message 真的傳得出去（3-arg 建構子）")
    void conflictMessageIsNotSwallowed() {
        existingRecord("deadbeef");
        IdempotencyConflictException ex = assertThrows(IdempotencyConflictException.class,
                () -> service.acquireLock(KEY, STORE_A, "CreateProduct", "abc123", String.class));
        // 修前四個呼叫端都誤用 2-arg 版（第二參數是 status 不是 message），於是所有
        // 衝突對外都變成「請求正在處理中」——一句暗示「稍後重試會成功」的話，而重試
        // 永遠失敗到 24–48 小時後清理為止。
        assertTrue(ex.getMessage().contains("請求內容不同"),
                "應說明是內容不同，實際：" + ex.getMessage());
        assertEquals("8002", ex.getStatus(), "status 欄位應是真正的狀態碼，不是訊息文字");
    }
}
