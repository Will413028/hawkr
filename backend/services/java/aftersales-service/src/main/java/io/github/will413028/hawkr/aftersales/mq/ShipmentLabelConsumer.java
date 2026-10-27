package io.github.will413028.hawkr.aftersales.mq;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.mq.payload.ShipmentAnomalyDetectedPayload;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.ShipmentAnomalySeverity;
import io.github.will413028.hawkr.common.utils.code.ShipmentAnomalyType;
import io.github.will413028.hawkr.common.utils.code.helper.ShipmentAnomalyRules;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.UUID;

/**
 * 7-11 退貨出貨便代碼回填消費者（§5.3.6）。
 *
 * <p>與 {@code order.shipping.label} 同時綁定 {@code shipment.label.updated} routing key。
 * 以 payload 內 {@code isReturn} 欄位分流：
 * <ul>
 *   <li>{@code isReturn=false} → 正向出貨，Order Service 處理，本 consumer 直接 ACK skip</li>
 *   <li>{@code isReturn=true}  → 退貨出貨便，快照 partner_id / odno / validation_no 至 aftersale_cases</li>
 * </ul>
 *
 * <p>冪等：Mapper 的 {@code updateLabelInfo} 含 {@code WHERE partner_id IS NULL} 鎖。
 */
@Component
public class ShipmentLabelConsumer {

    private static final ObsLogger log = ObsLogger.forClass(ShipmentLabelConsumer.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final AfterSaleCaseMapper caseMapper;
    private final ObsRabbitPublisher obsRabbitPublisher;
    private final io.github.will413028.hawkr.aftersales.grpc.client.SellerGrpcClient sellerGrpcClient;
    private final io.github.will413028.hawkr.aftersales.grpc.client.BuyerGrpcClient buyerGrpcClient;

    public ShipmentLabelConsumer(AfterSaleCaseMapper caseMapper,
                                 ObsRabbitPublisher obsRabbitPublisher,
                                 io.github.will413028.hawkr.aftersales.grpc.client.SellerGrpcClient sellerGrpcClient,
                                 io.github.will413028.hawkr.aftersales.grpc.client.BuyerGrpcClient buyerGrpcClient) {
        this.caseMapper = caseMapper;
        this.obsRabbitPublisher = obsRabbitPublisher;
        this.sellerGrpcClient = sellerGrpcClient;
        this.buyerGrpcClient = buyerGrpcClient;
    }

    @RabbitListener(queues = AfterSalesEventMqConfig.QUEUE_SHIPMENT_LABEL,
            containerFactory = "manualAckContainerFactory")
    public void onLabelUpdated(org.springframework.amqp.core.Message message, Channel channel,
                                @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);

        try {
            JsonNode payload = MAPPER.readTree(body);
            boolean isReturn = payload.path("is_return").asBoolean(false);

            // 正向出貨事件由 Order Service 處理，AfterSales 直接 ACK skip
            if (!isReturn) {
                channel.basicAck(deliveryTag, false);
                return;
            }

            String orderNo = payload.path("order_no").asText("");
            String partnerId = payload.path("partner_id").asText("");
            String odno = payload.path("odno").asText("");
            String validationNo = payload.path("validation_no").asText("");

            if (orderNo.isEmpty()) {
                log.error("ShipmentLabelConsumer.invalid", "orderNo 為空");
                channel.basicAck(deliveryTag, false);
                return;
            }

            if (partnerId.isEmpty() || odno.isEmpty() || validationNo.isEmpty()) {
                log.warn("ShipmentLabelConsumer.incomplete",
                        "三段碼不完整，發 PAYUNI_LGS_NO_CODE event 並跳過此事件 orderNo=" + orderNo
                                + " partnerId=" + partnerId + " odno=" + odno
                                + " validationNo=" + validationNo);
                // §5.21 B-143 即時偵測：PayUni 回 SUCCESS 但三段碼缺失 → 立即推 PAYUNI_LGS_NO_CODE 給賣家。
                // **By contract 此偵測限超商**：upstream publisher 為 ShippingService.processWebhookPrint
                // （超商 ApiType=Print webhook 處理路徑），宅配走 markLabelPrintSucceededHome 同步 PDF 路徑
                // 不會發送 shipment.label.updated event。payload 本身未帶 shippingMethod 欄位，
                // 維持既有 contract 不擴充；若未來新增其他 publisher（如非超商來源），務必同步擴充 payload 加守衛。
                publishPayUniLgsNoCodeAnomaly(orderNo);
                channel.basicAck(deliveryTag, false);
                return;
            }

            int rows = caseMapper.updateLabelInfo(orderNo, partnerId, odno, validationNo);

            if (rows == 0) {
                log.info("ShipmentLabelConsumer.idempotent",
                        "找不到活躍退貨案件或代碼已回填（冪等跳過）", "orderNo", orderNo);
            } else {
                log.info("ShipmentLabelConsumer.success",
                        "出貨便代碼快照完成 odno=" + odno, "orderNo", orderNo);
            }

            channel.basicAck(deliveryTag, false);
        } catch (Exception e) {
            log.error("ShipmentLabelConsumer.error", "處理出貨便代碼回填事件失敗", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "ShipmentLabelConsumer", e);
        }
    }

