package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 舊契約的帳號類型：角色的 bitmask，管理員是 4。擁有者是 identity（遷移計畫 A2；ADR `2026-09-25-codemeta-retirement.md` 過渡第 1 步），值由 `internal/legacy` 從角色換算，本表只是投影，不得單獨修改（`TestLegacyAccountTypeMatchesCodeMeta`）
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum AccountType implements CodeMeta {

    BUYER("BUYER", "1", "買家", "一般消費者"),
    SELLER("SELLER", "2", "賣家", "商家"),
    SELLER_AND_BUYER("SELLER_AND_BUYER", "3", "買賣雙重身份", "同時擁有買家與賣家權限"),
    ADMIN("ADMIN", "4", "管理員", "系統管理員");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    AccountType(String code, String value, String label, String description) {
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

    private static final Map<String, AccountType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(AccountType::code, e -> e));

    private static final Map<String, AccountType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(AccountType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static AccountType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static AccountType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
