package io.github.will413028.hawkr.payuni.webhook;

import io.github.will413028.hawkr.payuni.crypto.PayUniCryptoService;
import io.github.will413028.hawkr.payuni.webhook.event.AgentNotifyEvent;
import io.github.will413028.hawkr.payuni.webhook.event.PaymentUopEvent;
import io.github.will413028.hawkr.payuni.webhook.event.ShippingStatusEvent;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;

import java.util.LinkedHashMap;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

/**
 * PayUniWebhookParser 單元測試
 */
class PayUniWebhookParserTest {

    private static final String TEST_KEY = "TestOnlyFakeHashKey0123456789AB1";
    private static final String TEST_IV = "TestOnlyFakeIV00";

    private final PayUniCryptoService cryptoService = new PayUniCryptoService(TEST_KEY, TEST_IV);
    private final PayUniWebhookParser parser = new PayUniWebhookParser(cryptoService);

    @Nested
    @DisplayName("付款結果通知 (PAYMENT_UOP)")
    class PaymentUopTest {

        @Test
        @DisplayName("正常付款成功 → PaymentUopEvent")
        void paymentSuccess() {
            Map<String, String> params = new LinkedHashMap<>();
            params.put("MerID", "HAWK9");
            params.put("MerTradeNo", "TRD001");
            params.put("TradeNo", "P20260216001");
            params.put("TradeStatus", "1");
            params.put("TradeAmt", "1000");
            params.put("PayType", "Credit");
            params.put("Message", "付款成功");

            String encrypted = cryptoService.encrypt(params);
            String hash = cryptoService.calculateHash(encrypted);

            WebhookParseResult result = parser.parse(encrypted, hash);

            assertEquals(WebhookType.PAYMENT_UOP, result.type());
            assertInstanceOf(PaymentUopEvent.class, result.event());

            PaymentUopEvent event = (PaymentUopEvent) result.event();
            assertEquals("HAWK9", event.merId());
            assertEquals("TRD001", event.merTradeNo());
            assertEquals("P20260216001", event.tradeNo());
            assertEquals("1", event.tradeStatus());
            assertEquals("1000", event.tradeAmt());
            assertEquals("Credit", event.payType());

            assertNotNull(result.rawMap());
            assertEquals(encrypted, result.encryptInfo());
            assertEquals(hash, result.hashInfo());
        }

        @Test
        @DisplayName("付款失敗 → TradeStatus=2")
        void paymentFailed() {
            Map<String, String> params = new LinkedHashMap<>();
            params.put("MerTradeNo", "TRD002");
            params.put("TradeNo", "P20260216002");
            params.put("TradeStatus", "2");
            params.put("TradeAmt", "500");
            params.put("Message", "餘額不足");

            String encrypted = cryptoService.encrypt(params);
            String hash = cryptoService.calculateHash(encrypted);

            WebhookParseResult result = parser.parse(encrypted, hash);
            assertEquals(WebhookType.PAYMENT_UOP, result.type());

            PaymentUopEvent event = (PaymentUopEvent) result.event();
            assertEquals("2", event.tradeStatus());
        }
    }

    @Nested
    @DisplayName("物流貨態通知 (SHIPPING_STATUS)")
    class ShippingStatusTest {

        @Test
        @DisplayName("物流貨態 → ShippingStatusEvent")
        void shippingStatus() {
            Map<String, String> params = new LinkedHashMap<>();
            params.put("MerID", "HAWK9");
            params.put("ShipTradeNo", "SHP001");
            params.put("ShipStatus", "91");
            params.put("ShipStatusTime", "2026-02-16 12:00:00");
            params.put("ShipType", "1");
            params.put("LgsType", "B2C");

            String encrypted = cryptoService.encrypt(params);
            String hash = cryptoService.calculateHash(encrypted);

            WebhookParseResult result = parser.parse(encrypted, hash);

            assertEquals(WebhookType.SHIPPING_STATUS, result.type());
            assertInstanceOf(ShippingStatusEvent.class, result.event());

            ShippingStatusEvent event = (ShippingStatusEvent) result.event();
            assertEquals("SHP001", event.shipTradeNo());
            assertEquals("91", event.shipStatus());
            assertEquals("B2C", event.lgsType());
        }
    }

    @Nested
    @DisplayName("代理商通知 (AGENT_NOTIFY)")
    class AgentNotifyTest {

        @Test
        @DisplayName("商家狀態異動 → AgentNotifyEvent")
        void merStatusChange() {
            Map<String, String> params = new LinkedHashMap<>();
            params.put("MerID", "UTF8290000");
            params.put("NotifyType", "MerStatus");
            params.put("MerStatus", "S");
            params.put("MerStatusNote", "審核通過");
            params.put("ExecuteTime", "20260216120000");

            String encrypted = cryptoService.encrypt(params);
            String hash = cryptoService.calculateHash(encrypted);

            WebhookParseResult result = parser.parse(encrypted, hash);

            assertEquals(WebhookType.AGENT_NOTIFY, result.type());
            assertInstanceOf(AgentNotifyEvent.class, result.event());

            AgentNotifyEvent event = (AgentNotifyEvent) result.event();
            assertEquals("UTF8290000", event.merId());
            assertEquals("S", event.merStatus());
            assertEquals("審核通過", event.merStatusNote());
            assertEquals("20260216120000", event.executeTime());
        }
    }

    @Nested
    @DisplayName("週期扣款通知（不處理）")
    class PeriodNotificationTest {

        @Test
        @DisplayName("含 PeriodTradeNo 的通知即使帶 TradeAmt／TradeStatus 也判為 UNKNOWN，不誤判成 PAYMENT_UOP")
        void periodNotificationIsUnknown() {
            Map<String, String> params = new LinkedHashMap<>();
            params.put("MerID", "HAWK9");
            params.put("PeriodTradeNo", "PT002");
            params.put("PeriodOrderNo", "M123-S1_2");
            params.put("ThisPeriod", "2");
            params.put("TradeStatus", "2");
            params.put("TradeAmt", "990");

            String encrypted = cryptoService.encrypt(params);
            String hash = cryptoService.calculateHash(encrypted);

            WebhookParseResult result = parser.parse(encrypted, hash);

            assertEquals(WebhookType.UNKNOWN, result.type());
            assertNull(result.event());
        }
    }

    @Nested
    @DisplayName("錯誤處理")
    class ErrorTest {

        @Test
        @DisplayName("簽章驗證失敗 → WebhookVerifyException")
        void verifyFailed() {
            Map<String, String> params = Map.of("MerTradeNo", "TRD", "TradeStatus", "1", "TradeAmt", "100");
            String encrypted = cryptoService.encrypt(params);

            assertThrows(WebhookVerifyException.class, () ->
                    parser.parse(encrypted, "INVALID_HASH"));
        }

        @Test
        @DisplayName("無法辨識的類型 → UNKNOWN + event 為 null")
        void unknownType() {
            Map<String, String> params = new LinkedHashMap<>();
            params.put("SomeField", "value");

            String encrypted = cryptoService.encrypt(params);
            String hash = cryptoService.calculateHash(encrypted);

            WebhookParseResult result = parser.parse(encrypted, hash);
            assertEquals(WebhookType.UNKNOWN, result.type());
            assertNull(result.event());
        }
    }
}
