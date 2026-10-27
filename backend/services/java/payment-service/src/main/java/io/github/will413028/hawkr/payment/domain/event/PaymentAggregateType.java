package io.github.will413028.hawkr.payment.domain.event;

/**
 * 支付領域事件聚合類型常數
 */
public final class PaymentAggregateType {

    /** 支付交易 */
    public static final String PAYMENT = "payment";

    /** 退款交易 */
    public static final String REFUND = "refund";

    private PaymentAggregateType() {}
}
