package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 一張稅務文件（發票／收據）的當前狀態。⚠️ **折讓不是 document**：折讓走獨立的 `invoice_adjustments` ＋ `invoice_adjustment_lines`（58 §5 T-B04③ 定版，`chk_invdoc_kind` 維持三值），本值域不承載折讓單的狀態。**只表達當前值，歷程在 `invoice_status_history`**（58 §7.3 規則 2）。買家端看到的是本值域的**投影**而非原值（58 §6.1.3 的 `state` 欄，7 個值），因為法規要求 provider 異常時只顯示「處理中」。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum InvoiceDocumentStatus implements CodeMeta {

    PENDING("PENDING", "PENDING", "待開立", "文件已建立、ISSUE operation 尚未進入 SENDING"),
    ISSUING("ISSUING", "ISSUING", "開立中", "ISSUE operation 已進入 SENDING"),
    ISSUED("ISSUED", "ISSUED", "已開立", "provider 明確成功並回填發票號碼。**靜止態非封閉終態**——仍可作廢、被取代或被折讓"),
    ISSUE_FAILED("ISSUE_FAILED", "ISSUE_FAILED", "開立失敗", "operation 進入任一 FAILED_*；ADM 工作台可重試或轉人工"),
    AWAITING_MANUAL("AWAITING_MANUAL", "AWAITING_MANUAL", "待人工處理", "送出前的不變式斷言失敗、決策層 fail-closed、或逾法定期限；`manual_reason_code` 由 `chk_invdoc_manual_reason` 強制非空（理由碼見 InvoiceManualReason **32 值**）"),
    VOIDING("VOIDING", "VOIDING", "作廢中", "VOID operation 已進入 SENDING"),
    VOIDED("VOIDED", "VOIDED", "已作廢", "作廢確認完成（AMEGO f0501）。折讓單被 g0501 作廢是 `invoice_adjustments` 那條線的事，不落本欄"),
    SUPERSEDED("SUPERSEDED", "SUPERSEDED", "已被重開取代", "重開出來的新文件已 ISSUED，本件為舊件（終態）"),
    CANCELLED_BEFORE_ISSUE("CANCELLED_BEFORE_ISSUE", "CANCELLED_BEFORE_ISSUE", "開立前已取消", "開立前全額退款或訂單取消（終態）"),
    NOT_APPLICABLE("NOT_APPLICABLE", "NOT_APPLICABLE", "不由平台開立", "零元訂單／賣家自行開票／切點前訂單；理由碼見 InvoiceNotApplicableReason（終態）"),
    RESOLVED_EXTERNALLY("RESOLVED_EXTERNALLY", "RESOLVED_EXTERNALLY", "賣家自行開立並回報", "賣家回報自開的發票號碼；`provider_invoice_no` 必為 NULL，平台不冒充開立人（終態）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    InvoiceDocumentStatus(String code, String value, String label, String description) {
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

    private static final Map<String, InvoiceDocumentStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceDocumentStatus::code, e -> e));

    private static final Map<String, InvoiceDocumentStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceDocumentStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static InvoiceDocumentStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static InvoiceDocumentStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
