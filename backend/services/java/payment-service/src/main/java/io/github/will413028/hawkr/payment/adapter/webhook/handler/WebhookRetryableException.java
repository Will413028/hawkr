package io.github.will413028.hawkr.payment.adapter.webhook.handler;

/**
 * §2A.6 webhook handler atomicity：可重試的處理失敗。
 * <p>
 * Handler 拋出此例外時：
 * <ul>
 *   <li>外層 transaction 回滾（含 markAsProcessed），webhook 狀態保持 PENDING</li>
 *   <li>Consumer 端 try/catch 接到後交由 {@code MqErrorHandler.handleFailure}
 *       依 retry-count header 重新發布到原 queue（最多 3 次），超過進 DLQ</li>
 * </ul>
 * <p>
 * 典型情境：updateToPaid 版本衝突 reload + retry 一次後仍失敗
 * （表示有真實併發競爭，需重投讓下一輪取得最新 row）。
 */
public class WebhookRetryableException extends RuntimeException {

    public WebhookRetryableException(String message) {
        super(message);
    }

    public WebhookRetryableException(String message, Throwable cause) {
        super(message, cause);
    }
}
