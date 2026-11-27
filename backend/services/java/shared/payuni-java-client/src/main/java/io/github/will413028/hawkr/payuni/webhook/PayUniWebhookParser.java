package io.github.will413028.hawkr.payuni.webhook;

import io.github.will413028.hawkr.payuni.crypto.PayUniCryptoService;
import io.github.will413028.hawkr.payuni.webhook.event.*;
import io.github.will413028.hawkr.payuni.webhook.event.ShippingPrintEvent;

import java.util.Map;

/**
 * PayUni Webhook 解析器
 * <p>
 * 提供一站式 verify + decrypt + parse，產出強型別事件。
 */
public class PayUniWebhookParser {

    private final PayUniCryptoService cryptoService;

    public PayUniWebhookParser(PayUniCryptoService cryptoService) {
        this.cryptoService = cryptoService;
    }

    /**
     * 解析 Webhook 通知
     *
     * @param encryptInfo 加密資料
     * @param hashInfo    簽章
     * @return 解析結果（含強型別事件）
     * @throws WebhookVerifyException 簽章驗證失敗
     */
    public WebhookParseResult parse(String encryptInfo, String hashInfo) {
        if (!cryptoService.verifyHash(encryptInfo, hashInfo)) {
            throw new WebhookVerifyException("Webhook 簽章驗證失敗");
        }

        Map<String, String> rawMap = cryptoService.decrypt(encryptInfo);

        WebhookType type = resolveType(rawMap);
        PayUniWebhookEvent event = buildEvent(type, rawMap);

        return new WebhookParseResult(type, event, rawMap, encryptInfo, hashInfo);
    }

    /**
     * 依解密後欄位判斷 Webhook 類型（以 PayUni 實際回傳欄位為準）
     * <p>
     * 物流 Webhook 有三種子類型，必須按優先級判斷：
     * 1. ApiType=Print → 超商列印結果（含 Odno/ValidationNo，不含 ShipStatus）
     * 2. ShipTradeNo + LgsType + ShipStatus → 物流貨態更新
     * 3. ShipTradeNo + LgsType（無 ShipStatus）→ 建單結果
     * 三者都走 SHIPPING_STATUS routing key，由下游 ShippingWebhookConsumer 按 ApiType 分流。
     */
    private WebhookType resolveType(Map<String, String> data) {
        if ("Print".equals(data.get("ApiType"))) {
            return WebhookType.SHIPPING_PRINT;
        }

        if (data.containsKey("ShipTradeNo") && data.containsKey("LgsType")) {
            return WebhookType.SHIPPING_STATUS;
        }

        if (data.containsKey("NotifyType")) {
            return WebhookType.AGENT_NOTIFY;
        }

        return WebhookType.UNKNOWN;
    }

    /**
     * 建立強型別事件物件
     */
    private PayUniWebhookEvent buildEvent(WebhookType type, Map<String, String> data) {
        return switch (type) {
            case SHIPPING_STATUS -> new ShippingStatusEvent(
                    data.get("MerID"),
                    data.get("ShipTradeNo"),
                    data.get("ShipStatus"),
                    data.get("ShipStatusTime"),
                    data.get("ShipType"),
                    data.get("LgsType"),
                    data.get("Message")
            );
            case SHIPPING_PRINT -> new ShippingPrintEvent(
                    data.get("MerID"),
                    data.get("ShipTradeNo"),
                    data.get("PartnerId"),
                    data.get("Odno"),
                    data.get("ValidationNo"),
                    data.get("GoodsType"),
                    data.get("LgsType"),
                    data.get("ShipType")
            );
            case AGENT_NOTIFY -> new AgentNotifyEvent(
                    data.get("MerID"),
                    data.get("MerStatus"),
                    data.get("MerStatusNote"),
                    data.get("ExecuteTime")
            );
            case UNKNOWN -> null;
        };
    }
}
