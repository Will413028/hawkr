package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 稅務文件的種類。**三值，不多不少**（58 §5 `chk_invdoc_kind`）。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum InvoiceDocumentKind implements CodeMeta {

    B2C("B2C", "B2C", "二聯式（買受人為個人）", "BuyerIdentifier 固定 10 個 0；TaxAmount 一律 0；DetailVat=1（含稅）"),
    B2B("B2B", "B2B", "三聯式（買受人為營業人）", "買受人統編為 8 碼；DetailVat=1（含稅、明細與 B2C 同形），僅表頭拆稅——58 §5A.6.2 定版本設計永不送 DetailVat=0（未稅明細會讓 UnitPrice 變小數，與「金額一律整數元」正面衝突，且踩到一個官方未說明的捨入模式）"),
    RECEIPT("RECEIPT", "RECEIPT", "收據", "免用統一發票賣家的平台代製收據；不經 provider，無法定上傳期限");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    InvoiceDocumentKind(String code, String value, String label, String description) {
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

    private static final Map<String, InvoiceDocumentKind> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceDocumentKind::code, e -> e));

    private static final Map<String, InvoiceDocumentKind> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceDocumentKind::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static InvoiceDocumentKind fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static InvoiceDocumentKind fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
