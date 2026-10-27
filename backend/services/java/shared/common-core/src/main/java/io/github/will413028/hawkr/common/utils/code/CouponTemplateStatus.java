package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 優惠券模板生命週期狀態
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum CouponTemplateStatus implements CodeMeta {

    DRAFT("DRAFT", "0", "草稿", "尚未發布"),
    ACTIVE("ACTIVE", "1", "進行中", "可領取/使用"),
    INACTIVE("INACTIVE", "2", "已停用", "暫停使用"),
    EXPIRED("EXPIRED", "3", "已過期", "超過有效期限"),
    REVOKED("REVOKED", "4", "已收回", "已被管理員收回");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    CouponTemplateStatus(String code, String value, String label, String description) {
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

    private static final Map<String, CouponTemplateStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(CouponTemplateStatus::code, e -> e));

    private static final Map<String, CouponTemplateStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(CouponTemplateStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static CouponTemplateStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static CouponTemplateStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
