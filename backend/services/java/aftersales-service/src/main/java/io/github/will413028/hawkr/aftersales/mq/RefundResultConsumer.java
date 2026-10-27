package io.github.will413028.hawkr.aftersales.mq;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import com.baomidou.mybatisplus.core.conditions.update.LambdaUpdateWrapper;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.aftersales.entity.RefundTaskEntity;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.mapper.RefundTaskMapper;
import io.github.will413028.hawkr.aftersales.mapper.SellerCancelRequestMapper;
import io.github.will413028.hawkr.aftersales.service.AfterSaleAuditService;
import io.github.will413028.hawkr.aftersales.service.EventPublishFailedException;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAuditAction;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAuditActor;
import io.github.will413028.hawkr.common.utils.code.AfterSalePlatformDecisionOutcome;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.common.utils.code.WorkerTaskStatus;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.time.LocalDate;
import java.time.format.DateTimeParseException;
import java.util.HashMap;
import java.util.Map;

/**
 * 退款結果消費者。
 * 消費 queue {@code aftersales.payment.refund.result}（綁 {@code payment.events} 交換機的
 * routing key {@code refund.succeeded} / {@code refund.failed}；queue 名不是 routing key）。
 * 成敗一律由 <b>routing key</b> 判定（payload 不含 status 欄位），再依日期欄位存在性分流推進 task 狀態：
 * - refund.succeeded + bankDate  → ATM webhook 成功 → DONE
 * - refund.succeeded + estimatedRefundDate（無 bankDate）→ ATM 同步受理成功 → AWAITING_WEBHOOK
 * - refund.succeeded + 兩個日期都空 → CC Worker 已自行處理（冪等 skip）
 * - refund.failed + ATM → 退回 AWAITING_BUYER_REFUND_PAGE（§2.2 業務 pivot）
 * - refund.failed + CC/Apple Pay/Google Pay → 旁路補寫 PayUni 資訊，不動 state
 *
 * 詳細規格見 09 設計書 §5.4.3；Tx 載體規定見 §8.1.1。
 */
@Component
public class RefundResultConsumer {

