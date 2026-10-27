package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 一次稅務調整事件的種類。VOID 不產生新文件（把原文件打成 VOIDED）；ALLOWANCE 產生一張獨立的法定憑證，**但那張憑證落在 `invoice_adjustments` 不是 `invoice_documents`**（58 §5 T-B04③；`chk_invdoc_kind` 沒有 `ALLOWANCE`）。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum InvoiceAdjustmentType implements CodeMeta {

    VOID("VOID", "VOID", "作廢", "不產生新文件；AMEGO f0501，非同步"),
    ALLOWANCE("ALLOWANCE", "ALLOWANCE", "折讓", "產生一張獨立憑證（`invoice_adjustments` ＋ `invoice_adjustment_lines`）；AMEGO g0401。金額全部**未稅**，與開立端含稅語意相反——這正是它不併進 `invoice_documents` 的機械理由");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    InvoiceAdjustmentType(String code, String value, String label, String description) {
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

    private static final Map<String, InvoiceAdjustmentType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceAdjustmentType::code, e -> e));

    private static final Map<String, InvoiceAdjustmentType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceAdjustmentType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static InvoiceAdjustmentType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static InvoiceAdjustmentType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
