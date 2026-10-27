package io.github.will413028.hawkr.payment.scheduler;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.PaymentRescueAction;
import io.github.will413028.hawkr.common.utils.code.PaymentTransactionStatus;
import io.github.will413028.hawkr.payment.application.service.DuplicatePaymentService;
import io.github.will413028.hawkr.payment.application.service.PaymentApplicationService;
import io.github.will413028.hawkr.payment.application.service.RefundApplicationService;
import io.github.will413028.hawkr.payment.constant.PayUniConstants;
import io.github.will413028.hawkr.payment.domain.PaymentStatusRules;
import io.github.will413028.hawkr.payment.domain.event.PaymentDomainEvent;
import io.github.will413028.hawkr.payment.domain.model.PaymentTransaction;
import io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository;
import io.github.will413028.hawkr.payment.infrastructure.mq.PaymentOutboxWriter;
import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;
import io.github.will413028.hawkr.payuni.payment.PaymentClient;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.PaymentLogEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.PaymentLogMapper;
import org.springframework.stereotype.Service;
import org.springframework.transaction.support.TransactionTemplate;

import java.time.LocalDateTime;

/**
 * L2 補救路徑 — 單筆 reconcile 業務邏輯
 *
 * 設計參考：docs/詳細設計書/05-支付服務（Payment Service）詳細設計書.md §5.7.1
 *
 * 由 PendingPaymentReconcileScheduler 排程觸發，對單筆候選交易執行：
 * - INIT 路徑：呼叫 PaymentApplicationService.syncPaymentStatus 復用主路徑
 * - EXPIRED 路徑：偵測「PayUni 已成功」衝突，觸發 createRescueRefund 自動退款
 *
 * 不污染既有 syncPaymentStatus / createRefund 主路徑。
 */
@Service
public class PendingPaymentReconcileService {

    private static final ObsLogger log = ObsLogger.forClass(PendingPaymentReconcileService.class);

    private static final String TRIGGER_SOURCE = "RECONCILE";
    private static final String LOG_TYPE_RESCUE = "RESCUE";
    // §2A.6.6 改用 RescueReason CodeMeta enum
    private static final String RESCUE_REASON = io.github.will413028.hawkr.common.utils.code.RescueReason.RESCUE_RECONCILE_AFTER_EXPIRED.code();

    private final PaymentTransactionRepository transactionRepository;
    private final PaymentApplicationService paymentApplicationService;
    private final RefundApplicationService refundApplicationService;
    private final PaymentClient paymentClient;
    // §5.10.1 rescue 事件經 transactional outbox 與 markRescued 同 tx 原子落地（見 publishRescueEvents）
    private final PaymentOutboxWriter outboxWriter;
    private final PaymentLogMapper paymentLogMapper;
    private final DuplicatePaymentService duplicatePaymentService;
    // §05 §2A.6.2 管道 B / §2A.6.5 Tx 載體規定（對應 §40 §2.5）：reconcileExpired 由同 class
    // reconcileSingle self-invocation 呼叫，不能以 @Transactional 落地；rescue marker 與
    // outbox 事件寫入的原子化以 txTemplate 包覆
    private final TransactionTemplate transactionTemplate;

    public PendingPaymentReconcileService(
            PaymentTransactionRepository transactionRepository,
            PaymentApplicationService paymentApplicationService,
            RefundApplicationService refundApplicationService,
            PaymentClient paymentClient,
            PaymentOutboxWriter outboxWriter,
            PaymentLogMapper paymentLogMapper,
            DuplicatePaymentService duplicatePaymentService,
            TransactionTemplate transactionTemplate
    ) {
        this.transactionRepository = transactionRepository;
        this.paymentApplicationService = paymentApplicationService;
        this.refundApplicationService = refundApplicationService;
        this.paymentClient = paymentClient;
        this.outboxWriter = outboxWriter;
        this.paymentLogMapper = paymentLogMapper;
        this.duplicatePaymentService = duplicatePaymentService;
        this.transactionTemplate = transactionTemplate;
    }

