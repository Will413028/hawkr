package io.github.will413028.hawkr.shipping.mq;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;

import java.nio.charset.StandardCharsets;

/**
 * 從 Uber 回調的 raw body 取出<b>我方的配送識別碼</b>，用來定位 {@code shipments}。
 *
 * <h2>🔴 已登記的技術債：這是第二份 provider 知識</h2>
 * 「怎麼從 provider 的 payload 找出我方識別碼」本來就該和「怎麼正規化狀態」「怎麼算去重鍵」
 * 一起住在 {@code InboundCapability}（65 附錄 ADR-012 的整個論點）。
 * 它落在 consumer 這一層，唯一的理由是<b>檔案佔用</b>：
 * {@code provider/capability/InboundCapability.java} 由另一條 lane 持有，本波不得新增方法。
 *
 * <p>⇒ 待 {@code InboundCapability} 可編輯時，正解是加一支
 * {@code Optional<String> correlationKey(InboundPayload raw)}，把本類別整個刪掉。
 * <b>在那之前，第二個 JSON provider 進來時不得複製本類別</b> —— 複製就變成三份。
 *
 * <h2>欄名來源</h2>
 * 全部抄自 {@code docs/第三方整合/Uber Direct API 總覽.md}：
 * <ul>
 *   <li>§7.2（DaaS／Direct，<b>我方用這組</b>）逐字：「回傳我方識別碼的兩個欄位：
 *       {@code external_id}（我方送出的配送識別碼）、{@code manifest.reference}
 *       （我方送出的 manifest 識別碼）」。
 *       ⚠️ 同節列出的<b>頂層</b>欄位是 {@code id}／{@code kind}／{@code created}／{@code live_mode}／
 *       {@code status}／{@code data}／… —— {@code external_id} 與 {@code manifest} <b>不在那張頂層清單裡</b>
 *       ⇒ 它們在 delivery 物件（{@code data}）內。本類別先找 {@code data.*}，再退回頂層：
 *       文件沒有逐字寫死巢狀層級，兩處都找是<b>取聯集</b>，取第一個非空值，不會誤判</li>
 *   <li>§6.2（Eats 家族，我方不用）：{@code meta.external_order_id}。列在最後，
 *       只為了「真的收到那個家族的事件時不會完全抓不到」</li>
 * </ul>
 */
final class UberWebhookCorrelation {

    // §7.2 DaaS：delivery 物件與其中的我方識別碼
    static final String F_DATA = "data";
    static final String F_EXTERNAL_ID = "external_id";
    static final String F_MANIFEST = "manifest";
    static final String F_REFERENCE = "reference";

    // §6.2 Eats：meta.external_order_id
    static final String F_META = "meta";
    static final String F_EXTERNAL_ORDER_ID = "external_order_id";

    private static final ObjectMapper MAPPER = new ObjectMapper();

    private UberWebhookCorrelation() {}

    /**
     * @param rawBody Uber 送來的原始位元組
     * @return 我方識別碼；找不到時回空字串（呼叫端必須當成錯誤處理，<b>不得</b>當成「沒有變化」）
     */
    static String externalIdOf(byte[] rawBody) {
        if (rawBody == null || rawBody.length == 0) {
            return "";
        }
        JsonNode root;
        try {
            root = MAPPER.readTree(new String(rawBody, StandardCharsets.UTF_8));
        } catch (Exception e) {
            return "";
        }
        if (root == null || !root.isObject()) {
            return "";
        }

        JsonNode data = root.path(F_DATA);
        String fromData = text(data, F_EXTERNAL_ID);
        if (!fromData.isEmpty()) {
            return fromData;
        }
        String fromDataManifest = text(data.path(F_MANIFEST), F_REFERENCE);
        if (!fromDataManifest.isEmpty()) {
            return fromDataManifest;
        }
        String topLevel = text(root, F_EXTERNAL_ID);
        if (!topLevel.isEmpty()) {
            return topLevel;
        }
        String topLevelManifest = text(root.path(F_MANIFEST), F_REFERENCE);
        if (!topLevelManifest.isEmpty()) {
            return topLevelManifest;
        }
        return text(root.path(F_META), F_EXTERNAL_ORDER_ID);
    }

    private static String text(JsonNode node, String field) {
        if (node == null || node.isMissingNode() || node.isNull()) {
            return "";
        }
        JsonNode n = node.path(field);
        return n.isMissingNode() || n.isNull() || !n.isValueNode() ? "" : n.asText("").trim();
    }
}
