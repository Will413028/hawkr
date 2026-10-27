package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 對 provider 的一次操作的種類。六種共用同一套 InvoiceOperationStatus 值域。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum InvoiceOperationType implements CodeMeta {

    ISSUE("ISSUE", "ISSUE", "開立", "AMEGO f0401；PLATFORM_RECEIPT 也走本類型（本地產生 PDF，不打 AMEGO）"),
    QUERY("QUERY", "QUERY", "查詢對帳", "AMEGO invoice_query；只有兩個建立者：上傳狀態輪詢拿到 -1、以及收到 3040171"),
    VOID("VOID", "VOID", "作廢發票", "AMEGO f0501"),
    ALLOWANCE("ALLOWANCE", "ALLOWANCE", "開立折讓單", "AMEGO g0401"),
    REISSUE("REISSUE", "REISSUE", "作廢後重開", "作廢成功後開立新發票"),
    VOID_ALLOWANCE("VOID_ALLOWANCE", "VOID_ALLOWANCE", "作廢折讓單", "AMEGO g0501");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    InvoiceOperationType(String code, String value, String label, String description) {
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

    private static final Map<String, InvoiceOperationType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceOperationType::code, e -> e));

    private static final Map<String, InvoiceOperationType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceOperationType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static InvoiceOperationType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static InvoiceOperationType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
