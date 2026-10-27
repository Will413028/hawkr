package io.github.will413028.hawkr.order.worker;

import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.FinancialLogType;
import io.github.will413028.hawkr.common.utils.code.RefundSource;
import io.github.will413028.hawkr.order.entity.AutoRefundTaskEntity;
import io.github.will413028.hawkr.order.repository.AutoRefundTaskRepository;
import io.github.will413028.hawkr.order.mq.OrderRefundFinalizedOutboxWriter;
import io.github.will413028.hawkr.order.repository.FinancialLogRepository;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

/**
 * 自動退款完成的交易性處理。
 *
 * 從 AutoRefundWorker 獨立出來，確保 Spring @Transactional proxy 正常運作
 * （原 Worker 內 processTask → this.completeAutoRefund 為 self-invocation，
 * 不經 proxy，@Transactional 實際失效——「markDone + financial_log」曾非原子）。
 */
@Service
public class AutoRefundTxHandler {

    private final AutoRefundTaskRepository autoRefundTaskRepository;
    private final FinancialLogRepository financialLogRepository;
    private final OrderRefundFinalizedOutboxWriter refundFinalizedOutboxWriter;

    public AutoRefundTxHandler(AutoRefundTaskRepository autoRefundTaskRepository,
                               FinancialLogRepository financialLogRepository,
                               OrderRefundFinalizedOutboxWriter refundFinalizedOutboxWriter) {
        this.autoRefundTaskRepository = autoRefundTaskRepository;
        this.financialLogRepository = financialLogRepository;
        this.refundFinalizedOutboxWriter = refundFinalizedOutboxWriter;
    }

    /**
     * 在同一 DB 交易中完成自動退款：標記任務完成 + 冪等寫入 financial_logs
     * + 落 {@code order.refund.finalized} outbox（E2，58 §6.3.2.3）。
     */
    @Transactional
    public void completeAutoRefund(AutoRefundTaskEntity task, String externalRefId) {
        autoRefundTaskRepository.markDone(task.getTaskId());

        // 冪等寫入資金流水（自動退款一律為原路退款）
        financialLogRepository.insertIdempotent(
                TsidUtil.generate(),
                task.getOrderNo(),
                FinancialLogType.REFUND_ORIGINAL.value(),
                task.getAmount(),
                externalRefId
        );

        // E2（58 §6.3.2.3）：同事務落 outbox，refund_ref ＝ externalRefId（PayUni refundNo）。
        //
        // 這條路徑走 handleCancelledOrderPayment、從未發出過 order.paid ⇒ 該訂單沒有發票。
        // **仍然要發**：「每一筆落定的退款恰好發出一則 order.refund.finalized」比
        // 「除了這一種以外都發」容易驗證得多；invoice-service 查不到發票時寫 inbox、記 INFO、ACK。
        //
        // 不需要 E1 的 duplicate 守衛，這個不對稱是刻意的：本方法的入口是
        // auto_refund_tasks 的租約／狀態 CAS（AutoRefundWorker），沒有無限重發的 producer。
        refundFinalizedOutboxWriter.write(task.getOrderNo(), RefundSource.AUTO_REFUND,
                externalRefId, task.getAmount(), null, null, null);
    }
}
