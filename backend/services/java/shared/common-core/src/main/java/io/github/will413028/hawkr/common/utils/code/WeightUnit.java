package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 商品重量單位
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum WeightUnit implements CodeMeta {

    KG("KG", "3301", "公斤", "千克"),
    CATTY("CATTY", "3302", "台斤", "台斤=600公克"),
    GRAM("GRAM", "3303", "公克", "公克");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    WeightUnit(String code, String value, String label, String description) {
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

    private static final Map<String, WeightUnit> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(WeightUnit::code, e -> e));

    private static final Map<String, WeightUnit> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(WeightUnit::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static WeightUnit fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static WeightUnit fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
