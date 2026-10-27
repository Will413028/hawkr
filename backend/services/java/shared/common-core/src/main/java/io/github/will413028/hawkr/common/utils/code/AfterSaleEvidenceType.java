package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 售後證據類型。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum AfterSaleEvidenceType implements CodeMeta {

    PHOTO("PHOTO", "8601", "照片", "證據照片（買家申請/補件時上傳）"),
    INSPECTION_PHOTO("INSPECTION_PHOTO", "8602", "驗貨照片", "賣家確認收貨時上傳的驗貨照片（支持驗貨扣款）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    AfterSaleEvidenceType(String code, String value, String label, String description) {
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

    private static final Map<String, AfterSaleEvidenceType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(AfterSaleEvidenceType::code, e -> e));

    private static final Map<String, AfterSaleEvidenceType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(AfterSaleEvidenceType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static AfterSaleEvidenceType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static AfterSaleEvidenceType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
