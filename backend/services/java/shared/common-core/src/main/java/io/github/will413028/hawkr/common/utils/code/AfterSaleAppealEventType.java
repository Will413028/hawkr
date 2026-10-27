package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 售後申訴事件類型（供 timeline 展示）；詳見設計書 §10.4.4
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum AfterSaleAppealEventType implements CodeMeta {

    APPEAL_SUBMITTED("APPEAL_SUBMITTED", "APPEAL_SUBMITTED", "申訴提交", "買家提交申訴"),
    DECISION_MADE("DECISION_MADE", "DECISION_MADE", "平台裁決", "Admin 做出裁決"),
    AUTO_ESCALATED("AUTO_ESCALATED", "AUTO_ESCALATED", "系統自動升級", "賣家處理逾時，系統自動升級至 admin 仲裁（CaseProcessTimeoutWorker，§5.10）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    AfterSaleAppealEventType(String code, String value, String label, String description) {
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

    private static final Map<String, AfterSaleAppealEventType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(AfterSaleAppealEventType::code, e -> e));

    private static final Map<String, AfterSaleAppealEventType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(AfterSaleAppealEventType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static AfterSaleAppealEventType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static AfterSaleAppealEventType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
