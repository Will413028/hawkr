package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 官方行情最近兩個有效成交日的 NUMERIC 漲跌方向；Value 沿用官網契約。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum PricebookChangeDirection implements CodeMeta {

    UP("UP", "up", "上漲", "本期成交均價高於前期"),
    DOWN("DOWN", "down", "下跌", "本期成交均價低於前期"),
    FLAT("FLAT", "flat", "持平", "本期與前期成交均價相同"),
    UNKNOWN("UNKNOWN", "unknown", "無法比較", "尚無兩個有效成交日可比較");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    PricebookChangeDirection(String code, String value, String label, String description) {
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

    private static final Map<String, PricebookChangeDirection> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PricebookChangeDirection::code, e -> e));

    private static final Map<String, PricebookChangeDirection> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PricebookChangeDirection::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static PricebookChangeDirection fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static PricebookChangeDirection fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
