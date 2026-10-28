package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * PayUni Webhook 事件子類別。每個值對應一組 MQ exchange + routing key（在 Java enum 內以靜態欄位儲存）。獨立於 WebhookProvider（後者是金流商分類），抽象層級不同：未來接綠界時應新增 `ECPayEventType` 而非擴充本枚舉。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum PayUniEventType implements CodeMeta {

    PAYMENT_UOP("PAYMENT_UOP", "8251", "付款結果", "PayUni 付款結果通知（UOP）→ payuni.payment.uop"),
    SHIPPING_STATUS("SHIPPING_STATUS", "8252", "物流狀態", "PayUni 物流狀態通知 → payuni.shipping.status"),
    SHIPPING_PRINT("SHIPPING_PRINT", "8253", "超商列印", "超商列印結果（與 SHIPPING_STATUS 共用 routing key）"),
    AGENT_NOTIFY("AGENT_NOTIFY", "8254", "代理通知", "PayUni 代理通知（賣家 onboarding 等）→ seller.review.result"),
    REFUND_NOTIFY("REFUND_NOTIFY", "8255", "退款結果", "PayUni ATM 退款結果通知（含 RefundStatus / BankDate，匯款完成 / 失敗）→ payuni.refund.notify"),
    REFUND_ACCEPTED("REFUND_ACCEPTED", "8256", "退款受理", "PayUni ATM 退款受理通知（含 EstimatedRefundDate 無 RefundStatus；sync return URL 的 async 副本）→ payuni.refund.accepted"),
    UNKNOWN("UNKNOWN", "8259", "未知事件", "無法由 payload 辨識的 PayUni webhook，仍寫入 inbox 稽核並打印 warning，由人工或重送處理");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    PayUniEventType(String code, String value, String label, String description) {
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

    private static final Map<String, PayUniEventType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PayUniEventType::code, e -> e));

    private static final Map<String, PayUniEventType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PayUniEventType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static PayUniEventType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static PayUniEventType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
