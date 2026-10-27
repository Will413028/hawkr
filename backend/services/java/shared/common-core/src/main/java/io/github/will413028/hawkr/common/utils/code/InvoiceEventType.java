package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 電子發票事件 MQ 類型，由 invoice-service 的 outbox dispatcher 發布到 `hawkr.invoice` exchange（58 §6.3）。Code 與 Value 一致（UPPER_SNAKE），**與 routing key 不同**——routing key 是點分式 `invoice.issued`、event_type 是 `INVOICE_ISSUED`；consumer 用 routing key 分支，event_type 僅作事件 envelope 的 metadata。此為 Go 領域慣例，對齊 BuyerEventType／SocialEventType／DelegationEventType；routing key 常數另寫在 `pkg/mq/exchanges.go`。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum InvoiceEventType implements CodeMeta {

    INVOICE_ISSUED("INVOICE_ISSUED", "INVOICE_ISSUED", "發票已開立", "Routing key `invoice.issued`；consumer notification-service；冪等鍵 `INV:{invoice_document_id}`"),
    INVOICE_ISSUE_FAILED("INVOICE_ISSUE_FAILED", "INVOICE_ISSUE_FAILED", "發票開立失敗", "Routing key `invoice.issue_failed`；達重試上限或永久性錯誤；冪等鍵 `INVOP:{operation_id}:{attempt_count}`"),
    INVOICE_VOIDED("INVOICE_VOIDED", "INVOICE_VOIDED", "發票已作廢", "Routing key `invoice.voided`；冪等鍵 `INV:{invoice_document_id}`"),
    INVOICE_ALLOWANCE_ISSUED("INVOICE_ALLOWANCE_ISSUED", "INVOICE_ALLOWANCE_ISSUED", "折讓單已開立", "Routing key `invoice.allowance.issued`；冪等鍵 `INV:{allowance_document_id}`"),
    INVOICE_REISSUED("INVOICE_REISSUED", "INVOICE_REISSUED", "發票已重開", "Routing key `invoice.reissued`；冪等鍵 `INV:{new_document_id}`"),
    INVOICE_RECEIPT_ISSUED("INVOICE_RECEIPT_ISSUED", "INVOICE_RECEIPT_ISSUED", "收據已開立", "Routing key `invoice.receipt.issued`；免用統一發票賣家的平台代製收據"),
    INVOICE_OPERATION_REVIEW_REQUIRED("INVOICE_OPERATION_REVIEW_REQUIRED", "INVOICE_OPERATION_REVIEW_REQUIRED", "開票作業需人工處理", "Routing key `invoice.operation.review_required`；受控人工出口入口；冪等鍵 `INVOP:{operation_id}:{attempt_count}`"),
    INVOICE_COMPLAINT_CREATED("INVOICE_COMPLAINT_CREATED", "INVOICE_COMPLAINT_CREATED", "買家提出發票申訴", "Routing key `invoice.complaint.created`；冪等鍵 `INVCPL:{complaint_id}:created`"),
    INVOICE_COMPLAINT_RESPONDED("INVOICE_COMPLAINT_RESPONDED", "INVOICE_COMPLAINT_RESPONDED", "賣家已回覆申訴", "Routing key `invoice.complaint.responded`；冪等鍵 `INVCPL:{complaint_id}:responded`"),
    INVOICE_COMPLAINT_OVERDUE("INVOICE_COMPLAINT_OVERDUE", "INVOICE_COMPLAINT_OVERDUE", "申訴逾 3 工作天未回覆", "Routing key `invoice.complaint.overdue`；冪等鍵 `INVCPL:{complaint_id}:overdue`"),
    INVOICE_SELLER_TAX_PROFILE_STATUS_CHANGED("INVOICE_SELLER_TAX_PROFILE_STATUS_CHANGED", "INVOICE_SELLER_TAX_PROFILE_STATUS_CHANGED", "稅務身分審核狀態變更", "Routing key `invoice.seller_tax_profile.status_changed`；**唯一不帶 order_no 的事件**，收件人只有賣家"),
    INVOICE_RESOLVED_EXTERNALLY("INVOICE_RESOLVED_EXTERNALLY", "INVOICE_RESOLVED_EXTERNALLY", "賣家自行開立並回報", "Routing key `invoice.resolved_externally`；端點 62（賣家自報）與端點 56（ADM 代錄）共用同一段 domain 邏輯、發同一則；**只有買家向**（賣家是動作發起人，不發）；冪等鍵 `INVEXT:{invoice_document_id}`");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    InvoiceEventType(String code, String value, String label, String description) {
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

    private static final Map<String, InvoiceEventType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceEventType::code, e -> e));

    private static final Map<String, InvoiceEventType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceEventType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static InvoiceEventType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static InvoiceEventType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
