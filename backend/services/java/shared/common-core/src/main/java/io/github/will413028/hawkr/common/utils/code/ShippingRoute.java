package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 運費計算路線（依寄送/收件區域組合）
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ShippingRoute implements CodeMeta {

    DOMESTIC("DOMESTIC", "3501", "本島互寄", "本島 → 本島（標準費率）"),
    TO_OFFSHORE("TO_OFFSHORE", "3502", "寄離島", "本島 → 離島（加價）"),
    FROM_OFFSHORE("FROM_OFFSHORE", "3503", "離島寄出", "離島 → 本島（加價）"),
    OFFSHORE_TO_OFFSHORE("OFFSHORE_TO_OFFSHORE", "3504", "離島互寄", "離島 → 離島（加價）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ShippingRoute(String code, String value, String label, String description) {
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

    private static final Map<String, ShippingRoute> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ShippingRoute::code, e -> e));

    private static final Map<String, ShippingRoute> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ShippingRoute::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ShippingRoute fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ShippingRoute fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
