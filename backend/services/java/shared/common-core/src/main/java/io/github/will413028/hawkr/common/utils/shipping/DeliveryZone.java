package io.github.will413028.hawkr.common.utils.shipping;

/**
 * 配送區域（內部計算用）
 * 根據寄件/收件區域組合判定，用於黑貓費率查表
 */
public enum DeliveryZone {
    /** 雙方本島 */
    DOMESTIC,
    /** 一方離島 */
    OFFSHORE,
    /** 雙方離島 */
    INTER_ISLAND
}
