package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 結帳當下決定「這張訂單的發票歸戶方式從哪裡來」。**跨語言值域**：order-service（Java）寫入、invoice-service（Go）讀取並據以分支，兩端都必須用生成的常數而非字串字面值。USE_BUYER_DEFAULT 刻意不在結帳當下解析（那會在 hot path 多打一次 gRPC），由 invoice-service 在建立文件時解析，**只有兩步**：買家指定的預設（`isDefault=true`）→ 無載具發票。中間的 amego 載具兩步已於 D6／D7 移除（58 §6.1.2(b)）。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum InvoiceCarrierSelectionMode implements CodeMeta {

    USE_BUYER_DEFAULT("USE_BUYER_DEFAULT", "USE_BUYER_DEFAULT", "使用買家預設", "由 invoice-service 於開票時解析，**只有兩步**：買家指定的預設（isDefault=true）→ 無載具發票。amego 載具兩步已於 D6／D7 移除（58 §6.1.2(b)），InvoiceCarrierType 也沒有能承載它的值"),
    EXPLICIT_PREFERENCE("EXPLICIT_PREFERENCE", "EXPLICIT_PREFERENCE", "指定常用資料", "帶 buyer_invoice_preference_id；invoice-service 解析時**必驗歸屬**（IDOR）"),
    NONE("NONE", "NONE", "無載具／打統編", "B2B 一律為此值"),
    DONATION("DONATION", "DONATION", "捐贈", "帶 donation_code");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    InvoiceCarrierSelectionMode(String code, String value, String label, String description) {
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

    private static final Map<String, InvoiceCarrierSelectionMode> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceCarrierSelectionMode::code, e -> e));

    private static final Map<String, InvoiceCarrierSelectionMode> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceCarrierSelectionMode::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static InvoiceCarrierSelectionMode fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static InvoiceCarrierSelectionMode fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
