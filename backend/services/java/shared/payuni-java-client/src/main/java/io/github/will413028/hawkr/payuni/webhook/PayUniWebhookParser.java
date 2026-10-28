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
        // 1. 驗簽
        if (!cryptoService.verifyHash(encryptInfo, hashInfo)) {
            throw new WebhookVerifyException("Webhook 簽章驗證失敗");
        }

        // 2. 解密
        Map<String, String> rawMap = cryptoService.decrypt(encryptInfo);

        // 3. 判斷類型並建立事件
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
        // 超商列印結果（ApiType=Print，必須最先判斷）
        if ("Print".equals(data.get("ApiType"))) {
            return WebhookType.SHIPPING_PRINT;
        }

        // 含 ShipTradeNo + LgsType → 物流（建單結果 / 貨態更新）
        if (data.containsKey("ShipTradeNo") && data.containsKey("LgsType")) {
            return WebhookType.SHIPPING_STATUS;
        }

        // 退款結果通知（§6.7：含 RefundStatus，匯款完成 / 失敗）；必須先於 PAYMENT_UOP 判斷
        // 因退款 webhook 亦含 TradeStatus 外層欄位，用內層 RefundStatus 唯一鍵區分
        if (data.containsKey("RefundStatus")) {
            return WebhookType.REFUND_NOTIFY;
        }

        // 退款受理通知（§6.6：含 EstimatedRefundDate 無 RefundStatus；sync return URL 的 async 副本）
        // 必須先於 PAYMENT_UOP 判斷，避免被後面 TradeAmt 條件誤歸類
        if (data.containsKey("EstimatedRefundDate")) {
            return WebhookType.REFUND_ACCEPTED;
        }

        // 週期扣款通知（含 PeriodTradeNo）：系統沒有任何定期定額流程，不處理。
        // 必須先於 PAYMENT_UOP，否則帶 TradeAmt／TradeStatus 的週期 payload 會被誤判成一次性付款；
        // 判成 UNKNOWN 後走既有的未知類型處理（寫 inbox 稽核）。
        if (data.containsKey("PeriodTradeNo")) {
            return WebhookType.UNKNOWN;
        }

        // 含 TradeAmt + TradeStatus → 付款結果
        if (data.containsKey("TradeAmt") && data.containsKey("TradeStatus")) {
            return WebhookType.PAYMENT_UOP;
        }

        // 含 NotifyType → 代理商通知
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
            case PAYMENT_UOP -> new PaymentUopEvent(
                    data.get("MerID"),
                    data.get("MerTradeNo"),
                    data.get("TradeNo"),
                    data.get("TradeStatus"),
                    data.get("TradeAmt"),
                    data.get("PayType"),
                    data.get("Message"),
                    data.get("TradeDate")
            );
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
            case REFUND_NOTIFY -> new RefundNotifyEvent(
                    data.get("MerID"),
                    data.get("MerTradeNo"),
                    data.get("TradeNo"),
                    data.get("RefundStatus"),
                    data.get("RefundAmt"),
                    data.get("BankDate"),
                    data.get("Message")
            );
            case REFUND_ACCEPTED -> new RefundAcceptedEvent(
                    data.get("MerID"),
                    data.get("MerTradeNo"),
                    data.get("TradeNo"),
                    data.get("Status"),
                    data.get("Message"),
                    data.get("RefundAmt"),
                    data.get("PaymentType"),
                    data.get("EstimatedRefundDate")
            );
            case UNKNOWN -> null;
        };
    }
}
