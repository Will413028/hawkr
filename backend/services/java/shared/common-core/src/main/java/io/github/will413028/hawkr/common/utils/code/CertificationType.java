package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 農產品認證類型
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum CertificationType implements CodeMeta {

    ORGANIC("ORGANIC", "3701", "有機農產品", "有機農業認證"),
    TAP("TAP", "3702", "產銷履歷", "產銷履歷追溯"),
    CAS("CAS", "3703", "CAS 優良農產品", "CAS 認證"),
    HACCP("HACCP", "3704", "HACCP 認證", "HACCP 食品安全認證"),
    ISO("ISO", "3705", "ISO 認證", "ISO 國際認證"),
    GMP("GMP", "3706", "GMP 認證", "GMP 良好製造規範");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    CertificationType(String code, String value, String label, String description) {
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

    private static final Map<String, CertificationType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(CertificationType::code, e -> e));

    private static final Map<String, CertificationType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(CertificationType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static CertificationType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static CertificationType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
