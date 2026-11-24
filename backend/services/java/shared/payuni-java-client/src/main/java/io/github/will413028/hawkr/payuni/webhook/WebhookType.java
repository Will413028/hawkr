package io.github.will413028.hawkr.payuni.webhook;

/**
 * PayUni Webhook 事件類型
 */
public enum WebhookType {

    /** 物流貨態通知（建單結果 / 狀態變更） */
    SHIPPING_STATUS,

    /** 超商列印結果通知（ApiType=Print，含交貨便代碼） */
    SHIPPING_PRINT,

    /** 代理商通知（商家狀態異動等） */
    AGENT_NOTIFY,

    /** 無法辨識的類型 */
    UNKNOWN
}
