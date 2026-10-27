package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * reconcile rescue 動作結果。獨立於 PaymentStatus.REFUNDED 命名空間（前者表「rescue 程序的執行結果」，後者表「交易最終狀態」）。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum PaymentRescueAction implements CodeMeta {

    REFUNDED("REFUNDED", "8901", "已退款", "信用卡同步退款完成"),
    REFUND_PENDING_MANUAL("REFUND_PENDING_MANUAL", "8902", "待人工退款", "ATM 退款待人工頁面版完成");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    PaymentRescueAction(String code, String value, String label, String description) {
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

    private static final Map<String, PaymentRescueAction> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PaymentRescueAction::code, e -> e));

    private static final Map<String, PaymentRescueAction> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PaymentRescueAction::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static PaymentRescueAction fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static PaymentRescueAction fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
