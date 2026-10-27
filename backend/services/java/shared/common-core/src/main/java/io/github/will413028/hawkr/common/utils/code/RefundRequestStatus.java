package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 退款申請審核流程狀態
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum RefundRequestStatus implements CodeMeta {

    PENDING_AUDIT("PENDING_AUDIT", "0", "待審核", "待 Admin 審核"),
    WAIT_USER_ACTION("WAIT_USER_ACTION", "1", "待用戶操作", "審核通過，待用戶選擇退款方式"),
    PROCESSING("PROCESSING", "2", "處理中", "系統執行退款中"),
    COMPLETED("COMPLETED", "3", "完成", "退款已完成"),
    REJECTED("REJECTED", "9", "已駁回", "審核駁回");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    RefundRequestStatus(String code, String value, String label, String description) {
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

    private static final Map<String, RefundRequestStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(RefundRequestStatus::code, e -> e));

    private static final Map<String, RefundRequestStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(RefundRequestStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static RefundRequestStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static RefundRequestStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
