package io.github.will413028.hawkr.aftersales.mq;

import io.github.will413028.hawkr.aftersales.entity.RefundTaskEntity;
import io.github.will413028.hawkr.aftersales.mapper.RefundTaskMapper;
import io.github.will413028.hawkr.aftersales.service.AfterSaleAuditService;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAuditAction;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAuditActor;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.HashMap;
import java.util.Map;

/**
 * §2.2 業務 pivot (2026-05-14) — UNKNOWN webhook 保守失敗處理（退款側）。
 *
 * <p>訂閱 {@code payuni.events} exchange 的 {@code payuni.unknown.fallback} routing key，
 * 與 {@link io.github.will413028.hawkr.payment} 的 UnknownPaymentFallbackConsumer 並行接收同一 event。</p>
 *
 * <p>邏輯：
 * <ul>
 *   <li>用 payload.MerID 查 {@code refund_tasks} 最近一筆 awaiting (status IN 5,15) row（不限時間窗 — ATM 匯款 webhook 延遲可能多日）</li>
 *   <li>找到且 payment_method=ATM：呼叫 {@link RefundTaskMapper#revertFromAnyAwaitingToAwaitingPage}
 *       將 task 退回 5、stamp PayUni 訊息、attempt++，買家可在 App 看到失敗原因並重填</li>
 *   <li>找不到 / 非 ATM：log.warn 後 ACK（payment 端 consumer 接手或人工處理）</li>
 *   <li>不論結果：log.error 觸發 Obs alert</li>
 * </ul>
 *
 * <p>不違反 docs/開發規範/後端服務責任邊界規範.md：本 consumer 是 webhook 入站當下一次處理（webhook-driven），
 * 非定時 reconcile / polling worker。</p>
 */
@Component
public class UnknownRefundFallbackConsumer {

    private static final ObsLogger log = ObsLogger.forClass(UnknownRefundFallbackConsumer.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final RefundTaskMapper refundTaskMapper;
    private final AfterSaleAuditService auditService;
    private final io.github.will413028.hawkr.aftersales.service.AtmRefundReconciler atmRefundReconciler;
    private final TransactionTemplate txTemplate;

    public UnknownRefundFallbackConsumer(RefundTaskMapper refundTaskMapper,
                                          AfterSaleAuditService auditService,
                                          io.github.will413028.hawkr.aftersales.service.AtmRefundReconciler atmRefundReconciler,
                                          PlatformTransactionManager txManager) {
        this.refundTaskMapper = refundTaskMapper;
        this.auditService = auditService;
        this.atmRefundReconciler = atmRefundReconciler;
        this.txTemplate = new TransactionTemplate(txManager);
    }

    @RabbitListener(queues = MqConstants.QUEUE_AFTERSALES_PAYUNI_UNKNOWN_FALLBACK,
            containerFactory = "manualAckContainerFactory")
    public void onUnknownFallback(Message message, Channel channel,
                                   @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);
        try {
            JsonNode envelope = MAPPER.readTree(body);
            String merId = envelope.path("mer_id").asText("");
            String eventKey = envelope.path("event_key").asText("");
            JsonNode payload = envelope.path("payload");
            String payuniStatus = payload.path("Status").asText("");
            String payuniMessage = payload.path("Message").asText("");

            log.error("UnknownRefundFallbackConsumer.received",
                    "收到 UNKNOWN webhook fallback merId=" + merId + " status=" + payuniStatus
                            + " message=" + payuniMessage + " eventKey=" + eventKey,
                    "eventKey", eventKey);

            if (merId == null || merId.isBlank()) {
                log.warn("UnknownRefundFallbackConsumer.noMerId.skip",
                        "payload 無 MerID 無法綁定任何 refund_task，skip eventKey=" + eventKey);
                channel.basicAck(deliveryTag, false);
                return;
            }

            RefundTaskEntity task = refundTaskMapper.findLatestAwaitingByMerId(merId);
            if (task == null) {
                log.warn("UnknownRefundFallbackConsumer.noAwaitingTask",
                        "MerID=" + merId + " 找不到 status IN (5,15) 的 refund_task，"
                                + "讓 payment fallback consumer 接手或維運處理 eventKey=" + eventKey);
                channel.basicAck(deliveryTag, false);
                return;
            }

            if (!PaymentMethod.ATM.value().equals(task.getPaymentMethod())) {
                // 非 ATM 退款（信用卡 / Apple Pay / Google Pay）由 Worker 主導 retry，
                // UNKNOWN fallback 不介入避免干擾 worker state machine
                log.warn("UnknownRefundFallbackConsumer.nonAtm.skip",
                        "task paymentMethod=" + task.getPaymentMethod() + " 非 ATM，skip refundTaskId="
                                + task.getId());
                channel.basicAck(deliveryTag, false);
                return;
            }

            // 保守失敗處理：訊息為空時用通用文案，讓買家至少知道發生錯誤
            String effectiveMessage = (payuniMessage == null || payuniMessage.isBlank())
                    ? "退款處理發生錯誤，請重新填寫銀行帳號"
                    : payuniMessage;
            String effectiveStatus = (payuniStatus == null || payuniStatus.isBlank())
                    ? "UNKNOWN"
                    : payuniStatus;

            // 缺陷 B（issue #1288）：revert 前先確認 payment 端未終態成功；已 SUCCEEDED → 推進完成，不 revert。
            // P1-2：payment 端狀態查詢（含同步 gRPC）在 tx 外先做，只把判定結果帶進 tx 內做寫入。
            // revert + audit 於同一 tx 內完成（避免部分寫入）。
            final RefundTaskEntity boundTask = task;
            var paymentState = atmRefundReconciler.queryPaymentRefundState(boundTask);
            txTemplate.executeWithoutResult(status -> {
                if (atmRefundReconciler.advanceIfPaymentSucceeded(paymentState, boundTask, effectiveStatus, effectiveMessage)) {
                    return;
                }
                int rows = refundTaskMapper.revertFromAnyAwaitingToAwaitingPage(
                        boundTask.getId(), effectiveStatus, effectiveMessage);
                if (rows == 0) {
                    log.warn("UnknownRefundFallbackConsumer.revert.noop",
                            "task 已不在 5/15 狀態 refundTaskId=" + boundTask.getId());
                    return;
                }

                Map<String, Object> detail = new HashMap<>();
                detail.put("refundTaskId", boundTask.getId());
                detail.put("merId", merId);
                detail.put("payuniStatus", effectiveStatus);
                detail.put("payuniMessage", effectiveMessage);
                detail.put("previousStatus", boundTask.getStatus());
                detail.put("source", "UNKNOWN_FALLBACK");
                detail.put("eventKey", eventKey);
                auditService.log(boundTask.getCaseNo(), boundTask.getCancelId(), null,
                        AfterSaleAuditActor.SYSTEM.value(),
                        AfterSaleAuditAction.REFUND_FAILURE_REVERT_TO_AWAITING.value(),
                        detail);

                log.warn("UnknownRefundFallbackConsumer.bound",
                        "UNKNOWN webhook 已綁定到 refund_task，task 退回 AWAITING_BUYER_REFUND_PAGE 買家可重填 refundTaskId="
                                + boundTask.getId());
            });
            channel.basicAck(deliveryTag, false);

        } catch (Exception e) {
            log.error("UnknownRefundFallbackConsumer.error", "處理 UNKNOWN fallback 失敗", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag,
                    "UnknownRefundFallbackConsumer", e);
        }
    }
}
