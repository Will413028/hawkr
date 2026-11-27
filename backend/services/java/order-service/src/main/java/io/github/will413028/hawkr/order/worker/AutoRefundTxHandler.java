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

        financialLogRepository.insertIdempotent(
                TsidUtil.generate(),
                task.getOrderNo(),
                FinancialLogType.REFUND_ORIGINAL.value(),
                task.getAmount(),
                externalRefId
        );

        refundFinalizedOutboxWriter.write(task.getOrderNo(), RefundSource.AUTO_REFUND,
                externalRefId, task.getAmount(), null, null, null);
    }
}
