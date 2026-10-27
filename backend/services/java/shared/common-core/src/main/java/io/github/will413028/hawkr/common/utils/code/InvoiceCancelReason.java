package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 文件／作業／調整被取消或終止的機器可讀原因（**5 值**）。與 InvoiceManualReason 的分工：後者是「不自己處理、轉人工」，本值域是「這件事不做了」。🔴 **沒有 PROVIDER_DRAIN_TIMEOUT**（2026-08-25 依 58 §7.3.4 刪除）：它唯一的寫入者 P19（drain 期限屆滿硬停）已隨 DRAIN 分支整條刪除 ⇒ 留著就是一個 A05「依取消原因分類統計」永遠查不到資料的分類，而**空集合看起來就像今天沒有這類案件**。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum InvoiceCancelReason implements CodeMeta {

    FULLY_REFUNDED_BEFORE_ISSUE("FULLY_REFUNDED_BEFORE_ISSUE", "FULLY_REFUNDED_BEFORE_ISSUE", "開立前已全額退款", "累計退款 = 實付總額；document 與 operation 同轉 CANCELLED_BEFORE_ISSUE"),
    PROVIDER_SUSPENDED_BY_ADMIN("PROVIDER_SUSPENDED_BY_ADMIN", "PROVIDER_SUSPENDED_BY_ADMIN", "管理員停用連線（立即）", "ADM 停用（58 §7 P14，端點 45）；撤銷（P16，端點 67）來源非 SUSPENDED 時比照。該 issuer 全部未終態 operation 轉 FAILED_PERMANENT"),
    NO_DOCUMENT_TO_ADJUST("NO_DOCUMENT_TO_ADJUST", "NO_DOCUMENT_TO_ADJUST", "無可調整的發票", "原 document 進 CANCELLED_BEFORE_ISSUE／NOT_APPLICABLE／RESOLVED_EXTERNALLY；**退款本身不受影響**"),
    ADM_MANUAL("ADM_MANUAL", "ADM_MANUAL", "管理員手動取消", "A06 受控取消（端點 53，強制填理由 ＋ 二次確認）與未知結果的雙人確認取消（58 §7 J28／G18，端點 54 發起 → 55 覆核 ＋ 證據）共用本值；補充說明寫在 cancel_reason"),
    VOID_REJECTED("VOID_REJECTED", "VOID_REJECTED", "作廢被服務商拒絕", "買家的更正請求走作廢重開（58 §7.3.6 R04），而 AMEGO 拒絕了作廢（R05）⇒ R04 在同一交易內已建好的重開票 document 與 REISSUE operation 永遠不會開，兩者同轉 CANCELLED_BEFORE_ISSUE。**買家手上的原發票仍然有效**，只是內容改不了");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    InvoiceCancelReason(String code, String value, String label, String description) {
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

    private static final Map<String, InvoiceCancelReason> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceCancelReason::code, e -> e));

    private static final Map<String, InvoiceCancelReason> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceCancelReason::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static InvoiceCancelReason fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static InvoiceCancelReason fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
