package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 物流大類分類。獨立於 ShippingMethod（後者為具體物流商）：本枚舉表達「超商 C2C / 宅配到府」這個上層分類，供 onboarding 設定與 PayUni 對接時的分流判斷使用。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ShippingType implements CodeMeta {

    CVS_C2C("CVS_C2C", "3651", "超商店到店", "超商 C2C 取貨流程"),
    HOME_DELIVERY("HOME_DELIVERY", "3652", "宅配到府", "宅配 / 黑貓等到府配送"),
    MERCHANT_DELIVERY("MERCHANT_DELIVERY", "3654", "商家自送", "商家自有配送流程，對應運送方式 3005（RFQ 訂單的站外業者自行出貨）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ShippingType(String code, String value, String label, String description) {
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

    private static final Map<String, ShippingType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ShippingType::code, e -> e));

    private static final Map<String, ShippingType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ShippingType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ShippingType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ShippingType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
