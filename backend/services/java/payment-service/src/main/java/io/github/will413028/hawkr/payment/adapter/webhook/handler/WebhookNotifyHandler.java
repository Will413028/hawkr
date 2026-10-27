package io.github.will413028.hawkr.payment.adapter.webhook.handler;

import io.github.will413028.hawkr.common.utils.code.WebhookProvider;

import java.util.Map;

/**
 * Webhook 通知處理器介面
 *
 * 每種 NotifyType 對應一個 Handler 實作
 */
public interface WebhookNotifyHandler {

    /**
     * 對應的 Webhook 類型
     */
    WebhookProvider webhookType();

    /**
     * 從解密後的資料中提取冪等鍵
     *
     * @param decryptedData 解密後的 PayUni 通知資料
     * @return 冪等鍵（null 表示不進行冪等檢查）
     */
    String extractIdempotencyKey(Map<String, String> decryptedData);

    /**
     * 處理 Webhook 通知
     *
     * 此方法在 Transaction 內執行
     *
     * @param decryptedData 解密後的 PayUni 通知資料
     * @return 處理結果
     */
    WebhookHandleResult handle(Map<String, String> decryptedData);
}