    private static final ObsLogger log = ObsLogger.forClass(RefundResultConsumer.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final RefundTaskMapper refundTaskMapper;
    private final AfterSaleCaseMapper caseMapper;
    private final SellerCancelRequestMapper cancelMapper;
    private final AfterSaleAuditService auditService;
    private final AfterSalesEventPublisherImpl eventPublisher;
    private final io.github.will413028.hawkr.aftersales.service.AtmRefundReconciler atmRefundReconciler;
    // 09 §8.1.1 Tx 載體規定：同 class self-invocation handler 用 TransactionTemplate 程式式包覆
    // （而非 @Transactional annotation；Spring AOP CGLIB proxy 不攔截 this.handleXxx() 會讓 annotation 失效）
    private final TransactionTemplate txTemplate;

    public RefundResultConsumer(RefundTaskMapper refundTaskMapper,
                                 AfterSaleCaseMapper caseMapper,
                                 SellerCancelRequestMapper cancelMapper,
                                 AfterSaleAuditService auditService,
                                 AfterSalesEventPublisherImpl eventPublisher,
                                 io.github.will413028.hawkr.aftersales.service.AtmRefundReconciler atmRefundReconciler,
                                 PlatformTransactionManager txManager) {
        this.refundTaskMapper = refundTaskMapper;
        this.caseMapper = caseMapper;
        this.cancelMapper = cancelMapper;
        this.auditService = auditService;
        this.eventPublisher = eventPublisher;
        this.atmRefundReconciler = atmRefundReconciler;
        this.txTemplate = new TransactionTemplate(txManager);
    }

    @RabbitListener(queues = AfterSalesEventMqConfig.QUEUE_REFUND_RESULT,
            containerFactory = "manualAckContainerFactory")
    public void onRefundResult(org.springframework.amqp.core.Message message, Channel channel,
                                @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);
        String routingKey = message.getMessageProperties().getReceivedRoutingKey();

        try {
            JsonNode payload = MAPPER.readTree(body);
            // 欄位命名遵循專案契約 camelCase（docs/開發規範/API 設計規範.md、Java 微服務開發規範.md）
            // 所有其他 MQ consumer 皆如此，payment-service publisher 擴充時亦須對齊此命名
            final String refundNo = payload.path("refundNo").asText("");
            final String tradeNo = payload.path("tradeNo").asText("");
            final String payuniStatus = payload.path("payuniStatus").asText("");
            final String payuniMessage = payload.path("payuniMessage").asText("");
            final String estimatedRefundDate = payload.path("estimatedRefundDate").asText("");
            final String bankDate = payload.path("bankDate").asText("");

            // 成敗由 routing key 決定（payment publisher 對既有 payload 沒有 status 欄位；
            // 新 schema 即使帶 status 也以 routing key 為真源）
            boolean succeeded = MqConstants.ROUTING_KEY_REFUND_SUCCEEDED.equals(routingKey);
            boolean failed = MqConstants.ROUTING_KEY_REFUND_FAILED.equals(routingKey);

            if (refundNo.isEmpty() || (!succeeded && !failed)) {
                log.warn("RefundResultConsumer.invalid",
                        "缺少必要欄位或未知 routing key, refundNo=" + refundNo + ", routingKey=" + routingKey);
                channel.basicAck(deliveryTag, false);
                return;
            }

            final RefundTaskEntity refundTask = findRefundTask(refundNo, tradeNo);
            if (refundTask == null) {
                log.warn("RefundResultConsumer.taskNotFound",
                        "找不到退款任務, refundNo=" + refundNo + ", tradeNo=" + tradeNo);
                channel.basicAck(deliveryTag, false);
                return;
            }

            if (isTerminalState(refundTask)) {
                log.info("RefundResultConsumer.idempotent",
                        "退款任務已是終態，skip", "refundTaskId", String.valueOf(refundTask.getId()));
                channel.basicAck(deliveryTag, false);
                return;
            }

            // 09 §8.1.1 / §5.4.3：每個 handler 各自獨立短 tx，由 txTemplate 程式式包覆。
            // 禁止把 onRefundResult 整段包入 tx（含 channel.basicAck / MqErrorHandler.handleFailure
            // 等 MQ 協議操作，tx commit 失敗不應回滾 ack 狀態）。
            if (succeeded) {
                if (!bankDate.isEmpty()) {
                    // 出路 4 硬性邊界（《多副本排程搶佔規範》§四）：短 tx 內只准有那一列 CAS 與那一次
                    // publish。身分反解（seller/buyer 各一次無 deadline 的 gRPC）與 case 的
                    // store_id/buyer_id 反查一律在 tx **之外**先做好，當參數帶進去。
                    final CloseContext closeCtx = resolveCloseContext(refundTask);
                    txTemplate.executeWithoutResult(status ->
                            handleAtmWebhookSucceeded(refundTask, refundNo, payuniStatus, payuniMessage,
                                    bankDate, closeCtx));
                } else if (!estimatedRefundDate.isEmpty()) {
                    txTemplate.executeWithoutResult(status ->
                            handleAtmSyncAccepted(refundTask, refundNo, payuniStatus, payuniMessage, estimatedRefundDate));
                } else {
                    // CC Worker 已處理；冪等檢查已 skip，此處不應到達
                    log.info("RefundResultConsumer.ccRedundant",
                            "CC SUCCEEDED event（Worker 已自行處理），skip", "refundTaskId", String.valueOf(refundTask.getId()));
                }
            } else {
                // failed — §2.2 業務 pivot (2026-05-14)：ATM 退款任何失敗來源統一退回 AWAITING_BUYER_REFUND_PAGE(5)
                //   - sync 失敗（task 已是 5）：stamp message + attempt++
                //   - async webhook RefundStatus=2（task 在 15）：退回 5 + stamp + attempt++
                // 兩條路徑用同一 SQL revertFromAnyAwaitingToAwaitingPage（WHERE status IN ('5','15')）。
                // 信用卡 / Apple Pay / Google Pay 維持既有：由 Worker 管理 state / retry，consumer 僅補寫 snapshot。
                if (PaymentMethod.ATM.value().equals(refundTask.getPaymentMethod())) {
                    // 缺陷 B（#1288）+ P1-2：payment 端狀態查詢（含同步 gRPC）在 tx 外先做，
                    // 只把判定結果帶進 tx 內做 CAS 寫入，避免 gRPC 佔用 DB tx 連線。
                    var paymentState = atmRefundReconciler.queryPaymentRefundState(refundTask);
                    txTemplate.executeWithoutResult(status ->
                            revertAtmTaskToAwaitingPage(paymentState, refundTask, payuniStatus, payuniMessage));
                } else {
                    // CC/Apple Pay/Google Pay：由 Worker 管理 state / retry，consumer 僅補寫 snapshot
                    txTemplate.executeWithoutResult(status ->
                            enrichPayUniInfoOnly(refundTask, payuniStatus, payuniMessage));
                }
            }

            channel.basicAck(deliveryTag, false);
        } catch (Exception e) {
            log.error("RefundResultConsumer.error", "處理退款結果事件失敗", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "RefundResultConsumer", e);
        }
    }

