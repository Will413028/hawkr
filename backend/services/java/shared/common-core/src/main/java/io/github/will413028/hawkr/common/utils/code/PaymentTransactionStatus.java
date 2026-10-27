package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 支付交易內部狀態（Payment Service 內部使用）
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum PaymentTransactionStatus implements CodeMeta {

    INIT("INIT", "0", "待付款", "交易建立完成，等待付款"),
    PAID("PAID", "1", "已付款", "付款成功"),
    FAILED("FAILED", "2", "付款失敗", "付款未成功"),
    EXPIRED("EXPIRED", "3", "已逾期", "逾期未付款"),
    CONFLICT("CONFLICT", "4", "狀態衝突", "同步與 Webhook 結果不一致，需人工介入"),
    CLOSED("CLOSED", "5", "已關閉", "交易主動關閉"),
    CLOSED_BY_SWITCH("CLOSED_BY_SWITCH", "6", "因切換付款方式而關閉", "使用者主動切換付款方式造成的關閉，保留審計軌跡（見 [05-支付服務設計書 §2A.3](../詳細設計書/05-支付服務（Payment%20Service）詳細設計書.md)）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    PaymentTransactionStatus(String code, String value, String label, String description) {
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

    private static final Map<String, PaymentTransactionStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PaymentTransactionStatus::code, e -> e));

    private static final Map<String, PaymentTransactionStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PaymentTransactionStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static PaymentTransactionStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static PaymentTransactionStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
