package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * Pricebook 服務資料新鮮度分級。issue #2249
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum FreshnessLabel implements CodeMeta {

    FRESH("FRESH", "fresh", "新鮮", "發布後在 FreshWithin 門檻內（預設 36h）"),
    STALE("STALE", "stale", "已過期", "介於 FreshWithin 與 StaleWithin 之間（預設 36–72h）"),
    UNAVAILABLE("UNAVAILABLE", "unavailable", "無法取得", "超過 StaleWithin（預設 72h），或完全沒有可用觀測");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    FreshnessLabel(String code, String value, String label, String description) {
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

    private static final Map<String, FreshnessLabel> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(FreshnessLabel::code, e -> e));

    private static final Map<String, FreshnessLabel> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(FreshnessLabel::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static FreshnessLabel fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static FreshnessLabel fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
