package io.github.will413028.hawkr.aftersales.mq;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.config.AfterSalesConstants;
import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import io.github.will413028.hawkr.common.utils.code.helper.ReturnShippingRules;
import io.github.will413028.hawkr.aftersales.service.AfterSalesEventPublisher;
import io.github.will413028.hawkr.aftersales.service.EventPrincipals;
import io.github.will413028.hawkr.aftersales.service.EventPublishFailedException;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.utils.code.AfterSalesEventType;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAuditActor;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAuditAction;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.service.AfterSaleAuditService;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.AfterSaleCaseStatus;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.time.LocalDateTime;
import java.util.HashMap;
import java.util.Map;
import java.util.Set;

/**
 * 退貨物流狀態變更消費者（§5.3.4）。
 *
 * <p>2026-05-22 逆物流退貨政策修訂：
 * <ul>
 *   <li>PayUni 碼 32（WAITING_PICKUP）+ method=3002 → RETURN_AT_STORE(26)，stamp 72h deadline，發 FCM「退貨商品已抵達門市」</li>
 *   <li>PayUni 碼 11（PICKED_UP）→ RETURN_DELIVERED(27)，stamp 48h deadline + seller_picked_up_at，發 FCM「退貨商品已送達」</li>
 *   <li>PayUni 碼 33/53/44（異常 / 廠退 / 遺失）→ RETURN_FAILED(28)，stamp provider_status/message</li>
 *   <li>PayUni 碼 55（SELLER_NOT_PICKED，僅 C2C）→ 不變狀態，僅 stamp provider_status/message（我方 72h timer 是唯一真實來源）</li>
 *   <li>碼 31/92（IN_TRANSIT / PROCESSING）→ RETURN_SHIPPING(25)（既有路徑）</li>
 * </ul>
 */
@Component
public class ReturnShipmentStatusConsumer {

