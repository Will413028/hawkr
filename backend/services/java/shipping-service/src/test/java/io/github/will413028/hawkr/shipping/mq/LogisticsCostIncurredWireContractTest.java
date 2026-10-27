package io.github.will413028.hawkr.shipping.mq;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

import io.github.will413028.hawkr.shipping.service.dto.LogisticsCostIncurredPayload;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.lang.reflect.RecordComponent;
import java.util.Arrays;
import java.util.List;
import java.util.Set;
import java.util.TreeSet;
import java.util.stream.Collectors;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

/**
 * {@code logistics.cost.incurred} 的 <b>wire 契約</b>（70 §7.1 逐欄）。
 *
 * <h2>🔴 本檔最重要的一條：payload 不得帶 {@code responsibilityCause}</h2>
 * PM 的 Amendment 逐字：<b>Cost Fact ≠ Responsibility Decision</b>。
 * 舊版 §7.1 曾要求該欄必填且「未判定就不發」，與同書 §6.3／SD-15 的
 * 「成本已發生的事實<b>先記錄</b>，歸屬未定」直接矛盾——照舊版施工，
 * SD-15 圖裡那一步在物理上不可能發生。
 *
 * <p>⚠️ <b>在本檔之前，那條禁令只活在 javadoc 裡</b>：有人把欄位加回去，
 * 不會有任何東西轉紅。禁令沒有機械保證就只是一段註解。
 *
 * <p>🔴 把它加回來的後果不是「多一個欄位」，而是<b>兩份 responsibility truth</b>
 * （事件一份、N16 一份）——而「向賣家收錯錢」在雙 SSOT 下是<b>沒有訊號</b>的。
 *
 * <h2>型別：id 與金額一律 {@code String}</h2>
 * 65 §6A.1.3 通則。理由不是風格：notification-service 把 payload 解進
 * {@code map[string]interface{}}，{@code encoding/json} 對 {@code any} 用 <b>float64</b>
 * ⇒ 超過 2^53 的 TSID 會被<b>靜默截斷</b>。
 */
@DisplayName("logistics.cost.incurred 的 wire 契約")
class LogisticsCostIncurredWireContractTest {

    private static final ObjectMapper MAPPER = new ObjectMapper();

    private static Set<String> componentNames() {
        return Arrays.stream(LogisticsCostIncurredPayload.class.getRecordComponents())
                .map(RecordComponent::getName)
                .collect(Collectors.toCollection(TreeSet::new));
    }

    // =========================================================================
    // ① 禁止的欄位
    // =========================================================================

    @Test
    @DisplayName("🔴 payload 不得帶 responsibilityCause —— Cost Fact ≠ Responsibility Decision")
    void payloadCarriesNoResponsibilityCause() {
        Set<String> names = componentNames();

        // 對照組：先證明真的讀到了欄位。讀到 0 個時，下面每一條「不含」都會自動通過。
        assertFalse(names.isEmpty(), "量測本身失效：讀到 0 個 record component");
        assertTrue(names.contains("providerDeliveryRef"),
                "量測本身失效：連 providerDeliveryRef 都讀不到 ⇒ 反射抓錯類別。實際：" + names);

        for (String banned : List.of(
                // 🔴 本檔存在的理由。唯一的 responsibility authority 是 N16。
                "responsibilityCause", "responsibility_cause", "cause",
                // settlement counterparty 是純推導值（由 N16 的 cause 經 bearerOf 推得），
                // 落進 payload 就變成第二份 truth。
                "settlementCounterparty", "counterparty", "bearer",
                // 同一 leg 的費用 component 走 true-up，事件不承載費用分類。
                "feeType", "fee_type",
                // identity 已錨在 providerDeliveryRef；leg 自 identity 退場（§3.1.2）。
                "leg",
                // 責任判定的產物，不屬成本事實。
                "financiallyFinal", "refundAmount", "caseNo", "resolutionId")) {
            assertFalse(names.contains(banned),
                    "🔴 logistics.cost.incurred 的 payload 出現了 " + banned
                            + "——它不屬於成本事實。把它加回來會造成兩份 responsibility truth，"
                            + "而「向賣家收錯錢」在雙 SSOT 下是沒有訊號的。實際欄位：" + names);
        }
    }

    // =========================================================================
    // ② 必要的欄位與型別
    // =========================================================================

    @Test
    @DisplayName("欄位集合逐字等於 70 §7.1（多一個少一個都要有人回來改這裡）")
    void componentSetMatchesTheSpec() {
        assertEquals(
                new TreeSet<>(List.of(
                        // envelope（36 §7.4.1）
                        "eventId", "eventType", "eventVersion", "occurredAt",
                        "traceId", "correlationId",
                        // 業務欄
                        "provider", "providerDeliveryRef", "storeId", "orderNo",
                        "shipmentId", "estimatedCost")),
                componentNames(),
                "欄位集合與 70 §7.1 不符。🔴 加欄位前先問：它是 shipping domain 的事實嗎？"
                        + "不是的話它不屬於這條 wire");
    }

    @Test
    @DisplayName("🔴 id 與金額一律 String —— float64 會靜默截斷 TSID")
    void idsAndAmountsAreStrings() {
        for (RecordComponent c : LogisticsCostIncurredPayload.class.getRecordComponents()) {
            assertEquals(String.class, c.getType(),
                    "🔴 " + c.getName() + " 不是 String。65 §6A.1.3 通則：新事件的 id 與金額一律 string"
                            + "——notification-service 把 payload 解進 map[string]interface{}，"
                            + "encoding/json 對 any 用 float64 ⇒ 超過 2^53 的 TSID 會被靜默截斷");
        }
    }

    // =========================================================================
    // ③ 實際序列化出來的 wire key
    // =========================================================================

    @Test
    @DisplayName("wire key 是 snake_case，且 shipmentId 可為 null")
    void serialisesToSnakeCaseAndAllowsNullShipment() throws Exception {
        LogisticsCostIncurredPayload p = new LogisticsCostIncurredPayload(
                "evt-1", "LOGISTICS_COST_INCURRED", "v1", "2026-09-17T12:00:00Z",
                null, null,
                "3003", "del_abc", "2685914682", "O-1",
                null,          // 🔴 取消可能發生在 shipment 建立之前
                "115");

        JsonNode wire = MAPPER.readTree(MAPPER.writeValueAsString(p));

        assertTrue(wire.has("provider_delivery_ref"),
                "wire key 必須是 snake_case（對齊同 exchange 的 N16／N17）。實際：" + wire);
        assertTrue(wire.has("estimated_cost"), "實際：" + wire);
        assertTrue(wire.has("event_id"), "實際：" + wire);
        assertFalse(wire.has("providerDeliveryRef"),
                "不得同時出現 camelCase 鍵——consumer 是逐欄讀 snake 的");

        assertTrue(wire.get("shipment_id").isNull(),
                "🔴 shipment_id 必須能表達「沒有」。primitive 會把它靜默寫成 0，"
                        + "而 0 看起來像一個合法的 TSID");
        assertTrue(wire.get("estimated_cost").isTextual(),
                "🔴 金額在 wire 上必須是 quoted string，不得是 JSON 數字");
        assertTrue(wire.get("store_id").isTextual(),
                "🔴 id 在 wire 上必須是 quoted string");
    }
}
