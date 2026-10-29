package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * ADM 代行營運的稽核動作類型。Code 與 Value 一致（UPPER_SNAKE_CASE，直接存讀可辨識文字），對齊 EventType 系列字串式 value 設計。`riskLevel` 不是本 enum 的欄位，由 delegation 模組的 `RiskLevel()`（`internal/modules/delegation/internal/model`）純函式決定（未登錄的 action 一律回 high，忘記登記的後果必須是多記一次高風險而非靜默降級）；CodeMeta enum 內禁止放業務輔助邏輯。四個 READ 動作只在讀取被拒絕時寫入，result 恆為 FAILED、biz_code 僅 9262 / 9263 / 9275 三者，讀取成功一律不落稽核（可觀測性由 access log 承擔）。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum DelegationAuditAction implements CodeMeta {

    STOREFRONT_UPDATE("STOREFRONT_UPDATE", "STOREFRONT_UPDATE", "更新店面資料", "capability `storefront.write`；店面欄位 PATCH"),
    STOREFRONT_LOGO_UPDATE("STOREFRONT_LOGO_UPDATE", "STOREFRONT_LOGO_UPDATE", "更新商店 Logo", "capability `storefront.write`；以 staged token 轉正媒體"),
    STOREFRONT_COVER_UPDATE("STOREFRONT_COVER_UPDATE", "STOREFRONT_COVER_UPDATE", "更新商店封面", "capability `storefront.write`；以 staged token 轉正媒體"),
    PRODUCT_CREATE("PRODUCT_CREATE", "PRODUCT_CREATE", "建立商品", "capability `products.write`；建立時由代行者顯性選擇「暫不上架」（落 `ProductStatus` 未上架）或「並上架」，後者另需 `products.publish` 並額外記一列 PRODUCT_PUBLISH"),
    PRODUCT_UPDATE("PRODUCT_UPDATE", "PRODUCT_UPDATE", "編輯商品", "capability `products.write`；帶 expectedVersion 樂觀鎖"),
    PRODUCT_PUBLISH("PRODUCT_PUBLISH", "PRODUCT_PUBLISH", "上架商品", "capability `products.publish`；買家可見狀態變更"),
    PRODUCT_UNPUBLISH("PRODUCT_UNPUBLISH", "PRODUCT_UNPUBLISH", "下架商品", "capability `products.publish`；買家可見狀態變更"),
    PRODUCT_BATCH_UNPUBLISH("PRODUCT_BATCH_UNPUBLISH", "PRODUCT_BATCH_UNPUBLISH", "批次下架商品", "capability `products.publish`；整批一把 Idempotency-Key，稽核逐項一列"),
    POST_CREATE("POST_CREATE", "POST_CREATE", "建立貼文", "capability `posts.write`；建立時由代行者顯性選擇「暫不發布」（落 `PostPublishStatus` 未發布）或「並發布」，後者另需 `posts.publish` 並額外記一列 POST_PUBLISH"),
    POST_UPDATE("POST_UPDATE", "POST_UPDATE", "編輯貼文", "capability `posts.write`；帶 expectedVersion 樂觀鎖"),
    POST_PUBLISH("POST_PUBLISH", "POST_PUBLISH", "發布貼文", "capability `posts.publish`；publishMode=immediate"),
    POST_UNPUBLISH("POST_UNPUBLISH", "POST_UNPUBLISH", "下架貼文", "capability `posts.publish`；publish_status 轉 UNPUBLISHED，與平台強制下架 hidden_at 無關"),
    POST_SCHEDULE("POST_SCHEDULE", "POST_SCHEDULE", "排程發布貼文", "capability `posts.publish`；publishMode=scheduled，實際執行由 social-service 排程 worker 以 origin=scheduled 回寫"),
    ORDER_PII_REVEAL("ORDER_PII_REVEAL", "ORDER_PII_REVEAL", "查看訂單完整收件資訊", "capability `orders.read`；每次揭露都是獨立稽核事實，刻意不去重；與 ORDER_READ 是兩個不同事實不得互相取代"),
    STOREFRONT_READ("STOREFRONT_READ", "STOREFRONT_READ", "查看店面內容（被拒絕）", "僅讀取被拒時寫入，result 恆為 FAILED；快照兩欄皆 NULL"),
    PRODUCT_READ("PRODUCT_READ", "PRODUCT_READ", "查看商品（被拒絕）", "同上"),
    POST_READ("POST_READ", "POST_READ", "查看貼文（被拒絕）", "同上"),
    ORDER_READ("ORDER_READ", "ORDER_READ", "查看訂單（被拒絕）", "同上；跨店讀取嘗試（9275）是本案安全模型最在意的形狀，此列是唯一痕跡"),
    AUDIT_CORRECTION("AUDIT_CORRECTION", "AUDIT_CORRECTION", "稽核補正（維運）", "維運補正列的動作，不是代行操作");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    DelegationAuditAction(String code, String value, String label, String description) {
        this.code = code;
        this.value = value;
        this.label = label;
        this.description = description;
    }

    @Override
    public String code() { return code; }

    @Override
    public String value() { return value; }

    @Override
    public String label() { return label; }

    @Override
    public String description() { return description; }

    private static final Map<String, DelegationAuditAction> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(DelegationAuditAction::code, e -> e));

    private static final Map<String, DelegationAuditAction> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(DelegationAuditAction::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static DelegationAuditAction fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static DelegationAuditAction fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
