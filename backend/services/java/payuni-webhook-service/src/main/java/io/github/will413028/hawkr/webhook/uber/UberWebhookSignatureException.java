package io.github.will413028.hawkr.webhook.uber;

/**
 * Uber webhook 驗簽失敗。
 *
 * <p>🔴 <b>這是 ingress 唯一允許回 4xx 的情境</b>（66 §4.5.0／§0.3）。
 * 其餘任何失敗都必須成為 5xx —— Uber 只對 {@code 500/502/503/504} 與 timeout 重試，
 * 回 4xx 等於宣告「這個事件永久丟失」。
 *
 * <p>⚠️ 對驗簽失敗回 401 而不是 200 是刻意的：它不是我方的事件，
 * 沒有理由佔用 {@code webhook_inbox} 的 {@code uq_inbox_event} 鍵位
 * —— 而那個鍵位正是壓制攻擊要搶的東西。
 */
public class UberWebhookSignatureException extends RuntimeException {

    public UberWebhookSignatureException(String message) {
        super(message);
    }
}
