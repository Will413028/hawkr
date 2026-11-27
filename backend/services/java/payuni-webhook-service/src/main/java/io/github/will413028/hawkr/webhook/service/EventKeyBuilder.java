package io.github.will413028.hawkr.webhook.service;

import io.github.will413028.hawkr.common.utils.code.PayUniEventType;

import java.util.Map;

/**
 * 依 Webhook 類型組合去重用 eventKey
 *
 * eventKey 格式：
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
            case SHIPPING_STATUS -> {
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
            case UNKNOWN -> merId + ":UNKNOWN:" + System.currentTimeMillis();
        };
    }
}
