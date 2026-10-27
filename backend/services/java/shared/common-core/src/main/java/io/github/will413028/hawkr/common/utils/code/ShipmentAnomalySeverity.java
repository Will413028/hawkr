package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 配送異常嚴重度。由 `ShipmentAnomalyRules.severityOf(ShipmentAnomalyType)` 從 anomaly type 推導；HIGH 觸發即時 FCM、MEDIUM 早晚摘要、LOW 僅寫 inbox 不推 push（推送節奏由 notification-service 端策略決定）。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ShipmentAnomalySeverity implements CodeMeta {

    LOW("LOW", "LOW", "低", "流程慢但非急迫（如 RETURN_STUCK）"),
    MEDIUM("MEDIUM", "MEDIUM", "中", "賣家需注意，早晚摘要推送"),
    HIGH("HIGH", "HIGH", "高", "賣家需立即處理，即時推送");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ShipmentAnomalySeverity(String code, String value, String label, String description) {
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

    private static final Map<String, ShipmentAnomalySeverity> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ShipmentAnomalySeverity::code, e -> e));

    private static final Map<String, ShipmentAnomalySeverity> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ShipmentAnomalySeverity::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ShipmentAnomalySeverity fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ShipmentAnomalySeverity fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
