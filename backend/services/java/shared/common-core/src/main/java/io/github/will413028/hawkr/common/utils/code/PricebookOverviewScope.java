package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 官方行情雷達的查詢範圍；Value 沿用官網雷達小寫契約。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum PricebookOverviewScope implements CodeMeta {

    ALL("ALL", "all", "全部品項", "依目前市場範圍列出 active 品項"),
    WATCHLIST("WATCHLIST", "watchlist", "追蹤清單", "依帳號保存的品項與市場身分列出");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    PricebookOverviewScope(String code, String value, String label, String description) {
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

    private static final Map<String, PricebookOverviewScope> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PricebookOverviewScope::code, e -> e));

    private static final Map<String, PricebookOverviewScope> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PricebookOverviewScope::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static PricebookOverviewScope fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static PricebookOverviewScope fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
