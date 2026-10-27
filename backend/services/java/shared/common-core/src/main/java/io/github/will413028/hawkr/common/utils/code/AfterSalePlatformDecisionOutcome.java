package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 平台申訴裁決結果（Phase 1 三種）；詳見設計書 §10.4.3
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum AfterSalePlatformDecisionOutcome implements CodeMeta {

    FULL_REFUND("FULL_REFUND", "FULL_REFUND", "全額退款", "平台裁定全額退款，不需退貨"),
    PARTIAL_REFUND("PARTIAL_REFUND", "PARTIAL_REFUND", "部分退款", "平台裁定部分退款，金額由 Admin 指定"),
    REJECT_APPEAL("REJECT_APPEAL", "REJECT_APPEAL", "駁回申訴", "平台駁回買家申訴，維持賣家原決策");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    AfterSalePlatformDecisionOutcome(String code, String value, String label, String description) {
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

    private static final Map<String, AfterSalePlatformDecisionOutcome> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(AfterSalePlatformDecisionOutcome::code, e -> e));

    private static final Map<String, AfterSalePlatformDecisionOutcome> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(AfterSalePlatformDecisionOutcome::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static AfterSalePlatformDecisionOutcome fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static AfterSalePlatformDecisionOutcome fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
