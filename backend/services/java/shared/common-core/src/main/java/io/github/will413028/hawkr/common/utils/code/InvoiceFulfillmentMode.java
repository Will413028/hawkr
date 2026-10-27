package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 這張文件由誰、以什麼方式履行開立義務。在 order.paid 消費當下由決策樹判定一次並寫死，**不隨賣家後續狀態變動回溯改寫**。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum InvoiceFulfillmentMode implements CodeMeta {

    AMEGO_AUTO("AMEGO_AUTO", "AMEGO_AUTO", "平台自動開立", "賣家已核定應開統一發票、provider 連線可用、且已過自動化切點"),
    SELLER_SELF_ISSUE("SELLER_SELF_ISSUE", "SELLER_SELF_ISSUE", "賣家自行開立", "賣家已具結待審；document 恆為 NOT_APPLICABLE，回報號碼後轉 RESOLVED_EXTERNALLY"),
    PLATFORM_RECEIPT("PLATFORM_RECEIPT", "PLATFORM_RECEIPT", "平台代製收據", "免用統一發票賣家；本地產生 PDF，不打 AMEGO"),
    LEGACY_MANUAL("LEGACY_MANUAL", "LEGACY_MANUAL", "切點前／未串接（人工）", "自動化切點前的訂單、或賣家資格未核准／連線不可用");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    InvoiceFulfillmentMode(String code, String value, String label, String description) {
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

    private static final Map<String, InvoiceFulfillmentMode> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceFulfillmentMode::code, e -> e));

    private static final Map<String, InvoiceFulfillmentMode> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceFulfillmentMode::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static InvoiceFulfillmentMode fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static InvoiceFulfillmentMode fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
