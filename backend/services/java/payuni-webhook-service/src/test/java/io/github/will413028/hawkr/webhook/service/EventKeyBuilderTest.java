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
 * EventKeyBuilder 單元測試
 *
 * 對應設計書 10 §2.4 EventKey 組合規則 + §2.5 dedup 行為。
 *
 * 涵蓋 PM 提的 4 個邊緣 case：
 * 1. status 0 → 1：應產生不同 eventKey（兩次事件，不誤攔）
 * 2. 完全重複 payload：應產生相同 eventKey（dedup 命中）
 * 3. UNKNOWN → 1：tradeStatus 字串值不同 → 應產生不同 eventKey
 * 4. 同 merTradeNo + tradeNo + tradeStatus 但不同 tradeAmt：應產生相同 eventKey
 *    （dedup 命中；TradeAmt 不在 key 內，由 UNIQUE 約束作為金額守恆最外層防守，
 *    詳見設計書 §2.4 設計理由）
 *
 * 額外覆蓋 §2A.6 同 order 多筆獨立付款場景：不同 TradeNo 必須產生不同 eventKey。
 */
@DisplayName("EventKeyBuilder dedup 行為單元測試")
class EventKeyBuilderTest {

    private static final String MER_ID = "HAWK9";
    private static final String MER_TRADE_NO = "O123456789001";
    private static final String TRADE_NO = "1776493168257421093";

    @Nested
    @DisplayName("PM Case 1：status 0 → 1（兩次獨立事件）")
    class Case1_StatusProgression {

        @Test
        @DisplayName("不同 tradeStatus 必須產生不同 eventKey")
        void differentStatus_producesDifferentKey() {
            String keyStatus0 = EventKeyBuilder.build(PayUniEventType.PAYMENT_UOP, payload("0"));
            String keyStatus1 = EventKeyBuilder.build(PayUniEventType.PAYMENT_UOP, payload("1"));

            assertNotEquals(keyStatus0, keyStatus1,
                    "tradeStatus 0 與 1 為不同事件，eventKey 必須不同以避免 dedup 誤攔");
            assertTrue(keyStatus0.endsWith(":0"));
            assertTrue(keyStatus1.endsWith(":1"));
        }

        private Map<String, String> payload(String tradeStatus) {
            return Map.of(
                    "MerID", MER_ID,
                    "MerTradeNo", MER_TRADE_NO,
                    "TradeNo", TRADE_NO,
                    "TradeStatus", tradeStatus,
                    "TradeAmt", "1000"
            );
        }
    }

    @Nested
    @DisplayName("PM Case 2：完全重複 payload")
    class Case2_ExactDuplicate {

        @Test
        @DisplayName("相同 payload 兩次必須產生相同 eventKey（dedup 命中）")
        void samePayload_producesSameKey() {
            Map<String, String> payload = Map.of(
                    "MerID", MER_ID,
                    "MerTradeNo", MER_TRADE_NO,
                    "TradeNo", TRADE_NO,
                    "TradeStatus", "1",
                    "TradeAmt", "1000"
            );

            String key1 = EventKeyBuilder.build(PayUniEventType.PAYMENT_UOP, payload);
            String key2 = EventKeyBuilder.build(PayUniEventType.PAYMENT_UOP, payload);

            assertEquals(key1, key2,
                    "完全相同的 payload 必須產生相同 eventKey 以觸發 UNIQUE 約束 dedup");
        }
    }

    @Nested
    @DisplayName("PM Case 3：UNKNOWN → 1 中繼狀態")
    class Case3_UnknownThenPaid {

        @Test
        @DisplayName("tradeStatus=UNKNOWN 與 tradeStatus=1 必須產生不同 eventKey")
        void unknownStatus_producesDifferentKey() {
            String keyUnknown = EventKeyBuilder.build(PayUniEventType.PAYMENT_UOP, payload("UNKNOWN"));
            String keyPaid = EventKeyBuilder.build(PayUniEventType.PAYMENT_UOP, payload("1"));

            assertNotEquals(keyUnknown, keyPaid,
                    "中繼狀態與終態為不同事件，eventKey 必須不同");
        }

