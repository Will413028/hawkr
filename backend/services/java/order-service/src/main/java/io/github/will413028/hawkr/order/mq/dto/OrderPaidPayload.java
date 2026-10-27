package io.github.will413028.hawkr.order.mq.dto;

import com.fasterxml.jackson.databind.PropertyNamingStrategies;
import com.fasterxml.jackson.databind.annotation.JsonNaming;

import java.util.UUID;

/**
 * order.paid MQ 事件 Payload
 * 付款成功後發布，供 Shipping Service 自動建單及 Notification Service 推播。
 *
 * <p>{@code eventId} 由 publisher 端自動產生 UUID v4，供 consumer 端做冪等保護
 * （36 §7.4 Idempotency Conformance Contract）。
 *
 * <p>{@code sellerName} / {@code sellerAvatarKey} 由 publisher 端在 publish 前
 * 從 SellerGrpcClient.getSellerMeta 取得後填入（見 design.md D3）。
 * sellerAvatarKey 為 objectKey（49 §6.3/S3：MQ payload 只帶 key、名 *Key；
 * 消費端 notification worker 組 URL，S5）。賣家未設頭像則為 null。
 * （seller 批次改回 key 前值仍為 URL，worker BuildURL 對 http passthrough）
 *
 * <p>{@code buyerNickname} / {@code buyerAvatarKey} 由 publisher 端從
 * BuyerGrpcClient.batchGetBuyerDisplayByBuyerIds（PK 版本，非 account_id 軸的
 * batchGetBuyerMeta——那支對 buyer_id != account_id 的分岔買家會靜默查空，
 * 見該方法 JavaDoc 記的 2026-07-26 事故）取得後填入，供賣家端推播顯示具體買家
 * 而非「買家」通用字（issue #686）。**值為 display name**：買家已註銷時是墓碑字串
 * 而非真實暱稱（#2830）——本事件在付款當下發出、買家必然仍在，實務上打不到，
 * 但若日後有補發或重放路徑，語意以此為準。buyerAvatarKey 為 objectKey（49 §6.3/S3：
 * MQ payload 只帶 key、名 *Key；消費端 notification worker 組 URL，S5）。
 * 取不到時兩欄位皆為 null（best-effort）。
 *
 * <p><b>{@code notifyBuyerAccountId}</b>：由 {@code buyerId}（買家業務值 buyer_id）
 * 經 BuyerGrpcClient.resolvePrincipals 反解析而得的登入 account_id，僅供
 * notification-service 的 {@code order.paid} 模板路由使用
 * （{@code target_user_field} 指向此欄位；buyer_id 8-service rekey Phase 1，鏡像
 * {@code OrderCompletedPayload.notifySellerAccountId}）。解析失敗時帶 0（notification-service
 * 端 skip），不阻斷本次 order.paid 派送主流程。
 *
 * <p><b>{@code correlationId}</b>（58 §6.3.2.1）：付款事件消費當下的 obs correlation_id
 * （{@code ObsContext.getCorrelationIdOrNull()}）。invoice-service 把它寫進
 * {@code invoice_status_history.correlation_id} 與 {@code invoice_admin_audit_logs.correlation_id}，
 * ADM 稽核 UI 靠它一鍵對到 Loki 與 Tempo。<b>不能改走 AMQP header</b>：{@code pkg/mq} 的
 * consumer 不從 header 取 correlation，讀 payload 是目前唯一可行的機制。取不到時為 null
 * ——那是「這條鏈沒有 correlation」的誠實表示，不自己補一個 UUID（補了會產生一個
 * 對不到任何 log 的假 id，比空值更難查）。
 *
 * <p><b>{@code invoiceSnapshotId}</b>（58 §6.2.4 order ⑤）：{@code order_invoice_snapshots.id}
 * 的 TSID <b>數字字串</b>（不是 Long——int64 進 generic-map consumer 會被 float64 截斷）。
 * 讓 invoice-service 不必先猜「這張訂單有沒有快照」；真正的讀取仍走
 * {@code GetOrderInvoiceSnapshotInternal}（本欄只是 correlation 與除錯用的指標）。
 * 查不到快照時為 null。
 */
@JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy.class)
public record OrderPaidPayload(
        String eventId,
        String orderNo,
        String shippingMethod,
        String occurredAt,
        Long buyerId,
        Long notifyBuyerAccountId,
        Long storeId,
        Long totalActualAmount,
        String sellerName,
        String sellerAvatarKey,
        String buyerNickname,
        String buyerAvatarKey,
        String correlationId,
        String invoiceSnapshotId
) {
    public OrderPaidPayload(
            String orderNo,
            String shippingMethod,
            String occurredAt,
            Long buyerId,
            Long notifyBuyerAccountId,
            Long storeId,
            Long totalActualAmount,
            String sellerName,
            String sellerAvatarKey,
            String buyerNickname,
            String buyerAvatarKey,
            String correlationId,
            String invoiceSnapshotId
    ) {
        this(UUID.randomUUID().toString(), orderNo, shippingMethod, occurredAt,
                buyerId, notifyBuyerAccountId, storeId, totalActualAmount,
                sellerName, sellerAvatarKey, buyerNickname, buyerAvatarKey,
                correlationId, invoiceSnapshotId);
    }
}
