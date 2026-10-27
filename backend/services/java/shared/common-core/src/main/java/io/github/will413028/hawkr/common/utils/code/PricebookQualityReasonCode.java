package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * issue #2249
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum PricebookQualityReasonCode implements CodeMeta {

    PARTIAL_VARIANT_COVERAGE("PARTIAL_VARIANT_COVERAGE", "PARTIAL_VARIANT_COVERAGE", "子品項覆蓋不全", "大類聚合 status=partial（有效子品項數 < 有模型覆蓋的子品項數）"),
    VOLUME_WEIGHT_FALLBACK("VOLUME_WEIGHT_FALLBACK", "VOLUME_WEIGHT_FALLBACK", "退化為等權平均", "成交量權重全為 0，聚合退化成等權重算術平均"),
    INSUFFICIENT_MARKET_SUPPORT("INSUFFICIENT_MARKET_SUPPORT", "INSUFFICIENT_MARKET_SUPPORT", "支援市場數不足", "支援市場數（MarketSupportCount）介於 1 與門檻值之間");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    PricebookQualityReasonCode(String code, String value, String label, String description) {
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

    private static final Map<String, PricebookQualityReasonCode> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PricebookQualityReasonCode::code, e -> e));

    private static final Map<String, PricebookQualityReasonCode> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PricebookQualityReasonCode::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static PricebookQualityReasonCode fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static PricebookQualityReasonCode fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
