package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 官方行情漲跌比較的期間定義；不以休市日前一個日曆日補值。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum PricebookChangeBasis implements CodeMeta {

    LATEST_VS_PREVIOUS_OBSERVED_TRADE_DATE("LATEST_VS_PREVIOUS_OBSERVED_TRADE_DATE", "latest_vs_previous_observed_trade_date", "最近兩個有效成交日", "最新有效成交日與前一有效成交日的價格差異");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    PricebookChangeBasis(String code, String value, String label, String description) {
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

    private static final Map<String, PricebookChangeBasis> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PricebookChangeBasis::code, e -> e));

    private static final Map<String, PricebookChangeBasis> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PricebookChangeBasis::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static PricebookChangeBasis fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static PricebookChangeBasis fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
