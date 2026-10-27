package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * UGC 檢舉單的處理狀態。權威值域是 social-service 的 DB CHECK `chk_reports_status`（`services/social-service/db/schema/schema.sql`），不是 proto 註解——後者曾漂出過已廢棄的值。`APPEALED` 由 `AdminReopen` 寫入（被處置者申訴後重開），它與 `PENDING` 是兩件事：前者代表「處置過但被質疑」、後者代表「還沒有人看過」，合併會讓重開的案件混進未讀佇列。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ReportStatus implements CodeMeta {

    PENDING("PENDING", "PENDING", "待處理", "尚未有管理員處置，DB 預設值"),
    RESOLVED_HIDDEN("RESOLVED_HIDDEN", "RESOLVED_HIDDEN", "已隱藏", "處置為 `HIDE_TARGET`，被檢舉內容已下架"),
    RESOLVED_WARNED("RESOLVED_WARNED", "RESOLVED_WARNED", "已警告", "處置為 `WARN_USER`，內容保留"),
    RESOLVED_BANNED("RESOLVED_BANNED", "RESOLVED_BANNED", "已停權", "處置為 `BAN_USER`，被檢舉人帳號停權"),
    DISMISSED("DISMISSED", "DISMISSED", "已駁回", "處置為 `DISMISS`，檢舉不成立、內容保留"),
    APPEALED("APPEALED", "APPEALED", "申訴中", "已處置後被申訴重開（`AdminReopen`），等待覆核；與 PENDING 分開才分得出「沒人看過」與「看過但被質疑」");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ReportStatus(String code, String value, String label, String description) {
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

    private static final Map<String, ReportStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ReportStatus::code, e -> e));

    private static final Map<String, ReportStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ReportStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ReportStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ReportStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
