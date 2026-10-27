package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 管理員對一張檢舉單的**裁決**動作，寫進 `reports.resolved_action` 並由 `internal/usecase/report.go` 的 `actionToStatus` 一對一映射到 `ReportStatus` 的四個終態。**與 `ModerationActionType`（內部技術碼）是兩件事**：本 enum 是「這張單怎麼判」、後者是「稽核 log 上發生過什麼」，後者多出 `AUTO_REJECT`（系統自動處置，沒有檢舉單）與 `REOPEN`（把單子打回申訴，不是終態裁決）。兩張表兩個值域，合成一個會讓 `resolved_action` 有機會寫進不是終態的值。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ReportResolveAction implements CodeMeta {

    HIDE_TARGET("HIDE_TARGET", "HIDE_TARGET", "隱藏內容", "被檢舉內容下架（寫入 `hidden_at`），單子轉 `RESOLVED_HIDDEN`"),
    WARN_USER("WARN_USER", "WARN_USER", "警告使用者", "記錄警告、內容保留，單子轉 `RESOLVED_WARNED`"),
    BAN_USER("BAN_USER", "BAN_USER", "停權使用者", "停權被檢舉人帳號，單子轉 `RESOLVED_BANNED`"),
    DISMISS("DISMISS", "DISMISS", "駁回檢舉", "檢舉不成立、內容保留，單子轉 `DISMISSED`");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ReportResolveAction(String code, String value, String label, String description) {
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

    private static final Map<String, ReportResolveAction> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ReportResolveAction::code, e -> e));

    private static final Map<String, ReportResolveAction> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ReportResolveAction::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ReportResolveAction fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ReportResolveAction fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
