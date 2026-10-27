package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 買家發票開立模式——**誰對買家開票**。訂單快照六欄之一。**擴充 58 號既有 issuer→recipient 模型，不另立一套**。🔴 與 `collection_model` 由 `chk_orders_invoice_collection` 綁死：開票 issuer 必須與收款主體一致，否則收款證據與開票證據對不起來。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum InvoiceModel implements CodeMeta {

    SELLER_TO_BUYER("SELLER_TO_BUYER", "SELLER_TO_BUYER", "賣家對買家開立", "賣家為開票人。**現況全部訂單皆為本值**"),
    PLATFORM_TO_BUYER("PLATFORM_TO_BUYER", "PLATFORM_TO_BUYER", "平台對買家開立", "平台為開票人（受託代銷）。必須搭配 `collection_model = PLATFORM_AGENT_MERCHANT`");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    InvoiceModel(String code, String value, String label, String description) {
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

    private static final Map<String, InvoiceModel> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceModel::code, e -> e));

    private static final Map<String, InvoiceModel> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceModel::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static InvoiceModel fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static InvoiceModel fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
