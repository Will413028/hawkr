package io.github.will413028.hawkr.webhook.uber;

import io.github.will413028.hawkr.common.mq.ProviderWebhookEnvelope;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.nio.charset.StandardCharsets;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertArrayEquals;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * {@link ProviderWebhookEnvelope} 的 wire 契約：<b>producer 側與 consumer 側必須用同一組鍵名</b>。
 *
 * <p>本測試放在 payuni-webhook-service（producer 端所在的 module）而不是 common-core，
 * 因為它驗的是「producer 寫出去的東西 consumer 讀得回來」——在 producer 這一側跑才會在
 * 契約漂移時擋住真正該被擋住的那一次 commit。
 *
 * <h2>這支測試要防的 bug class</h2>
 * {@code PayUniWebhookEnvelope} 的 producer 自己另寫了一份 {@code Map.of("event_id", ...)}，
 * 該檔註解逐字寫「本方法的鍵名若與 parse() 不一致，consumer 讀到的 metadata 會全空<b>且不報錯</b>」。
 * 新型別把兩側收進同一個型別，這裡再用 round-trip 把它釘死。
 */
class ProviderWebhookEnvelopeTest {

    private static final ObjectMapper MAPPER = new ObjectMapper();

    /** 含 JSON 逸出、非 ASCII、非標準空白、尾端換行 —— 全是 re-serialize 會抹平的特徵。 */
    private static byte[] awkward() {
        return ("{\n  \"kind\"  : \"event.delivery_status\",\n"
                + "  \"id\": \"evt_\\u5b85\",\n"
                + "  \"note\": \"中文與 emoji 🚚\"\n}\n").getBytes(StandardCharsets.UTF_8);
    }

    @Test
    @DisplayName("round-trip：toWireMap() 寫出去、parse() 讀回來，每個欄位都對得上")
    void roundTripPreservesEveryField() throws Exception {
        byte[] raw = awkward();
        ProviderWebhookEnvelope out = ProviderWebhookEnvelope.ofJson(
                "8242", "event.delivery_status", "evt_7f3a", raw,
                Map.of("x-uber-signature", "sig"), "2026-09-12T01:02:03Z");

        String json = MAPPER.writeValueAsString(out.toWireMap());
        ProviderWebhookEnvelope back = ProviderWebhookEnvelope.parse(json.getBytes(StandardCharsets.UTF_8));

        assertEquals("8242", back.provider());
        assertEquals("event.delivery_status", back.webhookType());
        assertEquals("evt_7f3a", back.eventKey());
        assertEquals(Map.of("x-uber-signature", "sig"), back.headers());
        assertEquals(Map.of(), back.formParams());
        assertEquals("2026-09-12T01:02:03Z", back.receivedAt());
        assertArrayEquals(raw, back.rawBody(), "raw body 必須逐位元組還原");
    }

    @Test
    @DisplayName("wire 鍵名一律 snake_case（36 §7.4 Conformance Contract），硬寫字面值比對")
    void wireKeysAreSnakeCase() throws Exception {
        ProviderWebhookEnvelope out = ProviderWebhookEnvelope.ofJson(
                "8242", "event.delivery_status", "evt_1", awkward(), Map.of(), "2026-09-12T01:02:03Z");
        JsonNode root = MAPPER.readTree(MAPPER.writeValueAsString(out.toWireMap()));

        // 🔴 期望值硬寫字面值，不引用 ProviderWebhookEnvelope.KEY_*：
        //    引用常數的話，改名時測試會跟著改名而不會轉紅 —— 那就驗不到任何東西。
        assertTrue(root.has("provider"));
        assertTrue(root.has("webhook_type"));
        assertTrue(root.has("event_key"));
        assertTrue(root.has("form_params"));
        assertTrue(root.has("raw_body_base64"));
        assertTrue(root.has("headers"));
        assertTrue(root.has("received_at"));
        assertEquals(7, root.size(), "多了或少了欄位就是契約變更，必須同步 36 §3.5");
    }

    @Test
    @DisplayName("form-encoded provider：無 raw body，rawBody() 回 null 而不是空陣列")
    void formParamsFlavourHasNoRawBody() {
        ProviderWebhookEnvelope env = ProviderWebhookEnvelope.ofFormParams(
                "8241", "8252", "HAWK1:S1:STATUS:33", Map.of("ShipStatus", "33"), "2026-09-12T00:00:00Z");
        assertNull(env.rawBody());
        assertTrue(!env.hasRawBody());
        assertEquals("33", env.formParams().get("ShipStatus"));
    }

    @Test
    @DisplayName("缺 provider ⇒ 拋例外，不得靜默回一個欄位全空的 envelope")
    void missingProviderIsLoud() {
        assertThrows(IllegalArgumentException.class,
                () -> ProviderWebhookEnvelope.parse("{\"webhook_type\":\"x\"}".getBytes(StandardCharsets.UTF_8)));
    }

    @Test
    @DisplayName("base64 毀損 ⇒ 拋例外，不得回 null 吞掉（那會讓驗簽失敗被誤讀成金鑰錯）")
    void corruptBase64IsLoud() {
        ProviderWebhookEnvelope env = new ProviderWebhookEnvelope(
                "8242", "k", "e", Map.of(), "!!!not-base64!!!", Map.of(), "t");
        assertThrows(IllegalArgumentException.class, env::rawBody);
    }

    @Test
    @DisplayName("空 body ⇒ 拋例外（MQ 不該送空訊息，靜默回空 envelope 會讓它一路走到 adapter）")
    void emptyBodyIsLoud() {
        assertThrows(IllegalArgumentException.class, () -> ProviderWebhookEnvelope.parse(new byte[0]));
    }
}
