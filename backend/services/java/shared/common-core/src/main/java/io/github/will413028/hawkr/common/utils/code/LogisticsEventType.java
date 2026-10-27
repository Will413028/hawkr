package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 物流財務（線 B）領域的事件 MQ 類型，routing key 前綴 `logistics.`，落在 `hawkr.shipping` exchange。🔴 **本 enum 的 `Value` 欄是點分式 routing key**（同 `ShippingEventType`／`FulfillmentEventType`／`OrderEventType` 的慣例），`Code` 才是 UPPER_SNAKE。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum LogisticsEventType implements CodeMeta {

    LOGISTICS_COST_INCURRED("LOGISTICS_COST_INCURRED", "logistics.cost.incurred", "物流成本已發生", "provider 這條 fulfillment leg 已產生成本。🔴 **語意不是「送達完成」**——送達／取件後取消／買家無法投遞／返程 leg 四種情形都算（70 §3.1.1）。🔴 **Cost Fact ≠ Responsibility Decision**：本事件**不攜帶**責任成因，唯一的 responsibility authority 是 `fulfillment.responsibility.finalized`；兩個事實的 durable join 見 70 §3.1.4");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    LogisticsEventType(String code, String value, String label, String description) {
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

    private static final Map<String, LogisticsEventType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(LogisticsEventType::code, e -> e));

    private static final Map<String, LogisticsEventType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(LogisticsEventType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static LogisticsEventType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static LogisticsEventType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
