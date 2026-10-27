package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 退款執行方式
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum RefundMethod implements CodeMeta {

    CASH("CASH", "1", "原路退款", "退回原支付管道"),
    COUPON("COUPON", "2", "優惠券退款", "發放等值優惠券");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    RefundMethod(String code, String value, String label, String description) {
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

    private static final Map<String, RefundMethod> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(RefundMethod::code, e -> e));

    private static final Map<String, RefundMethod> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(RefundMethod::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static RefundMethod fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static RefundMethod fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
