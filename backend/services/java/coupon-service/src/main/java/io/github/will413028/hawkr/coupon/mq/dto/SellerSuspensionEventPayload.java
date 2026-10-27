package io.github.will413028.hawkr.coupon.mq.dto;

import com.fasterxml.jackson.databind.JsonNode;

/**
 * {@code seller.suspended} / {@code seller.unsuspended} 事件 payload 的解析結果（36 §3.6f / 07 §5.20）。
 *
 * <p>非 Jackson 自動綁定 DTO——來源 {@code SellerSuspendedEvent}/{@code SellerUnsuspendedEvent}
 * （{@code pkg/mq/events.go}）內嵌 Go {@code BaseEvent}（snake_case，如 {@code event_type}），而
 * {@code suspensionId}/{@code accountId}/{@code reasonCode} 為 camelCase 且以 JSON string
 * 序列化（TSID 精度，docs/開發規範/Java 微服務開發規範.md §2.1）；手動解析與既有 {@code product-service SellerDeletedConsumer}
 * 慣例一致。
 *
 * <p>{@code storeId}（Plan C Phase 0 擴充）：JSON key 是 {@code store_id}（snake_case），
 * 與本 record 其餘欄位的 camelCase key 不一致——這是 Phase 0 的刻意決定（wire event 該欄位
 * 本來就用 snake_case，因為本 record 是手動 JsonNode 解析，不經 Jackson 反射綁定，沒有
 * struct-wide casing 一致性需要維護）。目前視為必填（fromJson 缺此欄位回傳 null）——
 * 前提是 Phase 0 已先合併部署，事件一律帶 store_id；若 Phase 1 部署早於 Phase 0，
 * 這個必填會讓訊息被丟棄而非誤用 0 值，是刻意的 fail-closed 設計。
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
