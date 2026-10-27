package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 支付事件 MQ 類型（Value 同時作為 routing key）
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum PaymentEventType implements CodeMeta {

    PAYMENT_SUCCESS("PAYMENT_SUCCESS", "payment.success", "付款成功", "交易付款成功"),
    PAYMENT_FAILED("PAYMENT_FAILED", "payment.failed", "付款失敗", "交易付款失敗"),
    PAYMENT_EXPIRED("PAYMENT_EXPIRED", "payment.expired", "付款逾期", "交易逾期未付款"),
    PAYMENT_CLOSED("PAYMENT_CLOSED", "payment.closed", "交易關閉", "交易主動關閉"),
    PAYMENT_CONFLICT("PAYMENT_CONFLICT", "payment.conflict", "狀態衝突", "同步與 Webhook 結果不一致"),
    REFUND_SUCCEEDED("REFUND_SUCCEEDED", "refund.succeeded", "退款成功", "退款處理成功"),
    REFUND_FAILED("REFUND_FAILED", "refund.failed", "退款失敗", "退款處理失敗"),
    PAYMENT_RESCUE_REFUND_INITIATED("PAYMENT_RESCUE_REFUND_INITIATED", "payment.rescue.refund_initiated", "rescue 退款已發起", "reconcile 在 grace 視窗內偵測到 EXPIRED 但 PayUni 已成功，自動觸發退款"),
    PAYMENT_RESCUE_REFUND_COMPLETED("PAYMENT_RESCUE_REFUND_COMPLETED", "payment.rescue.refund_completed", "rescue 退款完成", "rescue 退款已成功完成（PayUni close API 回成功 + 本地 row CLOSED），需通知使用者");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    PaymentEventType(String code, String value, String label, String description) {
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

    private static final Map<String, PaymentEventType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PaymentEventType::code, e -> e));

    private static final Map<String, PaymentEventType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PaymentEventType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static PaymentEventType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static PaymentEventType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
