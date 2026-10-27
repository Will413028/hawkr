package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 買家指定的發票歸戶方式。⚠️ **本值域不是 AMEGO 的 CarrierType**：AMEGO 的線上值（3J0002／CQ0001／amego）存在 `invoice_documents.carrier_type` 與 `buyer_invoice_preferences.carrier_type`，兩者的對映是純函式（invoice-service internal/domain/carrier.go），不放進本 enum。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum InvoiceCarrierType implements CodeMeta {

    MOBILE_BARCODE("MOBILE_BARCODE", "MOBILE_BARCODE", "手機條碼", "AMEGO CarrierType = 3J0002；載具值為憑證等級，密文儲存且不得落 log"),
    CITIZEN_CERT("CITIZEN_CERT", "CITIZEN_CERT", "自然人憑證條碼", "AMEGO CarrierType = CQ0001；同上"),
    TAX_ID("TAX_ID", "TAX_ID", "打統編（三聯式）", "AMEGO CarrierType 為空；需同時帶買方抬頭"),
    DONATION("DONATION", "DONATION", "捐贈", "AMEGO CarrierType 為空；捐贈碼 3~7 碼純數字");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    InvoiceCarrierType(String code, String value, String label, String description) {
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

    private static final Map<String, InvoiceCarrierType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceCarrierType::code, e -> e));

    private static final Map<String, InvoiceCarrierType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceCarrierType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static InvoiceCarrierType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static InvoiceCarrierType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
