package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 訂單生命週期狀態
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum OrderStatus implements CodeMeta {

    PENDING_PAYMENT("PENDING_PAYMENT", "0", "待付款", "庫存已扣減，等待支付事件"),
    PAID("PAID", "10", "已付款", "已完成付款，等待賣家出貨"),
    PENDING_SHIPMENT("PENDING_SHIPMENT", "20", "待出貨", "物流單已建立，等待出貨"),
    IN_TRANSIT("IN_TRANSIT", "30", "配送中", "物流配送中"),
    WAITING_PICKUP("WAITING_PICKUP", "32", "待取貨", "超商到店待取（僅超商）"),
    SHIPPING_ABNORMAL("SHIPPING_ABNORMAL", "35", "物流異常", "物流異常，admin 端處理"),
    DELIVERED("DELIVERED", "40", "已送達", "消費者已取貨，等待確認收貨，可申請售後"),
    COMPLETED("COMPLETED", "50", "已完成（終態）", "買家確認收貨 → 自動撥款，不可再發起售後"),
    DELIVERY_FAILED_NO_REFUND("DELIVERY_FAILED_NO_REFUND", "54", "配送失敗未退款（終態）", "🔴 **配送未成功、不向買家退款的財務結案終態，不可逆**。唯一入邊 ＝ `SHIPPING_ABNORMAL(35) → 54`，唯一驅動者 ＝ `fulfillment.responsibility.finalized` 且 `responsibilityCause = BUYER_CAUSED`（履約責任判定已 financially final）。與 `50` 同屬撥款結案 band ⇒ payout eligible。**無出邊**：不得新增 54→70／54→80／任何 post-54 的 refund reversal。⚠️ 取 `54` 而非 `55` 是因為 `ShippingStatus` 已用 `55 = SELLER_NOT_PICKED`（責任方相反），見 03 §2.1"),
    PARTIALLY_REFUNDED("PARTIALLY_REFUNDED", "60", "部分退款", "已完成訂單的部分退款終態；唯一進入轉移 = 50→60。進行中訂單（10~40）的部分退款不改狀態，僅累加 `orders.refund_total_amount`（03 §5.7.1）"),
    FULLY_REFUNDED("FULLY_REFUNDED", "70", "全額退款（終態）", "全部商品已退款"),
    CANCELLED("CANCELLED", "80", "已取消（終態）", "未付/手動/超時取消");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    OrderStatus(String code, String value, String label, String description) {
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

    private static final Map<String, OrderStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(OrderStatus::code, e -> e));

    private static final Map<String, OrderStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(OrderStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static OrderStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static OrderStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
