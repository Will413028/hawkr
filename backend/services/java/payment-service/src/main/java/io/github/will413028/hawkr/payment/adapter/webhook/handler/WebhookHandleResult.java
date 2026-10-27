package io.github.will413028.hawkr.payment.adapter.webhook.handler;

import io.github.will413028.hawkr.common.utils.code.WebhookProcessStatus;

/**
 * Webhook 處理結果，僅攜帶處理狀態與訊息。
 * <p>
 * issue #1349 §5.10.1 L1'（收斂點 A）：付款結果 MQ 事件由 {@code PaymentNotifyHandler}
 * 在事務內經 {@code PaymentOutboxWriter.enqueueAndDispatch} 寫入 outbox，不由本結果攜帶。
 */
public record WebhookHandleResult(
        String processStatus,
        String message
) {

    /**
     * 處理成功
     */
    public static WebhookHandleResult success(String message) {
        return new WebhookHandleResult(WebhookProcessStatus.SUCCESS.value(), message);
    }

    /**
     * 跳過處理（交易已終態等）
     */
    public static WebhookHandleResult skipped(String message) {
        return new WebhookHandleResult(WebhookProcessStatus.SKIPPED.value(), message);
    }

    /**
     * 處理失敗
     */
    public static WebhookHandleResult failed(String message) {
        return new WebhookHandleResult(WebhookProcessStatus.FAILED.value(), message);
    }
}
