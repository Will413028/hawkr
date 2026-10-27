package io.github.will413028.hawkr.aftersales.mq;

import io.github.will413028.hawkr.aftersales.mq.payload.AppealSubmittedPayload;
import io.github.will413028.hawkr.aftersales.mq.payload.AwaitingBuyerRefundPagePayload;
import io.github.will413028.hawkr.aftersales.mq.payload.CancelCompletedPayload;
import io.github.will413028.hawkr.aftersales.mq.payload.CaseAutoEscalatedPayload;
import io.github.will413028.hawkr.aftersales.mq.payload.CaseAutoRejectedPayload;
import io.github.will413028.hawkr.aftersales.mq.payload.CaseTransitionPayload;
import io.github.will413028.hawkr.aftersales.mq.payload.PlatformDecisionMadePayload;
import io.github.will413028.hawkr.aftersales.mq.payload.RefundCompletedPayload;
import io.github.will413028.hawkr.aftersales.mq.payload.SellerAfterSaleUpdatedPayload;
import io.github.will413028.hawkr.aftersales.service.AfterSalesEventPublisher;
import io.github.will413028.hawkr.aftersales.service.EventPrincipals;
import io.github.will413028.hawkr.aftersales.service.PrincipalResolutionFailedException;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.AfterSalePlatformDecisionOutcome;
import io.github.will413028.hawkr.common.utils.code.AfterSalesEventType;
import io.github.will413028.hawkr.common.utils.code.SellerWorkspaceEventType;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.stereotype.Component;

import java.time.format.DateTimeFormatter;
import java.util.Map;

/**
 * 售後事件 MQ 發佈實作。
 *
 * <p>所有 payload 採 {@code @JsonNaming(SnakeCaseStrategy)} typed record，
 * 對齊 36 §7.4 Idempotency Conformance Contract（snake_case wire + event_id 必含）。
 *
 * <p>Publisher 使用 {@link ObsRabbitPublisher} 發佈事件，自動注入 observability headers。
 */
@Component
public class AfterSalesEventPublisherImpl implements AfterSalesEventPublisher {

