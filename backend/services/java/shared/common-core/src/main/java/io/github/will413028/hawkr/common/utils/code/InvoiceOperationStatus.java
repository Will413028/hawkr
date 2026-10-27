package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 對 provider 的一次操作（＝排程 job）的執行狀態。六種 `operation_type` 共用同一套值域（58 §7.3.0 規則 1）。**搶佔不用狀態欄**——租約走 `next_attempt_at` 的 CAS（多副本排程搶佔規範），所以 SENDING 就是「正在送」而不是「被誰鎖住」。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum InvoiceOperationStatus implements CodeMeta {

    CREATED("CREATED", "CREATED", "已建立", "剛落庫，尚未判定可否立即執行（同一事務內即離開）"),
    AWAITING_TRIGGER("AWAITING_TRIGGER", "AWAITING_TRIGGER", "等待觸發時點", "`issue_not_before > now`；**不在任何 worker 的候選述詞內**，由 INV.op 的 RunOnce 前置步驟 promote"),
    READY("READY", "READY", "待執行", "可被 claim"),
    HELD("HELD", "HELD", "暫停（買家修改中）", "買家送出付款後修改，`held_until = now + 60s`"),
    SENDING("SENDING", "SENDING", "送出中", "已 claim，租約 120 秒；租約過期即自動回收"),
    RETRY_SCHEDULED("RETRY_SCHEDULED", "RETRY_SCHEDULED", "等待重試", "可重試錯誤，`next_attempt_at = now + 2^attempt × 30s`"),
    WAITING_CONFIRMATION("WAITING_CONFIRMATION", "WAITING_CONFIRMATION", "等待結果確認", "逾時／結果不確定，落在 INV.rc 的候選集合"),
    PROVIDER_STATUS_RECHECK("PROVIDER_STATUS_RECHECK", "PROVIDER_STATUS_RECHECK", "重查中", "INV.rc 已 claim，租約 60 秒"),
    SUCCEEDED("SUCCEEDED", "SUCCEEDED", "已完成", "provider 明確成功（終態）"),
    FAILED_PERMANENT("FAILED_PERMANENT", "FAILED_PERMANENT", "永久失敗", "重試必然再失敗（統編錯／金額不平／字軌不足）；ADM 需先修正資料才可重試"),
    FAILED_RETRY_EXHAUSTED("FAILED_RETRY_EXHAUSTED", "FAILED_RETRY_EXHAUSTED", "重試次數用盡", "暫時性錯誤但預算用完；**ADM 直接重試是安全的**"),
    FAILED_UNKNOWN("FAILED_UNKNOWN", "FAILED_UNKNOWN", "結果未確認", "重試可能開出第二張發票；唯一出邊是重查"),
    SELLER_ACTION_REQUIRED("SELLER_ACTION_REQUIRED", "SELLER_ACTION_REQUIRED", "待賣家處理", "永久失敗，**但賣家自己解得掉**（AMEGO `3040111` 字軌不足 ⇒ 賣家須向財政部申請丁種字軌）。不在任何 worker 的候選述詞內，由 INV.trk 的 RunOnce 前置步驟在字軌恢復時 promote 回 READY"),
    CANCEL_REQUESTED("CANCEL_REQUESTED", "CANCEL_REQUESTED", "取消申請中", "ADM 已提出取消，等強制最終查詢"),
    CANCEL_BLOCKED_UNKNOWN("CANCEL_BLOCKED_UNKNOWN", "CANCEL_BLOCKED_UNKNOWN", "取消被擋（結果未明）", "仍無法確認 provider 端是否已開立，拒絕取消"),
    CANCELLED_BY_ADMIN("CANCELLED_BY_ADMIN", "CANCELLED_BY_ADMIN", "已由管理員取消", "雙人確認 ＋ 證據齊備"),
    MANUAL_PROCESS_ALLOWED("MANUAL_PROCESS_ALLOWED", "MANUAL_PROCESS_ALLOWED", "已解鎖人工處理", "第二人覆核解鎖，賣家可自行開立並回報"),
    RESOLVED_EXTERNALLY("RESOLVED_EXTERNALLY", "RESOLVED_EXTERNALLY", "賣家自行開立並回報", "賣家回報號碼（終態）"),
    NOT_APPLICABLE("NOT_APPLICABLE", "NOT_APPLICABLE", "不適用", "決策樹判定不由平台開立（終態）"),
    CANCELLED_BEFORE_ISSUE("CANCELLED_BEFORE_ISSUE", "CANCELLED_BEFORE_ISSUE", "開立前已取消", "開立前全額退款（終態）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    InvoiceOperationStatus(String code, String value, String label, String description) {
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

    private static final Map<String, InvoiceOperationStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceOperationStatus::code, e -> e));

    private static final Map<String, InvoiceOperationStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceOperationStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static InvoiceOperationStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static InvoiceOperationStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
