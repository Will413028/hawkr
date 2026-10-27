package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 一張稅務文件的**買家可見歷程**事件。🔴 **它不是 `InvoiceEventType`**——那是 MQ 事件型別（12 值、全部 `INVOICE_` 開頭、前端可見 No、不進任何 allowlist），兩者**交集為空**；把本欄的值拿去查 `InvoiceEventType` 的表只會查不到而顯示空白（`codeLabel` 查不到不報錯）。本值域是 `invoice_status_history` 的**投影**：後端組 `timeline[]` 時把三個 `subject_type` 的狀態轉移摺疊成買家看得懂的五件事，一個轉移最多產生一列。⚠️ 五值刻意**不含失敗與處理中**（`ISSUING`／`ISSUE_FAILED`／`AWAITING_MANUAL` 等內部階段）——買家端對這些的呈現由 `state` ＋ `stateNotice` 承擔（58 §6.1.3），時間軸只記「已經成立的法定事實」。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum InvoiceDocumentTimelineEvent implements CodeMeta {

    ISSUED("ISSUED", "ISSUED", "已開立", "投影自 `subject_type='DOCUMENT'` 且 `to_status='ISSUED'`（58 §7.3.1）"),
    UPDATED("UPDATED", "UPDATED", "已修改", "投影自 `subject_type='CORRECTION_REQUEST'` 且 `to_status='UPDATED_BEFORE_ISSUE'`（58 §7.3.6 **R03**，開票前改快照）"),
    VOIDED("VOIDED", "VOIDED", "已作廢", "投影自 `subject_type='DOCUMENT'` 且 `to_status='VOIDED'`（58 §7.3.1；該轉態由 §7.3.7 **G09** 的作廢副作用寫入）"),
    REISSUED("REISSUED", "REISSUED", "已重新開立", "投影自 `subject_type='DOCUMENT'` 且 `to_status='SUPERSEDED'`（58 §7.3.1「重開文件轉 `ISSUED`」那一列）。⚠️ 這一列落在**舊文件**的時間軸上；新文件的時間軸只有它自己的 `ISSUED`"),
    ALLOWANCE_APPLIED("ALLOWANCE_APPLIED", "ALLOWANCE_APPLIED", "已開立折讓", "投影自 `subject_type='ADJUSTMENT'` 且 `to_status='COMPLETED'`，並要求該 `invoice_adjustments.adjustment_type='ALLOWANCE'` 且 `document_id` ＝本文件（58 §7.3.7 **G09**／**G13**／**G17**）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    InvoiceDocumentTimelineEvent(String code, String value, String label, String description) {
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

    private static final Map<String, InvoiceDocumentTimelineEvent> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceDocumentTimelineEvent::code, e -> e));

    private static final Map<String, InvoiceDocumentTimelineEvent> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceDocumentTimelineEvent::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static InvoiceDocumentTimelineEvent fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static InvoiceDocumentTimelineEvent fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