    /**
     * §5.21 B-143 即時偵測：發 PAYUNI_LGS_NO_CODE event 給 notification-service。
     * <p>Fire-and-forget：不影響 consumer 主流程。
     */
    private void publishPayUniLgsNoCodeAnomaly(String orderNo) {
        try {
            // 從 aftersale_cases 查 seller/buyer 上下文（退貨 webhook 路徑下，case 必定存在）
            AfterSaleCaseEntity acase = caseMapper.selectOne(
                    new LambdaQueryWrapper<AfterSaleCaseEntity>()
                            .eq(AfterSaleCaseEntity::getOrderNo, orderNo)
                            .last("LIMIT 1"));
            Long storeId = acase == null ? null : acase.getStoreId();
            Long buyerId = acase == null ? null : acase.getBuyerId();
            // Plan C Phase 3 追加：sellerAccountId 對外是「已反解的登入 account_id」（供
            // notification-service 對地推播），非 aftersale_cases 的 store_id 代理鍵——發布前
            // 反解一次，未知/查詢失敗降級回 null（notification-service 端無法對地則不送達，
            // 與既有「seller 缺失即跳過」的降級哲學一致）。
            Long sellerAccountId = sellerGrpcClient.resolveAccountIdForStore(storeId);
            // 查無存活賣家時 SellerGrpcClient 回 0L（沒有收件人）；本檔的欄位以 null 表示沒有，與買家側相同，
            // notification 端的 extractUserID 對 null 與 0 都略過。
            if (sellerAccountId != null && sellerAccountId <= 0L) {
                sellerAccountId = null;
            }
            // 對稱處理：buyerAccountId 同樣對外是「已反解的登入 account_id」，非 aftersale_cases
            // 的 buyer_id 代理鍵；與 sellerAccountId 同一降級哲學（查詢失敗回 null）。
            Long buyerAccountId = resolveNotifyBuyerAccountId(buyerId);

            ShipmentAnomalyType type = ShipmentAnomalyType.PAYUNI_LGS_NO_CODE;
            ShipmentAnomalySeverity severity = ShipmentAnomalyRules.severityOf(type);
            ShipmentAnomalyDetectedPayload payload = new ShipmentAnomalyDetectedPayload(
                    UUID.randomUUID().toString(),
                    orderNo,
                    "",            // shipmentId 在 aftersales-service 端不可得，留空字串（notification 端可由 orderNo 反查）
                    sellerAccountId,
                    buyerAccountId,
                    type.value(),
                    type.label(),
                    severity.value(),
                    DateTimeUtils.formatIso8601(DateTimeUtils.nowUtc()),
                    storeId
            );
            byte[] bytes = MAPPER.writeValueAsBytes(payload);
            obsRabbitPublisher.publish(
                    MqConstants.EXCHANGE_SHIPPING,
                    MqConstants.ROUTING_KEY_SHIPMENT_ANOMALY_DETECTED,
                    bytes,
                    MqConstants.EVENT_TYPE_SHIPMENT_ANOMALY_DETECTED
            );
            log.info("ShipmentLabelConsumer.anomaly.published",
                    "B-143 三段碼缺失異常已發布，notification 將即時推送賣家", "orderNo", orderNo);
        } catch (Exception e) {
            log.error("ShipmentLabelConsumer.anomaly.publish.failed",
                    "PAYUNI_LGS_NO_CODE event 發布失敗 orderNo=" + orderNo + " err=" + e.getMessage());
        }
    }

    /**
     * 反解 buyer_id(PK，aftersale_cases.buyerId) → 登入 principal account_id，與上方
     * sellerAccountId 反解（{@link io.github.will413028.hawkr.aftersales.grpc.client.SellerGrpcClient#resolveAccountIdForStore}）
     * 同一「查詢失敗降級回 null」哲學。鏡像 {@code AfterSalesEventPublisherImpl.resolveNotifyBuyerAccountId}，
     * 差異僅在失敗時回 null 而非 0L——本欄位在此 payload 中供 notification 渲染上下文用，非路由必要欄位，
     * 與 sellerAccountId 缺失即跳過的既有降級策略一致。
     *
     * <p>查無存活買家（{@code unresolved_ids}）時回 null（與 {@code ReturnStuckAlertScheduler.resolveNotifyBuyerAccountId}
     * 相同），不回吐未反解的 raw buyer_id（單體的 buyer_id 與 account_id 是兩個序列，回吐值是另一個帳號）。
     */
    private Long resolveNotifyBuyerAccountId(Long buyerId) {
        if (buyerId == null) {
            return null;
        }
        try {
            long account = buyerGrpcClient.resolvePrincipals(java.util.List.of(buyerId)).principalOrZero(buyerId, "buyer_id");
            return account > 0L ? account : null;
        } catch (Exception e) {
            log.warn("ShipmentLabelConsumer.resolveNotifyBuyerAccountId.error",
                    "buyer_id 反解 account_id 失敗，降級回 null buyerId=" + buyerId + "，err=" + e.getMessage());
            return null;
        }
    }
}
