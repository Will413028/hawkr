package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 賣家報的 `totalAmount` 是否已含稅（#2538）。⛔ 這是**申報**不是計算依據：server 只驗 `total = goods + shipping + tax`，不依本欄反推或改寫任何金額。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProcurementTaxInclusionBasis implements CodeMeta {

    TAX_INCLUDED("TAX_INCLUDED", "TAX_INCLUDED", "含稅", "`totalAmount` 已含 `taxAmount`"),
    TAX_EXCLUDED("TAX_EXCLUDED", "TAX_EXCLUDED", "未稅", "`taxAmount` 另計，仍計入 `totalAmount` 的恆等式");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProcurementTaxInclusionBasis(String code, String value, String label, String description) {
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

    private static final Map<String, ProcurementTaxInclusionBasis> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementTaxInclusionBasis::code, e -> e));

    private static final Map<String, ProcurementTaxInclusionBasis> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementTaxInclusionBasis::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProcurementTaxInclusionBasis fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProcurementTaxInclusionBasis fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