    private static final ObsLogger log = ObsLogger.forClass(ReturnShipmentStatusConsumer.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();


    private final AfterSaleCaseMapper caseMapper;
    private final AfterSaleAuditService auditService;
    private final AfterSalesProperties properties;
    private final AfterSalesEventPublisher eventPublisher;
    /**
     * 出路 4 的 tx 載體（《多副本排程搶佔規範》§四）。四個 {@code markReturnXxx} 都是
     * <b>永久狀態轉換</b>（20→25→26→27／→28，候選述詞是 {@code status NOT IN (CLOSED, CANCELLED)}
     * 加上各自的 allowed-from），CAS 一旦提交、事件又被吞掉，重投時 CAS 回 0 ⇒ 通知永久遺失。
     * ⇒ CAS ＋ publish 必須同生共死。
     *
     * <p>同 class self-invocation ⇒ 用 TransactionTemplate 程式式包覆，不能標 {@code @Transactional}
     * （Spring AOP CGLIB proxy 不攔截 {@code this.handleXxx()}）。
     */
    private final TransactionTemplate txTemplate;

    public ReturnShipmentStatusConsumer(AfterSaleCaseMapper caseMapper,
                                         AfterSaleAuditService auditService,
                                         AfterSalesProperties properties,
                                         AfterSalesEventPublisher eventPublisher,
                                         PlatformTransactionManager txManager) {
        this.caseMapper = caseMapper;
        this.auditService = auditService;
        this.properties = properties;
        this.eventPublisher = eventPublisher;
        this.txTemplate = new TransactionTemplate(txManager);
    }

    @RabbitListener(queues = AfterSalesEventMqConfig.QUEUE_RETURN_STATUS,
            containerFactory = "manualAckContainerFactory")
    public void onReturnStatusChanged(org.springframework.amqp.core.Message message, Channel channel,
                                       @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);

        try {
            JsonNode payload = MAPPER.readTree(body);

            // 非退貨物流事件，直接跳過
            boolean isReturn = payload.path("is_return").asBoolean(false);
            if (!isReturn) {
                channel.basicAck(deliveryTag, false);
                return;
            }

            String orderNo = payload.path("order_no").asText("");
            String shippingStatus = payload.path("status").asText("");
            String shipmentNo = payload.path("shipment_no").asText("");

            if (orderNo.isEmpty() || shippingStatus.isEmpty()) {
                log.warn("ReturnShipmentStatusConsumer.invalid",
                        "缺少必要欄位, orderNo=" + orderNo + ", shippingStatus=" + shippingStatus);
                channel.basicAck(deliveryTag, false);
                return;
            }

            // 查詢進行中的售後案件（依 orderNo）
            AfterSaleCaseEntity caseEntity = caseMapper.selectOne(
                    new LambdaQueryWrapper<AfterSaleCaseEntity>()
                            .eq(AfterSaleCaseEntity::getOrderNo, orderNo)
                            .notIn(AfterSaleCaseEntity::getStatus,
                                    AfterSaleCaseStatus.CLOSED.value(),
                                    AfterSaleCaseStatus.CANCELLED.value())
                            .last("LIMIT 1")
            );

            if (caseEntity == null) {
                log.info("ReturnShipmentStatusConsumer.noCaseFound",
                        "找不到進行中的售後案件", "orderNo", orderNo);
                channel.basicAck(deliveryTag, false);
                return;
            }

            String caseNo = caseEntity.getCaseNo();

            // 🔴 ADR-013：分類走 common-core 的 canonical 分類器，入參是 ShippingStatus 枚舉。
            //    值域外一律 fail-loud（fromValue 拋例外 → 下方 catch → MqErrorHandler → 重試耗盡進 DLQ），
            //    **對齊正向 leg**。修復前這裡是 `else` ＋ log.info ＋ ACK ＝ 靜默丟棄，
            //    而代價是買家已交件卻被判定沒寄回、7 天後自動關成「不退款」終態（66 §7.3.2 U5）。
            ShippingStatus normalized = ShippingStatus.fromValue(shippingStatus);
            ReturnShippingRules.Disposition disposition = ReturnShippingRules.dispositionOf(normalized);

            switch (disposition) {
                case RETURN_IN_TRANSIT -> handleInTransit(caseEntity, shippingStatus, shipmentNo);
                case RETURN_STORE_ARRIVED -> handleStoreArrived(caseEntity, shippingStatus, shipmentNo);
                case RETURN_DELIVERED -> handleDelivered(caseEntity, shippingStatus, shipmentNo);
                case RETURN_ABNORMAL -> handleAbnormal(caseEntity, shippingStatus, shipmentNo, payload);
                case SELLER_NOT_PICKED -> handleSellerNotPicked(caseEntity, shippingStatus, payload);
                case NO_OP ->
                        // 🔴 **明確的 no-op，不是 else**：包裹還在買家手上（例如 Uber 逆向的
                        //    933 已指派外送員）。要分得出「我們決定不做事」與「我們沒認出這個值」。
                        log.info("ReturnShipmentStatusConsumer.noop",
                                "逆向 leg 明確不處理此貨態, status=" + shippingStatus, "caseNo", caseNo);
                case REQUIRES_MANUAL_REVIEW ->
                        // 🔴 值域內但逆向 leg 無對應售後語意（66 詳設 UP-8：Uber 逆向 canceled）。
                        //    誠實標註交人工，**不猜**一個狀態推進去。
                        log.warn("ReturnShipmentStatusConsumer.requiresManualReview",
                                "逆向 leg 收到無對應售後語意的貨態，需人工處置（66 UP-8）, status="
                                        + shippingStatus + ", orderNo=" + orderNo,
                                "caseNo", caseNo);
            }

            channel.basicAck(deliveryTag, false);
        } catch (Exception e) {
            log.error("ReturnShipmentStatusConsumer.error", "處理退貨物流狀態事件失敗", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "ReturnShipmentStatusConsumer", e);
        }
    }

    /**
     * 在進 tx 之前反解身分欄位（出路 4 硬性邊界：tx 內只准有那一列 CAS 與那一次 publish）。
     *
     * <p>走 best-effort（seller 失敗回 null、buyer 失敗回 0L）而非 strict：本路徑是 MQ consumer，
     * {@code MqErrorHandler} 只重投 3 次就進 DLQ，為了通知路由欄位把整筆物流狀態推進 DLQ
     * 代價過高。與修改前 {@code publishCaseTransitionWithContext} 內部的降級語意逐字相同，
     * 本次改的只是<b>反解發生的位置</b>與 publish 的失敗訊號。
     */
    private EventPrincipals resolvePrincipals(AfterSaleCaseEntity caseEntity) {
        return eventPublisher.resolveEventPrincipalsBestEffort(
                caseEntity.getStoreId(), caseEntity.getBuyerId());
    }

