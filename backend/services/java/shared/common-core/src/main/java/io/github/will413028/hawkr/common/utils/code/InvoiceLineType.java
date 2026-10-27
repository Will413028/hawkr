package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 發票品項行的種類。折扣以**負數品項行**表達而非調整單價（AMEGO 官方範例即以負數明細表示折抵），因此對每一行都成立同一條算式 net_amount = unit_price × quantity。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum InvoiceLineType implements CodeMeta {

    PRODUCT("PRODUCT", "PRODUCT", "商品", "對應 order_items 一列；net_amount >= 0"),
    SHIPPING("SHIPPING", "SHIPPING", "運費", "併入同一張發票的銷售額；net_amount >= 0"),
    DISCOUNT("DISCOUNT", "DISCOUNT", "折抵", "優惠券等折抵；net_amount < 0（由 chk_invline_sign 強制）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    InvoiceLineType(String code, String value, String label, String description) {
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

    private static final Map<String, InvoiceLineType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceLineType::code, e -> e));

    private static final Map<String, InvoiceLineType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceLineType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static InvoiceLineType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static InvoiceLineType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
