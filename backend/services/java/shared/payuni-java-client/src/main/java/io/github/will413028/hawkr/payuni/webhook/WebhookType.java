package io.github.will413028.hawkr.payuni.webhook;

/**
 * PayUni Webhook 事件類型
 */
public enum WebhookType {

    /** 付款結果通知（UOP） */
    PAYMENT_UOP,

    /** 物流貨態通知（建單結果 / 狀態變更） */
    SHIPPING_STATUS,

    /** 超商列印結果通知（ApiType=Print，含交貨便代碼） */
    SHIPPING_PRINT,

    /** 代理商通知（商家狀態異動等） */
    AGENT_NOTIFY,

    /** 退款結果通知（ATM async webhook，含 RefundStatus / BankDate；匯款完成 / 失敗） */
    REFUND_NOTIFY,

    /** 退款受理通知（ATM async webhook，含 EstimatedRefundDate 無 RefundStatus；sync return URL 的 async 副本） */
    REFUND_ACCEPTED,

    /** 續期收款結果通知（PayUni page_id=306；含 PeriodTradeNo／PeriodOrderNo／ThisPeriod） */
    PAYMENT_RECURRING,

    /** 無法辨識的類型 */
    UNKNOWN
}
