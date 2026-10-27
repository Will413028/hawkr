package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 對外付款狀態語意（簡化版，供前端顯示）
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum PaymentStatus implements CodeMeta {

    UNPAID("UNPAID", "1101", "未付款", "尚未付款"),
    SUCCESS("SUCCESS", "1102", "付款成功", "付款完成"),
    FAILED("FAILED", "1103", "付款失敗", "付款未成功"),
    REFUNDED("REFUNDED", "1104", "已退款", "款項已退回"),
    PARTIAL_REFUNDED("PARTIAL_REFUNDED", "1105", "部分退款", "僅退回部分金額");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    PaymentStatus(String code, String value, String label, String description) {
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

    private static final Map<String, PaymentStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PaymentStatus::code, e -> e));

    private static final Map<String, PaymentStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PaymentStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static PaymentStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static PaymentStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