    private RefundTaskEntity findRefundTask(String refundNo, String tradeNo) {
        RefundTaskEntity task = refundTaskMapper.selectOne(
                new LambdaQueryWrapper<RefundTaskEntity>()
                        .eq(RefundTaskEntity::getExternalRefId, refundNo)
                        .last("LIMIT 1")
        );
        if (task != null) {
            return task;
        }
        return refundTaskMapper.selectOne(
                new LambdaQueryWrapper<RefundTaskEntity>()
                        .eq(RefundTaskEntity::getTransactionId, tradeNo)
                        .in(RefundTaskEntity::getStatus,
                                WorkerTaskStatus.PROCESSING.value(),
                                WorkerTaskStatus.AWAITING_WEBHOOK.value(),
                                WorkerTaskStatus.AWAITING_BUYER_REFUND_PAGE.value(),
                                WorkerTaskStatus.FAILED.value())
                        .last("LIMIT 1")
        );
    }

    /** 終態：DONE，或 FAILED 且無重試計劃（next_retry_time=NULL）。 */
    private boolean isTerminalState(RefundTaskEntity task) {
        String s = task.getStatus();
        if (WorkerTaskStatus.DONE.value().equals(s)) {
            return true;
        }
        return WorkerTaskStatus.FAILED.value().equals(s) && task.getNextRetryTime() == null;
    }

    /**
     * ATM 同步受理成功：task 推進到 AWAITING_WEBHOOK，寫入 estimated_refund_date。
     * 09 §8.1.1 / §5.4.3：必須由 caller 包進 txTemplate.execute(...)。禁止標 @Transactional
     * （self-invocation 會繞過 Spring AOP CGLIB proxy 讓 annotation 失效）。
     */
    private void handleAtmSyncAccepted(RefundTaskEntity task, String refundNo,
                                       String payuniStatus, String payuniMessage,
                                       String estimatedRefundDateStr) {
        refundTaskMapper.update(null,
                new LambdaUpdateWrapper<RefundTaskEntity>()
                        .eq(RefundTaskEntity::getId, task.getId())
                        .set(RefundTaskEntity::getStatus, WorkerTaskStatus.AWAITING_WEBHOOK.value())
                        .set(RefundTaskEntity::getExternalRefId, refundNo)
                        .set(RefundTaskEntity::getPayuniStatus, payuniStatus)
                        .set(RefundTaskEntity::getPayuniMessage, payuniMessage)
                        .set(RefundTaskEntity::getEstimatedRefundDate, parseLocalDate(estimatedRefundDateStr))
                        .set(RefundTaskEntity::getUpdatedAt, DateTimeUtils.nowUtc()));

        log.info("RefundResultConsumer.atmSyncAccepted",
                "ATM 同步受理成功，等待 webhook, estimatedRefundDate=" + estimatedRefundDateStr,
                "refundTaskId", String.valueOf(task.getId()));
    }

