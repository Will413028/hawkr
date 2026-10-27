package io.github.will413028.hawkr.webhook.adapter.http;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.webhook.service.WebhookReceiveService;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

/**
 * PayUni Webhook 統一接收端點
 *
 * PayUni 後台僅允許設定一組 Webhook URL，所有類型的通知（付款/物流/代理商）
 * 皆透過同一端點接收，由 SDK 的 PayUniWebhookParser 自動分類。
 *
 * 請求格式：application/x-www-form-urlencoded
 * - EncryptInfo：AES-256-GCM 加密後的 hex 字串
 * - HashInfo：SHA256 驗簽 hash
 */
@RestController
@RequestMapping("/webhooks/payuni")
public class PayUniWebhookController {

    private static final ObsLogger log = ObsLogger.forClass(PayUniWebhookController.class);

    private final WebhookReceiveService receiveService;

    public PayUniWebhookController(WebhookReceiveService receiveService) {
        this.receiveService = receiveService;
    }

    /**
     * PayUni Webhook 統一入口（付款結果 / 物流貨態 / 代理商通知）
     */
    @PostMapping(value = "/notify", consumes = MediaType.APPLICATION_FORM_URLENCODED_VALUE)
    public ResponseEntity<String> handleWebhook(
            @RequestParam("EncryptInfo") String encryptInfo,
            @RequestParam("HashInfo") String hashInfo) {
        log.info("PayUniWebhookController.receive.start", "收到 PayUni Webhook");
        receiveService.receive(encryptInfo, hashInfo);
        return ResponseEntity.ok("SUCCESS");
    }
}
