package io.github.will413028.hawkr.webhook.service;

import io.github.will413028.hawkr.common.utils.code.PayUniEventType;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;

import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * EventKeyBuilder 單元測試：同一事件重送得到同一個 eventKey（dedup 命中），不同事件得到不同 eventKey。
 */
@DisplayName("EventKeyBuilder dedup 行為單元測試")
class EventKeyBuilderTest {

    private static Map<String, String> shippingStatus(String shipStatus) {
        return Map.of("MerID", "HAWK9", "ShipTradeNo", "SHP123", "ShipStatus", shipStatus);
    }

    @Nested
    @DisplayName("dedup")
    class Dedup {

        @Test
        @DisplayName("相同 payload 兩次必須產生相同 eventKey（dedup 命中）")
        void samePayload_producesSameKey() {
            assertEquals(EventKeyBuilder.build(PayUniEventType.SHIPPING_STATUS, shippingStatus("91")),
                    EventKeyBuilder.build(PayUniEventType.SHIPPING_STATUS, shippingStatus("91")));
        }

        @Test
        @DisplayName("貨態推進（不同 ShipStatus）必須產生不同 eventKey（不誤攔）")
        void differentStatus_producesDifferentKey() {
            assertNotEquals(EventKeyBuilder.build(PayUniEventType.SHIPPING_STATUS, shippingStatus("91")),
                    EventKeyBuilder.build(PayUniEventType.SHIPPING_STATUS, shippingStatus("11")));
        }
    }

    @Nested
    @DisplayName("eventKey 格式驗證")
    class FormatVerification {

        @Test
        @DisplayName("SHIPPING_STATUS 加 STATUS: 前綴與 SHIPPING_PRINT 區分")
        void shippingStatus_hasStatusPrefix() {
            String statusKey = EventKeyBuilder.build(PayUniEventType.SHIPPING_STATUS, Map.of(
                    "MerID", "HAWK9",
                    "ShipTradeNo", "SHP123",
                    "ShipStatus", "91"
            ));
            String printKey = EventKeyBuilder.build(PayUniEventType.SHIPPING_PRINT, Map.of(
                    "MerID", "HAWK9",
                    "ShipTradeNo", "SHP123",
                    "Odno", "ODN001"
            ));
            assertTrue(statusKey.contains(":STATUS:"));
            assertTrue(printKey.contains(":PRINT:"));
            assertNotEquals(statusKey, printKey,
                    "SHIPPING_STATUS 與 SHIPPING_PRINT 必須產生不同 eventKey");
        }

        @Test
        @DisplayName("SHIPPING_STATUS 在 ShipStatus 為空時 fallback 到 TradeStatus")
        void shippingStatus_fallbackToTradeStatus() {
            String key = EventKeyBuilder.build(PayUniEventType.SHIPPING_STATUS, Map.of(
                    "MerID", "HAWK9",
                    "ShipTradeNo", "SHP123",
                    "TradeStatus", "S"
            ));
            assertEquals("HAWK9:SHP123:STATUS:S", key);
        }

        @Test
        @DisplayName("UNKNOWN 使用 timestamp 確保唯一，避免 UNIQUE 衝突")
        void unknown_usesTimestamp() {
            String key1 = EventKeyBuilder.build(PayUniEventType.UNKNOWN, Map.of("MerID", "HAWK9"));
            // 短暫延遲避免同毫秒
            try { Thread.sleep(2); } catch (InterruptedException ignored) {}
            String key2 = EventKeyBuilder.build(PayUniEventType.UNKNOWN, Map.of("MerID", "HAWK9"));
            assertNotEquals(key1, key2,
                    "UNKNOWN 事件每次必須產生唯一 key，避免 dedup 誤攔不同次的無法辨識事件");
            assertTrue(key1.startsWith("HAWK9:UNKNOWN:"));
        }
    }
}