    /**
     * 發事件，失敗即拋 → 短 tx rollback → 狀態沒轉、事件沒發，等下一次重投。
     *
     * <p>不這麼做的話：CAS 已提交（狀態永久前進）、publish 被吞 ⇒ 重投時 CAS 回 0 走冪等 skip
     * ⇒ 該則買賣家通知<b>永久遺失且零告警</b>。
     */
    private void publishOrRollback(String routingKey, String eventType,
                                    AfterSaleCaseEntity caseEntity,
                                    Map<String, Object> extra,
                                    EventPrincipals principals) {
        boolean ok = eventPublisher.tryPublishCaseTransitionWithContext(
                routingKey, eventType,
                caseEntity.getCaseNo(), caseEntity.getOrderNo(),
                caseEntity.getBuyerId(), caseEntity.getStoreId(),
                "system", extra, principals);
        if (!ok) {
            throw new EventPublishFailedException(eventType, caseEntity.getCaseNo());
        }
    }

    /** 碼 31/92：RETURN_PENDING(20) → RETURN_SHIPPING(25) */
    private void handleInTransit(AfterSaleCaseEntity caseEntity, String shippingStatus, String shipmentNo) {
        EventPrincipals principals = resolvePrincipals(caseEntity);
        txTemplate.executeWithoutResult(status ->
                inTransitBody(caseEntity, shippingStatus, shipmentNo, principals));
    }

    /** 出路 4 tx body — 必須由 caller 包進 txTemplate。禁止標 @Transactional（self-invocation）。 */
    private void inTransitBody(AfterSaleCaseEntity caseEntity, String shippingStatus, String shipmentNo,
                               EventPrincipals principals) {
        String caseNo = caseEntity.getCaseNo();
        int rows = caseMapper.markReturnInTransit(caseNo);
        if (rows > 0) {
            auditService.log(caseNo, null, null, AfterSaleAuditActor.SYSTEM.value(),
                    AfterSaleAuditAction.RETURN_IN_TRANSIT.value(),
                    Map.of("shipmentNo", shipmentNo, "shippingStatus", shippingStatus));

            // #1226：補發 case transition 事件 → 賣家通知「買家已寄出退貨」+ 賣家工作區 realtime
            // （與 handleStoreArrived / handleDelivered 一致；原本此轉換不發任何 domain event）
            Map<String, Object> extra = new HashMap<>();
            extra.put("shipping_method", caseEntity.getReturnShippingMethod());
            publishOrRollback(
                    AfterSalesEventType.CASE_RETURN_IN_TRANSIT.value(),
                    MqConstants.EVENT_TYPE_AFTERSALES_CASE_RETURN_IN_TRANSIT,
                    caseEntity, extra, principals);

            log.info("ReturnShipmentStatusConsumer.transitUpdate",
                    "退貨運送中, shipmentNo=" + shipmentNo, "caseNo", caseNo);
        } else {
            log.info("ReturnShipmentStatusConsumer.transitSkipped",
                    "case 已不在 RETURN_PENDING(20)，跳過, shipmentNo=" + shipmentNo, "caseNo", caseNo);
        }
    }

    /**
     * 碼 32：RETURN_PENDING(20) / RETURN_SHIPPING(25) → RETURN_AT_STORE(26)
     * 僅超商退貨（returnShippingMethod=3002）才處理；宅配收到 32 直接 skip。
     * 退貨跳號 / 到店事件先於在途事件到達時，可自 RETURN_PENDING(20) 直達（downward-closure）。
     */
    private void handleStoreArrived(AfterSaleCaseEntity caseEntity, String shippingStatus, String shipmentNo) {
        if (!AfterSalesConstants.RETURN_SHIPPING_METHOD_CVS.equals(caseEntity.getReturnShippingMethod())) {
            log.info("ReturnShipmentStatusConsumer.storeArrivedIgnored",
                    "非超商退貨方式收到碼 32，忽略, method=" + caseEntity.getReturnShippingMethod(),
                    "caseNo", caseEntity.getCaseNo());
            return;
        }
        EventPrincipals principals = resolvePrincipals(caseEntity);
        txTemplate.executeWithoutResult(status ->
                storeArrivedBody(caseEntity, shippingStatus, shipmentNo, principals));
    }

