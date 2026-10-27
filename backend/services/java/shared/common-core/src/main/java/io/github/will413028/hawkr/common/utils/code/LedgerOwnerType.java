package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 雙分錄帳本的科目歸屬主體。🔴 **落地順序**：CodeMeta → `generate.sh --all` → DDL 的 CHECK → 程式碼；反過來做的話 `ledger_entries` 的 INSERT 會在 CHECK 上失敗，而症狀是「撥款流程 500」，不會有人聯想到是碼表沒生成。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum LedgerOwnerType implements CodeMeta {

    PLATFORM("PLATFORM", "PLATFORM", "平台", "`owner_ref_id` 恆為 0"),
    SELLER("SELLER", "SELLER", "賣家", "`owner_ref_id` = `sellers.store_id`（store surrogate，非 account_id）"),
    PROVIDER("PROVIDER", "PROVIDER", "物流商", "`owner_ref_id` = provider 識別；用於物流成本與對帳");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    LedgerOwnerType(String code, String value, String label, String description) {
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

    private static final Map<String, LedgerOwnerType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(LedgerOwnerType::code, e -> e));

    private static final Map<String, LedgerOwnerType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(LedgerOwnerType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static LedgerOwnerType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static LedgerOwnerType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