        private Map<String, String> payload(String tradeStatus) {
            return Map.of(
                    "MerID", MER_ID,
                    "MerTradeNo", MER_TRADE_NO,
                    "TradeNo", TRADE_NO,
                    "TradeStatus", tradeStatus,
                    "TradeAmt", "1000"
            );
        }
    }

    @Nested
    @DisplayName("PM Case 4：同 key 不同 tradeAmt（防守性）")
    class Case4_AmountMismatch {

        @Test
        @DisplayName("同 merTradeNo + tradeNo + tradeStatus 但不同 tradeAmt 必須產生相同 eventKey")
        void differentAmtSameKey_producesSameKey() {
            Map<String, String> payload100 = Map.of(
                    "MerID", MER_ID,
                    "MerTradeNo", MER_TRADE_NO,
                    "TradeNo", TRADE_NO,
                    "TradeStatus", "1",
                    "TradeAmt", "100"
            );
            Map<String, String> payload999 = Map.of(
                    "MerID", MER_ID,
                    "MerTradeNo", MER_TRADE_NO,
                    "TradeNo", TRADE_NO,
                    "TradeStatus", "1",
                    "TradeAmt", "999"
            );

            String key100 = EventKeyBuilder.build(PayUniEventType.PAYMENT_UOP, payload100);
            String key999 = EventKeyBuilder.build(PayUniEventType.PAYMENT_UOP, payload999);

            assertEquals(key100, key999,
                    "TradeAmt 不在 eventKey 內 — 同 TradeNo 必然同金額（PayUni 端資料模型保證），"
                  + "dedup 攔下第二筆即為金額守恆最外層防守，詳見設計書 §2.4 設計理由");
        }
    }

    @Nested
    @DisplayName("§2A.6 重複付款場景：同 order 多筆獨立 PayUni 交易")
    class Case2A6_DuplicatePayment {

        @Test
        @DisplayName("同 MerTradeNo 但不同 TradeNo 必須產生不同 eventKey（dedup 不誤攔）")
        void differentTradeNo_producesDifferentKey() {
            String keyCreditCard = EventKeyBuilder.build(PayUniEventType.PAYMENT_UOP, payload("1776493168257421093"));
            String keyAtm = EventKeyBuilder.build(PayUniEventType.PAYMENT_UOP, payload("1777122945891382535"));

            assertNotEquals(keyCreditCard, keyAtm,
                    "同 order 信用卡 + ATM 兩筆獨立 PayUni 交易，TradeNo 不同 → eventKey 必須不同，"
                  + "否則 §2A.6 重複付款偵測永遠無法觸發（live e2e bug，commit 599cdff4）");
        }

        private Map<String, String> payload(String tradeNo) {
            return Map.of(
                    "MerID", MER_ID,
                    "MerTradeNo", MER_TRADE_NO,
                    "TradeNo", tradeNo,
                    "TradeStatus", "1",
                    "TradeAmt", "1000"
            );
        }
    }

    @Nested
    @DisplayName("eventKey 格式驗證")
    class FormatVerification {

        @Test
        @DisplayName("PAYMENT_RECURRING 格式包含訂閱、期數與終態")
        void recurringPayment_format() {
            String key = EventKeyBuilder.build(PayUniEventType.PAYMENT_RECURRING, Map.of(
                    "MerID", "HAWK9",
                    "PeriodTradeNo", "PT001",
                    "PeriodOrderNo", "M123-S1_1",
                    "TradeNo", "P20260216003",
                    "ThisPeriod", "1",
                    "TradeStatus", "1"
            ));
            assertEquals("HAWK9:PT001:P20260216003:1:1", key);
        }

        @Test
        @DisplayName("PAYMENT_UOP 格式為 {merId}:{merTradeNo}:{tradeNo}:{tradeStatus}")
        void paymentUop_format() {
            String key = EventKeyBuilder.build(PayUniEventType.PAYMENT_UOP, Map.of(
                    "MerID", "HAWK9",
                    "MerTradeNo", "O123456789001",
                    "TradeNo", "1776493168257421093",
                    "TradeStatus", "1",
                    "TradeAmt", "1000"
            ));
            assertEquals("HAWK9:O123456789001:1776493168257421093:1", key);
        }

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
