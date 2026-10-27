package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 優惠券實例使用狀態
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum CouponInstanceStatus implements CodeMeta {

    UNUSED("UNUSED", "0", "未使用", "尚未使用"),
    RESERVED("RESERVED", "1", "已鎖定", "結帳鎖定中"),
    USED("USED", "2", "已使用", "已核銷"),
    EXPIRED("EXPIRED", "3", "已過期", "超過有效期限"),
    REVOKED("REVOKED", "4", "已收回", "已被管理員收回"),
    ANNUL("ANNUL", "5", "已作廢", "已作廢（退款等）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    CouponInstanceStatus(String code, String value, String label, String description) {
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

    private static final Map<String, CouponInstanceStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(CouponInstanceStatus::code, e -> e));

    private static final Map<String, CouponInstanceStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(CouponInstanceStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static CouponInstanceStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static CouponInstanceStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
