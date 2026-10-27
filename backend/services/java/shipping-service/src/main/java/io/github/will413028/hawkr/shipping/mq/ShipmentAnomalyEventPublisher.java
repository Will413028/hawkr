package io.github.will413028.hawkr.shipping.mq;

import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.payload.ShipmentAnomalyDetectedPayload;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.ShipmentAnomalySeverity;
import io.github.will413028.hawkr.common.utils.code.ShipmentAnomalyType;
import io.github.will413028.hawkr.common.utils.code.helper.ShipmentAnomalyRules;
import io.github.will413028.hawkr.seller.grpc.BatchGetSellerMetaResponse;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import io.github.will413028.hawkr.shipping.client.BuyerGrpcClient;
import io.github.will413028.hawkr.shipping.client.SellerGrpcClient;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.stereotype.Component;

import java.util.List;
import java.util.UUID;

/**
 * §5.21 配送異常賣家通知事件 publisher。
 *
 * <p>由 5 個既有偵測點共用（{@link io.github.will413028.hawkr.shipping.scheduler.TcatCallStuckAlertScheduler}、
 * {@link io.github.will413028.hawkr.shipping.scheduler.ShipmentLabelReconcileScheduler}、
 * {@link io.github.will413028.hawkr.shipping.scheduler.ShipmentStatusReconcileScheduler} 等），
 * 偵測到 anomaly 時 fire-and-forget 發出 event 給 notification-service 消費。
 *
 * <p>失敗策略：只 log error，不丟例外 — 異常通知非主流程，不可影響 scheduler / consumer 既有業務。
 *
 * <p>設計書：06 §5.21.5、36 §3.3
 */
@Component
public class ShipmentAnomalyEventPublisher {

    private static final ObsLogger log = ObsLogger.forClass(ShipmentAnomalyEventPublisher.class);

    private final ObsRabbitPublisher rabbitPublisher;
    private final ObjectMapper objectMapper;
    private final SellerGrpcClient sellerGrpcClient;
    private final BuyerGrpcClient buyerGrpcClient;

    public ShipmentAnomalyEventPublisher(ObsRabbitPublisher rabbitPublisher, ObjectMapper objectMapper,
                                          SellerGrpcClient sellerGrpcClient, BuyerGrpcClient buyerGrpcClient) {
        this.rabbitPublisher = rabbitPublisher;
        this.objectMapper = objectMapper;
        this.sellerGrpcClient = sellerGrpcClient;
        this.buyerGrpcClient = buyerGrpcClient;
    }

    /**
     * 發布 shipment.anomaly.detected event。
     *
     * @param type           異常類型（不可為 null）
     * @param orderNo        訂單編號
     * @param shipmentId     shipments.id（toString）；RETURN_STUCK 可傳空字串
     * @param storeId 賣家 store 代理鍵（ShipmentEntity.storeId）；本方法內部會解析為
     *                登入 principal account_id 後才放進 payload.sellerAccountId，
     *                供 gateway per-user 推播路由使用
     * @param buyerId 買家 buyer_id 代理鍵（ShipmentEntity.buyerId，Phase 2 已改名）；
     *                本方法內部會解析為登入 principal account_id
     *                後才放進 payload.buyerAccountId，與 storeId 對稱處理（可為 null）
     */
    public void publishAnomalyDetected(ShipmentAnomalyType type,
                                       String orderNo,
                                       String shipmentId,
                                       Long storeId,
                                       Long buyerId) {
        // 單一 choke point：TcatCallStuckAlertScheduler / ShipmentStatusReconcileScheduler /
        // ShipmentLabelReconcileScheduler / ShippingService 共 7 個呼叫點都經此解析，無需逐一修改
        Long resolvedSellerAccountId = resolveSellerAccountId(storeId);
        Long resolvedBuyerAccountId = resolveNotifyBuyerAccountId(buyerId);
        try {
            ShipmentAnomalySeverity severity = ShipmentAnomalyRules.severityOf(type);
            ShipmentAnomalyDetectedPayload payload = new ShipmentAnomalyDetectedPayload(
                    UUID.randomUUID().toString(),
                    orderNo,
                    shipmentId,
                    resolvedSellerAccountId,
                    resolvedBuyerAccountId,
                    type.value(),
                    type.label(),
                    severity.value(),
                    DateTimeUtils.formatIso8601(DateTimeUtils.nowUtc()),
                    storeId
            );
            byte[] bytes = objectMapper.writeValueAsBytes(payload);
            rabbitPublisher.publish(
                    MqConstants.EXCHANGE_SHIPPING,
                    MqConstants.ROUTING_KEY_SHIPMENT_ANOMALY_DETECTED,
                    bytes,
                    MqConstants.EVENT_TYPE_SHIPMENT_ANOMALY_DETECTED
            );
            log.info("ShipmentAnomalyEventPublisher.publishAnomalyDetected.success",
                    "配送異常事件發布成功 anomalyType=" + type.value() + " severity=" + severity.value(),
                    "orderNo", orderNo);
        } catch (Exception e) {
            // §5.21 fire-and-forget：不丟例外影響 scheduler / consumer 主流程
            log.error("ShipmentAnomalyEventPublisher.publishAnomalyDetected.error",
                    "配送異常事件發布失敗 orderNo=" + orderNo + " type=" + type.value()
                            + " err=" + e.getMessage());
        }
    }

