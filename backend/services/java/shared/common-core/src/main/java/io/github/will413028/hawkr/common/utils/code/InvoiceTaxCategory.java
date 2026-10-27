package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 品項行的課稅別。Phase 1 每一行都複製賣家的 default_tax_category（唯一合法來源是 ADM 依核定函所填的單一稅別），因此一張文件內必然同值。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum InvoiceTaxCategory implements CodeMeta {

    TAXABLE("TAXABLE", "1", "應稅", "稅率 5%；Phase 1 的預設值"),
    ZERO_RATED("ZERO_RATED", "2", "零稅率", "⚠️ AMEGO 另需 CustomsClearanceMark／ZeroTaxRateReason，Phase 1 送出前一律斷言擋下（理由碼 ZERO_RATE_NOT_SUPPORTED）"),
    TAX_EXEMPT("TAX_EXEMPT", "3", "免稅", "需以主管稽徵機關核定為準，不得由系統依商品名稱自動判定");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    InvoiceTaxCategory(String code, String value, String label, String description) {
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

    private static final Map<String, InvoiceTaxCategory> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceTaxCategory::code, e -> e));

    private static final Map<String, InvoiceTaxCategory> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceTaxCategory::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static InvoiceTaxCategory fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static InvoiceTaxCategory fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
