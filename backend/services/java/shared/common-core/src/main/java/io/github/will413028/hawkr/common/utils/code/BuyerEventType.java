package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 買家領域 MQ 事件類型。Code 與 Value 一致（UPPER_SNAKE_CASE），與 routing key 不同 — routing key 為點分式 `buyer.favorite.added` / `buyer.favorite.removed`，寫於 `pkg/mq/exchanges.go`；event_type 僅作 metadata 與 dedup 表索引欄位。對齊 [OrderEventType](#OrderEventType) 設計。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum BuyerEventType implements CodeMeta {

    BUYER_FAVORITE_ADDED("BUYER_FAVORITE_ADDED", "BUYER_FAVORITE_ADDED", "買家新增收藏", "Routing key `buyer.favorite.added`；product-service consume 後 `product.favorite_count += 1`"),
    BUYER_FAVORITE_REMOVED("BUYER_FAVORITE_REMOVED", "BUYER_FAVORITE_REMOVED", "買家取消收藏", "Routing key `buyer.favorite.removed`；product-service consume 後 `product.favorite_count -= 1`（DB CHECK 防負值）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    BuyerEventType(String code, String value, String label, String description) {
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

    private static final Map<String, BuyerEventType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(BuyerEventType::code, e -> e));

    private static final Map<String, BuyerEventType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(BuyerEventType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static BuyerEventType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static BuyerEventType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
