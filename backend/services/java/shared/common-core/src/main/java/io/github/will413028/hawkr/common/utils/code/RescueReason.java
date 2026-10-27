package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * rescue 退款的觸發原因，下游 consumer 退款邏輯統一，但日誌與告警可依此分類統計。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum RescueReason implements CodeMeta {

    RESCUE_RECONCILE_AFTER_EXPIRED("RESCUE_RECONCILE_AFTER_EXPIRED", "9001", "EXPIRED reconcile rescue", "reconcile 在 grace 視窗內偵測到 EXPIRED 但 PayUni 已成功"),
    RESCUE_DUPLICATE_PAYMENT("RESCUE_DUPLICATE_PAYMENT", "9002", "重複付款偵測", "§2A.6 重複付款處理觸發的退款（webhook 段 3 / reconcile 補救偵測）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    RescueReason(String code, String value, String label, String description) {
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

    private static final Map<String, RescueReason> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(RescueReason::code, e -> e));

    private static final Map<String, RescueReason> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(RescueReason::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static RescueReason fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static RescueReason fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