    /**
     * 解析 store_id → 賣家登入 principal account_id；<b>解析不到就回 {@code 0L}，不回吐 store_id</b>
     * （issue #2789 階段 B2）。
     *
     * <p><b>為什麼是 0 而不是 echo-back</b>：本值唯一的去處是
     * {@code shipment.anomaly.detected.seller} 的 {@code seller_account_id}
     * （notification-service 的 {@code target_user_field}）。echo 回來的 store_id 是
     * 另一個身分空間的合法非零值 —— 它不會投遞到任何人，卻會在 {@code notifications.user_id}
     * 留下一列寫錯身分空間的髒資料，而且<b>完全無聲</b>。回 0 之後
     * {@code template_handler.extractUserID} 對 {@code id <= 0} 回 error ⇒ 跳過該則並留
     * {@code cannot extract user_id} WARN ⇒ 投遞結果不變（本來就送不到），但變成看得見。
     *
     * <p>本 publisher 的 7 個呼叫點中 6 個是可自癒排程（下一 tick 重試），
     * 因此「跳過一則」的代價可接受；{@code storeId == null} 維持回 null（那是「本來就沒有賣家」，
     * 不是「解析失敗」）。
     *
     * <p>⚠️ {@code batchGetSellerMeta} 走 {@code GetSellerMetaByStoreIDs}（PK lookup，
     * 刻意不濾 {@code deleted_at}／{@code is_current}，且<b>不做 COALESCE echo-back</b>），
     * 所以這裡的 map-miss 意義明確就是「查無此 store_id」。
     */
    private Long resolveSellerAccountId(Long storeId) {
        if (storeId == null) {
            return null;
        }
        try {
            BatchGetSellerMetaResponse resp = sellerGrpcClient.batchGetSellerMeta(List.of(storeId));
            SellerMeta seller = resp.getSellersOrDefault(storeId, null);
            if (seller != null) {
                return seller.getAccountId();
            }
            log.warn("ShipmentAnomalyEventPublisher.resolveSellerAccountId.notFound",
                    "seller-service 沒有回傳這個 store_id 的 meta（可能是不存在的 store_id），"
                            + "本則配送異常通知的 sellerAccountId 帶 0 ⇒ notification-service 會跳過並留 WARN，"
                            + "不寫入錯身分空間的髒列",
                    "storeId", String.valueOf(storeId));
        } catch (Exception e) {
            log.warn("resolveSellerAccountId.error",
                    "解析賣家 accountId 失敗，storeId=" + storeId + "，本則通知的 sellerAccountId 帶 0: "
                            + e.getMessage());
        }
        return 0L;
    }

    /**
     * 與 {@link #resolveSellerAccountId(Long)} 對稱：反解 buyer_id(PK，ShipmentEntity.buyerId) →
     * 登入 principal account_id；<b>解析不到同樣回 {@code 0L}</b>，理由見該方法 javadoc。
     *
     * <p>買家軸的 {@code ResolvePrincipals} 會把查無在架列的 id <b>原樣回吐進 map</b>
     * （{@code buyer.proto} 的 {@code unresolved_ids} 逐字寫明那是 UNVERIFIED echo-back），
     * 所以「map 裡沒有」這條分支在買家軸是死碼 —— 真正的判別依據是
     * {@link PrincipalResolution#isUnresolved(Long)}，不是 {@code getOrDefault} 的 default。
     */
    private Long resolveNotifyBuyerAccountId(Long buyerId) {
        if (buyerId == null) {
            return null;
        }
        try {
            PrincipalResolution resolution = buyerGrpcClient.resolvePrincipals(List.of(buyerId));
            if (!resolution.isUnresolved(buyerId)) {
                return resolution.principals().get(buyerId);
            }
            log.warn("ShipmentAnomalyEventPublisher.resolveNotifyBuyerAccountId.unresolved",
                    "buyer-service 回報這個 buyer_id 查無在架買家（unresolved_ids），"
                            + "本則配送異常通知的 buyerAccountId 帶 0，不回吐未驗證的 buyer_id",
                    "buyerId", String.valueOf(buyerId));
        } catch (Exception e) {
            log.warn("resolveNotifyBuyerAccountId.error",
                    "解析買家 accountId 失敗，buyerId=" + buyerId + "，本則通知的 buyerAccountId 帶 0: "
                            + e.getMessage());
        }
        return 0L;
    }
}
