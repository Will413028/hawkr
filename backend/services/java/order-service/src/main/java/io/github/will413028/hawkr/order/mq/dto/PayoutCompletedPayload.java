package io.github.will413028.hawkr.order.mq.dto;

import com.fasterxml.jackson.databind.PropertyNamingStrategies;
import com.fasterxml.jackson.databind.annotation.JsonNaming;

import java.util.UUID;

/**
 * payout.completed MQ 事件 Payload（#1286）。
 *
 * <p>平台撥款指示成功後（{@code GrantPaymentWorker} 撥款 task → DONE）發布，通知賣家款項已入帳。
 * 走既有通知管線：notification-service visible template → 寫 inbox + FCM 推播（手機）+ emit
 * {@code notification.inbox.new}（→ gateway SSE → web 即時鈴鐺），FCM 與 SSE 雙通道一致。
 *
 * <p>wire 命名 snake_case（36 §7.4.1 L1）；{@code eventId} 由 publisher 產生 UUID v4。
 * notification 端以 reference_id {@code payout_{order_no}}（per-order UNIQUE）保證撥款 worker
 * 重試也只通知一次。
 *
 * <p><b>{@code storeId} / {@code store_id}</b>：店家的 {@code store_id} surrogate（原始值，
 * 非登入 account_id），供 identity_migrated 遷移後的 {@code target_user_field} 路由使用。
 *
 * <p><b>{@code notifySellerAccountId} / {@code notify_seller_account_id}</b>：由 store_id
 * 解析而來的登入 account_id（既有欄位，保留供尚未遷移的路徑使用）。舊名 {@code sellerAccountId}
 * 已於 Phase C（2026-08-06）移除——2026-08-05 Task 4 起雙寫、notification-service 模板同批切讀
 * 新名。本 payload 為直發（非 order_outbox 持久化重播），移除舊欄位無反序列化相容性顧慮。
 */
@JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy.class)
public record PayoutCompletedPayload(
        String eventId,
        String orderNo,
        String storeId,
        String notifySellerAccountId,
        String payoutAmount,
        String tradeNo,
        String grantedAt
) {
    public PayoutCompletedPayload(String orderNo, String storeId, String notifySellerAccountId,
                                  String payoutAmount, String tradeNo, String grantedAt) {
        this(UUID.randomUUID().toString(), orderNo, storeId, notifySellerAccountId, payoutAmount,
                tradeNo, grantedAt);
    }
}
