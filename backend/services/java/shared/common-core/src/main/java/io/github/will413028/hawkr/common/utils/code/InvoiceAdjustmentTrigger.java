package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 一次稅務調整事件的**觸發來源**。它同時是冪等鍵的一段（58 §5.3.2），且與兩個配對 CHECK 綁死：`chk_invadj_refund_pair`（`ORDER_REFUND` ⇔ `refund_no` 非空）與 `chk_invadj_correction_pair`（`BUYER_CORRECTION` ⇔ `correction_request_id` 非空）。⇒ **加值必須同批改那兩條 CHECK**，否則新來源的配對欄位無人守。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum InvoiceAdjustmentTrigger implements CodeMeta {

    ORDER_REFUND("ORDER_REFUND", "ORDER_REFUND", "訂單退款", "由 `order.refund.finalized` 觸發；`refund_no` 必填（＝權威退款單號）"),
    BUYER_CORRECTION("BUYER_CORRECTION", "BUYER_CORRECTION", "買家更正發票資料", "作廢重開鏈的作廢段，emitter ＝ 58 §7.3.6 **R04**（預檢通過且 document 已 `ISSUED` ⇒ 建 VOID operation，`adjustment_type=VOID`）；`correction_request_id` 必填"),
    ADM_MANUAL("ADM_MANUAL", "ADM_MANUAL", "管理員手動發起", "ADM 在異常工作台直接建立調整；兩個配對欄位皆為空");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    InvoiceAdjustmentTrigger(String code, String value, String label, String description) {
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

    private static final Map<String, InvoiceAdjustmentTrigger> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceAdjustmentTrigger::code, e -> e));

    private static final Map<String, InvoiceAdjustmentTrigger> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceAdjustmentTrigger::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static InvoiceAdjustmentTrigger fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static InvoiceAdjustmentTrigger fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
