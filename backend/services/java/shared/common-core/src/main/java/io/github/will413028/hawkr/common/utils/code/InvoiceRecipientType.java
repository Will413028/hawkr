package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 受票主體（買受人）的類型。通用 issuer→recipient 模型的另一半。`recipient_ref_id` 的語意隨本值改變，三者必須成對解讀。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum InvoiceRecipientType implements CodeMeta {

    BUYER("BUYER", "BUYER", "買家", "recipient_ref_id = buyers.buyer_id（buyer surrogate，非 account_id）"),
    SELLER("SELLER", "SELLER", "賣家", "recipient_ref_id = sellers.store_id。此時賣家是進項憑證持有人，不是開票人"),
    EXTERNAL("EXTERNAL", "EXTERNAL", "非平台會員", "recipient_ref_id 恆為 NULL；抬頭／統編／email 只存在於本文件的快照欄位");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    InvoiceRecipientType(String code, String value, String label, String description) {
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

    private static final Map<String, InvoiceRecipientType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceRecipientType::code, e -> e));

    private static final Map<String, InvoiceRecipientType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceRecipientType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static InvoiceRecipientType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static InvoiceRecipientType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
