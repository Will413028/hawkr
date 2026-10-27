package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 補貨通知訂閱狀態機（買家對單一商品的補貨訂閱）。`ACTIVE` 為唯一活躍態（partial UNIQUE `(buyer_id, product_id) WHERE status='9501'`）；補貨 fan-out consumer 將 ACTIVE 轉為 `NOTIFIED` 並寫入 `notified_at`（one-shot 模式）；買家主動取消則轉為 `CANCELLED`。NOTIFIED / CANCELLED 行皆保留作審計（不物理刪除），亦不參與 unique constraint，買家可重複訂閱建立新 ACTIVE 行。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum RestockSubscriptionStatus implements CodeMeta {

    ACTIVE("ACTIVE", "9501", "活躍中", "已訂閱，等待補貨事件；唯一可被 fan-out consumer 通知的狀態"),
    NOTIFIED("NOTIFIED", "9502", "已通知", "補貨 fan-out 已寫 inbox 並推 FCM；one-shot 機制下不再參與後續通知；終態（保留審計）"),
    CANCELLED("CANCELLED", "9503", "已取消", "買家主動取消訂閱；終態（保留審計）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    RestockSubscriptionStatus(String code, String value, String label, String description) {
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

    private static final Map<String, RestockSubscriptionStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(RestockSubscriptionStatus::code, e -> e));

    private static final Map<String, RestockSubscriptionStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(RestockSubscriptionStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static RestockSubscriptionStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static RestockSubscriptionStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
