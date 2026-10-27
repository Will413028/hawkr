package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 售後單生命週期狀態
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum AfterSaleCaseStatus implements CodeMeta {

    PENDING("PENDING", "0", "待處理", "買家已申請，等待賣家處理"),
    RETURN_PENDING("RETURN_PENDING", "20", "待退貨", "賣家同意退貨退款，等待買家寄回"),
    RETURN_SHIPPING("RETURN_SHIPPING", "25", "退貨運送中", "買家已寄出，物流配送中"),
    RETURN_AT_STORE("RETURN_AT_STORE", "26", "退貨已到店待取", "超商退貨配達退貨門市，等待賣家領取（72 小時內未取則系統自動完成確認收貨）。僅超商退貨（returnShippingMethod=3002）使用"),
    RETURN_DELIVERED("RETURN_DELIVERED", "27", "退貨已送達", "超商退貨：賣家從門市實際取貨完成；宅配退貨：包裹送達賣家地址並簽收。等待賣家確認收貨（48 小時內未確認則系統自動完成）"),
    RETURN_FAILED("RETURN_FAILED", "28", "退貨物流失敗", "退貨物流發生 PayUni ABNORMAL 狀態（包裹遺失/退回買家等），需 Admin 介入或走申訴流程"),
    RETURN_RECEIVED("RETURN_RECEIVED", "30", "已收退貨", "賣家手動確認收貨完成（含 Admin 強制確認）"),
    AUTO_COMPLETED_STORE_PICKUP_TIMEOUT("AUTO_COMPLETED_STORE_PICKUP_TIMEOUT", "31", "超商待取逾時自動完成", "超商退貨配達門市後賣家逾 72 小時未領取，系統自動推進完成確認收貨並退款。前端可直接以本狀態 label 顯示"),
    AUTO_COMPLETED_CONFIRM_RECEIPT_TIMEOUT("AUTO_COMPLETED_CONFIRM_RECEIPT_TIMEOUT", "32", "確認收貨逾時自動完成", "退貨送達後賣家逾 48 小時未確認收貨，系統自動推進完成確認收貨並退款。前端可直接以本狀態 label 顯示"),
    AUTO_COMPLETED_RETURN_PENDING_TIMEOUT("AUTO_COMPLETED_RETURN_PENDING_TIMEOUT", "33", "待寄回逾時自動關案", "進入 RETURN_PENDING(20) 後買家逾 ship_back_deadline_at（預設 7 天）未寄回，系統自動關案，退款資格解除，訂單款項照常撥付賣家。本狀態為「不退款」終態（不轉 REFUND_PROCESSING）；買家不服可走 §10 申訴流程，Admin 裁決退款時 status 改回 REFUND_PROCESSING(40)，裁決駁回時轉 CLOSED(50)。前端可直接以本狀態 label 顯示"),
    REFUND_PROCESSING("REFUND_PROCESSING", "40", "退款處理中", "退款任務建立並執行中"),
    CLOSED("CLOSED", "50", "已結案", "退款完成或優惠券已發放"),
    REJECTED("REJECTED", "60", "已拒絕", "賣家拒絕售後申請（買家不再可重啟，僅可走 §10 申訴）"),
    CANCELLED("CANCELLED", "70", "已取消", "買家自行取消"),
    CANCELLED_BY_CONFIRM_RECEIPT("CANCELLED_BY_CONFIRM_RECEIPT", "71", "買家已確認收貨", "買家於售後待處理階段（PENDING(0)）主動「確認收貨」，系統自動取消售後案件（#1225）。label 採「買家已確認收貨」純描述動作（避免「確認收貨取消」被誤讀為取消按鈕）；售後被取消的語意由詳情頁長描述補足。語意等同買家主動取消，但與一般買家取消 CANCELLED(70) 區分，以利對帳與客服追溯撥款歸屬。本狀態為終態，所有判定（isActive / isTerminalOverrideToCompleted / overridesOrderLifecycle / countActiveCasesByOrder）行為一律比照 CANCELLED(70)：非活躍、不退款、售後未成立（訂單回歸自身狀態）。僅作用於 MANUAL 確認收貨（有活躍 case 時訂單永不自動完成）。前端可直接以本狀態 label 顯示");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    AfterSaleCaseStatus(String code, String value, String label, String description) {
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

    private static final Map<String, AfterSaleCaseStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(AfterSaleCaseStatus::code, e -> e));

    private static final Map<String, AfterSaleCaseStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(AfterSaleCaseStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static AfterSaleCaseStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static AfterSaleCaseStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
