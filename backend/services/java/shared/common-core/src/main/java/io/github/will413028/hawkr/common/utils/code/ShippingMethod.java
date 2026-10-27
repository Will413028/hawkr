package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 運送方式
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ShippingMethod implements CodeMeta {

    TAQBIN("TAQBIN", "3001", "黑貓宅急便", "宅配到府"),
    SEVEN_ELEVEN_STORE_PICKUP("SEVEN_ELEVEN_STORE_PICKUP", "3002", "7-11店到店", "超商門市取貨"),
    UBER_INSTANT("UBER_INSTANT", "3003", "Uber 即時配送", "即時外送到府"),
    SEVEN_ELEVEN_BULK_WAREHOUSE("SEVEN_ELEVEN_BULK_WAREHOUSE", "3004", "7-11大宗寄倉", "賣家批次交寄至指定物流中心，買家至門市取貨（B2C）。與 3002 走同一組 PayUni 端點，差別在 LgsType"),
    MERCHANT_DELIVERY("MERCHANT_DELIVERY", "3005", "商家自行配送", "賣家自有車隊或承運商配送，無外部 provider API");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ShippingMethod(String code, String value, String label, String description) {
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

    private static final Map<String, ShippingMethod> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ShippingMethod::code, e -> e));

    private static final Map<String, ShippingMethod> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ShippingMethod::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ShippingMethod fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ShippingMethod fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
