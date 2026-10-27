package io.github.will413028.hawkr.webhook.uber;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;

import java.nio.charset.StandardCharsets;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * {@link UberWebhookParser} 的值域與路由判定。
 *
 * <h2>fixture 來源</h2>
 * 欄名與 kind 值一律抄自 {@code docs/第三方整合/Uber Direct API 總覽.md}
 * （§6.2 事件 payload、§6.3.2 訂閱決策、§7.1 DaaS status、§7.2 DaaS webhook payload 欄位），
 * <b>不從 66 號設計書轉抄</b>——那是二手，而本測試要盯的正是「實作有沒有跟上官方文件」。
 * 期望值一律硬寫字面值，不呼叫被測物。
 */
class UberWebhookParserTest {

    private static byte[] body(String json) {
        return json.getBytes(StandardCharsets.UTF_8);
    }

    @Nested
    @DisplayName("§6.3.2 四種事件與訂閱決策")
    class RouteDecision {

        @Test
        @DisplayName("delivery_status → 進 MQ（唯一一個 PUBLISH）")
        void deliveryStatusPublishes() {
            // §7.1 逐字：webhook kind = event.delivery_status
            assertEquals(UberWebhookEvent.Route.PUBLISH,
                    UberWebhookParser.routeOf("event.delivery_status"));
            // §6.3.2 的表列的是不帶前綴的寫法，兩種都要認得
            assertEquals(UberWebhookEvent.Route.PUBLISH,
                    UberWebhookParser.routeOf("delivery_status"));
        }

        @Test
        @DisplayName("courier_update 不進 MQ —— 每 20 秒一發，全進 MQ ＝ 流量放大 14 倍（陷阱 A3）")
        void courierUpdateDoesNotPublish() {
            assertEquals(UberWebhookEvent.Route.INBOX_ONLY,
                    UberWebhookParser.routeOf("event.courier_update"));
            assertNotEquals(UberWebhookEvent.Route.PUBLISH,
                    UberWebhookParser.routeOf("event.courier_update"));
        }

        @Test
        @DisplayName("refund_request 不進 MQ —— 本波尚無落點，錢的事不自動化（§6.3.6）")
        void refundRequestDoesNotPublish() {
            assertEquals(UberWebhookEvent.Route.IGNORE,
                    UberWebhookParser.routeOf("event.refund_request"));
        }

        @Test
        @DisplayName("shopping_progress 我方未訂閱 → IGNORE，且不得報錯")
        void shoppingProgressIgnored() {
            assertEquals(UberWebhookEvent.Route.IGNORE,
                    UberWebhookParser.routeOf("event.shopping_progress"));
        }

        @Test
        @DisplayName("Eats 家族的 dapi.status_changed 不在訂閱決策表內 → IGNORE（§7.3：我方不用那一組）")
        void eatsFamilyIgnored() {
            assertEquals(UberWebhookEvent.Route.IGNORE,
                    UberWebhookParser.routeOf("dapi.status_changed"));
        }
    }

    @Nested
    @DisplayName("§6.3.3 去重鍵")
    class DedupKey {

        @Test
        @DisplayName("級 1：有 provider event id 就直接用它")
        void tier1UsesProviderEventId() {
            UberWebhookEvent e = UberWebhookParser.parse(body(
                    "{\"id\":\"evt_7f3a\",\"kind\":\"event.delivery_status\",\"delivery_id\":\"del_123\"}"));
            assertEquals("evt_7f3a", e.eventKey());
            assertEquals(UberWebhookEvent.KeyTier.PROVIDER_EVENT_ID, e.keyTier());
            assertEquals("event.delivery_status", e.kind());
        }

        @Test
        @DisplayName("級 1：Eats 家族的 event_id 也認（§6.2）")
        void tier1AlsoReadsEventId() {
            UberWebhookEvent e = UberWebhookParser.parse(body(
                    "{\"event_id\":\"9c11\",\"event_type\":\"dapi.status_changed\"}"));
            assertEquals("9c11", e.eventKey());
            assertEquals(UberWebhookEvent.KeyTier.PROVIDER_EVENT_ID, e.keyTier());
        }

        @Test
        @DisplayName("級 2：無 event id 時組 {delivery_id}:{kind}:{timestamp}:{sha256(raw)[0:16]}")
        void tier2CompositeKey() {
            String json = "{\"kind\":\"event.courier_update\",\"created\":\"2026-09-12T03:04:05Z\","
                    + "\"delivery_id\":\"del_9\"}";
            UberWebhookEvent e = UberWebhookParser.parse(body(json));
            assertEquals(UberWebhookEvent.KeyTier.FALLBACK_COMPOSITE, e.keyTier());
            assertTrue(e.eventKey().startsWith("del_9:event.courier_update:2026-09-12T03:04:05Z:"),
                    "級 2 前三段必須是 delivery_id:kind:timestamp，實得：" + e.eventKey());
            // 第四段：16 個 hex 字元
            String hash = e.eventKey().substring(e.eventKey().lastIndexOf(':') + 1);
            assertEquals(16, hash.length(), "sha256 前綴長度必須是 16");
            assertTrue(hash.matches("[0-9a-f]{16}"), "第四段必須是小寫 hex，實得：" + hash);
        }

        @Test
        @DisplayName("級 2：body 內容不同 → 去重鍵不同（否則狀態變更會被誤判重複）")
        void tier2DistinguishesDifferentBodies() {
            String a = "{\"kind\":\"event.delivery_status\",\"delivery_id\":\"d\",\"created\":\"t\",\"status\":\"pickup\"}";
            String b = "{\"kind\":\"event.delivery_status\",\"delivery_id\":\"d\",\"created\":\"t\",\"status\":\"dropoff\"}";
            assertNotEquals(UberWebhookParser.parse(body(a)).eventKey(),
                    UberWebhookParser.parse(body(b)).eventKey());
        }
    }

    @Nested
    @DisplayName("落不了地也要能落地（§6.3.4「能落地就 200」）")
    class Degenerate {

        @Test
        @DisplayName("body 不是 JSON → 仍回一個可寫入的 event，不拋例外")
        void garbageBodyStillLands() {
            UberWebhookEvent e = UberWebhookParser.parse("<html>502 Bad Gateway</html>".getBytes(StandardCharsets.UTF_8));
            assertEquals("unparseable", e.kind());
            assertEquals(UberWebhookEvent.Route.IGNORE, e.route());
            assertTrue(e.eventKey().startsWith("UNPARSEABLE:"));
        }

        @Test
        @DisplayName("body 為空 → 同上，且不得與任意 unparseable body 撞鍵")
        void emptyBodyStillLands() {
            UberWebhookEvent empty = UberWebhookParser.parse(new byte[0]);
            assertEquals("unparseable", empty.kind());
            assertTrue(empty.eventKey().startsWith("EMPTY:"));
            assertNotEquals(empty.eventKey(),
                    UberWebhookParser.parse("x".getBytes(StandardCharsets.UTF_8)).eventKey());
        }

        @Test
        @DisplayName("JSON 合法但沒有 kind → IGNORE，不猜")
        void jsonWithoutKindIsIgnored() {
            UberWebhookEvent e = UberWebhookParser.parse(body("{\"id\":\"evt_1\"}"));
            assertEquals(UberWebhookEvent.Route.IGNORE, e.route());
        }
    }
}
