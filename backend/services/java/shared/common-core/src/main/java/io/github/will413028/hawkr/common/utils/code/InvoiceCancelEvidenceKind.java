package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 「這張票確實沒有開出去」的證據來源（**4 值**，值域逐字取自 58 §6.4）。🔴 **它不是 InvoiceCancelReason 的細分**：後者答「為什麼不做了」，本值域答「憑什麼相信沒開出去」。兩者同時存在才構成 §6.4 要求的四樣紀律（門檻／證據／出口／回頭路）中的「證據」。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum InvoiceCancelEvidenceKind implements CodeMeta {

    PROVIDER_QUERY_71("PROVIDER_QUERY_71", "PROVIDER_QUERY_71", "AMEGO 查無此筆", "invoice_query 回 code=71。**唯一可由系統自動採信的證據**（58 §7.5.4 J24 由 INV.rc 自動落地，無第二人）"),
    PROVIDER_LIST_ABSENT("PROVIDER_LIST_ABSENT", "PROVIDER_LIST_ABSENT", "列表比對後仍無", "invoice_list 翻完該區間分頁仍未比對到。同屬 provider 直接作證"),
    MOF_PLATFORM_SCREENSHOT("MOF_PLATFORM_SCREENSHOT", "MOF_PLATFORM_SCREENSHOT", "財政部平台查詢截圖", "賣家提供。**需人判讀 ⇒ 只能走雙人確認（J28）**；截圖走 media 私有資產，列為 5 年保存例外"),
    AMEGO_RECONCILE_FILE("AMEGO_RECONCILE_FILE", "AMEGO_RECONCILE_FILE", "光貿對帳檔", "對帳檔比對結果。同需人判讀，只能走 J28");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    InvoiceCancelEvidenceKind(String code, String value, String label, String description) {
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

    private static final Map<String, InvoiceCancelEvidenceKind> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceCancelEvidenceKind::code, e -> e));

    private static final Map<String, InvoiceCancelEvidenceKind> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceCancelEvidenceKind::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static InvoiceCancelEvidenceKind fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static InvoiceCancelEvidenceKind fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
