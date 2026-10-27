package io.github.will413028.hawkr.order.mq;

import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.doAnswer;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;

/**
 * seller.order.updated 的 sellerId 必須與 gateway SSE 訂閱端同一個身分空間（store_id）。
 *
 * <p>這支測試的前身斷言的正好相反 —— 「sellerId 必須是反解析後的 login account_id」。
 * 那在 2026-08-30 之前是對的：當時 gateway 的 {@code resolveNotificationUserID} 對賣家
 * 回傳 account_id。commit {@code 57cefff9f} 把訂閱端改成 current store_id（為了讓買賣兩端
 * 對稱、修 device_token 寫出永遠查不到的列），發布端與這支測試都沒有跟著回灌 ⇒ 對
 * {@code store_id != account_id} 的賣家，發布 topic 與訂閱 topic 永不相等，事件 100% 送不到，
 * 而且 Hub 以精確鍵 fan-out，查無訂閱者不產生任何錯誤或 log。
 */
class OrderEventPublisherSellerOrderUpdatedTest {

    @Test
    @DisplayName("emitSellerOrderUpdated 的 sellerId 是 store_id 本身，不做身分反解析")
    void emitSellerOrderUpdated_publishesStoreIdVerbatim() throws Exception {
        ObsRabbitPublisher rabbitPublisher = mock(ObsRabbitPublisher.class);
        OrderRepository orderRepository = mock(OrderRepository.class);
        // 分岔賣家：store_id 與 login account_id 不相等（2026-09-02 身分分離後的常態）。
        // 兩者若相等，這支測試對「有沒有反解析」零鑑別力 —— 那正是本 bug 活到部署的原因。
        long storeId = 2685914682L;
        long loginAccountId = 2600000012L;
        assertNotEquals(storeId, loginAccountId, "fixture 必須用分岔身分，否則本測試無鑑別力");

        OrderEventPublisher publisher = new OrderEventPublisher(rabbitPublisher, orderRepository);

        // ObsRabbitPublisher.publish is overloaded (byte[] vs Message); an ArgumentCaptor<byte[]>.capture()
        // inline in the verify() call is ambiguous to javac between the two overloads (same issue already
        // worked around in ShipmentAnomalyEventPublisherTest via any(byte[].class)). Capture via doAnswer instead.
        byte[][] capturedBody = new byte[1][];
        doAnswer(invocation -> {
            capturedBody[0] = invocation.getArgument(2, byte[].class);
            return null;
        }).when(rabbitPublisher).publish(anyString(), anyString(), any(byte[].class), anyString());

        publisher.emitSellerOrderUpdated(storeId, "ORD1", OrderEventPublisher.SRC_SYSTEM, "10", "80", null);

        verify(rabbitPublisher).publish(anyString(), anyString(), any(byte[].class), anyString());
        JsonNode published = new ObjectMapper().readTree(capturedBody[0]);
        assertEquals(storeId, published.get("sellerId").asLong(),
                "sellerId 必須是 store_id（gateway 的訂閱 topic 用 current store_id 建）");
        assertNotEquals(loginAccountId, published.get("sellerId").asLong(),
                "反解析成 login account_id 會讓事件落在沒有人訂閱的 topic 上");
    }

    @Test
    @DisplayName("storeId 缺失時跳過發布（gateway 無法解析 topic）")
    void emitSellerOrderUpdated_skipsWhenStoreIdMissing() {
        ObsRabbitPublisher rabbitPublisher = mock(ObsRabbitPublisher.class);
        OrderRepository orderRepository = mock(OrderRepository.class);
        OrderEventPublisher publisher = new OrderEventPublisher(rabbitPublisher, orderRepository);

        publisher.emitSellerOrderUpdated(null, "ORD1", OrderEventPublisher.SRC_SYSTEM, "10", "80", null);
        publisher.emitSellerOrderUpdated(0L, "ORD1", OrderEventPublisher.SRC_SYSTEM, "10", "80", null);

        verify(rabbitPublisher, never()).publish(anyString(), anyString(), any(byte[].class), anyString());
    }
}