    /**
     * Reconcile 單筆候選交易
     *
     * @return ReconcileOutcome 處理結果（供 Scheduler 累計 metrics）
     */
    public ReconcileOutcome reconcileSingle(PaymentTransaction candidate) {
        String tradeNo = candidate.tradeNo();
        try {
            ReconcileOutcome outcome;
            if (candidate.status() == PaymentTransactionStatus.INIT) {
                outcome = reconcileInit(candidate);
            } else if (candidate.status() == PaymentTransactionStatus.EXPIRED) {
                outcome = reconcileExpired(candidate);
            } else {
                // 防禦：理論上 SQL where 已過濾
                outcome = ReconcileOutcome.NO_OP;
            }
            return outcome;
        } catch (Exception e) {
            log.error("PendingPaymentReconcileService.reconcileSingle.failed",
                    "reconcile 處理失敗: " + e.getMessage() + ", trade_no=" + tradeNo, e);
            return ReconcileOutcome.FAILED;
        }
        // issue #2294 H6：原本這裡有一段 finally 內的 updateLastReconciledAt（「避免熱循環」）。
        // stamp 已於 PendingPaymentReconcileScheduler 的 claimForReconcile 閘門完成 —— 那是同一欄
        // 同一語意的寫入，只是從「副作用之後」移到「副作用之前」並加上 CAS 述詞。
        // 留在這裡會變成第二次無條件寫入，把 claim 的 CAS 保證洗掉（另一個 pod 的租約會被覆蓋）。
    }

    /**
     * INIT 路徑：完全復用主路徑 syncPaymentStatus
     */
    private ReconcileOutcome reconcileInit(PaymentTransaction candidate) {
        var before = candidate.status();
        var detail = paymentApplicationService.syncPaymentStatus(candidate.tradeNo());
        if (detail.isEmpty()) {
            return ReconcileOutcome.NO_OP;
        }
        // 重撈確認狀態是否變化（用 id 精準鎖定原 row，避免 §2A.6 多 row 情境誤抓）
        var after = transactionRepository.findById(candidate.id())
                .map(PaymentTransaction::status)
                .orElse(before);
        if (after != before && PaymentStatusRules.isTerminal(after)) {
            log.info("PendingPaymentReconcileService.reconcileInit.updated",
                    String.format("INIT reconcile 已更新狀態 trade_no=%s from=%s to=%s",
                            candidate.tradeNo(), before.value(), after.value()));
            return ReconcileOutcome.UPDATED;
        }
        return ReconcileOutcome.NO_OP;
    }

