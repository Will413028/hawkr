package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 優惠券發放目標類型
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum CouponTargetType implements CodeMeta {

    ALL("ALL", "1", "全體買家", "發券時已註冊的所有買家"),
    MEMBER("MEMBER", "2", "賣家會員", "與賣家有完成交易紀錄的買家"),
    DESIGNATED("DESIGNATED", "3", "指定買家", "賣家手動指定的買家名單"),
    COMPENSATION("COMPENSATION", "4", "補償券", "售後/退款補償發放");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    CouponTargetType(String code, String value, String label, String description) {
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

    private static final Map<String, CouponTargetType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(CouponTargetType::code, e -> e));

    private static final Map<String, CouponTargetType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(CouponTargetType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static CouponTargetType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static CouponTargetType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
