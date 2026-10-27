package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 支付方式
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum PaymentMethod implements CodeMeta {

    CREDIT_CARD("CREDIT_CARD", "2001", "信用卡", "信用卡一次付清"),
    APPLE_PAY("APPLE_PAY", "2002", "Apple Pay", "Apple 行動支付"),
    GOOGLE_PAY("GOOGLE_PAY", "2003", "Google Pay", "Google 行動支付"),
    ATM("ATM", "2004", "ATM 轉帳", "銀行虛擬帳戶轉帳"),
    ZERO_AMOUNT("ZERO_AMOUNT", "2900", "零元訂單", "全額折抵後無需付款，不呼叫 PayUni 直接標 PAID（見 [05-支付服務設計書 §2A.1](../詳細設計書/05-支付服務（Payment%20Service）詳細設計書.md)）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    PaymentMethod(String code, String value, String label, String description) {
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

    private static final Map<String, PaymentMethod> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PaymentMethod::code, e -> e));

    private static final Map<String, PaymentMethod> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PaymentMethod::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static PaymentMethod fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static PaymentMethod fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
