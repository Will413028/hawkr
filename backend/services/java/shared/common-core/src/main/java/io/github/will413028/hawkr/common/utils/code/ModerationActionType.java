package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 內容審核稽核 log（`moderation_actions`，append-only）的動作類型。本表記錄「發生過什麼」，含沒有檢舉單的系統自動處置與把單子打回申訴的動作，因此值域是 `ReportResolveAction`（業務類型碼，四個終態裁決）的**嚴格超集**。兩者刻意分開：`reports.resolved_action` 只能是終態裁決，若共用同一個 enum，`AUTO_REJECT`／`REOPEN` 就有機會被寫進那一欄，而它們不對應任何 `ReportStatus` 終態。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ModerationActionType implements CodeMeta {

    AUTO_REJECT("AUTO_REJECT", "AUTO_REJECT", "系統自動下架", "沒有檢舉單的系統處置（路徑 D），`report_id` 為 NULL。**本期無 emitter**，見上方說明"),
    HIDE_TARGET("HIDE_TARGET", "HIDE_TARGET", "隱藏內容", "對應 `ReportResolveAction.HIDE_TARGET`"),
    WARN_USER("WARN_USER", "WARN_USER", "警告使用者", "對應 `ReportResolveAction.WARN_USER`"),
    BAN_USER("BAN_USER", "BAN_USER", "停權使用者", "對應 `ReportResolveAction.BAN_USER`"),
    DISMISS("DISMISS", "DISMISS", "駁回檢舉", "對應 `ReportResolveAction.DISMISS`"),
    REOPEN("REOPEN", "REOPEN", "重開申訴", "被處置者申訴後由管理員重開，檢舉單轉 `APPEALED`；不是終態裁決，故不存在於 `ReportResolveAction`");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ModerationActionType(String code, String value, String label, String description) {
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

    private static final Map<String, ModerationActionType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ModerationActionType::code, e -> e));

    private static final Map<String, ModerationActionType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ModerationActionType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ModerationActionType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ModerationActionType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
