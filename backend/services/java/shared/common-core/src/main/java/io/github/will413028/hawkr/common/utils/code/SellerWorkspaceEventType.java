package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 賣家工作區 realtime invalidation 事件 MQ 類型。Code 與 Value 一致（UPPER_SNAKE_CASE），與 routing key 不同 — routing key 為點分式 `seller.order.updated` / `seller.after_sale.updated`，寫於 `MqConstants` / `pkg/mq/exchanges.go`；event_type 僅作 metadata。對齊 [BuyerEventType](#BuyerEventType) / [SellerEventType](#SellerEventType) 設計。發布到 `hawkr.seller_workspace` exchange，唯一 consumer 為 gateway SSE（推賣家 web 訂單管理頁，[B-207] #1181）。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum SellerWorkspaceEventType implements CodeMeta {

    SELLER_ORDER_UPDATED("SELLER_ORDER_UPDATED", "SELLER_ORDER_UPDATED", "賣家訂單狀態更新", "Routing key `seller.order.updated`；order-service 所有 order 狀態 commit 後發布，gateway SSE 推賣家訂單管理頁刷新"),
    SELLER_AFTER_SALE_UPDATED("SELLER_AFTER_SALE_UPDATED", "SELLER_AFTER_SALE_UPDATED", "賣家售後狀態更新", "Routing key `seller.after_sale.updated`；aftersales-service case 狀態流轉後發布，gateway SSE 推賣家售後列表刷新");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    SellerWorkspaceEventType(String code, String value, String label, String description) {
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

    private static final Map<String, SellerWorkspaceEventType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(SellerWorkspaceEventType::code, e -> e));

    private static final Map<String, SellerWorkspaceEventType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(SellerWorkspaceEventType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static SellerWorkspaceEventType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static SellerWorkspaceEventType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
