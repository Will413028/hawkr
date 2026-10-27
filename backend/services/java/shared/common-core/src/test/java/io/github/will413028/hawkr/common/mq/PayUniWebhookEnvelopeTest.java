package io.github.will413028.hawkr.common.mq;

import org.junit.jupiter.api.Test;

import java.nio.charset.StandardCharsets;

import static org.junit.jupiter.api.Assertions.*;

/**
 * PayUniWebhookEnvelope 單元測試
 *
 * 覆蓋：標準 envelope、裸 payload、欄位缺失、空 body、payloadField 便利方法
 */
class PayUniWebhookEnvelopeTest {

    @Test
    void parseStandardEnvelope() {
        String json = """
                {
                  "event_id": "abc-123",
                  "provider": "PAYUNI",
                  "webhook_type": "SHIPPING_PRINT",
                  "mer_id": "HAWK9",
                  "event_key": "HAWK9:S001:PRINT:S654",
                  "occurred_at": "2026-03-18T22:38:37",
                  "payload": {
                    "MerID": "HAWK9",
                    "ShipTradeNo": "S001",
                    "ApiType": "Print",
                    "PartnerId": "8B2",
                    "Odno": "S6546109",
                    "ValidationNo": "0323"
                  },
                  "meta": { "version": "1.0" }
                }
                """;
        PayUniWebhookEnvelope env = PayUniWebhookEnvelope.parse(json.getBytes(StandardCharsets.UTF_8));

        assertEquals("abc-123", env.eventId());
        assertEquals("PAYUNI", env.provider());
        assertEquals("SHIPPING_PRINT", env.webhookType());
        assertEquals("HAWK9", env.merId());
        assertEquals("HAWK9:S001:PRINT:S654", env.eventKey());
        assertEquals("2026-03-18T22:38:37", env.occurredAt());

        assertEquals("S001", env.payloadField("ShipTradeNo"));
        assertEquals("Print", env.payloadField("ApiType"));
        assertEquals("8B2", env.payloadField("PartnerId"));
        assertEquals("S6546109", env.payloadField("Odno"));
        assertEquals("0323", env.payloadField("ValidationNo"));

        assertEquals("1.0", env.meta().get("version"));
    }

    @Test
    void parseRawPayload() {
        // 裸 payload：沒有 eventId，整個 JSON 視為 payload
        String json = """
                {
                  "MerID": "HAWK9",
                  "ShipTradeNo": "S002",
                  "ApiType": "Print",
                  "PartnerId": "8B2"
                }
                """;
        PayUniWebhookEnvelope env = PayUniWebhookEnvelope.parse(json.getBytes(StandardCharsets.UTF_8));

        // envelope 欄位為空
        assertEquals("", env.eventId());
        assertEquals("", env.provider());
        assertEquals("", env.webhookType());

        // payload 正確取出
        assertEquals("HAWK9", env.payloadField("MerID"));
        assertEquals("S002", env.payloadField("ShipTradeNo"));
        assertEquals("Print", env.payloadField("ApiType"));
    }

    @Test
    void parseEnvelopeWithMissingPayload() {
        String json = """
                {
                  "event_id": "def-456",
                  "provider": "PAYUNI",
                  "webhook_type": "PAYMENT_UOP"
                }
                """;
        PayUniWebhookEnvelope env = PayUniWebhookEnvelope.parse(json.getBytes(StandardCharsets.UTF_8));

        assertEquals("def-456", env.eventId());
        assertTrue(env.payload().isEmpty());
        assertEquals("", env.payloadField("MerTradeNo"));
    }

    @Test
    void parseEnvelopeWithNullPayload() {
        String json = """
                {
                  "event_id": "ghi-789",
                  "provider": "PAYUNI",
                  "webhook_type": "PAYMENT_UOP",
                  "payload": null,
                  "meta": null
                }
                """;
        PayUniWebhookEnvelope env = PayUniWebhookEnvelope.parse(json.getBytes(StandardCharsets.UTF_8));

        assertEquals("ghi-789", env.eventId());
        assertTrue(env.payload().isEmpty());
        assertTrue(env.meta().isEmpty());
    }

    @Test
    void payloadFieldReturnsEmptyForMissingKey() {
        String json = """
                {
                  "event_id": "x",
                  "payload": { "A": "1" }
                }
                """;
        PayUniWebhookEnvelope env = PayUniWebhookEnvelope.parse(json.getBytes(StandardCharsets.UTF_8));

        assertEquals("1", env.payloadField("A"));
        assertEquals("", env.payloadField("NotExist"));
    }

    @Test
    void parseNullBodyThrows() {
        assertThrows(IllegalArgumentException.class, () -> PayUniWebhookEnvelope.parse(null));
    }

    @Test
    void parseEmptyBodyThrows() {
        assertThrows(IllegalArgumentException.class, () -> PayUniWebhookEnvelope.parse(new byte[0]));
    }

    @Test
    void parseInvalidJsonThrows() {
        assertThrows(RuntimeException.class,
                () -> PayUniWebhookEnvelope.parse("not json".getBytes(StandardCharsets.UTF_8)));
    }

    @Test
    void parsePaymentUopEnvelope() {
        // 模擬實際的付款結果 envelope
        String json = """
                {
                  "event_id": "pay-001",
                  "provider": "PAYUNI",
                  "webhook_type": "PAYMENT_UOP",
                  "mer_id": "HAWK9",
                  "event_key": "HAWK9:TRD123:1:1000",
                  "occurred_at": "2026-03-18T10:00:00",
                  "payload": {
                    "MerTradeNo": "TRD123",
                    "TradeNo": "UNI456",
                    "TradeStatus": "1",
                    "TradeAmt": "1000"
                  },
                  "meta": { "version": "1.0" }
                }
                """;
        PayUniWebhookEnvelope env = PayUniWebhookEnvelope.parse(json.getBytes(StandardCharsets.UTF_8));

        assertEquals("PAYMENT_UOP", env.webhookType());
        assertEquals("TRD123", env.payloadField("MerTradeNo"));
        assertEquals("UNI456", env.payloadField("TradeNo"));
        assertEquals("1", env.payloadField("TradeStatus"));
        assertEquals("1000", env.payloadField("TradeAmt"));

        // payload() 作為完整 Map 使用
        assertEquals(4, env.payload().size());
    }
}
