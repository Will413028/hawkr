package io.github.will413028.hawkr.order.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.AfterSalePlatformDecisionOutcome;
import io.github.will413028.hawkr.common.utils.code.AfterSalesEventType;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import io.github.will413028.hawkr.order.service.OrderCountdownService;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;

import java.io.IOException;
import java.nio.charset.StandardCharsets;

/**
 * 售後事件 MQ 消費者
 *
 * 監聽三個 Queue：
 * 1. order.aftersales.refund.completed       — 退款完成（交易段委託 AfterSalesRefundEventHandler；commit 後嘗試恢復倒數）
 * 2. order.aftersales.cancel.completed       — 取消出貨完成（清 need_refund_page）
 * 3. order.aftersales.awaiting.buyer.page    — ATM 退款等待買家前往 PayUni 頁面（SET need_refund_page = TRUE）
 *
 * 手動 ACK，at-least-once，冪等處理
 */
@Component
public class AfterSalesEventConsumer {

    private static final ObsLogger log = ObsLogger.forClass(AfterSalesEventConsumer.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final OrderRepository orderRepository;
    private final AfterSalesRefundEventHandler refundEventHandler;
    private final SellerCancelRefundTxHandler sellerCancelRefundTxHandler;
    private final OrderCountdownService countdownService;

    public AfterSalesEventConsumer(OrderRepository orderRepository,
                                    AfterSalesRefundEventHandler refundEventHandler,
                                    SellerCancelRefundTxHandler sellerCancelRefundTxHandler,
                                    OrderCountdownService countdownService) {
        this.orderRepository = orderRepository;
        this.refundEventHandler = refundEventHandler;
        this.sellerCancelRefundTxHandler = sellerCancelRefundTxHandler;
        this.countdownService = countdownService;
    }

    /**
     * 退款完成事件（03 §5.7.1，ORD-02 雙 queue 收斂：單一 consumer）
     *
     * Payload: { event_id, case_no, order_no, refund_outcome, refund_amount, payment_method, external_ref_id, occurred_at }
     *
     * 處理分兩段：
     * 1. 交易段（{@link AfterSalesRefundEventHandler}，獨立 @Service 確保 @Transactional proxy 生效）：
     *    financial_log 冪等守門 → 依 outcome 分流（FULL → 70 CAS；PARTIAL → 單一原子 UPDATE 累加
     *    refund_total_amount，僅 50 轉 60）→ 清 need_refund_page，同事務原子。
     * 2. 倒數恢復段（本方法，交易段 commit 後執行）：tryResume——內含 AfterSales HasActiveCase gRPC
     *    反查，該反查為 fail-safe（例外時吞掉並回「仍有活躍 case」→ 本輪靜默不恢復、不拋錯）。
     *    因此恢復<b>沒有重投保證</b>：反查失敗時訊息仍會被 ack，漏恢復由 CountdownRecoveryWorker
     *    （03 §5.8.7 對帳兜底）補救；僅 resumeCountdown 本身的 DB 例外會走 MqErrorHandler 重投
     *    （交易段有 financial_log 冪等守門，可安全重跑）。
     *
     * Fail-loud（→ MqErrorHandler → DLQ）：refund_outcome 為空 / 不在 enum 內 / REJECT_APPEAL、
     * refund_amount 非合法數字、PARTIAL 且 refund_amount<=0。
     */
    @RabbitListener(queues = MqConstants.QUEUE_ORDER_AFTERSALES_REFUND_COMPLETED, containerFactory = "manualAckContainerFactory")
    public void onRefundCompleted(org.springframework.amqp.core.Message message, Channel channel,
                                   @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);

        try {
            JsonNode payload = MAPPER.readTree(body);
            String orderNo = payload.path("order_no").asText("");
            String refundOutcome = payload.path("refund_outcome").asText("");
            String refundAmountStr = payload.path("refund_amount").asText("0");
            String externalRefId = payload.path("external_ref_id").asText("");
            String caseNo = payload.path("case_no").asText("");

            if (orderNo.isEmpty()) {
                log.error("AfterSalesEventConsumer.refundCompleted.invalid", "orderNo 為空");
                channel.basicAck(deliveryTag, false);
                return;
            }

            // fail-loud：金額非數字 → DLQ（退款記帳不容許靜默當 0）
            long refundAmount;
            try {
                refundAmount = Long.parseLong(refundAmountStr);
            } catch (NumberFormatException e) {
                throw new IllegalStateException(
                        "refund.completed payload 的 refund_amount 非合法數字: '" + refundAmountStr
                                + "'，orderNo=" + orderNo + "; caseNo=" + caseNo, e);
            }

            // fail-loud：null / 未知值 / REJECT_APPEAL 進 DLQ
            if (refundOutcome.isEmpty() || !AfterSalePlatformDecisionOutcome.isValidValue(refundOutcome)) {
                throw new IllegalStateException(
                        "refund.completed payload 缺少或不合法的 refund_outcome: '" + refundOutcome
                                + "'，orderNo=" + orderNo + "; caseNo=" + caseNo);
            }
            AfterSalePlatformDecisionOutcome outcome = AfterSalePlatformDecisionOutcome.fromValue(refundOutcome);
            if (outcome == AfterSalePlatformDecisionOutcome.REJECT_APPEAL) {
                throw new IllegalStateException(
                        "refund.completed payload 不應帶 REJECT_APPEAL outcome, orderNo=" + orderNo
                                + "; caseNo=" + caseNo);
            }
            // fail-loud：PARTIAL 沒有正金額 = 事件本身矛盾（部分退款必有退款額）
            if (outcome == AfterSalePlatformDecisionOutcome.PARTIAL_REFUND && refundAmount <= 0) {
                throw new IllegalStateException(
                        "refund.completed PARTIAL_REFUND 但 refund_amount<=0: " + refundAmount
                                + "，orderNo=" + orderNo + "; caseNo=" + caseNo);
            }

            // 交易段（返回即 commit）
            refundEventHandler.handleRefundCompleted(orderNo, outcome, refundAmount, externalRefId, caseNo);

            // 倒數恢復段（post-commit；語意與兜底見方法 javadoc 第 2 點）
            countdownService.tryResume(orderNo, AfterSalesEventType.REFUND_COMPLETED.value());

            log.info("AfterSalesEventConsumer.refundCompleted.done",
                    "售後退款完成事件處理完畢, refundOutcome=" + refundOutcome, "orderNo", orderNo);
            channel.basicAck(deliveryTag, false);
        } catch (Exception e) {
            log.error("AfterSalesEventConsumer.refundCompleted.error", "處理售後退款完成事件失敗", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "AfterSalesEventConsumer", e);
        }
    }