    private static final ObsLogger log = ObsLogger.forClass(AfterSalesEventPublisherImpl.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final ObsRabbitPublisher obsRabbitPublisher;
    private final io.github.will413028.hawkr.aftersales.grpc.client.SellerGrpcClient sellerGrpcClient;
    private final io.github.will413028.hawkr.aftersales.grpc.client.BuyerGrpcClient buyerGrpcClient;

    public AfterSalesEventPublisherImpl(ObsRabbitPublisher obsRabbitPublisher,
                                         io.github.will413028.hawkr.aftersales.grpc.client.SellerGrpcClient sellerGrpcClient,
                                         io.github.will413028.hawkr.aftersales.grpc.client.BuyerGrpcClient buyerGrpcClient) {
        this.obsRabbitPublisher = obsRabbitPublisher;
        this.sellerGrpcClient = sellerGrpcClient;
        this.buyerGrpcClient = buyerGrpcClient;
    }

    @Override
    public void publishAwaitingBuyerRefundPage(String orderNo, String caseNo, Long refundTaskId, Long buyerId) {
        publish(MqConstants.EXCHANGE_AFTERSALES,
                AfterSalesEventType.AWAITING_BUYER_PAGE.value(),
                buildAwaitingBuyerRefundPagePayload(orderNo, caseNo, refundTaskId, buyerId),
                MqConstants.EVENT_TYPE_AFTERSALES_AWAITING_BUYER_REFUND_PAGE,
                orderNo);
    }

    @Override
    public boolean tryPublishAwaitingBuyerRefundPage(String orderNo, String caseNo,
                                                      Long refundTaskId, Long buyerId,
                                                      EventPrincipals principals) {
        try {
            return tryPublish(MqConstants.EXCHANGE_AFTERSALES,
                    AfterSalesEventType.AWAITING_BUYER_PAGE.value(),
                    buildAwaitingBuyerRefundPagePayload(orderNo, caseNo, refundTaskId, buyerId,
                            principals.notifyBuyerAccountId()),
                    MqConstants.EVENT_TYPE_AFTERSALES_AWAITING_BUYER_REFUND_PAGE,
                    orderNo);
        } catch (Exception e) {
            log.error("AfterSalesEventPublisherImpl.tryPublishAwaitingBuyerRefundPage.error",
                    "payload 組裝失敗, orderNo=" + orderNo + ", caseNo=" + caseNo, e);
            return false;
        }
    }

    private AwaitingBuyerRefundPagePayload buildAwaitingBuyerRefundPagePayload(
            String orderNo, String caseNo, Long refundTaskId, Long buyerId) {
        return buildAwaitingBuyerRefundPagePayload(orderNo, caseNo, refundTaskId, buyerId,
                resolveNotifyBuyerAccountId(buyerId));
    }

    private AwaitingBuyerRefundPagePayload buildAwaitingBuyerRefundPagePayload(
            String orderNo, String caseNo, Long refundTaskId, Long buyerId,
            Long notifyBuyerAccountId) {
        return new AwaitingBuyerRefundPagePayload(
                orderNo,
                caseNo,
                refundTaskId != null ? String.valueOf(refundTaskId) : null,
                nowIso8601(),
                buyerId != null ? String.valueOf(buyerId) : null,
                String.valueOf(notifyBuyerAccountId));
    }

    @Override
    public void publishCancelCompleted(String orderNo, Long cancelId) {
        publish(MqConstants.EXCHANGE_AFTERSALES,
                AfterSalesEventType.CANCEL_COMPLETED.value(),
                buildCancelCompletedPayload(orderNo, cancelId),
                MqConstants.EVENT_TYPE_AFTERSALES_CANCEL_COMPLETED,
                orderNo);
    }

    @Override
    public boolean tryPublishCancelCompleted(String orderNo, Long cancelId) {
        try {
            return tryPublish(MqConstants.EXCHANGE_AFTERSALES,
                    AfterSalesEventType.CANCEL_COMPLETED.value(),
                    buildCancelCompletedPayload(orderNo, cancelId),
                    MqConstants.EVENT_TYPE_AFTERSALES_CANCEL_COMPLETED,
                    orderNo);
        } catch (Exception e) {
            log.error("AfterSalesEventPublisherImpl.tryPublishCancelCompleted.error",
                    "payload 組裝失敗, orderNo=" + orderNo + ", cancelId=" + cancelId, e);
            return false;
        }
    }

    private CancelCompletedPayload buildCancelCompletedPayload(String orderNo, Long cancelId) {
        return new CancelCompletedPayload(
                orderNo,
                cancelId != null ? String.valueOf(cancelId) : null,
                nowIso8601());
    }

    /**
     * 發佈退款完成事件。
     * 由 RefundResultConsumer / RefundTaskWorker / RefundSyncWorker / AtmRefundReconciler
     * 在退款成功後呼叫。
     *
     * <p>{@code outcome} 強型別必填（FULL_REFUND / PARTIAL_REFUND），決定 order-service 將訂單推進到
     * FULLY_REFUNDED(70) 或 PARTIALLY_REFUNDED(60)。null 一律 fail-loud；REJECT_APPEAL 不應走到此 method
     * （案件被駁回不建立 refund task，不會觸發本事件）。
     *
     * <p>issue #1626：{@code storeId}/{@code buyerId} 解析出的 seller_account_id/notify_buyer_account_id
     * 是這次新增——這個事件從建立起就沒有身份欄位，是 notification-service 持續
     * "cannot extract user_id" 的根因。{@code storeId} 參數原本誤名 {@code sellerAccountId}
     * （呼叫端傳的實際上一直是 store_id），這次一併正名（2026-08-01 commit 7cb0a01c8
     * 的整檔盲改把這句歷史敘述也一起改掉了，2026-08-05 全量掃描修回）。
     */
    public void publishRefundCompleted(String caseNo, String orderNo, Long storeId, Long buyerId,
                                        AfterSalePlatformDecisionOutcome outcome,
                                        Long refundAmount, String paymentMethod, String externalRefId) {
        if (outcome == null) {
            throw new IllegalArgumentException(
                    "publishRefundCompleted: outcome 為必填，caseNo=" + caseNo + ", orderNo=" + orderNo);
        }
        if (outcome == AfterSalePlatformDecisionOutcome.REJECT_APPEAL) {
            throw new IllegalArgumentException(
                    "publishRefundCompleted: REJECT_APPEAL 不應觸發 refund.completed，caseNo=" + caseNo);
        }
        EventPrincipals principals = resolveBestEffort(storeId, buyerId);
        // 退款完成（REFUND_PROCESSING 40 → CLOSED 50）：發 domain event + 補發賣家工作區 realtime（#1226）
        publishCaseEvent(AfterSalesEventType.REFUND_COMPLETED.value(),
                buildRefundCompletedPayload(caseNo, orderNo, storeId, buyerId, outcome,
                        refundAmount, paymentMethod, externalRefId, principals),
                MqConstants.EVENT_TYPE_AFTERSALES_REFUND_COMPLETED,
                storeId, orderNo, caseNo);
    }

    /**
     * {@link #publishRefundCompleted} 的<b>會傳遞失敗 ＋ 零 gRPC</b> 變體。
     *
     * <p>issue #2294 後續：{@code RefundResultConsumer} 的 ATM webhook 成功路徑是
     * 「refund_task → DONE ＋ case → CLOSED(50)」這種<b>永久狀態轉換</b>，重投時
     * {@code isTerminalState} 直接 ack skip、{@code closeIfNotClosed} CAS 回 0
     * ⇒ 事件一旦遺失就<b>永遠不會重來</b>（錢已退、訂單卻永遠停在未退款）。
     * ⇒ 該路徑必須拿得到 publish 的失敗訊號，讓短 tx rollback（《多副本排程搶佔規範》§四 出路 4）。
     *
     * <p>{@code principals} 必須由呼叫端<b>在進 tx 之前</b>解析好——本方法內零 gRPC
     * （出路 4 硬性邊界：tx 內只准有那一列 CAS 與那一次 publish）。
     *
     * @return true = domain event 已送出；false = 送出失敗（已記 error log）
     */
    public boolean tryPublishRefundCompleted(String caseNo, String orderNo, Long storeId, Long buyerId,
                                              AfterSalePlatformDecisionOutcome outcome,
                                              Long refundAmount, String paymentMethod, String externalRefId,
                                              EventPrincipals principals) {
        if (outcome == null) {
            throw new IllegalArgumentException(
                    "tryPublishRefundCompleted: outcome 為必填，caseNo=" + caseNo + ", orderNo=" + orderNo);
        }
        if (outcome == AfterSalePlatformDecisionOutcome.REJECT_APPEAL) {
            throw new IllegalArgumentException(
                    "tryPublishRefundCompleted: REJECT_APPEAL 不應觸發 refund.completed，caseNo=" + caseNo);
        }
        try {
            return tryPublishCaseEvent(AfterSalesEventType.REFUND_COMPLETED.value(),
                    buildRefundCompletedPayload(caseNo, orderNo, storeId, buyerId, outcome,
                            refundAmount, paymentMethod, externalRefId, principals),
                    MqConstants.EVENT_TYPE_AFTERSALES_REFUND_COMPLETED,
                    storeId, orderNo, caseNo);
        } catch (Exception e) {
            log.error("AfterSalesEventPublisherImpl.tryPublishRefundCompleted.error",
                    "payload 組裝失敗, caseNo=" + caseNo + ", orderNo=" + orderNo, e);
            return false;
        }
    }

    private RefundCompletedPayload buildRefundCompletedPayload(
            String caseNo, String orderNo, Long storeId, Long buyerId,
            AfterSalePlatformDecisionOutcome outcome,
            Long refundAmount, String paymentMethod, String externalRefId,
            EventPrincipals principals) {
        Long resolvedSellerAccountId = principals.sellerAccountId();
        return new RefundCompletedPayload(
                caseNo,
                orderNo,
                outcome.value(),
                refundAmount != null ? String.valueOf(refundAmount) : null,
                paymentMethod,
                externalRefId,
                nowIso8601(),
                "system",
                resolvedSellerAccountId != null ? String.valueOf(resolvedSellerAccountId) : null,
                storeId != null ? String.valueOf(storeId) : null,
                buyerId != null ? String.valueOf(buyerId) : null,
                String.valueOf(principals.notifyBuyerAccountId()));
    }

    @Override
    public void publishAppealSubmitted(String caseNo, String orderNo, Long buyerId, Long storeId, String reason) {
        EventPrincipals principals = resolveBestEffort(storeId, buyerId);
        Long resolvedSellerAccountId = principals.sellerAccountId();
        Long notifyBuyerAccountId = principals.notifyBuyerAccountId();
        AppealSubmittedPayload payload = new AppealSubmittedPayload(
                caseNo,
                orderNo,
                buyerId != null ? String.valueOf(buyerId) : null,
                String.valueOf(notifyBuyerAccountId),
                resolvedSellerAccountId != null ? String.valueOf(resolvedSellerAccountId) : null,
                storeId != null ? String.valueOf(storeId) : null,
                reason,
                nowIso8601());

        // 買家提交申訴（→ APPEALED）：發 domain event + 補發賣家工作區 realtime（#1226）
        publishCaseEvent(AfterSalesEventType.APPEAL_SUBMITTED.value(),
                payload,
                MqConstants.EVENT_TYPE_AFTERSALES_APPEAL_SUBMITTED,
                storeId, orderNo, caseNo);
    }

    @Override
    public void publishCaseAutoRejected(String caseNo, String orderNo, Long buyerId,
                                         Long storeId, int originalDeadlineHours) {
        EventPrincipals principals = resolveBestEffort(storeId, buyerId);
        // 賣家處理逾時自動拒絕（→ REJECTED 60）：發 domain event + 補發賣家工作區 realtime（#1226）
        publishCaseEvent(AfterSalesEventType.CASE_AUTO_REJECTED.value(),
                buildCaseAutoRejectedPayload(caseNo, orderNo, buyerId, originalDeadlineHours, principals),
                MqConstants.EVENT_TYPE_AFTERSALES_CASE_AUTO_REJECTED,
                storeId, orderNo, caseNo);
    }

    @Override
    public boolean tryPublishCaseAutoRejected(String caseNo, String orderNo, Long buyerId,
                                               Long storeId, int originalDeadlineHours,
                                               EventPrincipals principals) {
        try {
            return tryPublishCaseEvent(AfterSalesEventType.CASE_AUTO_REJECTED.value(),
                    buildCaseAutoRejectedPayload(caseNo, orderNo, buyerId, originalDeadlineHours, principals),
                    MqConstants.EVENT_TYPE_AFTERSALES_CASE_AUTO_REJECTED,
                    storeId, orderNo, caseNo);
        } catch (Exception e) {
            log.error("AfterSalesEventPublisherImpl.tryPublishCaseAutoRejected.error",
                    "payload 組裝失敗, caseNo=" + caseNo + ", orderNo=" + orderNo, e);
            return false;
        }
    }

    private CaseAutoRejectedPayload buildCaseAutoRejectedPayload(String caseNo, String orderNo, Long buyerId,
                                                                  int originalDeadlineHours,
                                                                  EventPrincipals principals) {
        Long resolvedSellerAccountId = principals.sellerAccountId();
        Long notifyBuyerAccountId = principals.notifyBuyerAccountId();
        return new CaseAutoRejectedPayload(
                caseNo,
                orderNo,
                String.valueOf(buyerId),
                String.valueOf(notifyBuyerAccountId),
                String.valueOf(resolvedSellerAccountId),
                String.valueOf(originalDeadlineHours),
                "system",
                "PROCESS_TIMEOUT",
                nowIso8601());
    }

    @Override
    public void publishCaseAutoEscalated(String caseNo, String orderNo, Long buyerId,
                                          Long storeId, int originalDeadlineHours) {
        EventPrincipals principals = resolveBestEffort(storeId, buyerId);
        // 賣家處理逾時自動升級（→ APPEALED）：發 domain event + 補發賣家工作區 realtime（#1226）
        publishCaseEvent(AfterSalesEventType.CASE_AUTO_ESCALATED.value(),
                buildCaseAutoEscalatedPayload(caseNo, orderNo, buyerId, storeId, originalDeadlineHours, principals),
                MqConstants.EVENT_TYPE_AFTERSALES_CASE_AUTO_ESCALATED,
                storeId, orderNo, caseNo);
    }

    @Override
    public boolean tryPublishCaseAutoEscalated(String caseNo, String orderNo, Long buyerId,
                                                Long storeId, int originalDeadlineHours,
                                                EventPrincipals principals) {
        try {
            return tryPublishCaseEvent(AfterSalesEventType.CASE_AUTO_ESCALATED.value(),
                    buildCaseAutoEscalatedPayload(caseNo, orderNo, buyerId, storeId, originalDeadlineHours, principals),
                    MqConstants.EVENT_TYPE_AFTERSALES_CASE_AUTO_ESCALATED,
                    storeId, orderNo, caseNo);
        } catch (Exception e) {
            log.error("AfterSalesEventPublisherImpl.tryPublishCaseAutoEscalated.error",
                    "payload 組裝失敗, caseNo=" + caseNo + ", orderNo=" + orderNo, e);
            return false;
        }
    }

    private CaseAutoEscalatedPayload buildCaseAutoEscalatedPayload(String caseNo, String orderNo, Long buyerId,
                                                                    Long storeId,
                                                                    int originalDeadlineHours,
                                                                    EventPrincipals principals) {
        Long resolvedSellerAccountId = principals.sellerAccountId();
        Long notifyBuyerAccountId = principals.notifyBuyerAccountId();
        return new CaseAutoEscalatedPayload(
                caseNo,
                orderNo,
                String.valueOf(buyerId),
                String.valueOf(notifyBuyerAccountId),
                String.valueOf(resolvedSellerAccountId),
                storeId != null ? String.valueOf(storeId) : null,
                String.valueOf(originalDeadlineHours),
                "system",
                "PROCESS_TIMEOUT",
                nowIso8601());
    }

    @Override
    public void publishCaseTransition(String routingKey, String eventType,
                                       String caseNo, String orderNo,
                                       Long buyerId, Long storeId,
                                       String actor) {
        publishCaseTransitionWithContext(routingKey, eventType, caseNo, orderNo,
                buyerId, storeId, actor, null);
    }

    @Override
    public void publishCaseTransitionWithContext(String routingKey, String eventType,
                                                  String caseNo, String orderNo,
                                                  Long buyerId, Long storeId,
                                                  String actor,
                                                  Map<String, Object> extraPayload) {
        EventPrincipals principals = resolveBestEffort(storeId, buyerId);
        // [B-207] #1181 + #1226：案件狀態流轉 — 發 domain event + 補發賣家工作區 realtime
        publishCaseEvent(routingKey,
                buildCaseTransitionPayload(caseNo, orderNo, buyerId, storeId, actor, extraPayload, principals),
                eventType, storeId, orderNo, caseNo);
    }

    @Override
    public boolean tryPublishCaseTransitionWithContext(String routingKey, String eventType,
                                                        String caseNo, String orderNo,
                                                        Long buyerId, Long storeId,
                                                        String actor,
                                                        Map<String, Object> extraPayload) {
        return tryPublishCaseTransitionWithContext(routingKey, eventType, caseNo, orderNo,
                buyerId, storeId, actor, extraPayload, resolveBestEffort(storeId, buyerId));
    }

    @Override
    public boolean tryPublishCaseTransitionWithContext(String routingKey, String eventType,
                                                        String caseNo, String orderNo,
                                                        Long buyerId, Long storeId,
                                                        String actor,
                                                        Map<String, Object> extraPayload,
                                                        EventPrincipals principals) {
        try {
            return tryPublishCaseEvent(routingKey,
                    buildCaseTransitionPayload(caseNo, orderNo, buyerId, storeId, actor, extraPayload, principals),
                    eventType, storeId, orderNo, caseNo);
        } catch (Exception e) {
            log.error("AfterSalesEventPublisherImpl.tryPublishCaseTransitionWithContext.error",
                    "payload 組裝失敗, eventType=" + eventType + ", caseNo=" + caseNo, e);
            return false;
        }
    }

    private CaseTransitionPayload buildCaseTransitionPayload(String caseNo, String orderNo,
                                                              Long buyerId,
                                                              Long storeId,
                                                              String actor,
                                                              Map<String, Object> extraPayload,
                                                              EventPrincipals principals) {
        Long resolvedSellerAccountId = principals.sellerAccountId();
        Long notifyBuyerAccountId = principals.notifyBuyerAccountId();
        return new CaseTransitionPayload(
                caseNo,
                orderNo,
                buyerId != null ? String.valueOf(buyerId) : null,
                String.valueOf(notifyBuyerAccountId),
                resolvedSellerAccountId != null ? String.valueOf(resolvedSellerAccountId) : null,
                storeId != null ? String.valueOf(storeId) : null,
                actor,
                nowIso8601(),
                extraPayload);
    }

    @Override
    public void publishPlatformDecisionMade(String caseNo, String orderNo, Long buyerId, Long storeId,
                                             String outcome, Long refundAmount) {
        EventPrincipals principals = resolveBestEffort(storeId, buyerId);
        Long resolvedSellerAccountId = principals.sellerAccountId();
        Long notifyBuyerAccountId = principals.notifyBuyerAccountId();
        PlatformDecisionMadePayload payload = new PlatformDecisionMadePayload(
                caseNo,
                orderNo,
                buyerId != null ? String.valueOf(buyerId) : null,
                String.valueOf(notifyBuyerAccountId),
                resolvedSellerAccountId != null ? String.valueOf(resolvedSellerAccountId) : null,
                storeId != null ? String.valueOf(storeId) : null,
                outcome,
                refundAmount != null ? String.valueOf(refundAmount) : null,
                nowIso8601());

        // [B-207] #1181 + #1226：平台仲裁決策（非賣家自身操作，需即時刷新）— 發 domain event + 補發賣家工作區 realtime
        publishCaseEvent(AfterSalesEventType.PLATFORM_DECISION_MADE.value(),
                payload,
                MqConstants.EVENT_TYPE_AFTERSALES_PLATFORM_DECISION_MADE,
                storeId, orderNo, caseNo);
    }

    // ==================== 身分欄位解析（issue #2294 P1-2） ====================

    /**
     * {@inheritDoc}
     *
     * <p>strict 版：任一側 gRPC 失敗即拋，<b>不降級</b>。只給 tx 外的預先解析用。
     */
    @Override
    public EventPrincipals resolveEventPrincipals(Long storeId, Long buyerId) {
        Long sellerAccountId = sellerGrpcClient.resolveAccountIdForStore(storeId);
        // SellerGrpcClient 對非 null storeId 的成功呼叫必回非 null（查無存活店家回 0L），
        // 因此「storeId 非 null 但結果為 null」⇔「該次 gRPC 失敗」（該 client 內部 catch → 回 null）。
        if (storeId != null && sellerAccountId == null) {
            throw new PrincipalResolutionFailedException("store_id -> seller account_id", storeId);
        }

        if (buyerId == null) {
            return new EventPrincipals(sellerAccountId, 0L);
        }
        try {
            Long notifyBuyerAccountId = buyerGrpcClient.resolvePrincipals(java.util.List.of(buyerId))
                    .principalOrZero(buyerId, "buyer_id");
            return new EventPrincipals(sellerAccountId, notifyBuyerAccountId);
        } catch (Exception e) {
            throw new PrincipalResolutionFailedException("buyer_id -> notify account_id", buyerId, e);
        }
    }

    /**
     * best-effort 版：維持既有降級語意（seller 失敗回 {@code null}、buyer 失敗回 {@code 0L}），
     * 供 API 路徑與既有 {@code publishXxx} 呼叫點使用——它們沒有「下一個 tick」，
     * 不能因為身分反解抖一下就整批失敗。
     *
     * <p>抽出來的第二個作用：把「一次事件最多只反解一次」變成結構性事實。
     * <p>⚠️ 解析結果現在<b>只</b>餵給 domain event payload。{@link #emitSellerAfterSaleUpdated}
     * 自 2026-09-03 起直接用 store_id（賣家工作區 SSE 的訂閱 topic 就是 store_id 空間），
     * 不再需要這個反解值。
     */
    private EventPrincipals resolveBestEffort(Long storeId, Long buyerId) {
        return new EventPrincipals(
                sellerGrpcClient.resolveAccountIdForStore(storeId),
                resolveNotifyBuyerAccountId(buyerId));
    }

    /**
     * {@inheritDoc}
     *
     * <p>issue #2294 B5/B6/B7：把 {@link #resolveBestEffort} 開放給 API 路徑的呼叫端，
     * 讓它們也能「tx 外解好、當參數傳進 tx」——降級語意不變，改變的只是<b>解析發生的位置</b>。
     */
    @Override
    public EventPrincipals resolveEventPrincipalsBestEffort(Long storeId, Long buyerId) {
        return resolveBestEffort(storeId, buyerId);
    }

    // ==================== 私有方法 ====================

    /**
     * 統一 case 事件發送（#1226 根治覆蓋缺口）：發 domain event 到 {@code aftersales.events}
     * + 同步補發賣家工作區 realtime {@code seller.after_sale.updated}。
     *
     * <p>所有「會改 case 狀態」的 publisher 方法一律經此 helper，把「補發 realtime」結構性綁定在
     * 「發 case 事件」這件事上，而非個別方法各自記得呼叫——杜絕日後新增轉換又漏推。
     * {@code storeId} 缺失時 {@link #emitSellerAfterSaleUpdated} 內部跳過 realtime，
     * 不影響 domain event 發送。
     */
    private void publishCaseEvent(String routingKey, Object payload, String eventType,
                                   Long storeId, String orderNo, String caseNo) {
        publish(MqConstants.EXCHANGE_AFTERSALES, routingKey, payload, eventType, orderNo);
        emitSellerAfterSaleUpdated(storeId, orderNo, caseNo);
    }

    /**
     * {@link #publishCaseEvent} 的會傳遞失敗變體（issue #2294 前置 (i)）。
     *
     * <p>回傳值<b>只</b>反映 domain event 的成敗；賣家工作區 realtime 維持 best-effort
     * （它的漏推有前端 {@code ready.bufferMiss} + REST 全刷兜底，不該讓業務 tx rollback）。
     */
    private boolean tryPublishCaseEvent(String routingKey, Object payload, String eventType,
                                         Long storeId, String orderNo, String caseNo) {
        boolean ok = tryPublish(MqConstants.EXCHANGE_AFTERSALES, routingKey, payload, eventType, orderNo);
        emitSellerAfterSaleUpdated(storeId, orderNo, caseNo);
        return ok;
    }

    /**
     * 發布 seller.after_sale.updated 事件（賣家工作區 realtime invalidation，41 §四之二）。
     * best-effort：失敗僅記 error，不影響主流程；漏推由前端 {@code ready.bufferMiss} + REST 全刷兜底。
     * source 固定 {@code after_sale}。{@code storeId} 缺失則跳過（gateway 無法解析 topic）。
     */
    private void emitSellerAfterSaleUpdated(Long storeId, String orderNo, String caseNo) {
        if (storeId == null || storeId <= 0) {
            return;
        }
        // sellerId 直接帶 store_id：gateway 的 SSE 訂閱端
        // （notification_inbox.go#resolveNotificationUserID）自 2026-08-30、commit 57cefff9f
        // 起對賣家回傳 current store_id，訂閱 topic 因此是 notification:user:{store_id}，
        // 而 seller_workspace_channel.go#ResolveTopic 直接拿本 payload 的 sellerId 組發布 topic。
        //
        // 這裡曾經把 storeId 反解析成登入 account_id（理由是「SSE topic 是 account_id」——
        // 該前提已於上述 commit 失效）。兩端空間不同的後果是對 store_id != account_id 的賣家
        // 100% 送不到，且 Hub 以精確鍵 fan-out ⇒ 查無訂閱者不產生任何錯誤或 log。
        SellerAfterSaleUpdatedPayload payload = new SellerAfterSaleUpdatedPayload(
                storeId, orderNo, caseNo, "after_sale", null, nowIso8601());
        publish(MqConstants.EXCHANGE_SELLER_WORKSPACE,
                MqConstants.ROUTING_KEY_SELLER_AFTER_SALE_UPDATED,
                payload,
                SellerWorkspaceEventType.SELLER_AFTER_SALE_UPDATED.value(),
                orderNo);
    }

    /**
     * best-effort 發佈：失敗僅記錄，呼叫端拿不到訊號。
     * <b>只給 API 路徑與「事件遺失可自癒」的路徑用</b>；排程 + 永久狀態轉換一律改用
     * {@link #tryPublish}（issue #2294 前置 (i)）。
     */
    private void publish(String exchange, String routingKey, Object payload,
                          String eventType, String orderNo) {
        tryPublish(exchange, routingKey, payload, eventType, orderNo);
    }

    /**
     * 會傳遞失敗的發佈（issue #2294 前置 (i)）：回傳 false 代表事件沒送出去。
     * 失敗照樣記 error log（觀測性與 best-effort 版一致），差別只在呼叫端拿得到訊號。
     */
    private boolean tryPublish(String exchange, String routingKey, Object payload,
                                String eventType, String orderNo) {
        try {
            byte[] body = MAPPER.writeValueAsBytes(payload);
            obsRabbitPublisher.publish(exchange, routingKey, body, eventType);
            log.info("AfterSalesEventPublisherImpl.publish.success",
                    "事件發布成功, eventType=" + eventType, "orderNo", orderNo);
            return true;
        } catch (Exception e) {
            log.error("AfterSalesEventPublisherImpl.publish.error",
                    "事件發布失敗, eventType=" + eventType + ", orderNo=" + orderNo, e);
            return false;
        }
    }

    private static String nowIso8601() {
        return DateTimeUtils.nowUtc().format(DateTimeFormatter.ISO_LOCAL_DATE_TIME);
    }

    /**
     * 反解 buyer_id → 登入 principal account_id，供 notification-service 推播路由用
     * （device_tokens 以登入 account_id 為鍵）。fail-closed 回 0L + error log（不可靜默吞掉，
     * 0 是安全的「查無此人」路由值，不會誤投給別人）。鏡像 order-service
     * {@code OrderService.completeOrder} 的 {@code notifyBuyerAccountId} 手法。
     * buyer_id 8-service rekey Phase 7 新增。
     *
     * <p>查無存活買家（{@code unresolved_ids}）時回 0L（{@code PrincipalResolution#principalOrZero}，記 warn），
     * 不回吐未反解的 raw buyer_id（單體的 buyer_id 與 account_id 是兩個序列，回吐值是另一個帳號）。
     */
    private Long resolveNotifyBuyerAccountId(Long buyerId) {
        if (buyerId == null) {
            return 0L;
        }
        try {
            return buyerGrpcClient.resolvePrincipals(java.util.List.of(buyerId))
                    .principalOrZero(buyerId, "buyer_id");
        } catch (Exception e) {
            log.error("AfterSalesEventPublisherImpl.resolveNotifyBuyerAccountId.error",
                    "buyer_id -> account_id 解析失敗，買家通知將帶 0，buyerId=" + buyerId, e);
            return 0L;
        }
    }
}
