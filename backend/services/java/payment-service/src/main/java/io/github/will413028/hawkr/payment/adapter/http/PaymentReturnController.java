package io.github.will413028.hawkr.payment.adapter.http;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.WebhookProcessStatus;
import io.github.will413028.hawkr.payment.application.service.PaymentResultDisplay;
import io.github.will413028.hawkr.common.utils.code.WebhookProvider;
import io.github.will413028.hawkr.payment.adapter.webhook.handler.PaymentNotifyHandler;
import io.github.will413028.hawkr.payment.adapter.webhook.handler.WebhookHandleResult;
import io.github.will413028.hawkr.payuni.crypto.PayUniCryptoService;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.PaymentWebhookEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.PaymentWebhookMapper;
import com.fasterxml.jackson.databind.ObjectMapper;
import jakarta.servlet.http.HttpServletRequest;
import org.springframework.http.MediaType;
import org.springframework.transaction.support.TransactionTemplate;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;

import java.time.LocalDateTime;
import java.util.Map;

/**
 * PayUni 付款完成後的回調頁面 Controller
 *
 * PayUni 信用卡 UPP 付款完成後，以 POST 方式將使用者瀏覽器導回此 URL。 POST body 包含加密的交易結果（EncryptInfo
 * + HashInfo）。
 *
 * 處理流程： 1. 驗簽 + 解密 EncryptInfo 2. 記錄完整交易結果到 payment_webhooks（稽核） 3. 事務內更新交易狀態
 * + 寫入 outbox 付款結果事件（冪等，與 Webhook 共用 PaymentNotifyHandler，§5.10.1 afterCommit 即時投遞下游）
 * 4. 渲染 HTML 付款結果頁面 → deep link 回 App
 *
 * 注意：此 endpoint 與 Webhook 通知是「誰先到誰處理」的關係，
 * 透過冪等鍵（tradeNo:providerTradeNo）確保不重複處理。
 */
@RestController
public class PaymentReturnController {

    private static final ObsLogger log = ObsLogger.forClass(PaymentReturnController.class);

    /**
     * 來源標識：區分 return 回調與 MQ webhook
     */
    private static final String SOURCE_RETURN = "RETURN";

    private final PayUniCryptoService cryptoService;
    private final PaymentNotifyHandler notifyHandler;
    private final PaymentWebhookMapper webhookMapper;
    private final TransactionTemplate transactionTemplate;
    private final ObjectMapper objectMapper;

    public PaymentReturnController(
            PayUniCryptoService cryptoService,
            PaymentNotifyHandler notifyHandler,
            PaymentWebhookMapper webhookMapper,
            TransactionTemplate transactionTemplate,
            ObjectMapper objectMapper
    ) {
        this.cryptoService = cryptoService;
        this.notifyHandler = notifyHandler;
        this.webhookMapper = webhookMapper;
        this.transactionTemplate = transactionTemplate;
        this.objectMapper = objectMapper;
    }

