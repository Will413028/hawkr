package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 系統通用狀態碼，適用於無專屬狀態定義的資源
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum CommonStatus implements CodeMeta {

    INACTIVE("INACTIVE", "9003", "未啟用", "已存在但暫不開放使用；在分類管理場景表示軟刪除（仍佔用 unique 名稱位置）"),
    ACTIVE("ACTIVE", "9004", "啟用", "正常可用狀態");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    CommonStatus(String code, String value, String label, String description) {
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

    private static final Map<String, CommonStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(CommonStatus::code, e -> e));

    private static final Map<String, CommonStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(CommonStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static CommonStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static CommonStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
