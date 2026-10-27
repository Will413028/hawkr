package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 優惠券稽核日誌動作類型。獨立命名空間——其 REVOKE / ANNUL / EXPIRE 與 CouponInstanceStatus 的 REVOKED / ANNUL / EXPIRED 字面雖相同但語意不同（前者「進行的動作」，後者「實例的當前狀態」）。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum CouponRedemptionAction implements CodeMeta {

    ISSUE("ISSUE", "8301", "發放", "賣家/Admin 發券動作"),
    RECEIVE("RECEIVE", "8302", "領取", "買家領取動作"),
    RESERVE("RESERVE", "8303", "鎖定", "結帳階段鎖定"),
    CONSUME("CONSUME", "8304", "核銷", "結帳完成核銷"),
    RELEASE("RELEASE", "8305", "釋放", "鎖定解除（取消結帳）"),
    REVOKE("REVOKE", "8306", "收回", "賣家/Admin 收回"),
    ANNUL("ANNUL", "8307", "作廢", "退款等情況作廢"),
    EXPIRE("EXPIRE", "8308", "過期", "自動過期");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    CouponRedemptionAction(String code, String value, String label, String description) {
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

    private static final Map<String, CouponRedemptionAction> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(CouponRedemptionAction::code, e -> e));

    private static final Map<String, CouponRedemptionAction> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(CouponRedemptionAction::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static CouponRedemptionAction fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static CouponRedemptionAction fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