    /**
     * 關案／發事件所需、且必須在 <b>tx 之外</b>取得的上下文（出路 4 硬性邊界）。
     *
     * @param storeId    案件所屬 store_id（tx 外由 caseNo 反查；賣家取消分支為 null）
     * @param buyerId    案件所屬 buyer_id（同上）
     * @param principals 兩個身分欄位的反解結果（tx 外先打完 gRPC）
     */
    private record CloseContext(Long storeId, Long buyerId,
                                io.github.will413028.hawkr.aftersales.service.EventPrincipals principals) {}

    /**
     * 在進 tx 之前備好關案事件的 payload 上下文。
     *
     * <p>身分反解刻意走 <b>best-effort</b>（seller 失敗回 null、buyer 失敗回 0L）而非 strict：
     * 本路徑是 MQ consumer，重試次數由 {@code MqErrorHandler} 限成 3 次後就進 DLQ，
     * 而 PayUni 不會再送一次 webhook ⇒ 為了「通知路由欄位」把整筆退款完成推進 DLQ，
     * 代價遠大於一則路由不到的通知（與修改前的行為逐字相同，本次只改 publish 的失敗訊號）。
     */
    private CloseContext resolveCloseContext(RefundTaskEntity task) {
        if (task.getCaseNo() == null) {
            // 賣家取消分支：cancel.completed payload 無身分欄位，零 gRPC
            return new CloseContext(null, null,
                    new io.github.will413028.hawkr.aftersales.service.EventPrincipals(null, 0L));
        }
        Long storeId = caseMapper.findStoreIdByCaseNo(task.getCaseNo());
        Long buyerId = caseMapper.findBuyerIdByCaseNo(task.getCaseNo());
        return new CloseContext(storeId, buyerId,
                eventPublisher.resolveEventPrincipalsBestEffort(storeId, buyerId));
    }

    /**
     * ATM webhook 成功：task 完成 + 關 case + 發 aftersales.refund.completed。
     * 09 §8.1.1 / §5.4.3：必須由 caller 包進 txTemplate.execute(...)。禁止標 @Transactional。
     */
    private void handleAtmWebhookSucceeded(RefundTaskEntity task, String refundNo,
                                           String payuniStatus, String payuniMessage,
                                           String bankDateStr, CloseContext closeCtx) {
        refundTaskMapper.update(null,
                new LambdaUpdateWrapper<RefundTaskEntity>()
                        .eq(RefundTaskEntity::getId, task.getId())
                        .set(RefundTaskEntity::getStatus, WorkerTaskStatus.DONE.value())
                        .set(RefundTaskEntity::getExternalRefId, refundNo)
                        .set(RefundTaskEntity::getPayuniStatus, payuniStatus)
                        .set(RefundTaskEntity::getPayuniMessage, payuniMessage)
                        .set(RefundTaskEntity::getBankDate, parseLocalDate(bankDateStr))
                        .set(RefundTaskEntity::getUpdatedAt, DateTimeUtils.nowUtc()));

        // 重讀取最新 task 以確保 closeParentAndPublish 使用 DB 中的最新值
        RefundTaskEntity fresh = refundTaskMapper.selectById(task.getId());
        closeParentAndPublish(fresh != null ? fresh : task, refundNo, closeCtx);

        log.info("RefundResultConsumer.atmWebhookSucceeded",
                "ATM webhook 匯款成功, bankDate=" + bankDateStr,
                "refundTaskId", String.valueOf(task.getId()));
    }

