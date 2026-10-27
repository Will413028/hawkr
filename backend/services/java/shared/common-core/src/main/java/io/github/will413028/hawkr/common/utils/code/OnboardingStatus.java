package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 賣家物流建店狀態（PayUni 建店審核）
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum OnboardingStatus implements CodeMeta {

    PENDING_REVIEW("PENDING_REVIEW", "3601", "待審核", "建店申請已提交，等待 PayUni 審核"),
    ACTIVE("ACTIVE", "3602", "已啟用", "審核通過，可使用此配送方式"),
    REJECTED("REJECTED", "3603", "審核不通過", "PayUni 審核未通過"),
    CLOSED("CLOSED", "3604", "已關閉", "賣家主動關閉或被停用"),
    PERMANENTLY_CLOSED("PERMANENTLY_CLOSED", "3605", "永久關閉", "永久停用，不可重新開通");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    OnboardingStatus(String code, String value, String label, String description) {
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

    private static final Map<String, OnboardingStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(OnboardingStatus::code, e -> e));

    private static final Map<String, OnboardingStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(OnboardingStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static OnboardingStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static OnboardingStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
