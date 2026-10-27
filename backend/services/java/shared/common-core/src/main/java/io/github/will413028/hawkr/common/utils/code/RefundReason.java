package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 退款/退貨原因
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum RefundReason implements CodeMeta {

    DEFECTIVE("DEFECTIVE", "5001", "商品瑕疵", "商品本身問題"),
    WRONG_ITEM("WRONG_ITEM", "5002", "寄錯商品", "出貨錯誤"),
    NOT_AS_DESCRIBED("NOT_AS_DESCRIBED", "5003", "與描述不符", "與商品頁描述不同"),
    CHANGE_OF_MIND("CHANGE_OF_MIND", "5004", "改變心意", "個人因素"),
    OTHER("OTHER", "5005", "其他原因", "其他退款原因");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    RefundReason(String code, String value, String label, String description) {
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

    private static final Map<String, RefundReason> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(RefundReason::code, e -> e));

    private static final Map<String, RefundReason> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(RefundReason::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static RefundReason fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static RefundReason fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
