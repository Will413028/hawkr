package io.github.will413028.hawkr.payment.logistics.domain;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import java.time.LocalDateTime;
import java.time.ZoneOffset;
import java.time.format.DateTimeFormatter;
import java.time.format.DateTimeParseException;

/**
 * {@link ResponsibilityImpactPreview} 的 JSONB 快照編解碼（WP2A-R2）。
 *
 * <p>🔴 <b>存在的理由</b>：在此之前 {@code impact_preview} 是「呼叫端給什麼就存什麼」的
 * 原始 JSON 字串（null 就存 {@code "{}"}），零驗證、零解析、零讀取。
 * 那讓覆核時<b>無從重算比對</b>——而重算比對正是 65 硬約束 2 的全部內容。
 *
 * <p>🔴 <b>嚴格 parse</b>：欄位缺一、型別不對、多出不認得的欄位一律拒絕。
 * 寬鬆 parse 會讓一份壞掉的快照在覆核當下「看起來還能用」，
 * 而那時錢已經要退出去了。
 *
 * <p>⚠️ <b>金額在 JSON 裡是字串</b>（CLAUDE.md §4／65 的 A-4 逐字 {@code refundAmount: string}）：
 * 這份快照最終會被 ADM 讀去顯示，而 JSON number 在 JavaScript 端超過 2^53 會失真。
 * 內部型別仍是 {@code long}，只在這個邊界轉換。
 */
public final class ResponsibilityImpactPreviewCodec {

    /** 與 {@code ResponsibilityCaseService.ISO_UTC} 同一個格式（ISO 8601 UTC，帶 Z）。 */
    private static final DateTimeFormatter ISO_UTC =
            DateTimeFormatter.ofPattern("yyyy-MM-dd'T'HH:mm:ss'Z'");

    private static final String F_REFUND = "refundAmount";
    private static final String F_COMPUTED_AT = "computedAt";

    private ResponsibilityImpactPreviewCodec() {
    }

    public static String toJson(ObjectMapper mapper, ResponsibilityImpactPreview preview) {
        ObjectNode n = mapper.createObjectNode();
        n.put(F_REFUND, String.valueOf(preview.refundAmount()));
        n.put(F_COMPUTED_AT, preview.computedAt().atOffset(ZoneOffset.UTC).format(ISO_UTC));
        return n.toString();
    }

    /**
     * 嚴格解析持久化的快照。
     *
     * @throws IllegalStateException 快照不是合法的 {@link ResponsibilityImpactPreview}
     *         ——這代表有寫入路徑繞過了本 codec，屬不變式損壞
     */
    public static ResponsibilityImpactPreview fromJson(ObjectMapper mapper, String json) {
        if (json == null || json.isBlank()) {
            throw new IllegalStateException(
                    "responsibility: impact_preview 為空——責任提案必須帶 server 算出的預覽快照");
        }
        JsonNode root;
        try {
            root = mapper.readTree(json);
        } catch (Exception e) {
            throw new IllegalStateException("responsibility: impact_preview 不是合法 JSON：" + json, e);
        }
        if (!root.isObject()) {
            throw new IllegalStateException("responsibility: impact_preview 必須是 JSON object：" + json);
        }
        // 🔴 多出來的欄位一律拒絕：platformFeeDelta／sellerNetDelta 這類「看起來合理但沒有
        //    canonical 算法」的欄位若被悄悄塞進來，覆核者會以為它們被驗過了。
        if (root.size() != 2 || !root.has(F_REFUND) || !root.has(F_COMPUTED_AT)) {
            throw new IllegalStateException(
                    "responsibility: impact_preview 的欄位集合必須恰為 {" + F_REFUND + ", "
                            + F_COMPUTED_AT + "}（收到 " + root.fieldNames() + "）："
                            + json);
        }
        JsonNode refund = root.get(F_REFUND);
        if (!refund.isTextual()) {
            throw new IllegalStateException(
                    "responsibility: " + F_REFUND + " 必須是字串（避免 JS 精度失真）：" + json);
        }
        long amount;
        try {
            amount = Long.parseLong(refund.asText());
        } catch (NumberFormatException e) {
            throw new IllegalStateException(
                    "responsibility: " + F_REFUND + " 不是整數金額：" + json, e);
        }
        LocalDateTime computedAt;
        try {
            computedAt = LocalDateTime.parse(root.get(F_COMPUTED_AT).asText(), ISO_UTC);
        } catch (DateTimeParseException e) {
            throw new IllegalStateException(
                    "responsibility: " + F_COMPUTED_AT + " 不是 ISO 8601 UTC：" + json, e);
        }
        return new ResponsibilityImpactPreview(amount, computedAt);
    }
}