    /**
     * §2.2 業務 pivot (2026-05-14)：ATM 退款失敗（sync 或 async webhook）統一退回 AWAITING_BUYER_REFUND_PAGE(5)。
     * <p>原子 SQL（{@code revertFromAnyAwaitingToAwaitingPage}）：
     * <ul>
     *   <li>WHERE status IN ('5','15') — 5/15 之外的狀態（DONE/CANCELLED/...）不動</li>
     *   <li>SET status='5'、stamp payuni_status/payuni_message、current_attempt++、estimated_refund_date=NULL</li>
     * </ul>
     * <p>不重新呼叫 createRefund（保留 payment_refunds 既有 row 作 audit；下次 buyer 重填透過既有 sync return URL 路徑
     * 的 {@code ackAtmRefundAccepted} 冪等推進）。
     * <p>09 §8.1.1 / §5.4.3：必須由 caller 包進 txTemplate.execute(...)。禁止標 @Transactional。
     */
    private void revertAtmTaskToAwaitingPage(
            io.github.will413028.hawkr.aftersales.service.AtmRefundReconciler.PaymentRefundState paymentState,
            RefundTaskEntity task,
            String payuniStatus, String payuniMessage) {
        // 缺陷 B（issue #1288）：revert 前先確認 payment 端未終態成功；
        // 已 SUCCEEDED → reconciler 推進完成，不 revert 打回買家重填。
        // paymentState 已於 tx 外查妥（P1-2），此處僅做 DB 寫入。
        if (atmRefundReconciler.advanceIfPaymentSucceeded(paymentState, task, payuniStatus, payuniMessage)) {
            return;
        }
        int rows = refundTaskMapper.revertFromAnyAwaitingToAwaitingPage(
                task.getId(), payuniStatus, payuniMessage);
        if (rows == 0) {
            log.warn("RefundResultConsumer.revert.noop",
                    "task 已不在 5/15 狀態，跳過 revert refundTaskId=" + task.getId());
            return;
        }
        Map<String, Object> detail = new HashMap<>();
        detail.put("refundTaskId", task.getId());
        detail.put("payuniStatus", payuniStatus);
        detail.put("payuniMessage", payuniMessage);
        detail.put("previousStatus", task.getStatus());
        auditService.log(task.getCaseNo(), task.getCancelId(), null,
                AfterSaleAuditActor.SYSTEM.value(),
                AfterSaleAuditAction.REFUND_FAILURE_REVERT_TO_AWAITING.value(),
                detail);
        log.warn("RefundResultConsumer.revertToAwaiting",
                "ATM 退款失敗，退回 AWAITING_BUYER_REFUND_PAGE，買家可重填 payuniStatus=" + payuniStatus,
                "refundTaskId", String.valueOf(task.getId()));
    }

    /**
     * CC FAILED event 旁路補寫：只更新 payuni_status / payuni_message，避免與 Worker 的 state 管理競爭。
     * 09 §8.1.1 / §5.4.3：必須由 caller 包進 txTemplate.execute(...)。禁止標 @Transactional。
     */
    private void enrichPayUniInfoOnly(RefundTaskEntity task, String payuniStatus, String payuniMessage) {
        refundTaskMapper.update(null,
                new LambdaUpdateWrapper<RefundTaskEntity>()
                        .eq(RefundTaskEntity::getId, task.getId())
                        .set(RefundTaskEntity::getPayuniStatus, payuniStatus)
                        .set(RefundTaskEntity::getPayuniMessage, payuniMessage)
                        .set(RefundTaskEntity::getUpdatedAt, DateTimeUtils.nowUtc()));

        log.info("RefundResultConsumer.ccFailedEnriched",
                "補寫 CC task PayUni 資訊，未動 state / retry, payuniStatus=" + payuniStatus,
                "refundTaskId", String.valueOf(task.getId()));
    }

