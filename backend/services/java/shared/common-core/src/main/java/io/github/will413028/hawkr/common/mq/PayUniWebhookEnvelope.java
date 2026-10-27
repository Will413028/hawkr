package io.github.will413028.hawkr.common.mq;

import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;

import java.nio.charset.StandardCharsets;
import java.util.Collections;
import java.util.Map;

/**
 * webhook-service 發布到 MQ 的標準信封格式
 *
 * 所有消費 payuni.events exchange 的 Java Consumer 統一使用此類解析。
 * PayUni 原始欄位在 payload 子物件中，讀取時必須從 payload 取。
 *
 * 兼容兩種格式：
 * 1. 標準 envelope（有 event_id + payload 子物件；envelope 層欄位一律 snake_case）
 * 2. 裸 payload（直接就是 PayUni 欄位，向下兼容）
 */
public record PayUniWebhookEnvelope(
    String eventId,
    String provider,
    String webhookType,
    String merId,
    String eventKey,
    String occurredAt,
    Map<String, String> payload,
    Map<String, String> meta
) {
    private static final ObjectMapper MAPPER = new ObjectMapper();
    private static final TypeReference<Map<String, String>> MAP_TYPE = new TypeReference<>() {};

    /**
     * 從 MQ 消息 body（byte[]）解析 envelope
     *
     * 判定邏輯：
     * - JSON 根層含 "event_id" 欄位 → 標準 envelope，從 payload 子物件提取
     * - JSON 根層不含 "event_id" → 裸 payload（整個 JSON 視為 payload）
     *
     * @param body MQ 消息 body
     * @return 解析後的 envelope
     * @throws IllegalArgumentException body 為 null 或空
     * @throws RuntimeException JSON 解析失敗
     */
    public static PayUniWebhookEnvelope parse(byte[] body) {
        if (body == null || body.length == 0) {
            throw new IllegalArgumentException("Webhook MQ 消息 body 不得為空");
        }

        try {
            JsonNode root = MAPPER.readTree(new String(body, StandardCharsets.UTF_8));

            if (root.has("event_id")) {
                // 標準 envelope 格式（36 §7.4 Conformance Contract snake_case）
                String eventId = root.path("event_id").asText("");
                String provider = root.path("provider").asText("");
                String webhookType = root.path("webhook_type").asText("");
                String merId = root.path("mer_id").asText("");
                String eventKey = root.path("event_key").asText("");
                String occurredAt = root.path("occurred_at").asText("");

                JsonNode payloadNode = root.path("payload");
                Map<String, String> payload = payloadNode.isMissingNode() || payloadNode.isNull()
                        ? Collections.emptyMap()
                        : MAPPER.convertValue(payloadNode, MAP_TYPE);

                JsonNode metaNode = root.path("meta");
                Map<String, String> meta = metaNode.isMissingNode() || metaNode.isNull()
                        ? Collections.emptyMap()
                        : MAPPER.convertValue(metaNode, MAP_TYPE);

                return new PayUniWebhookEnvelope(eventId, provider, webhookType, merId, eventKey, occurredAt, payload, meta);
            } else {
                // 裸 payload（向下兼容）
                Map<String, String> payload = MAPPER.convertValue(root, MAP_TYPE);
                return new PayUniWebhookEnvelope("", "", "", "", "", "", payload, Collections.emptyMap());
            }
        } catch (IllegalArgumentException e) {
            throw e;
        } catch (Exception e) {
            throw new RuntimeException("解析 Webhook Envelope 失敗: " + e.getMessage(), e);
        }
    }

    /**
     * 取得 PayUni 原始欄位值
     *
     * @param fieldName PayUni 欄位名（如 "ShipTradeNo"、"MerTradeNo"）
     * @return 欄位值，不存在時回傳空字串
     */
    public String payloadField(String fieldName) {
        return payload != null ? payload.getOrDefault(fieldName, "") : "";
    }
}
