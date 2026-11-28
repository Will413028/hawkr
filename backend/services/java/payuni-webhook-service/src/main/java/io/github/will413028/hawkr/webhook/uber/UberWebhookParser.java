package io.github.will413028.hawkr.webhook.uber;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.HexFormat;

/**
 * Uber webhook raw body → {@link UberWebhookEvent}（ingress 層路由資訊）。
 *
 * <h2>🔴 本類別讀進 raw bytes，但<b>絕不</b>把它 re-serialize 回去</h2>
 * 驗簽是 HMAC-SHA256 over raw body，先 parse 再 re-serialize 會因欄位順序不同而<b>偶發</b>失敗
 *（66 §6.3.1 陷阱 A2）。本類別回傳的是<b>抽出來的字串</b>，原始位元組由呼叫端原封不動往下傳。
 *
 * <h2>欄名來源</h2>
 * 全部抄自 {@code docs/third-party-integrations/uber-direct-api-overview.md}：
 * <ul>
 *   <li>§7.2 DaaS webhook 頂層：{@code id}／{@code kind}／{@code created}／{@code delivery_id}
 *       （🔴 我方整合的是 DaaS／Direct 家族）</li>
 *   <li>§6.2 Eats 家族：{@code event_id}／{@code event_type}／{@code event_time}
 *       —— 我方<b>不用</b>這一組，但兩家族共用同一個 webhook 端點形狀，讀得到就收下當 fallback，
 *       路由上它會落到 {@link UberWebhookEvent.Route#IGNORE}（§6.3.2 沒有它）</li>
 * </ul>
 */
public final class UberWebhookParser {

    static final String F_ID = "id";
    static final String F_KIND = "kind";
    static final String F_CREATED = "created";
    static final String F_DELIVERY_ID = "delivery_id";

    static final String F_EVENT_ID = "event_id";
    static final String F_EVENT_TYPE = "event_type";
    static final String F_EVENT_TIME = "event_time";

    /**
     * §6.3.2 的四種事件。
     *
     * <p>⚠️ 文件兩處寫法不同：§7.1 逐字 {@code kind = event.delivery_status}（帶前綴），
     * §6.3.2 的表列的是不帶前綴的 {@code delivery_status}。
     * ⇒ 比對時<b>剝掉可選的 {@code event.} 前綴</b>，兩種寫法都認得；
     * 而 {@code webhook_type} 欄位存的是<b>原樣的 kind</b>（66 §6.3.3：provider 原生事件型別）。
     */
    static final String PREFIX = "event.";
    static final String KIND_DELIVERY_STATUS = "delivery_status";
    static final String KIND_COURIER_UPDATE = "courier_update";
    static final String KIND_REFUND_REQUEST = "refund_request";
    static final String KIND_SHOPPING_PROGRESS = "shopping_progress";

    /** body 連 JSON 都不是時寫進 {@code webhook_type} 的值。 */
    static final String KIND_UNPARSEABLE = "unparseable";

    private static final ObjectMapper MAPPER = new ObjectMapper();

    private UberWebhookParser() {}

    /**
     * @param rawBody HTTP body 的原始位元組（不得先經 String 來回）
     * @return 永不回 null —— body 壞掉也要回一個可落地的 event
     *         （66 §6.3.4：「能落地就 200」，丟掉才是永久遺失）
     */
    public static UberWebhookEvent parse(byte[] rawBody) {
        if (rawBody == null || rawBody.length == 0) {
            return new UberWebhookEvent(KIND_UNPARSEABLE, "EMPTY:" + sha256Prefix(new byte[0]),
                    UberWebhookEvent.KeyTier.FALLBACK_COMPOSITE, UberWebhookEvent.Route.IGNORE);
        }

        JsonNode root;
        try {
            root = MAPPER.readTree(new String(rawBody, StandardCharsets.UTF_8));
        } catch (Exception e) {
            return new UberWebhookEvent(KIND_UNPARSEABLE, "UNPARSEABLE:" + sha256Prefix(rawBody),
                    UberWebhookEvent.KeyTier.FALLBACK_COMPOSITE, UberWebhookEvent.Route.IGNORE);
        }
        if (root == null || !root.isObject()) {
            return new UberWebhookEvent(KIND_UNPARSEABLE, "UNPARSEABLE:" + sha256Prefix(rawBody),
                    UberWebhookEvent.KeyTier.FALLBACK_COMPOSITE, UberWebhookEvent.Route.IGNORE);
        }

        String kind = text(root, F_KIND);
        if (kind.isEmpty()) {
            kind = text(root, F_EVENT_TYPE);
        }

        String providerEventId = text(root, F_ID);
        if (providerEventId.isEmpty()) {
            providerEventId = text(root, F_EVENT_ID);
        }

        String eventKey;
        UberWebhookEvent.KeyTier tier;
        if (!providerEventId.isEmpty()) {
            eventKey = providerEventId;
            tier = UberWebhookEvent.KeyTier.PROVIDER_EVENT_ID;
        } else {
            String timestamp = text(root, F_CREATED);
            if (timestamp.isEmpty()) {
                timestamp = text(root, F_EVENT_TIME);
            }
            eventKey = String.join(":",
                    text(root, F_DELIVERY_ID),
                    kind,
                    timestamp,
                    sha256Prefix(rawBody));
            tier = UberWebhookEvent.KeyTier.FALLBACK_COMPOSITE;
        }

        return new UberWebhookEvent(kind.isEmpty() ? KIND_UNPARSEABLE : kind, eventKey, tier, routeOf(kind));
    }

    /** 66 §6.3.2「四種事件與訂閱決策」。 */
    static UberWebhookEvent.Route routeOf(String kind) {
        String bare = kind == null ? "" : kind.startsWith(PREFIX) ? kind.substring(PREFIX.length()) : kind;
        return switch (bare) {
            case KIND_DELIVERY_STATUS -> UberWebhookEvent.Route.PUBLISH;
            case KIND_COURIER_UPDATE -> UberWebhookEvent.Route.INBOX_ONLY;
            case KIND_REFUND_REQUEST -> UberWebhookEvent.Route.IGNORE;
            case KIND_SHOPPING_PROGRESS -> UberWebhookEvent.Route.IGNORE;
            default -> UberWebhookEvent.Route.IGNORE;
        };
    }

    private static String text(JsonNode root, String field) {
        JsonNode n = root.path(field);
        return n.isMissingNode() || n.isNull() ? "" : n.asText("");
    }

    /** {@code sha256(raw_body)[0:16]}（16 個 hex 字元），66 §6.3.3 級 2 的第四段。 */
    static String sha256Prefix(byte[] raw) {
        try {
            byte[] digest = MessageDigest.getInstance("SHA-256").digest(raw);
            return HexFormat.of().formatHex(digest).substring(0, 16);
        } catch (NoSuchAlgorithmException e) {
            throw new IllegalStateException("SHA-256 不可用", e);
        }
    }
}