    /** 出路 4 tx body — 必須由 caller 包進 txTemplate。禁止標 @Transactional（self-invocation）。 */
    private void storeArrivedBody(AfterSaleCaseEntity caseEntity, String shippingStatus, String shipmentNo,
                                  EventPrincipals principals) {
        String caseNo = caseEntity.getCaseNo();
        LocalDateTime now = DateTimeUtils.nowUtc();
        LocalDateTime storePickupDeadlineAt = now.plusHours(
                properties.getReturnConfig().getStorePickupTimeoutHours());
        int rows = caseMapper.markReturnAtStore(caseNo, now, storePickupDeadlineAt);

        if (rows > 0) {
            auditService.log(caseNo, null, null, AfterSaleAuditActor.SYSTEM.value(),
                    AfterSaleAuditAction.RETURN_IN_TRANSIT.value(),
                    Map.of("shipmentNo", shipmentNo, "shippingStatus", shippingStatus,
                            "storeArrivedAt", now.toString(),
                            "storePickupDeadlineAt", storePickupDeadlineAt.toString()));

            Map<String, Object> extra = new HashMap<>();
            extra.put("shipping_method", caseEntity.getReturnShippingMethod());
            extra.put("deadline_at", storePickupDeadlineAt.toString());

            publishOrRollback(
                    AfterSalesEventType.CASE_RETURN_AT_STORE.value(),
                    MqConstants.EVENT_TYPE_AFTERSALES_CASE_RETURN_AT_STORE,
                    caseEntity, extra, principals);

            log.info("ReturnShipmentStatusConsumer.storeArrived",
                    "退貨到店待取 → RETURN_AT_STORE(26), storePickupDeadlineAt=" + storePickupDeadlineAt
                            + ", shipmentNo=" + shipmentNo, "caseNo", caseNo);
        } else {
            log.info("ReturnShipmentStatusConsumer.storeArrivedSkipped",
                    "case 已不在 RETURN_PENDING(20)/RETURN_SHIPPING(25)，跳過, shipmentNo=" + shipmentNo, "caseNo", caseNo);
        }
    }

    /**
     * 碼 11：RETURN_PENDING(20) / RETURN_SHIPPING(25) / RETURN_AT_STORE(26) → RETURN_DELIVERED(27)
     * 宅配：25→27 直接（包裹簽收到賣家）；超商：26→27（賣家從超商取貨完成）；
     * 退貨跳號 / 送達事件先於在途/到店事件到達時，可自 RETURN_PENDING(20) 直達（downward-closure）。
     */
    private void handleDelivered(AfterSaleCaseEntity caseEntity, String shippingStatus, String shipmentNo) {
        EventPrincipals principals = resolvePrincipals(caseEntity);
        txTemplate.executeWithoutResult(status ->
                deliveredBody(caseEntity, shippingStatus, shipmentNo, principals));
    }

    /** 出路 4 tx body — 必須由 caller 包進 txTemplate。禁止標 @Transactional（self-invocation）。 */
    private void deliveredBody(AfterSaleCaseEntity caseEntity, String shippingStatus, String shipmentNo,
                               EventPrincipals principals) {
        String caseNo = caseEntity.getCaseNo();
        LocalDateTime now = DateTimeUtils.nowUtc();
        LocalDateTime autoConfirmAt = now.plusHours(
                properties.getReturnConfig().getAutoConfirmReceiptHours());
        int rows = caseMapper.markReturnDelivered(caseNo, now, autoConfirmAt, now);

        if (rows > 0) {
            auditService.log(caseNo, null, null, AfterSaleAuditActor.SYSTEM.value(),
                    AfterSaleAuditAction.RETURN_DELIVERED.value(),
                    Map.of("shipmentNo", shipmentNo, "shippingStatus", shippingStatus,
                            "autoConfirmAt", autoConfirmAt.toString(),
                            "sellerPickedUpAt", now.toString()));

            Map<String, Object> extra = new HashMap<>();
            extra.put("shipping_method", caseEntity.getReturnShippingMethod());
            extra.put("deadline_at", autoConfirmAt.toString());

            publishOrRollback(
                    AfterSalesEventType.CASE_RETURN_DELIVERED.value(),
                    MqConstants.EVENT_TYPE_AFTERSALES_CASE_RETURN_DELIVERED,
                    caseEntity, extra, principals);

            log.info("ReturnShipmentStatusConsumer.delivered",
                    "退貨已送達 → RETURN_DELIVERED(27), autoConfirmAt=" + autoConfirmAt
                            + ", shipmentNo=" + shipmentNo, "caseNo", caseNo);
        } else {
            log.info("ReturnShipmentStatusConsumer.deliveredSkipped",
                    "case 已不在 RETURN_PENDING(20)/RETURN_SHIPPING(25)/RETURN_AT_STORE(26)，跳過, shipmentNo=" + shipmentNo,
                    "caseNo", caseNo);
        }
    }

