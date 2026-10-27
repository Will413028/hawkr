package io.github.will413028.hawkr.order.mq;

import io.github.will413028.hawkr.buyer.grpc.BuyerDisplay;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.OrderEventType;
import io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient;
import io.github.will413028.hawkr.order.entity.OrderOutboxEntity;
import io.github.will413028.hawkr.order.grpc.client.BuyerGrpcClient;
import io.github.will413028.hawkr.order.mq.dto.OrderCancelledPayload;
import io.github.will413028.hawkr.order.mq.dto.OrderCompletedPayload;
import io.github.will413028.hawkr.order.mq.dto.OrderDeliveredPayload;
import io.github.will413028.hawkr.order.mq.dto.OrderPaidPayload;
import io.github.will413028.hawkr.order.repository.OrderOutboxMapper;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.stereotype.Component;

import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.Map;

/**
 * 訂單事件發件匣派送器（03 §8.10）。
 *
 * <p>order.paid 在派送前 best-effort 補 seller/buyer meta（payload 形狀與原
 * PaymentEventHandler.afterCommit 等效，下游 IM/通知不受影響）；order.cancelled（#1368）與
 * order.completed（issue #2294 B8）同樣在派送前補身分反解析；其餘事件原樣派送。
 * 失敗時指數退避重試。
 */
@Component
public class OrderOutboxPublisher {

