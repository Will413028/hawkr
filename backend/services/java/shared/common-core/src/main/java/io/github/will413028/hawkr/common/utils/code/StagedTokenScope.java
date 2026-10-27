package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 多媒體上傳 Staged Token 用途範圍
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum StagedTokenScope implements CodeMeta {

    MEDIA_UPLOAD_SESSION("MEDIA_UPLOAD_SESSION", "7201", "MEDIA_UPLOAD_SESSION", "公開上傳會話 Token"),
    MEDIA_SECURE_UPLOAD("MEDIA_SECURE_UPLOAD", "7202", "MEDIA_SECURE_UPLOAD", "機敏上傳會話 Token");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    StagedTokenScope(String code, String value, String label, String description) {
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

    private static final Map<String, StagedTokenScope> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(StagedTokenScope::code, e -> e));

    private static final Map<String, StagedTokenScope> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(StagedTokenScope::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static StagedTokenScope fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static StagedTokenScope fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
