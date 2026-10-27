package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 售後事件 MQ 類型（Value 同時作為 routing key），由 `AfterSalesEventPublisherImpl` 發布到 `aftersales.events` exchange。Order Service `AfterSalesCountdownConsumer` 依不同 routing key 暫停或恢復訂單 60h 自動完成倒數。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum AfterSalesEventType implements CodeMeta {

    AWAITING_BUYER_PAGE("AWAITING_BUYER_PAGE", "aftersales.refund.awaiting_buyer_page", "等待買家退款頁面", "ATM 退款等待買家前往 PayUni 頁面填寫銀行資訊"),
    REFUND_COMPLETED("REFUND_COMPLETED", "aftersales.refund.completed", "退款完成", "售後退款任務完成（CC 或 ATM webhook 成功）"),
    CANCEL_COMPLETED("CANCEL_COMPLETED", "aftersales.cancel.completed", "取消出貨完成", "賣家取消出貨流程完成"),
    APPEAL_SUBMITTED("APPEAL_SUBMITTED", "aftersales.appeal.submitted", "申訴提交", "買家提交平台申訴"),
    PLATFORM_DECISION_MADE("PLATFORM_DECISION_MADE", "aftersales.platform_decision.made", "平台仲裁決策", "平台仲裁決策做出"),
    CASE_AUTO_REJECTED("CASE_AUTO_REJECTED", "aftersales.case.auto_rejected", "自動拒絕", "§5.10 賣家處理逾期自動代為拒絕"),
    CASE_AUTO_ESCALATED("CASE_AUTO_ESCALATED", "aftersales.case.auto_escalated", "自動升級", "§5.10 賣家處理逾期自動升級至平台"),
    CASE_APPLIED("CASE_APPLIED", "aftersales.case.applied", "案件已申請", "買家提出售後申請"),
    CASE_RETURN_APPROVED("CASE_RETURN_APPROVED", "aftersales.case.return_approved", "退貨已核准", "賣家核准退貨"),
    CASE_REFUND_APPROVED("CASE_REFUND_APPROVED", "aftersales.case.refund_approved", "退款已核准", "賣家核准退款"),
    CASE_COUPON_COMPENSATED("CASE_COUPON_COMPENSATED", "aftersales.case.coupon_compensated", "優惠券補償", "賣家以優惠券補償"),
    CASE_REJECTED("CASE_REJECTED", "aftersales.case.rejected", "案件已拒絕", "賣家主動拒絕"),
    CASE_CANCELLED_BY_BUYER("CASE_CANCELLED_BY_BUYER", "aftersales.case.cancelled_by_buyer", "買家取消案件", "買家主動取消售後申請"),
    CASE_CANCELLED_BY_CONFIRM_RECEIPT("CASE_CANCELLED_BY_CONFIRM_RECEIPT", "aftersales.case.cancelled_by_confirm_receipt", "確認收貨取消案件", "買家於售後待處理階段（PENDING(0)）主動「確認收貨」，aftersales 同步 CAS 終結案件至 CANCELLED_BY_CONFIRM_RECEIPT(71)（#1225）；經 publishCaseTransitionWithContext 自動補發 seller.after_sale.updated realtime + 通知賣家。訂單隨後完成，故 order-service 不需據此恢復倒數（倒數已隨案件終結而與已完成訂單無關）"),
    CASE_RETURN_RECEIVED("CASE_RETURN_RECEIVED", "aftersales.case.return_received", "退貨已收件", "賣家收到退貨包裹"),
    CASE_RETURN_IN_TRANSIT("CASE_RETURN_IN_TRANSIT", "aftersales.case.return_in_transit", "退貨運送中", "買家寄出退貨、PayUni 物流碼 31/92 → RETURN_SHIPPING(25)；通知賣家「買家已寄出退貨」並補發賣家工作區 realtime"),
    CASE_RETURN_AT_STORE("CASE_RETURN_AT_STORE", "aftersales.case.return_at_store", "退貨已到店待取", "超商退貨配達退貨門市，賣家待取（72h 倒數開始）"),
    CASE_RETURN_AT_STORE_24H("CASE_RETURN_AT_STORE_24H", "aftersales.case.return_at_store_24h", "超商待取剩 24h", "超商待取剩餘 24 小時未取貨預警"),
    CASE_RETURN_DELIVERED("CASE_RETURN_DELIVERED", "aftersales.case.return_delivered", "退貨已送達", "超商賣家已取貨 / 宅配已簽收（48h 倒數開始）"),
    CASE_RETURN_DELIVERED_24H("CASE_RETURN_DELIVERED_24H", "aftersales.case.return_delivered_24h", "確認收貨剩 24h", "賣家確認收貨剩餘 24 小時未操作預警"),
    CASE_RETURN_FAILED("CASE_RETURN_FAILED", "aftersales.case.return_failed", "退貨失敗", "退貨物流異常"),
    CASE_AUTO_COMPLETED_STORE_PICKUP_TIMEOUT("CASE_AUTO_COMPLETED_STORE_PICKUP_TIMEOUT", "aftersales.case.auto_completed_store_pickup_timeout", "超商待取逾時自動完成", "賣家逾 72h 未從超商取回退貨包裹，系統自動確認收貨並退款"),
    CASE_AUTO_COMPLETED_CONFIRM_RECEIPT_TIMEOUT("CASE_AUTO_COMPLETED_CONFIRM_RECEIPT_TIMEOUT", "aftersales.case.auto_completed_confirm_receipt_timeout", "確認收貨逾時自動完成", "賣家逾 48h 未確認收貨，系統自動確認收貨並退款"),
    CASE_RETURN_PENDING_24H("CASE_RETURN_PENDING_24H", "aftersales.case.return_pending_24h", "待寄回剩 24h", "買家於 RETURN_PENDING(20) 距 ship_back_deadline_at 剩 24h 仍未寄出，ReturnTwentyFourHourWarnScheduler D-1 預警 FCM（推目標 = 買家）；冪等靠 notification 端 notifications_inbox_dedup_idx（reference_id = case_return_pending_24h_#{case_no}）；warned_ship_back_24h_at 為事後記錄非閘門，見 issue #2294 §1-3"),
    CASE_AUTO_COMPLETED_RETURN_PENDING_TIMEOUT("CASE_AUTO_COMPLETED_RETURN_PENDING_TIMEOUT", "aftersales.case.auto_completed_return_pending_timeout", "待寄回逾時自動關案", "買家逾 ship_back_deadline_at（預設 7 天）未寄回，ReturnTimeoutWorker CAS 自動推進 20→33，退款資格解除；通知買賣雙方（買家 visible：「逾時未寄回，退款資格已解除」；賣家 visible：「售後案件已關閉，款項照常撥付」）；同時通知 order-service 嘗試恢復倒數");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    AfterSalesEventType(String code, String value, String label, String description) {
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

    private static final Map<String, AfterSalesEventType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(AfterSalesEventType::code, e -> e));

    private static final Map<String, AfterSalesEventType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(AfterSalesEventType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static AfterSalesEventType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static AfterSalesEventType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
