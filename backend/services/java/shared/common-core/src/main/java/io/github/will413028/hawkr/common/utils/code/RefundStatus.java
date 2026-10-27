package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * PayUni 退款交易狀態
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum RefundStatus implements CodeMeta {

    REQUESTED("REQUESTED", "0", "申請中", "退款申請已建立"),
    PROCESSING("PROCESSING", "1", "處理中", "PayUni 處理中"),
    SUCCEEDED("SUCCEEDED", "2", "退款成功", "退款已完成"),
    FAILED("FAILED", "3", "退款失敗", "退款未成功，需人工介入"),
    APPROVED("APPROVED", "4", "已核准", "退款已核准、尚未送出至金流"),
    MANUAL_REVIEW("MANUAL_REVIEW", "5", "人工審核中", "退款需人工介入判定，未進入自動處理");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    RefundStatus(String code, String value, String label, String description) {
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

    private static final Map<String, RefundStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(RefundStatus::code, e -> e));

    private static final Map<String, RefundStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(RefundStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static RefundStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static RefundStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
