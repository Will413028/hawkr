package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 履約（fulfillment）領域的事件 MQ 類型，routing key 前綴 `fulfillment.`，落在 `hawkr.shipping` exchange。🔴 **本 enum 的 `Value` 欄是點分式 routing key**（同 `ShippingEventType`／`OrderEventType` 的慣例），`Code` 才是 UPPER_SNAKE。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum FulfillmentEventType implements CodeMeta {

    FULFILLMENT_RESPONSIBILITY_UNRESOLVED("FULFILLMENT_RESPONSIBILITY_UNRESOLVED", "fulfillment.responsibility.unresolved", "履約責任判定未決", "配送失敗但成因證據不足，責任歸屬待裁決。producer ＝ payment-service outbox，與責任案件落列同交易。🔴 **不帶任何 financially-final 語意**（不帶負擔方、不帶退款決定、不帶 resolved 的成因）——「退不退」在此時尚未決定"),
    FULFILLMENT_RESPONSIBILITY_FINALIZED("FULFILLMENT_RESPONSIBILITY_FINALIZED", "fulfillment.responsibility.finalized", "履約責任判定已定案", "producer ＝ payment-service outbox，與 resolution `APPROVED` ＋ 案件 `RESOLVED` 同交易。🔴 `responsibility_cause` 只能是四個 resolved 值；`finalized` 不是 `resolved` 的同義詞——前者才表示「不再有任何會改變退款權利的待覆核提案」");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    FulfillmentEventType(String code, String value, String label, String description) {
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

    private static final Map<String, FulfillmentEventType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(FulfillmentEventType::code, e -> e));

    private static final Map<String, FulfillmentEventType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(FulfillmentEventType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static FulfillmentEventType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static FulfillmentEventType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
