package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 售後稽核日誌操作者類型。獨立於 AccountType（前者表「稽核行為的觸發者」，後者表「使用者帳號類型」）。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum AfterSaleAuditActor implements CodeMeta {

    BUYER("BUYER", "8401", "買家", "買家行為觸發"),
    SELLER("SELLER", "8402", "賣家", "賣家行為觸發"),
    SYSTEM("SYSTEM", "8403", "系統", "排程/Worker 觸發"),
    ADMIN("ADMIN", "8404", "管理員", "平台管理員/ops 行為觸發（含 retry API、申訴裁決等）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    AfterSaleAuditActor(String code, String value, String label, String description) {
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

    private static final Map<String, AfterSaleAuditActor> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(AfterSaleAuditActor::code, e -> e));

    private static final Map<String, AfterSaleAuditActor> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(AfterSaleAuditActor::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static AfterSaleAuditActor fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static AfterSaleAuditActor fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
