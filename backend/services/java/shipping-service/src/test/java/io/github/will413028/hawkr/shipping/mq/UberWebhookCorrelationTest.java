package io.github.will413028.hawkr.shipping.mq;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.nio.charset.StandardCharsets;

import static org.junit.jupiter.api.Assertions.assertEquals;

/**
 * {@link UberWebhookCorrelation} 的欄位優先序。
 *
 * <h2>fixture 來源（硬性）</h2>
 * 欄名一律抄自 {@code docs/第三方整合/Uber Direct API 總覽.md}：
 * §7.2「回傳我方識別碼的兩個欄位：{@code external_id}、{@code manifest.reference}」、
 * §6.2「{@code meta.external_order_id}」。期望值硬寫字面值。
 */
class UberWebhookCorrelationTest {

    private static byte[] b(String s) {
        return s.getBytes(StandardCharsets.UTF_8);
    }

    @Test
    @DisplayName("§7.2 data.external_id 優先（DaaS，我方用這組）")
    void dataExternalIdWins() {
        assertEquals("S000000123", UberWebhookCorrelation.externalIdOf(b(
                "{\"kind\":\"event.delivery_status\",\"data\":{\"external_id\":\"S000000123\","
                        + "\"manifest\":{\"reference\":\"M-9\"}},\"meta\":{\"external_order_id\":\"E-9\"}}")));
    }

    @Test
    @DisplayName("§7.2 沒有 external_id 時退到 data.manifest.reference")
    void dataManifestReferenceIsSecond() {
        assertEquals("M-9", UberWebhookCorrelation.externalIdOf(b(
                "{\"data\":{\"manifest\":{\"reference\":\"M-9\"}},\"meta\":{\"external_order_id\":\"E-9\"}}")));
    }

    @Test
    @DisplayName("欄位放在頂層（文件未逐字寫死巢狀層級）也抓得到")
    void topLevelIsAlsoAccepted() {
        assertEquals("S1", UberWebhookCorrelation.externalIdOf(b("{\"external_id\":\"S1\"}")));
        assertEquals("M1", UberWebhookCorrelation.externalIdOf(b("{\"manifest\":{\"reference\":\"M1\"}}")));
    }

    @Test
    @DisplayName("§6.2 Eats 家族的 meta.external_order_id 排在最後")
    void eatsMetaIsLast() {
        assertEquals("E-9", UberWebhookCorrelation.externalIdOf(b(
                "{\"event_type\":\"dapi.status_changed\",\"meta\":{\"external_order_id\":\"E-9\"}}")));
    }

    @Test
    @DisplayName("🔴 都沒有 → 回空字串（呼叫端必須當錯誤，不得當成「沒有變化」）")
    void absentReturnsEmpty() {
        assertEquals("", UberWebhookCorrelation.externalIdOf(b("{\"kind\":\"event.delivery_status\"}")));
        assertEquals("", UberWebhookCorrelation.externalIdOf(b("<html>502</html>")));
        assertEquals("", UberWebhookCorrelation.externalIdOf(new byte[0]));
        assertEquals("", UberWebhookCorrelation.externalIdOf(null));
    }

    @Test
    @DisplayName("空字串與空白不算命中（否則會拿 \"\" 去查 shipments 並靜默查不到）")
    void blankIsNotAHit() {
        assertEquals("M-9", UberWebhookCorrelation.externalIdOf(b(
                "{\"data\":{\"external_id\":\"   \",\"manifest\":{\"reference\":\"M-9\"}}}")));
    }

    @Test
    @DisplayName("非字串型別不算命中（物件／陣列不得被 asText 變成 \"[...]\"）")
    void nonScalarIsNotAHit() {
        assertEquals("", UberWebhookCorrelation.externalIdOf(b(
                "{\"data\":{\"external_id\":{\"nested\":1}}}")));
    }
}
