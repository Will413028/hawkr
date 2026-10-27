package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 賣家**自述**的開票義務。⚠️ 這是自述不是判定依據——判定結果在 review_status（APPROVED_INVOICE_REQUIRED／APPROVED_INVOICE_EXEMPT）。兩者不一致是正常的，ADM 審核時要看得到差異。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum SellerTaxObligation implements CodeMeta {

    REQUIRED("REQUIRED", "REQUIRED", "應開統一發票", "賣家自述已辦稅籍登記且應開立統一發票"),
    EXEMPT("EXEMPT", "EXEMPT", "免用統一發票", "賣家自述屬免用統一發票範圍"),
    UNKNOWN("UNKNOWN", "UNKNOWN", "尚未填答", "預設值；DRAFT 階段尚未選擇");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    SellerTaxObligation(String code, String value, String label, String description) {
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

    private static final Map<String, SellerTaxObligation> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(SellerTaxObligation::code, e -> e));

    private static final Map<String, SellerTaxObligation> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(SellerTaxObligation::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static SellerTaxObligation fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static SellerTaxObligation fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