    /** 碼 33/53/44：任意進行中 → RETURN_FAILED(28) */
    private void handleAbnormal(AfterSaleCaseEntity caseEntity, String shippingStatus, String shipmentNo,
                                 JsonNode payload) {
        EventPrincipals principals = resolvePrincipals(caseEntity);
        txTemplate.executeWithoutResult(status ->
                abnormalBody(caseEntity, shippingStatus, shipmentNo, payload, principals));
    }

    /** 出路 4 tx body — 必須由 caller 包進 txTemplate。禁止標 @Transactional（self-invocation）。 */
    private void abnormalBody(AfterSaleCaseEntity caseEntity, String shippingStatus, String shipmentNo,
                              JsonNode payload, EventPrincipals principals) {
        String caseNo = caseEntity.getCaseNo();
        String providerStatus = payload.path("provider_status").asText("");
        String providerMessage = payload.path("provider_message").asText("");
        int rows = caseMapper.markReturnFailed(caseNo,
                providerStatus.isEmpty() ? shippingStatus : providerStatus,
                providerMessage.isEmpty() ? null : providerMessage);
        if (rows > 0) {
            Map<String, Object> auditExtra = new HashMap<>();
            auditExtra.put("shipmentNo", shipmentNo);
            auditExtra.put("shippingStatus", shippingStatus);
            auditExtra.put("providerStatus", providerStatus);
            auditExtra.put("providerMessage", providerMessage);
            auditService.log(caseNo, null, null, AfterSaleAuditActor.SYSTEM.value(),
                    AfterSaleAuditAction.RETURN_ABNORMAL.value(), auditExtra);

            // extraPayload=null ＝ 原 publishCaseTransition 的等價形式（它就是 WithContext(..., null)）
            publishOrRollback(
                    AfterSalesEventType.CASE_RETURN_FAILED.value(),
                    MqConstants.EVENT_TYPE_AFTERSALES_CASE_RETURN_FAILED,
                    caseEntity, null, principals);

            log.warn("ReturnShipmentStatusConsumer.failed",
                    "退貨物流失敗 → RETURN_FAILED(28), caseNo=" + caseNo
                            + ", providerStatus=" + providerStatus
                            + ", providerMessage=" + providerMessage);
        } else {
            log.info("ReturnShipmentStatusConsumer.failedSkipped",
                    "case 已 28 或終態，跳過: caseNo=" + caseNo + ", status=" + shippingStatus);
        }
    }

    /**
     * 碼 55（SELLER_NOT_PICKED，僅 C2C）：**不變狀態**，僅 stamp provider 訊息。
     * 我方 72h timer 是賣家未取自動完成的唯一真實來源；PayUni 碼 55 因保留期較長，
     * 通常在我方 case 已 status=30 之後才到達。本處理僅供 Admin / Seller UI 顯示參考。
     */
    private void handleSellerNotPicked(AfterSaleCaseEntity caseEntity, String shippingStatus, JsonNode payload) {
        String caseNo = caseEntity.getCaseNo();
        String providerStatus = payload.path("provider_status").asText("");
        String providerMessage = payload.path("provider_message").asText("");
        if (providerStatus.isEmpty()) providerStatus = shippingStatus;
        if (providerMessage.isEmpty()) providerMessage = "賣家未取（PayUni 碼 55）";

        caseMapper.stampReturnProviderInfo(caseNo, providerStatus, providerMessage);

        log.info("ReturnShipmentStatusConsumer.sellerNotPickedStamped",
                "PayUni 碼 55 賣家未取訊息持久化（不變狀態），status=" + caseEntity.getStatus()
                        + ", providerStatus=" + providerStatus, "caseNo", caseNo);
    }
}