    /**
     * PayUni UPP 付款完成回調
     *
     * PayUni POST form data: Status, MerID, Version, EncryptInfo, HashInfo
     */
    @PostMapping(value = "/payment/return",
            consumes = MediaType.APPLICATION_FORM_URLENCODED_VALUE,
            produces = MediaType.TEXT_HTML_VALUE)
    public String handlePaymentReturn(
            @RequestParam("Status") String status,
            @RequestParam("MerID") String merId,
            @RequestParam("EncryptInfo") String encryptInfo,
            @RequestParam("HashInfo") String hashInfo,
            HttpServletRequest request
    ) {
        log.info("PaymentReturnController.received",
                "收到 PayUni 付款回調: status=" + status + ", merId=" + merId);

        // 1. 驗簽
        if (!cryptoService.verifyHash(encryptInfo, hashInfo)) {
            log.warn("PaymentReturnController.verifyFailed", "簽章驗證失敗", "merId", merId);
            return renderHtml(PaymentResultDisplay.FAILED, "", "", "簽章驗證失敗");
        }

        // 2. 解密
        Map<String, String> decryptedData;
        try {
            decryptedData = cryptoService.decrypt(encryptInfo);
        } catch (Exception e) {
            log.error("PaymentReturnController.decryptFailed", "解密失敗", e);
            return renderHtml(PaymentResultDisplay.FAILED, "", "", "資料解密失敗");
        }

        log.info("PaymentReturnController.decrypted", "解密成功: " + decryptedData);

        String tradeNo = decryptedData.get("MerTradeNo");
        String tradeAmt = decryptedData.get("TradeAmt");
        String payuniMessage = decryptedData.get("Message");
        String sourceIp = request.getRemoteAddr();

        // 3. 判斷付款結果：TradeStatus=1 為成功，其餘一律失敗
        //    PayUni POST 回來就是終態，不會有「處理中」
        // 注意：query 參數 status 是 PayUni 第三方協定字串（"SUCCESS"），與 PaymentResultDisplay 命名空間獨立
        boolean isSuccess = "SUCCESS".equals(status) && "1".equals(decryptedData.get("TradeStatus"));
        PaymentResultDisplay displayStatus = isSuccess ? PaymentResultDisplay.SUCCESS : PaymentResultDisplay.FAILED;
        String errorMsg = isSuccess ? "" : (payuniMessage != null ? payuniMessage : "付款失敗");

        // 4. 冪等檢查
        String idempotencyKey = notifyHandler.extractIdempotencyKey(decryptedData);
        if (idempotencyKey != null
                && webhookMapper.existsProcessed(WebhookProvider.PAYUNI.value(), idempotencyKey)) {
            log.info("PaymentReturnController.idempotent.skip", "已處理過（冪等跳過）",
                    "key", idempotencyKey);
            return renderHtml(displayStatus, tradeNo, tradeAmt, errorMsg);
        }

        // 5. 儲存 Inbox 記錄（稽核用）
        Long webhookId = saveInbox(encryptInfo, decryptedData, idempotencyKey, sourceIp);

        // 6. Transaction 內執行業務邏輯（更新交易狀態 + outbox 事件寫入 + 標記已處理，三者原子）
        //    §5.10.1 L1'：付款結果事件由 PaymentNotifyHandler 經 PaymentOutboxWriter 在此事務內
        //    寫入 outbox 並註冊 afterCommit 即時投遞
        //    PaymentNotifyHandler 內部對缺少 TradeStatus 的情況會返回 FAILED，不影響冪等
        WebhookHandleResult result = transactionTemplate.execute(txStatus -> {
            WebhookHandleResult r = notifyHandler.handle(decryptedData);
            webhookMapper.markAsProcessed(webhookId, r.processStatus(),
                    r.message(), DateTimeUtils.nowUtc());
            return r;
        });

        log.info("PaymentReturnController.completed",
                "處理完成:tradeNo=" + tradeNo + ", displayStatus=" + displayStatus
                + ", processStatus=" + (result != null ? result.processStatus() : "null"));

        // 8. 渲染 HTML（用 PayUni 的 Status 判斷結果，不依賴 handler 訊息）
        return renderHtml(displayStatus, tradeNo, tradeAmt, errorMsg);
    }

    /**
     * 儲存 Inbox 記錄到 payment_webhooks
     */
    private Long saveInbox(String rawEncryptInfo, Map<String, String> decryptedData,
            String idempotencyKey, String sourceIp) {
        PaymentWebhookEntity entity = new PaymentWebhookEntity();
        entity.setWebhookType(WebhookProvider.PAYUNI.value());
        entity.setTradeNo(decryptedData.get("MerTradeNo"));
        entity.setProviderTradeNo(decryptedData.get("TradeNo"));
        entity.setIdempotencyKey(idempotencyKey);
        entity.setRawRequest(rawEncryptInfo);
        entity.setDecryptedPayload(toJson(decryptedData));
        entity.setIsValid(true);
        entity.setIsProcessed(false);
        entity.setProcessStatus(WebhookProcessStatus.PENDING.value());
        entity.setReceivedAt(DateTimeUtils.nowUtc());
        entity.setSourceIp(SOURCE_RETURN + ":" + sourceIp);

        webhookMapper.insert(entity);
        return entity.getId();
    }

    private String toJson(Object obj) {
        try {
            return objectMapper.writeValueAsString(obj);
        } catch (Exception e) {
            return "{}";
        }
    }