    /**
     * EXPIRED 路徑：偵測 grace 視窗內的衝突，觸發 rescue
     * <p>
     * §05 §2A.6.2 管道 B / §2A.6.5 Tx 載體規定（對應 §40 §2.5）：
     * 本方法由同 class {@link #reconcileSingle} self-invocation 呼叫，**禁止標 @Transactional**
     * （Spring AOP CGLIB proxy 不攔截 self-invocation 會讓 annotation 失效）。
     * <p>
     * Tx 邊界：
     * <ul>
     *   <li>PayUni queryTransaction（外部 HTTP RPC）— tx 外</li>
     *   <li>writeRescueAuditLog — tx 外（已有 try-catch 吞例外，設計上要求即使後續退款失敗仍保留 audit）</li>
     *   <li>duplicatePaymentService.handleDuplicateFromReconcile / refundApplicationService.createRescueRefund
     *       — tx 外，下游 service 各自 @Transactional 走 proxy 完整 commit</li>
     *   <li>markRescued + publishRescueEvents（outbox INSERT）— 包進 txTemplate 短 tx 保證 rescue marker 與事件寫入原子化</li>
     * </ul>
     * visibility 改為 {@code private} 以表達「不可由外部 caller 進入」的意圖；既有 caller 僅 reconcileSingle。
     */
    private ReconcileOutcome reconcileExpired(PaymentTransaction candidate) {
        String tradeNo = candidate.tradeNo();

        // 1. 直接呼叫 PayUni 查詢（不能走 syncPaymentStatus，它對終態早退）
        PayUniApiResponse response = paymentClient.queryTransaction(candidate.merId(), tradeNo);
        if (!response.isSuccess()) {
            log.info("PendingPaymentReconcileService.reconcileExpired.queryFailed",
                    "EXPIRED 查詢失敗，跳過: " + response.getErrorMessage(), "trade_no", tradeNo);
            return ReconcileOutcome.NO_OP;
        }

        // 2. 解析 TradeStatus（PayUniApiResponse.getTradeStatus 已支援 v2.0 Result 陣列）
        String providerStatus = response.getTradeStatus();
        log.info("PendingPaymentReconcileService.reconcileExpired.payUniResp",
                "trade_no=" + tradeNo + " TradeStatus=" + providerStatus);

        if (providerStatus == null) {
            return ReconcileOutcome.NO_OP;
        }
        int statusCode;
        try {
            statusCode = Integer.parseInt(providerStatus.trim());
        } catch (NumberFormatException e) {
            log.warn("PendingPaymentReconcileService.reconcileExpired.statusParseFail",
                    "TradeStatus 解析失敗: " + providerStatus + ", trade_no=" + tradeNo);
            return ReconcileOutcome.NO_OP;
        }
        if (statusCode != PayUniConstants.TRADE_STATUS_SUCCESS) {
            // PayUni 也認為非成功 → EXPIRED 是正確結果，不需要 rescue
            return ReconcileOutcome.NO_OP;
        }

        log.warn("PendingPaymentReconcileService.reconcileExpired.conflict",
                "偵測到 EXPIRED + PayUni SUCCESS 衝突，觸發 rescue", "trade_no", tradeNo);

        // 3. 寫入 audit log（必須在後續退款失敗時也保留紀錄）
        writeRescueAuditLog(candidate, response);

        // 3A. §2A.6.2 管道 B：先檢查同 order 是否已有其他 PAID row
        // 若有 → 走重複付款處理（DuplicatePaymentService.handleDuplicateFromReconcile）
        // 否則 → 維持原有單筆 rescue 路徑
        var otherPaidRows = transactionRepository.findOtherPaidRowsForSameOrder(
                candidate.orderId(), candidate.id());
        if (!otherPaidRows.isEmpty()) {
            log.warn("PendingPaymentReconcileService.reconcileExpired.duplicateDetected",
                    "管道 B 偵測：同 order 已有其他 PAID row，走重複付款處理 trade_no=" + tradeNo
                            + " other_paid_id=" + otherPaidRows.get(0).id());
            try {
                duplicatePaymentService.handleDuplicateFromReconcile(candidate, otherPaidRows.get(0));
                // 標 candidate 為 rescued（避免下次 reconcile 重複處理）— 包進短 tx 對齊 Tx 載體規定
                transactionTemplate.executeWithoutResult(status ->
                        transactionRepository.markRescued(tradeNo, DateTimeUtils.nowUtc(),
                                PaymentRescueAction.REFUNDED.value()));
                return ReconcileOutcome.RESCUE_DUPLICATE_DETECTED;
            } catch (Exception e) {
                log.error("PendingPaymentReconcileService.reconcileExpired.duplicateFailed",
                        "管道 B 重複付款處理失敗 trade_no=" + tradeNo + ": " + e.getMessage(), e);
                return ReconcileOutcome.RESCUE_FAILED;
            }
        }

        // 4. 觸發 rescue 退款
        var rescueResult = refundApplicationService.createRescueRefund(
                tradeNo,
                candidate.amount(),
                RESCUE_REASON,
                "rescue:" + tradeNo
        );

        // 5+6. 標記 rescued_at + rescue_action（失敗也標 REFUNDED 紀錄已嘗試）+ outbox 事件寫入
        // 對齊 §05 §2A.6.2 管道 B / §5.10.1 Tx 載體規定：包進 txTemplate 短 tx，markRescued 與 rescue
        // 事件經 outbox INSERT 同 tx 原子落地（rollback 皆不落地、commit 一併落地），杜絕
        // markRescued 已 commit（設 rescued_at → candidate SQL 永久排除該 row）但事件遺失 →
        // RescueRefundConsumer 永不觸發 → status 永久卡 EXPIRED 的原子化破口
        String rescueAction = rescueResult.requiresManual()
                ? PaymentRescueAction.REFUND_PENDING_MANUAL.value()
                : PaymentRescueAction.REFUNDED.value();
        transactionTemplate.executeWithoutResult(status -> {
            transactionRepository.markRescued(tradeNo, DateTimeUtils.nowUtc(), rescueAction);
            publishRescueEvents(candidate);
        });

        return rescueResult.requiresManual()
                ? ReconcileOutcome.RESCUE_MANUAL_REQUIRED
                : (rescueResult.succeeded() ? ReconcileOutcome.RESCUE_SUCCESS : ReconcileOutcome.RESCUE_FAILED);
    }

