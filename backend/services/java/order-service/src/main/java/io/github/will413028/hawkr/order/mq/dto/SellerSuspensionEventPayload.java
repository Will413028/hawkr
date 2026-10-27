package io.github.will413028.hawkr.order.mq.dto;

import com.fasterxml.jackson.databind.JsonNode;

/**
 * {@code seller.suspended} 事件 payload 的解析結果（36 §3.6f / 03 §5.1a.2）。
 *
 * <p>非 Jackson 自動綁定 DTO——來源 {@code SellerSuspendedEvent}（{@code pkg/mq/events.go}）
 * 內嵌 Go {@code BaseEvent}（snake_case，如 {@code event_type}），而
 * {@code suspensionId}/{@code accountId}/{@code reasonCode} 為 camelCase 且以 JSON string
 * 序列化（TSID 精度，docs/開發規範/Java 微服務開發規範.md §2.1）；手動解析與既有 {@code product-service SellerDeletedConsumer}
 * 慣例一致。order-service 不消費 {@code seller.unsuspended}，本 record 僅供 suspended 使用。
 *
 * <p>{@code storeId}（Plan C Phase 0 擴充，比照 coupon-service 同名 record）：JSON key 是
 * {@code store_id}（snake_case），與本 record 其餘欄位的 camelCase key 不一致——wire event
 * 本來就用 snake_case，本 record 是手動 JsonNode 解析不經 Jackson 反射綁定，沒有 struct-wide
 * casing 一致性需要維護。視為必填（fromJson 缺此欄位回傳 null），fail-closed：批次取消掃描
 * 若誤用 accountId 對分岔賣家會靜默 0 筆，寧可整批訊息被丟棄等重放，不可誤用。
 */
public record SellerSuspensionEventPayload(String eventType, Long suspensionId, Long accountId, String reasonCode, Long storeId) {

    /**
     * @return 解析失敗（suspensionId/accountId/storeId 缺失或非數字）回傳 {@code null}，呼叫端記 WARN + ACK
     */
    public static SellerSuspensionEventPayload fromJson(JsonNode payload) {
        String eventType = textOrEmpty(payload, "event_type");
        Long suspensionId = parseLongOrNull(textOrEmpty(payload, "suspensionId"));
        Long accountId = parseLongOrNull(textOrEmpty(payload, "accountId"));
        String reasonCode = textOrEmpty(payload, "reasonCode");
        Long storeId = parseLongOrNull(textOrEmpty(payload, "store_id"));
        if (suspensionId == null || accountId == null || storeId == null) {
            return null;
        }
        return new SellerSuspensionEventPayload(eventType, suspensionId, accountId, reasonCode, storeId);
    }

    private static String textOrEmpty(JsonNode node, String key) {
        JsonNode v = node.get(key);
        return v == null || v.isNull() ? "" : v.asText("");
    }

    private static Long parseLongOrNull(String s) {
        if (s == null || s.isEmpty()) {
            return null;
        }
        try {
            return Long.parseLong(s);
        } catch (NumberFormatException e) {
            return null;
        }
    }
}
