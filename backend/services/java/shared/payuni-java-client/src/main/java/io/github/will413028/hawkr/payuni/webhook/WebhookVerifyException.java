package io.github.will413028.hawkr.payuni.webhook;

/**
 * Webhook 簽章驗證失敗例外
 */
public class WebhookVerifyException extends RuntimeException {

    public WebhookVerifyException(String message) {
        super(message);
    }
}
