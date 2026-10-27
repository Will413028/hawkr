package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 申訴時間軸上「誰選了哪個動作」。⚠️ **與 `InvoiceComplaintEventType` 是同一張表的兩個正交欄位、不是二選一**：`event_type` 答「發生了什麼」（含系統自動事件），`action` 答「賣家或 ADM 在畫面上點了哪一顆」。沒有人做選擇的列（買家提申訴、SLA 逾期、逾時自動升級、**買家不接受（端點 63）**、買家撤回、買家重開）`action` 為 NULL——**買家的動作一律不進本軸**，本 enum 只裝賣家與 ADM 在畫面上點的按鈕。⚠️ ADM 端點 60 的另外兩個 action（`REQUEST_REWORK`／`RECOMMEND_SUSPENSION`）**刻意不寫時間軸**——前者副作用只有重置 SLA，後者只寫建議與稽核（`invoice_admin_audit_logs`），所以本表是 6 值不是 8 值。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum InvoiceComplaintAction implements CodeMeta {

    RESENT("RESENT", "RESENT", "已重新寄送", "actor = SELLER；端點 36。原發票存在，賣家重寄通知／載具"),
    REISSUED("REISSUED", "REISSUED", "已補開或更正", "actor = SELLER；端點 36"),
    NEED_PLATFORM_HELP("NEED_PLATFORM_HELP", "NEED_PLATFORM_HELP", "需要平台協助", "actor = SELLER；端點 36。必填說明，同時把案件推進 `ESCALATED_TO_ADM`（58 §7 K03）"),
    REMINDED("REMINDED", "REMINDED", "平台已提醒賣家", "actor = ADMIN；端點 60 的 `REMIND_SELLER`。不改變狀態也不改變 SLA"),
    DECIDED_RESOLVED("DECIDED_RESOLVED", "DECIDED_RESOLVED", "平台判定已解決", "actor = ADMIN；端點 60 的 `DECIDE_RESOLVED`（58 §7 K08）"),
    DECIDED_VIOLATION("DECIDED_VIOLATION", "DECIDED_VIOLATION", "平台判定賣家違規", "actor = ADMIN；端點 60 的 `DECIDE_VIOLATION`（58 §7 K09）。**只有這個值計入 90 天違規計數**");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    InvoiceComplaintAction(String code, String value, String label, String description) {
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

    private static final Map<String, InvoiceComplaintAction> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceComplaintAction::code, e -> e));

    private static final Map<String, InvoiceComplaintAction> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceComplaintAction::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static InvoiceComplaintAction fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static InvoiceComplaintAction fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
