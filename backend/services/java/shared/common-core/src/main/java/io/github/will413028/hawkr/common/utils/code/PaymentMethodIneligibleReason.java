package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 付款方式因「訂單金額不符該方式合法區間」而不可用的理由碼。由 order-service 在結帳預覽 / 訂單詳情計算 `availablePaymentMethods` 時填入（`eligible=false` 才有值）。前端依此碼的 label 顯示禁用理由（**通用文案、不帶具體數字**；限制值的 SSOT 留後端 [PaymentMethodLimitRules](../詳細設計書/20-結帳模組（Checkout%20Module）詳細設計書.md)，前端零複製）。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum PaymentMethodIneligibleReason implements CodeMeta {

    AMOUNT_BELOW_MIN("AMOUNT_BELOW_MIN", "2101", "金額未達此付款方式下限", "訂單金額低於該付款方式允許的最小金額（具體限制值見 PaymentMethodLimitRules）"),
    AMOUNT_ABOVE_MAX("AMOUNT_ABOVE_MAX", "2102", "金額超過此付款方式上限", "訂單金額高於該付款方式允許的最大金額（具體限制值見 PaymentMethodLimitRules）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    PaymentMethodIneligibleReason(String code, String value, String label, String description) {
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

    private static final Map<String, PaymentMethodIneligibleReason> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PaymentMethodIneligibleReason::code, e -> e));

    private static final Map<String, PaymentMethodIneligibleReason> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PaymentMethodIneligibleReason::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static PaymentMethodIneligibleReason fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static PaymentMethodIneligibleReason fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
