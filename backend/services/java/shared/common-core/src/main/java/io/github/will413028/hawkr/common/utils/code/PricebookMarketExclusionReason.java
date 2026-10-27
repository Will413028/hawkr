package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * exact crop 全市場 forecast aggregation.excludedMarkets 的額外排除原因。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum PricebookMarketExclusionReason implements CodeMeta {

    NO_POSITIVE_VOLUME("NO_POSITIVE_VOLUME", "no_positive_volume", "無正成交量權重", "其他可用市場具有正成交量，而此市場近期權重為零或缺失");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    PricebookMarketExclusionReason(String code, String value, String label, String description) {
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

    private static final Map<String, PricebookMarketExclusionReason> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PricebookMarketExclusionReason::code, e -> e));

    private static final Map<String, PricebookMarketExclusionReason> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PricebookMarketExclusionReason::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static PricebookMarketExclusionReason fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static PricebookMarketExclusionReason fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
