package io.github.will413028.hawkr.payuni.webhook;

import io.github.will413028.hawkr.payuni.webhook.event.PayUniWebhookEvent;

import java.util.Map;

/**
 * Webhook 解析結果
 *
 * @param type        事件類型
 * @param event       強型別事件物件
 * @param rawMap      解密後的原始 Key-Value
 * @param encryptInfo 原始加密字串（供稽核）
 * @param hashInfo    原始簽章（供稽核）
 */
public record WebhookParseResult(
        WebhookType type,
        PayUniWebhookEvent event,
        Map<String, String> rawMap,
        String encryptInfo,
        String hashInfo
) {
}
