package io.github.will413028.hawkr.webhook.service;

import io.github.will413028.hawkr.common.utils.code.PayUniEventType;

import java.util.Map;

/**
 * 依 Webhook 類型組合去重用 eventKey
 *
 * eventKey 格式：
 * - PAYMENT_UOP：    {merId}:{merTradeNo}:{tradeNo}:{tradeStatus}
 *                   （tradeNo 是 PayUni 內部 serial，唯一識別一筆 PayUni 交易；
 *                    §2A.6 同 order 多筆獨立付款場景下，各筆 tradeNo 不同 → eventKey 不同 → dedup 不誤攔）
 * - SHIPPING_STATUS：{merId}:{shipTradeNo}:STATUS:{shipStatus|tradeStatus}
 * - SHIPPING_PRINT： {merId}:{shipTradeNo}:PRINT:{odno}
 * - AGENT_NOTIFY：   {merId}:{merStatus}:{executeTime}
 *
 * 注意：SHIPPING_STATUS 和 SHIPPING_PRINT 必須產生不同的 eventKey，
 * 否則去重 UNIQUE 約束會導致後到的事件被丟棄。
 * 建單回傳無 ShipStatus（有 TradeStatus），貨態回傳有 ShipStatus，
 * 兩者都以 "STATUS:" 前綴區分於 Print 事件。
 */
public final class EventKeyBuilder {

    private EventKeyBuilder() {}

    /**
     * 依 webhookType 和解密後 payload 組合 eventKey
     */
    public static String build(PayUniEventType type, Map<String, String> payload) {
        String merId = payload.getOrDefault("MerID", "UNKNOWN");
        return switch (type) {
            case PAYMENT_UOP -> String.join(":",
                    merId,
                    payload.getOrDefault("MerTradeNo", ""),
                    payload.getOrDefault("TradeNo", ""),       // PayUni 內部 serial（§2A.6 必要）
                    payload.getOrDefault("TradeStatus", ""));
            case SHIPPING_STATUS -> {
                // 貨態更新有 ShipStatus；建單結果只有 TradeStatus
                String statusValue = payload.getOrDefault("ShipStatus", "");
                if (statusValue.isEmpty()) {
                    statusValue = payload.getOrDefault("TradeStatus", "");
                }
                yield String.join(":",
                        merId,
                        payload.getOrDefault("ShipTradeNo", ""),
                        "STATUS",
                        statusValue);
            }
            case SHIPPING_PRINT -> String.join(":",
                    merId,
                    payload.getOrDefault("ShipTradeNo", ""),
                    "PRINT",
                    payload.getOrDefault("Odno", ""));
            case AGENT_NOTIFY -> String.join(":",
                    merId,
                    payload.getOrDefault("MerStatus", ""),
                    payload.getOrDefault("ExecuteTime", ""));
            case REFUND_NOTIFY -> String.join(":",
                    merId,
                    payload.getOrDefault("MerTradeNo", ""),
                    payload.getOrDefault("TradeNo", ""),
                    payload.getOrDefault("RefundStatus", ""));
            case REFUND_ACCEPTED -> String.join(":",
                    merId,
                    payload.getOrDefault("MerTradeNo", ""),
                    payload.getOrDefault("TradeNo", ""),
                    "ACCEPTED",
                    payload.getOrDefault("EstimatedRefundDate", ""));
            // UNKNOWN：使用 timestamp 確保唯一，避免去重 UNIQUE 衝突
            case UNKNOWN -> merId + ":UNKNOWN:" + System.currentTimeMillis();
        };
    }
}