    /**
     * 退款成功後關閉上層實體並發佈完成事件。
     * 09 §2.1 / §2.3 INV-3：透過 caseMapper.closeIfNotClosed / cancelMapper.completeIfNotCompleted
     * CAS（WHERE status &lt;&gt; 終態）原子完成 transition，避免 updateById 全欄位覆蓋與其他 consumer / scheduler / RPC 路徑 race。
     * CAS rows=0 視為「已完成」冪等 skip，不重複發送 audit / event。
     *
     * <p><b>publish 失敗必須讓整個短 tx rollback</b>（《多副本排程搶佔規範》§四 出路 4）：
     * 本方法所在的 tx 已把 refund_task 推到 DONE、case 推到 CLOSED(50)，兩者都是
     * <b>永久狀態轉換</b>——重投時 {@code isTerminalState} 直接 ack skip、
     * {@code closeIfNotClosed} 回 0。本服務<b>沒有任何路徑能把 DONE 重推</b>
     * （{@code RefundTaskWorker} 的候選述詞只涵蓋 PENDING / FAILED / PROCESSING；
     * {@code AfterSalesGrpcService} 對 refund_tasks 的寫入只守 AWAITING_BUYER_REFUND_PAGE），
     * 所以這條路徑自己不會再發一次事件。
     *
     * <p><b>但事件不會永久消失</b>——{@code RefundSyncWorker} 的候選述詞是
     * {@code status = DONE AND updated_at < NOW()-1h} 且<b>無終止條件</b>，會週期性重發
     * {@code refund.completed}。⚠️ 那是<b>租來的</b>安全網：補發延遲隨歷史 DONE 筆數線性成長，
     * 而該 worker 自己的 javadoc 已把「無終止條件」列為相鄰缺陷。
     * <b>修掉它的收斂問題之前，必須先確認本處已走出路 4</b>，否則這裡會從
     * 「延遲補發」退化成真的永久遺失：order-service 不會把訂單推進
     * FULLY_REFUNDED(70) / PARTIALLY_REFUNDED(60)，crm 退款統計也少一筆。
     *
     * <p>rollback 之後該筆仍滿足 webhook 重投的前置狀態，由 {@code MqErrorHandler}
     * 重送（殘餘窗口 = 一次重投），不必等那條兜底。
     */
    private void closeParentAndPublish(RefundTaskEntity task, String refundNo, CloseContext closeCtx) {
        Long taskId = task.getId();
        if (task.getCaseNo() != null) {
            int updated = caseMapper.closeIfNotClosed(task.getCaseNo(), DateTimeUtils.nowUtc());
            if (updated > 0) {
                auditService.log(task.getCaseNo(), null, null, AfterSaleAuditActor.SYSTEM.value(),
                        AfterSaleAuditAction.REFUND_SUCCEEDED.value(),
                        Map.of("refundNo", refundNo, "refundTaskId", taskId));

                AfterSalePlatformDecisionOutcome outcome = AfterSalePlatformDecisionOutcome.fromValue(
                        task.getRefundOutcome());
                // #1226：補發賣家工作區 realtime 需 storeId，task 不帶，由 caseNo 反查（已在 tx 外查妥）
                boolean published = eventPublisher.tryPublishRefundCompleted(
                        task.getCaseNo(), task.getOrderNo(), closeCtx.storeId(), closeCtx.buyerId(),
                        outcome, task.getRefundAmount(),
                        task.getPaymentMethod(), refundNo, closeCtx.principals());
                if (!published) {
                    throw new EventPublishFailedException(
                            MqConstants.EVENT_TYPE_AFTERSALES_REFUND_COMPLETED, task.getCaseNo());
                }
            }
        } else if (task.getCancelId() != null) {
            int updated = cancelMapper.completeIfNotCompleted(task.getCancelId());
            if (updated > 0) {
                auditService.log(null, task.getCancelId(), null, AfterSaleAuditActor.SYSTEM.value(),
                        AfterSaleAuditAction.CANCEL_REFUND_SUCCEEDED.value(),
                        Map.of("refundNo", refundNo, "refundTaskId", taskId));

                if (!eventPublisher.tryPublishCancelCompleted(task.getOrderNo(), task.getCancelId())) {
                    throw new EventPublishFailedException(
                            MqConstants.EVENT_TYPE_AFTERSALES_CANCEL_COMPLETED,
                            "cancelId=" + task.getCancelId());
                }
            }
        }
    }

    /** PayUni 日期字串（YYYY-MM-DD）解析為 LocalDate；異常值（如 "-" / "0000-00-00"）回傳 null。 */
    private LocalDate parseLocalDate(String dateStr) {
        if (dateStr == null || dateStr.isEmpty() || "-".equals(dateStr) || "0000-00-00".equals(dateStr)) {
            return null;
        }
        try {
            return LocalDate.parse(dateStr);
        } catch (DateTimeParseException e) {
            log.warn("RefundResultConsumer.parseDateFailed",
                    "日期解析失敗, dateStr=" + dateStr);
            return null;
        }
    }
}