    /**
     * 寫入 rescue audit log
     */
    private void writeRescueAuditLog(PaymentTransaction candidate, PayUniApiResponse response) {
        try {
            PaymentLogEntity logEntity = new PaymentLogEntity();
            logEntity.setTradeNo(candidate.tradeNo());
            logEntity.setLogType(LOG_TYPE_RESCUE);
            logEntity.setLogLevel("WARNING");
            logEntity.setMessage("reconcile 偵測 EXPIRED + PayUni SUCCESS 衝突，觸發 rescue 退款。"
                    + "PayUni TradeStatus=" + response.getTradeStatus()
                    + ", ProviderTradeNo=" + response.getProviderTradeNo());
            logEntity.setFromStatus(candidate.status().value());
            logEntity.setTriggerSource(TRIGGER_SOURCE);
            paymentLogMapper.insert(logEntity);
        } catch (Exception e) {
            log.warn("PendingPaymentReconcileService.writeRescueAuditLog.failed",
                    "audit log 寫入失敗: " + e.getMessage());
        }
    }

    /**
     * 發布 rescue 事件（{@code PaymentRescueRefundInitiated}）。
     * <p>
     * 經 {@link PaymentOutboxWriter#enqueueAndDispatch} 走 §5.10.1 transactional outbox：
     * 於當前 txTemplate 事務內 INSERT {@code outbox_events}，與 caller 的 {@code markRescued}
     * 同 tx 原子落地；commit 後由 afterCommit 即時投遞、失敗交 OutboxEventPublisherScheduler
     * 每 5s 兜底，publisher-confirm 保證至少一次投遞。
     * <p>
     * <b>不得 catch 吞例外</b>：序列化失敗時 enqueueAndDispatch 自行拋例外使 txTemplate rollback
     * （markRescued 一併回滾 → rescued_at 未設 → 下次 reconcile 重掃重試）。若在此 catch 吞例外，
     * 會重演「rescued_at 已 commit 但事件遺失 → candidate SQL 永久排除該 row →
     * RescueRefundConsumer 永不觸發 → status 永久卡 EXPIRED」的原子化破口。
     * <p>
     * issue #1369：ATM rescue「需人工」不再發 MQ 孤兒事件（原 {@code PaymentRescueRequiresManual}
     * 無 consumer）。人工待辦狀態已持久化於 {@code payment_transactions.rescue_action='8902'}
     * （{@code REFUND_PENDING_MANUAL}），由 sql_exporter 計數 + PaymentRescueManualPending 業務告警
     * 主動通知運維，取代靜默孤兒事件。
     */
    private void publishRescueEvents(PaymentTransaction candidate) {
        LocalDateTime now = DateTimeUtils.nowUtc();
        // §2A.6.6 record 簽名：(tradeNo, orderId, merId, providerTradeNo, amount, paymentType, rescueReason, occurredAt)
        // reconcile 場景：refund 已由 RefundApplicationService 建好，
        // 重點傳 merId + providerTradeNo 給 consumer 走 PayUni close API
        outboxWriter.enqueueAndDispatch(new PaymentDomainEvent.PaymentRescueRefundInitiated(
                candidate.tradeNo(),
                candidate.orderId(),
                candidate.merId(),
                candidate.providerTradeNo(),
                candidate.amount(),
                candidate.paymentType() != null ? candidate.paymentType().value() : "",
                RESCUE_REASON,
                now
        ));
    }

    /**
     * Reconcile 單筆處理結果
     */
    public enum ReconcileOutcome {
        NO_OP,
        UPDATED,
        RESCUE_SUCCESS,
        RESCUE_MANUAL_REQUIRED,
        RESCUE_FAILED,
        RESCUE_DUPLICATE_DETECTED,
        FAILED
    }
}