    /**
     * 取消出貨完成事件
     *
     * Payload: { event_id, order_no, cancel_id, occurred_at }
     *
     * 交易段委託 {@link SellerCancelRefundTxHandler}（獨立 @Service 確保 @Transactional proxy 生效）：
     * 1. 冪等寫入 financial_log（取消記錄——本方法過去只寫在 JavaDoc、從未實作，58 §6.3.2.3 E3 補上）
     * 2. 落 {@code order.refund.finalized} outbox
     * 3. 清除 need_refund_page 標記
     *
     * Fail-loud（→ MqErrorHandler → DLQ）：cancel_id 為空（它是 refund_ref 的唯一組成）。
     */
    @RabbitListener(queues = MqConstants.QUEUE_ORDER_AFTERSALES_CANCEL_COMPLETED, containerFactory = "manualAckContainerFactory")
    public void onCancelCompleted(org.springframework.amqp.core.Message message, Channel channel,
                                   @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);

        try {
            JsonNode payload = MAPPER.readTree(body);
            String orderNo = payload.path("order_no").asText("");
            String cancelId = payload.path("cancel_id").asText("");

            if (orderNo.isEmpty()) {
                log.error("AfterSalesEventConsumer.cancelCompleted.invalid", "orderNo 為空");
                channel.basicAck(deliveryTag, false);
                return;
            }

            // 交易段（返回即 commit）：financial_log + order.refund.finalized outbox + need_refund_page
            sellerCancelRefundTxHandler.handleCancelCompleted(orderNo, cancelId);

            log.info("AfterSalesEventConsumer.cancelCompleted.done",
                    "售後取消完成事件處理完畢, cancelId=" + cancelId, "orderNo", orderNo);
            channel.basicAck(deliveryTag, false);
        } catch (Exception e) {
            log.error("AfterSalesEventConsumer.cancelCompleted.error", "處理售後取消完成事件失敗", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "AfterSalesEventConsumer", e);
        }
    }

    /**
     * ATM 退款等待買家前往 PayUni 頁面事件
     *
     * Payload: { event_id, order_no, case_no, refund_task_id, occurred_at }
     *
     * SET need_refund_page = TRUE
     */
    @RabbitListener(queues = MqConstants.QUEUE_ORDER_AFTERSALES_AWAITING_BUYER_PAGE, containerFactory = "manualAckContainerFactory")
    public void onAwaitingBuyerRefundPage(org.springframework.amqp.core.Message message, Channel channel,
                                           @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);

        try {
            JsonNode payload = MAPPER.readTree(body);
            String orderNo = payload.path("order_no").asText("");

            if (orderNo.isEmpty()) {
                log.error("AfterSalesEventConsumer.awaitingBuyerPage.invalid", "orderNo 為空");
                channel.basicAck(deliveryTag, false);
                return;
            }

            orderRepository.updateNeedRefundPage(orderNo, true);

            log.info("AfterSalesEventConsumer.awaitingBuyerPage.done",
                    "已設定 need_refund_page=TRUE", "orderNo", orderNo);
            channel.basicAck(deliveryTag, false);
        } catch (Exception e) {
            log.error("AfterSalesEventConsumer.awaitingBuyerPage.error", "處理 ATM 退款等待買家事件失敗", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "AfterSalesEventConsumer", e);
        }
    }
}
