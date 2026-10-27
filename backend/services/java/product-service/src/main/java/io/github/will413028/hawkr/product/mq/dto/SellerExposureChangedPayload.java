package io.github.will413028.hawkr.product.mq.dto;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;

import java.io.IOException;

/**
 * {@code seller.exposure.changed} 事件內容：店家目前可不可曝光，與該店單調遞增的版本。
 *
 * <p>{@code store_id}、{@code exposure_version} 容忍 JSON number 或 string；{@code exposable} 必須是 JSON boolean。
 * 缺欄位、null 或格式不符一律丟 {@link IllegalArgumentException}，由 consumer 送進 DLQ。
 */
public record SellerExposureChangedPayload(String eventType, long storeId, boolean exposable, long exposureVersion) {

    public static final String EVENT_TYPE = "SELLER_EXPOSURE_CHANGED";

    private static final ObjectMapper MAPPER = new ObjectMapper();

    public static SellerExposureChangedPayload parse(String body) {
        JsonNode root;
        try {
            root = MAPPER.readTree(body);
        } catch (IOException e) {
            throw new IllegalArgumentException("payload 不是合法 JSON: " + e.getMessage(), e);
        }
        if (root == null || !root.isObject()) {
            throw new IllegalArgumentException("payload 不是 JSON object");
        }
        String eventType = required(root, "event_type").asText("");
        if (!EVENT_TYPE.equals(eventType)) {
            throw new IllegalArgumentException("event_type 不符: " + eventType);
        }
        long storeId = positiveLong(required(root, "store_id"), "store_id");
        long version = positiveLong(required(root, "exposure_version"), "exposure_version");
        JsonNode exposable = required(root, "exposable");
        if (!exposable.isBoolean()) {
            throw new IllegalArgumentException("exposable 不是 boolean");
        }
        return new SellerExposureChangedPayload(eventType, storeId, exposable.booleanValue(), version);
    }

    private static JsonNode required(JsonNode root, String key) {
        JsonNode v = root.get(key);
        if (v == null || v.isNull()) {
            throw new IllegalArgumentException("缺少 " + key);
        }
        return v;
    }

    private static long positiveLong(JsonNode v, String key) {
        if (!v.isIntegralNumber() && !v.isTextual()) {
            throw new IllegalArgumentException(key + " 不是整數");
        }
        long parsed;
        try {
            parsed = Long.parseLong(v.asText().trim());
        } catch (NumberFormatException e) {
            throw new IllegalArgumentException(key + " 不是整數: " + v.asText(), e);
        }
        if (parsed <= 0) {
            throw new IllegalArgumentException(key + " 必須為正數: " + parsed);
        }
        return parsed;
    }
}
