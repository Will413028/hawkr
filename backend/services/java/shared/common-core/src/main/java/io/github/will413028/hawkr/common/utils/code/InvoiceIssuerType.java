package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 開票主體的類型。通用 issuer→recipient 模型的一半。**兩個值 Phase 1 都可用**：SELLER 走自動線（訂單付款成功）與 ADM 手動；PLATFORM 走 ADM 手動開票（端點 57／畫面 A11），涵蓋平台代製收據、平台對賣家的服務費、平台對買家的訂閱費、平台對非會員的對外收費。PLATFORM 的自動線（週期結算／訂閱週期）為 Phase 2。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum InvoiceIssuerType implements CodeMeta {

    SELLER("SELLER", "SELLER", "賣家", "issuer_ref_id = sellers.store_id（store surrogate，非 account_id）"),
    PLATFORM("PLATFORM", "PLATFORM", "平台", "issuer_ref_id 恆為 0。用於平台代製收據（PLATFORM_RECEIPT）**以及平台以自己名義開立的發票**（對賣家的服務費／對買家的訂閱費／對非會員的對外收費）；Phase 1 由 ADM 手動觸發");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    InvoiceIssuerType(String code, String value, String label, String description) {
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

    private static final Map<String, InvoiceIssuerType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceIssuerType::code, e -> e));

    private static final Map<String, InvoiceIssuerType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceIssuerType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static InvoiceIssuerType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static InvoiceIssuerType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
