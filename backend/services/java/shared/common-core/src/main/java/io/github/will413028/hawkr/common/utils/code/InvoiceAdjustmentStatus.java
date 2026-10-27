package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 一次稅務調整事件（作廢／折讓）的狀態。兩個 BLOCKED_* 是「凍結」語意——調整已受理但尚不可送出，喚醒發生在閘門釋放的**同一個事務**內，不是掃描器（58 §7.3.7）。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum InvoiceAdjustmentStatus implements CodeMeta {

    PENDING("PENDING", "PENDING", "待送出", "已建立調整與明細，等待 operation claim"),
    BLOCKED_UNKNOWN_DOCUMENT("BLOCKED_UNKNOWN_DOCUMENT", "BLOCKED_UNKNOWN_DOCUMENT", "凍結（原發票結果未確認）", "原 document 開票結果未明，不建 operation；由 document 轉態的同事務喚醒"),
    BLOCKED_PENDING_CORRECTION("BLOCKED_PENDING_CORRECTION", "BLOCKED_PENDING_CORRECTION", "凍結（更正處理中）", "序列閘門被 correction 佔用；喚醒時 document_id 改指向生效中的那張發票"),
    SENDING("SENDING", "SENDING", "送出中", "對應 operation 已 claim"),
    WAITING_CONFIRMATION("WAITING_CONFIRMATION", "WAITING_CONFIRMATION", "等待結果確認", "逾時／結果不確定"),
    COMPLETED("COMPLETED", "COMPLETED", "已完成", "provider 明確成功；同事務累加原發票的 allowance_total_amount（終態）"),
    FAILED_PERMANENT("FAILED_PERMANENT", "FAILED_PERMANENT", "永久失敗", "明確不可重試；理由碼落 blocked_reason_code"),
    FAILED_UNKNOWN("FAILED_UNKNOWN", "FAILED_UNKNOWN", "結果未確認", "逾 24 小時仍無法確認"),
    CANCELLED("CANCELLED", "CANCELLED", "已撤銷", "ADM 撤銷或確認無調整對象；唯一鍵同時釋放（終態）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    InvoiceAdjustmentStatus(String code, String value, String label, String description) {
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

    private static final Map<String, InvoiceAdjustmentStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceAdjustmentStatus::code, e -> e));

    private static final Map<String, InvoiceAdjustmentStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceAdjustmentStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static InvoiceAdjustmentStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static InvoiceAdjustmentStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
