package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 賣家領域 MQ 事件類型。Code 與 Value 一致（UPPER_SNAKE_CASE），與 routing key 不同 — routing key 為點分式 `seller.deleted` / `seller.profile.changed` / `seller.suspended` / `seller.unsuspended`，寫於 `pkg/mq/exchanges.go`；event_type 僅作 metadata。對齊 [BuyerEventType](#BuyerEventType) / [OrderEventType](#OrderEventType) 設計。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum SellerEventType implements CodeMeta {

    SELLER_DELETED("SELLER_DELETED", "SELLER_DELETED", "賣家帳號刪除", "Routing key `seller.deleted`；product-service consume 後批次下架該賣家 ACTIVE 商品為 INACTIVE"),
    SELLER_PROFILE_CHANGED("SELLER_PROFILE_CHANGED", "SELLER_PROFILE_CHANGED", "賣家資料變更", "Routing key `seller.profile.changed`；賣家改店名 / logo / 封面後 emit，cart-service consume 後 evict `seller:meta:{sellerAccountId}`（#1426 方案 A）"),
    SELLER_SUSPENDED("SELLER_SUSPENDED", "SELLER_SUSPENDED", "賣家帳號停權", "Routing key `seller.suspended`；product/order/coupon/notification/im consume 後分別下架 ACTIVE 商品（可逆快照）/ 批次取消未付款訂單（`OrderCancelReason.SELLER_SUSPENDED`）/ 停用優惠券模板 / 通知賣買雙方 / 斷 WS（46 §5）"),
    SELLER_UNSUSPENDED("SELLER_UNSUSPENDED", "SELLER_UNSUSPENDED", "賣家帳號解除停權", "Routing key `seller.unsuspended`；下游 consumer 依可逆快照精準還原停權前狀態（46 §5）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    SellerEventType(String code, String value, String label, String description) {
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

    private static final Map<String, SellerEventType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(SellerEventType::code, e -> e));

    private static final Map<String, SellerEventType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(SellerEventType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static SellerEventType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static SellerEventType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
