package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 商品名稱重複的分類，UI 區分「同 job 內重名（必須改）」與「與既有商品重名（可勾選 allow）」。詳見 [40 §5.5 重名處理](../詳細設計書/40-商品匯入精靈服務（Product%20Import%20Service）詳細設計書.md)。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum DuplicateNameKind implements CodeMeta {

    IN_JOB("IN_JOB", "IN_JOB", "同 job 內重名", "配 DUPLICATE_NAME_IN_JOB；必須改名解決"),
    VS_EXISTING("VS_EXISTING", "VS_EXISTING", "與既有商品重名", "配 DUPLICATE_NAME_VS_EXISTING；可勾選 allow_duplicate_name 通過");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    DuplicateNameKind(String code, String value, String label, String description) {
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

    private static final Map<String, DuplicateNameKind> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(DuplicateNameKind::code, e -> e));

    private static final Map<String, DuplicateNameKind> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(DuplicateNameKind::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static DuplicateNameKind fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static DuplicateNameKind fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
