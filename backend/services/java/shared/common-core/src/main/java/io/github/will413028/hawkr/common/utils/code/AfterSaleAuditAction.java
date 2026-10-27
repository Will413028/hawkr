package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 售後稽核日誌動作類型。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum AfterSaleAuditAction implements CodeMeta {

    CREATE_CASE("CREATE_CASE", "8501", "建立案件", "買家申請售後"),
    SUPPLEMENT_EVIDENCE("SUPPLEMENT_EVIDENCE", "8502", "補充證據", "補上傳證據照片"),
    CANCEL_CASE("CANCEL_CASE", "8503", "取消案件", "買家撤銷申請"),
    RESOLVE_RETURN_REFUND("RESOLVE_RETURN_REFUND", "8505", "同意退貨退款", "賣家決策"),
    RESOLVE_PARTIAL_REFUND("RESOLVE_PARTIAL_REFUND", "8506", "部分退款", "賣家決策"),
    RESOLVE_FULL_REFUND_NO_RETURN("RESOLVE_FULL_REFUND_NO_RETURN", "8507", "全額退款（免退貨）", "賣家決策"),
    RESOLVE_COUPON_COMPENSATION("RESOLVE_COUPON_COMPENSATION", "8508", "補償優惠券", "賣家決策"),
    RESOLVE_REJECT("RESOLVE_REJECT", "8509", "拒絕售後", "賣家決策"),
    CONFIRM_RETURN_RECEIPT("CONFIRM_RETURN_RECEIPT", "8510", "確認收退貨", "賣家確認"),
    CANCEL_SHIPMENT("CANCEL_SHIPMENT", "8511", "取消出貨", "賣家取消"),
    REFUND_PAGE_REQUESTED("REFUND_PAGE_REQUESTED", "8512", "請求退款頁面", "買家請求 PayUni 頁面"),
    REFUND_SUCCEEDED("REFUND_SUCCEEDED", "8513", "退款成功", "系統"),
    REFUND_FAILED("REFUND_FAILED", "8514", "退款失敗", "系統"),
    REFUND_MANUAL_INTERVENTION("REFUND_MANUAL_INTERVENTION", "8515", "退款需人工", "系統"),
    CANCEL_REFUND_SUCCEEDED("CANCEL_REFUND_SUCCEEDED", "8516", "取消退款成功", "系統"),
    RETURN_IN_TRANSIT("RETURN_IN_TRANSIT", "8517", "退貨運送中", "系統"),
    RETURN_DELIVERED("RETURN_DELIVERED", "8518", "退貨已送達", "系統"),
    RETURN_ABNORMAL("RETURN_ABNORMAL", "8519", "退貨異常", "系統"),
    RETURN_TIMEOUT_ESCALATED("RETURN_TIMEOUT_ESCALATED", "8520", "退貨逾時升級", "系統（**Deprecated 2026-06-15**：原 30 天逾時只寫 audit log 流程；新流程於 status=20 自動關案改寫 RETURN_TIMEOUT_AUTO_CLOSED(8528)。enum 保留作歷史相容，新寫入不再使用）"),
    AUTO_CANCEL_PROCESSING_TIMEOUT("AUTO_CANCEL_PROCESSING_TIMEOUT", "8521", "逾期自動取消", "系統排程：賣家逾期未處理訂單，自動取消出貨"),
    LABEL_RETRY_AUTO("LABEL_RETRY_AUTO", "8522", "出貨便列印自動重試", "Worker 掃到 5 分鐘未回填，自動觸發第 2 次列印"),
    LABEL_RETRY_EXHAUSTED("LABEL_RETRY_EXHAUSTED", "8523", "出貨便列印重試耗盡", "自動重試後仍無代碼，停手等待 ops 介入"),
    LABEL_RETRY_MANUAL("LABEL_RETRY_MANUAL", "8524", "出貨便列印人工重試", "ops 透過 retry API 手動觸發列印"),
    REFUND_FAILURE_REVERT_TO_AWAITING("REFUND_FAILURE_REVERT_TO_AWAITING", "8525", "ATM 退款失敗退回填寫", "ATM 退款任務於任何失敗來源（sync return / 合規 webhook RefundStatus=2 / UNKNOWN 變體）後，stamp PayUni 訊息 + 退回 AWAITING_BUYER_REFUND_PAGE 並 attempt++，讓買家重填銀行帳號"),
    AUTO_CONFIRM_RECEIPT("AUTO_CONFIRM_RECEIPT", "8526", "自動確認收貨", "系統排程：賣家未於 auto_confirm_at 期限內確認收貨，scheduler 自動確認並啟動退款"),
    FORCE_CONFIRM_RECEIPT("FORCE_CONFIRM_RECEIPT", "8527", "Admin 強制確認收貨", "Admin 介入：PayUni 漏發送達 / 物流失敗 / 賣家失聯，強制推進 status→30 並啟動退款（extra 含 reason、priorStatus）"),
    RETURN_TIMEOUT_AUTO_CLOSED("RETURN_TIMEOUT_AUTO_CLOSED", "8528", "退貨待寄回逾時自動關案", "系統（ReturnTimeoutWorker）：buyer 進入 RETURN_PENDING(20) 後逾 ship_back_deadline_at 未寄回，CAS 自動推進至 AUTO_COMPLETED_RETURN_PENDING_TIMEOUT(33)，退款資格解除；extra 含 shipBackDeadlineAt、returnPendingAt（= deadline - timeoutDays）"),
    CANCELLED_BY_CONFIRM_RECEIPT("CANCELLED_BY_CONFIRM_RECEIPT", "8529", "確認收貨自動取消", "系統（order-service confirmReceipt 同步觸發 #1225）：買家於售後待處理階段（PENDING(0)）主動確認收貨，aftersales CAS 自動推進至 CANCELLED_BY_CONFIRM_RECEIPT(71)；actor=buyer；extra 含 orderNo、priorStatus。與買家手動撤銷 CANCEL_CASE(8503) 區分（前者由確認收貨副作用觸發）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    AfterSaleAuditAction(String code, String value, String label, String description) {
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

    private static final Map<String, AfterSaleAuditAction> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(AfterSaleAuditAction::code, e -> e));

    private static final Map<String, AfterSaleAuditAction> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(AfterSaleAuditAction::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static AfterSaleAuditAction fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static AfterSaleAuditAction fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
