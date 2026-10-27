package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 訂單事件 MQ 類型，由 `OrderEventPublisher` 發布到 `hawkr.order` exchange。🔴 **本 enum 的 `Value` 欄是點分式 routing key**（`order.paid`），`Code` 才是 UPPER_SNAKE（`ORDER_PAID`）——`OrderCompletionTxHandler` 傳給 publisher 的正是 `.value()`，也就是點分式。⚠️ **repo 內兩種 `Value` 慣例並存**：本 enum（Java 領域）的 `Value` 是 routing key；Go 領域的 `InvoiceEventType`／`BuyerEventType`／`SocialEventType` 的 `Value` 是 UPPER_SNAKE 的 event_type，routing key 另寫在 `pkg/mq/exchanges.go`。新增列一律跟**該 enum 自己的**慣例，不要跨領域套用。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum OrderEventType implements CodeMeta {

    ORDER_PAID("ORDER_PAID", "order.paid", "訂單已付款", "付款成功，觸發 Shipping Service 自動建單"),
    ORDER_COMPLETED("ORDER_COMPLETED", "order.completed", "訂單已完成", "訂單流程結束，觸發下游撥款"),
    PAYOUT_COMPLETED("PAYOUT_COMPLETED", "payout.completed", "撥款完成", "平台撥款指示成功後通知賣家款項已入帳（GrantPaymentWorker emit，#1286）"),
    ORDER_CANCELLED("ORDER_CANCELLED", "order.cancelled", "訂單取消", "觸發 Product Service 非同步庫存歸還"),
    ORDER_DELIVERED("ORDER_DELIVERED", "order.delivered", "訂單已送達", "物流送達，通知前端/推播"),
    ORDER_RECEIPT_CONFIRMED("ORDER_RECEIPT_CONFIRMED", "order.receipt.confirmed", "確認收貨", "通知 Shipping Service 標記 receipt_confirmed"),
    ORDER_SALES_ACCUMULATED("ORDER_SALES_ACCUMULATED", "order.sales.accumulated", "訂單銷量累計", "付款成功時發出，product-service 依 productId 累計 sales_count（見 [35 設計書](../詳細設計書/35-商品列表排序與銷量反正規化%EF%BC%88Product%20Sort%20&%20Sales%20Count%EF%BC%89詳細設計書.md)）"),
    ORDER_SHIPMENT_CREATE_REQUESTED("ORDER_SHIPMENT_CREATE_REQUESTED", "order.shipment.create_requested", "重試建立物流單", "Backstop 於付款後逾期仍無物流單時請 Shipping 冪等重建（見 03 §8.11）"),
    ORDER_SHIPMENT_CREATION_FAILED("ORDER_SHIPMENT_CREATION_FAILED", "order.shipment.creation_failed", "建單逾時取消", "Backstop 重試上限仍無物流單，請 AfterSales 系統取消 + 退款（見 03 §8.11）"),
    ORDER_REFUND_FINALIZED("ORDER_REFUND_FINALIZED", "order.refund.finalized", "訂單退款落定", "訂單層唯一權威退款事件，三個發布點皆在 order-service 交易內落 order_outbox；invoice-service 據此判定作廢／折讓（58 §6.3.2.3）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    OrderEventType(String code, String value, String label, String description) {
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

    private static final Map<String, OrderEventType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(OrderEventType::code, e -> e));

    private static final Map<String, OrderEventType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(OrderEventType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static OrderEventType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static OrderEventType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