    /**
     * 渲染付款結果 HTML 頁面
     */
    private String renderHtml(PaymentResultDisplay display, String tradeNo, String amount, String errorMsg) {
        String icon, title, subtitle, bgColor;

        switch (display) {
            case SUCCESS -> {
                icon = "&#x2705;"; // ✅
                title = "付款成功";
                subtitle = "感謝您的購買！";
                bgColor = "#f0fdf4";
            }
            case FAILED -> {
                icon = "&#x274C;"; // ❌
                title = "付款失敗";
                subtitle = "請重新嘗試或選擇其他付款方式";
                bgColor = "#fef2f2";
            }
            case ERROR -> {
                icon = "&#x26A0;&#xFE0F;"; // ⚠️
                title = "系統異常";
                subtitle = (errorMsg != null && !errorMsg.isEmpty()) ? errorMsg : "請稍後重試";
                bgColor = "#fffbeb";
            }
            default -> {
                icon = "&#x23F3;"; // ⏳
                title = "付款處理中";
                subtitle = "請稍候，系統正在確認付款結果";
                bgColor = "#eff6ff";
            }
        }

        String safeTradeNo = tradeNo != null ? tradeNo : "";
        String deepLink = "hawkrbuyer://payment/result?tradeNo=" + safeTradeNo + "&status=" + display.code();

        String amountHtml = "";
        if (amount != null && !amount.isEmpty()) {
            amountHtml = "<p style=\"font-size:18px;color:#374151;margin:8px 0;\">金額：NT$ " + amount + "</p>";
        }

        String tradeNoHtml = "";
        if (safeTradeNo != null && !safeTradeNo.isEmpty()) {
            tradeNoHtml = "<p style=\"font-size:14px;color:#6b7280;margin:4px 0;\">交易編號：" + safeTradeNo + "</p>";
        }

        return """
                <!DOCTYPE html>
                <html lang="zh-TW">
                <head>
                    <meta charset="UTF-8">
                    <meta name="viewport" content="width=device-width, initial-scale=1.0">
                    <title>%s - Hawkr</title>
                    <style>
                        * { margin: 0; padding: 0; box-sizing: border-box; }
                        body {
                            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
                            display: flex; justify-content: center; align-items: center;
                            min-height: 100vh; background: %s;
                        }
                        .container {
                            text-align: center; padding: 40px 24px;
                            background: white; border-radius: 16px;
                            box-shadow: 0 4px 24px rgba(0,0,0,0.08);
                            max-width: 400px; width: 90%%;
                        }
                        .icon { font-size: 64px; margin-bottom: 16px; }
                        .title { font-size: 24px; font-weight: 600; color: #1f2937; margin-bottom: 8px; }
                        .btn {
                            display: inline-block; margin-top: 24px; padding: 14px 32px;
                            background: #10b981; color: white; border: none; border-radius: 12px;
                            font-size: 16px; font-weight: 500; text-decoration: none; cursor: pointer;
                        }
                        .btn:active { background: #059669; }
                        .countdown { font-size: 13px; color: #9ca3af; margin-top: 16px; }
                    </style>
                </head>
                <body>
                    <div class="container">
                        <div class="icon">%s</div>
                        <div class="title">%s</div>
                        <p style="font-size:15px;color:#6b7280;margin-bottom:12px;">%s</p>
                        %s
                        %s
                        <a class="btn" id="returnBtn" href="%s">返回 App</a>
                        <p class="countdown" id="countdown">10 秒後自動返回 App...</p>
                    </div>
                    <script>
                        var seconds = 10;
                        var deepLink = "%s";
                        var timer = setInterval(function() {
                            seconds--;
                            var el = document.getElementById("countdown");
                            if (seconds > 0) {
                                el.textContent = seconds + " 秒後自動返回 App...";
                            } else {
                                clearInterval(timer);
                                el.textContent = "正在返回 App...";
                                window.location.href = deepLink;
                            }
                        }, 1000);
                    </script>
                </body>
                </html>""".formatted(title, bgColor, icon, title, subtitle,
                amountHtml, tradeNoHtml, deepLink, deepLink);
    }
}