    private static final ObsLogger log = ObsLogger.forClass(OrderOutboxPublisher.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final ObsRabbitPublisher rabbitPublisher;
    private final OrderOutboxMapper outboxMapper;
    private final SellerGrpcClient sellerGrpcClient;
    private final BuyerGrpcClient buyerGrpcClient;

    public OrderOutboxPublisher(ObsRabbitPublisher rabbitPublisher,
                                OrderOutboxMapper outboxMapper,
                                SellerGrpcClient sellerGrpcClient,
                                BuyerGrpcClient buyerGrpcClient) {
        this.rabbitPublisher = rabbitPublisher;
        this.outboxMapper = outboxMapper;
        this.sellerGrpcClient = sellerGrpcClient;
        this.buyerGrpcClient = buyerGrpcClient;
    }

    /**
     * 派送單筆 outbox 事件。
     *
     * @return true=成功，false=失敗（已標 FAILED + backoff）
     */
    public boolean publish(OrderOutboxEntity event) {
        try {
            byte[] body = buildBody(event);
            rabbitPublisher.publish(event.getExchange(), event.getRoutingKey(), body, event.getEventType());

            // issue #2294 H1：具名守衛 UPDATE（status <> SENT），取代 updateById 全欄覆寫——
            // 後者會把另一個 pod 剛寫入的 claim 租約整列蓋掉。
            outboxMapper.markSent(event.getId(), DateTimeUtils.nowUtc());

            log.info("OrderOutboxPublisher.publish.success",
                    "事件已派送 id=" + event.getId() + " routing_key=" + event.getRoutingKey(),
                    "orderNo", event.getAggregateId());
            return true;
        } catch (Exception e) {
            log.error("OrderOutboxPublisher.publish.failed",
                    "事件派送失敗 id=" + event.getId() + " routing_key=" + event.getRoutingKey(), e);
            markFailed(event, e.getMessage());
            return false;
        }
    }

    private byte[] buildBody(OrderOutboxEntity event) throws Exception {
        // order.paid：派送前補 seller/buyer meta（與原 afterCommit 等效）
        if (OrderEventType.ORDER_PAID.value().equals(event.getEventType())) {
            OrderPaidPayload core = MAPPER.readValue(event.getPayload(), OrderPaidPayload.class);
            return MAPPER.writeValueAsBytes(enrich(core));
        }
        // order.cancelled：派送前補 seller/buyer 反解析與 sellerName/avatar（#1368，與原 TX 外直發等效）
        if (OrderEventType.ORDER_CANCELLED.value().equals(event.getEventType())) {
            OrderCancelledPayload core = MAPPER.readValue(event.getPayload(), OrderCancelledPayload.class);
            return MAPPER.writeValueAsBytes(enrichCancelled(core));
        }
        // order.completed：派送前補 seller/buyer 的登入 account_id 反解析（issue #2294 B8，
        // 與原 TX 外直發等效——反解析從完成事務內移到派送時，事務內零 gRPC）
        if (OrderEventType.ORDER_COMPLETED.value().equals(event.getEventType())) {
            OrderCompletedPayload core = MAPPER.readValue(event.getPayload(), OrderCompletedPayload.class);
            return MAPPER.writeValueAsBytes(enrichCompleted(core));
        }
        // order.delivered：派送前補買家的登入 account_id 反解析（本次修復——狀態 CAS 與 outbox
        // 同事務落地後，反解析從 consumer 執行緒移到派送時，事務內零 gRPC）
        if (OrderEventType.ORDER_DELIVERED.value().equals(event.getEventType())) {
            OrderDeliveredPayload core = MAPPER.readValue(event.getPayload(), OrderDeliveredPayload.class);
            return MAPPER.writeValueAsBytes(enrichDelivered(core));
        }
        // order.receipt.confirmed：payload 只有 orderNo / confirmType，無需 enrich
        // order.sales.accumulated：payload 只有 orderNo / occurredAt / items，無身分欄位，無需 enrich
        return event.getPayload().getBytes(StandardCharsets.UTF_8);
    }

    /**
     * order.delivered 派送時 enrich。
     *
     * <p>讀 core payload 的 {@code buyerId}（consumer 從自己 orders 記錄取得的本地事實），
     * 反解析成登入 {@code account_id} 填進 {@code notifyBuyerAccountId}（notification 路由用）。
     * {@code buyerId} 本身維持原始值不動。
     *
     * <p>反解析失敗帶 0（notification-service 端 skip），與原 consumer 內直接反解的降級行為
     * 逐字相同；不阻斷派送（阻斷會讓整筆事件卡在 outbox 重試，代價高於一則路由不到的通知）。
     */
    private OrderDeliveredPayload enrichDelivered(OrderDeliveredPayload core) {
        String notifyBuyerAccountId = resolveOrZero(
                core.buyerId(),
                buyerId -> buyerGrpcClient.resolvePrincipals(List.of(buyerId)).principalOrZero(buyerId, "buyer_id"),
                "OrderOutboxPublisher.enrichDelivered.resolveBuyerFailed",
                "buyer_id -> account_id 解析失敗，買家通知將帶 0，buyerId=");

        // 用完整建構子保留 core 的 eventId（跨重試冪等鍵穩定）
        return new OrderDeliveredPayload(
                core.eventId(), core.orderNo(), core.buyerId(),
                notifyBuyerAccountId, core.autoCompleteAt());
    }

    /**
     * order.completed 派送時 enrich（issue #2294 B8）。
     *
     * <p>讀 core payload 的 {@code storeId} / {@code buyerId}（本地事實、原始業務鍵），
     * 反解析成登入 {@code account_id} 填進 {@code notifySellerAccountId} /
     * {@code notifyBuyerAccountId}（notification 路由用）。
     * {@code storeId} / {@code buyerId} 本身<b>維持原始值不動</b>——crm-service 的統計與
     * {@code GetOrderAggregates} reconcile 以它們為 key。
     *
     * <p>反解析失敗帶 0（notification-service 端 skip），與原 TX 外直發的降級行為逐字相同；
     * 不阻斷派送（阻斷會讓整筆事件卡在 outbox 重試，代價高於一則路由不到的通知）。
     */
    private OrderCompletedPayload enrichCompleted(OrderCompletedPayload core) {
        String notifySellerAccountId = resolveOrZero(
                core.storeId(),
                storeId -> sellerGrpcClient.resolvePrincipals(List.of(storeId))
                        .principalOrZero(storeId, "store_id"),
                "OrderOutboxPublisher.enrichCompleted.resolveSellerFailed",
                "store_id -> account_id 解析失敗，商家通知將帶 0，storeId=");

        String notifyBuyerAccountId = resolveOrZero(
                core.buyerId(),
                buyerId -> buyerGrpcClient.resolvePrincipals(List.of(buyerId)).principalOrZero(buyerId, "buyer_id"),
                "OrderOutboxPublisher.enrichCompleted.resolveBuyerFailed",
                "buyer_id -> account_id 解析失敗，買家通知將帶 0，buyerId=");

        // 用完整建構子保留 core 的 eventId（跨重試冪等鍵穩定）
        return new OrderCompletedPayload(
                core.eventId(), core.orderNo(), core.buyerId(), notifyBuyerAccountId,
                core.storeId(), notifySellerAccountId, core.totalActualAmount(),
                core.confirmType(), core.completedAt());
    }

    /**
     * 把數字字串業務鍵反解析成登入 account_id 字串；失敗回 {@code "0"}。
     */
    private String resolveOrZero(String rawId,
                                 java.util.function.LongFunction<Long> resolver,
                                 String errorEvent,
                                 String errorMessagePrefix) {
        try {
            long id = Long.parseLong(rawId);
            return String.valueOf(resolver.apply(id));
        } catch (Exception e) {
            log.error(errorEvent, errorMessagePrefix + rawId, e);
            return "0";
        }
    }

    /**
     * order.cancelled 派送時 enrich（#1368）——讀 core payload 的 {@code storeId}，反解析成登入
     * account_id 填進 {@code notifySellerAccountId}（notification 商家路由用），並補 buyer 反解析與
     * sellerName/avatar。
     */
    private OrderCancelledPayload enrichCancelled(OrderCancelledPayload core) {
        Long storeId = core.storeId();
        Long buyerId = core.buyerId();

        // 取賣家上下文（best-effort，失敗時 sellerName/sellerAvatarKey 留 null，不阻斷派送）
        String sellerName = null;
        String sellerAvatarKey = null;
        try {
            SellerMeta meta = sellerGrpcClient.getSellerMeta(storeId);
            sellerName = meta.getSellerName();
            String logoKey = meta.getLogoKey();
            if (logoKey != null && !logoKey.isBlank()) {
                sellerAvatarKey = logoKey; // 49 §6.3/S3：MQ payload 只帶 objectKey，消費端 worker 組 URL
            }
        } catch (Exception e) {
            log.error("OrderOutboxPublisher.enrichCancelled.sellerFailed",
                    "取賣家資訊失敗，storeId=" + storeId, e);
        }

        // buyer_id -> 登入 account_id（notification 買家路由，失敗帶 0 → notification-service 端 skip）
        Long notifyBuyerAccountId;
        try {
            notifyBuyerAccountId = buyerGrpcClient.resolvePrincipals(List.of(buyerId))
                    .principalOrZero(buyerId, "buyer_id");
        } catch (Exception e) {
            log.error("OrderOutboxPublisher.enrichCancelled.resolveBuyerFailed",
                    "buyer_id -> account_id 解析失敗，買家通知將帶 0，buyerId=" + buyerId, e);
            notifyBuyerAccountId = 0L;
        }

        // store_id -> 登入 account_id（notification 商家路由；order.cancelled.merchant 模板
        // target_user_field='notify_seller_account_id'）。失敗帶 0。
        Long notifySellerAccountId;
        try {
            notifySellerAccountId = sellerGrpcClient.resolvePrincipals(List.of(storeId))
                    .principalOrZero(storeId, "store_id");
        } catch (Exception e) {
            log.error("OrderOutboxPublisher.enrichCancelled.resolveSellerFailed",
                    "store_id -> account_id 解析失敗，商家通知將帶 0，storeId=" + storeId, e);
            notifySellerAccountId = 0L;
        }

        // 用完整建構子保留 core 的 eventId（跨重試冪等鍵穩定）。
        return new OrderCancelledPayload(
                core.eventId(), core.orderNo(), core.stockTxId(), core.items(), core.occurredAt(),
                buyerId, notifyBuyerAccountId, storeId, notifySellerAccountId,
                sellerName, sellerAvatarKey, core.cancelReason());
    }

    private OrderPaidPayload enrich(OrderPaidPayload core) {
        String sellerName = null;
        String sellerAvatar = null;
        try {
            SellerMeta meta = sellerGrpcClient.getSellerMeta(core.storeId());
            sellerName = meta.getSellerName();
            String logoKey = meta.getLogoKey();
            if (logoKey != null && !logoKey.isBlank()) {
                sellerAvatar = logoKey; // 49 §6.3/S3：MQ payload 只帶 objectKey，消費端 worker 組 URL
            }
        } catch (Exception e) {
            log.error("OrderOutboxPublisher.enrich.sellerFailed",
                    "取賣家資訊失敗，storeId=" + core.storeId(), e);
        }

        String buyerNickname = null;
        String buyerAvatarKey = null;
        try {
            Map<Long, BuyerDisplay> metas = buyerGrpcClient.batchGetBuyerDisplayByBuyerIds(List.of(core.buyerId()));
            BuyerDisplay bm = metas.get(core.buyerId());
            if (bm != null) {
                buyerNickname = bm.getDisplayName();
                buyerAvatarKey = bm.getAvatarKey(); // 49 §6.3/S3：MQ payload 只帶 objectKey，消費端 worker 組 URL
            }
        } catch (Exception e) {
            log.error("OrderOutboxPublisher.enrich.buyerFailed",
                    "取買家資訊失敗，buyerId=" + core.buyerId(), e);
        }

        // buyer_id -> 登入 account_id 解析（notification routing 用，鏡像賣家側 notifySellerAccountId）。
        // 失敗時帶 0（notification-service 端 skip），不阻斷本次 order.paid 派送主流程。
        Long notifyBuyerAccountId;
        try {
            notifyBuyerAccountId = buyerGrpcClient.resolvePrincipals(List.of(core.buyerId()))
                    .principalOrZero(core.buyerId(), "buyer_id");
        } catch (Exception e) {
            log.error("OrderOutboxPublisher.enrich.resolvePrincipalsForNotifyFailed",
                    "buyer_id -> account_id 解析失敗，買家通知將帶 0（notification-service 端 skip），buyerId="
                            + core.buyerId(), e);
            notifyBuyerAccountId = 0L;
        }

        // correlationId / invoiceSnapshotId 由 PaymentEventHandler 在 CAS 交易內寫進 core payload，
        // enrich 只補 seller/buyer meta ⇒ 這兩欄原樣帶過。漏帶不會編譯錯也不會測試紅，
        // 只會讓 invoice-service 的 correlation 欄恆空（58 §6.3.2.1 的失效模式）。
        return new OrderPaidPayload(core.orderNo(), core.shippingMethod(), core.occurredAt(),
                core.buyerId(), notifyBuyerAccountId, core.storeId(), core.totalActualAmount(),
                sellerName, sellerAvatar, buyerNickname, buyerAvatarKey,
                core.correlationId(), core.invoiceSnapshotId());
    }

    /**
     * 標記派送失敗 + 指數退避。
     *
     * <p>issue #2294 H1：具名守衛 UPDATE（{@code status <> SENT}），取代 {@code updateById}
     * 全欄覆寫——後者會把「另一個 pod 已標 SENT」的結果回寫成 FAILED，造成重複投遞一輪。
     * {@code error} 目前只進 log（{@code order_outbox} 無 {@code last_error} 欄）。
     */
    private void markFailed(OrderOutboxEntity event, String error) {
        int retry = (event.getRetryCount() == null ? 0 : event.getRetryCount()) + 1;
        // 指數退避：5s, 15s, 45s, 135s, 405s（上限）
        long delaySec = (long) (5 * Math.pow(3, Math.min(retry - 1, 4)));
        outboxMapper.markFailedGuarded(event.getId(), retry,
                DateTimeUtils.nowUtc().plusSeconds(delaySec));
    }
}
