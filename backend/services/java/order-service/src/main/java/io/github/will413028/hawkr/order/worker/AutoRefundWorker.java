package io.github.will413028.hawkr.order.worker;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.order.constant.OrderConstants;
import io.github.will413028.hawkr.order.entity.AutoRefundTaskEntity;
import io.github.will413028.hawkr.order.grpc.client.PaymentGrpcClient;
import io.github.will413028.hawkr.order.repository.AutoRefundTaskRepository;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import java.util.List;

/**
 * 自動退款 Worker
 *
 * 處理場景：訂單已取消但收到付款事件 → 自動向 Payment Service 發起退款
 *
 * 依付款方式分流：
 * 呼叫 CreateRefund gRPC 自動退款。
 *
 * <p><b>搶佔語意（issue #2294 C2）</b>：claim 是 <b>lease on {@code next_retry_time}</b>
 * （{@link #CLAIM_LEASE_SEC} 秒），不是 {@code SET status=PROCESSING} 的永久旗標；
 * {@code status} 保留但降為可觀測資訊，候選述詞同步涵蓋 {@code PROCESSING}。零 migration。
 * 詳見 {@code AutoRefundTaskRepository.claimTask} 與
 * {@code docs/開發規範/多副本排程搶佔規範.md}。
 */
@Component
public class AutoRefundWorker {

    private static final ObsLogger log = ObsLogger.forClass(AutoRefundWorker.class);
    private static final int BATCH_SIZE = 20;

    /**
     * issue #2294 C2：claim 租約長度（秒）。
     *
     * <p><b>上界</b>（《多副本排程搶佔規範》§三第 1–3 步）：候選述詞
     * （{@code AutoRefundTaskRepository.findRetryableByStatus}）只有 {@code status IN (...)}
     * ＋ {@code next_retry_time <= NOW()}，<b>無時間上界</b> ⇒ 候選窗口寬度 <b>∞</b>
     * ⇒ 任何租約長度都合格；tick 30 秒遠小於窗口，第 4 步的極端情況不成立。
     *
     * <p><b>下界</b>（同規範第 5 步）：claim 之後唯一的長工作是
     * {@link PaymentGrpcClient#createRefund}（payment 端寫列並排 job），其最壞耗時由
     * {@link PaymentGrpcClient#MONEY_MOVEMENT_DEADLINE_SEC}（100 秒）硬性封頂
     * （在此之前該呼叫<b>沒有任何 deadline</b>，下界無從建立）。
     * 取 <b>120 秒</b> = 100 秒 deadline ＋ 20 秒餘裕（claim→呼叫之間與失敗路徑的 DB 寫入）。
     *
     * <p>⚠️ 加長租約不會拖慢正常重試：失敗路徑 {@code updateFailed} 寫的指數退避值與租約
     * <b>寫同一欄</b>且在 claim 之後執行，天然覆蓋（設計如此）。
     */
    static final long CLAIM_LEASE_SEC = 120L;

    private final AutoRefundTaskRepository autoRefundTaskRepository;
    private final PaymentGrpcClient paymentGrpcClient;
    private final AutoRefundTxHandler autoRefundTxHandler;

    public AutoRefundWorker(AutoRefundTaskRepository autoRefundTaskRepository,
                            PaymentGrpcClient paymentGrpcClient,
                            AutoRefundTxHandler autoRefundTxHandler) {
        this.autoRefundTaskRepository = autoRefundTaskRepository;
        this.paymentGrpcClient = paymentGrpcClient;
        this.autoRefundTxHandler = autoRefundTxHandler;
    }

    /**
     * 每 30 秒掃描一次（掃描 PENDING / FAILED）
     */
    @Scheduled(fixedDelay = 30000, initialDelay = 15000)
    public void process() {
        List<AutoRefundTaskEntity> tasks = autoRefundTaskRepository.findRetryable(BATCH_SIZE);
        for (AutoRefundTaskEntity task : tasks) {
            processTask(task);
        }
    }

    private void processTask(AutoRefundTaskEntity task) {
        if (task.getRetryCount() >= task.getMaxRetryCount()) {
            autoRefundTaskRepository.markManualIntervention(task.getTaskId());
            log.info("AutoRefundWorker.manualIntervention", "超過最大重試次數，轉人工處理",
                    "taskId", String.valueOf(task.getTaskId()));
            return;
        }

        int claimed = autoRefundTaskRepository.claimTask(
                task.getTaskId(), DateTimeUtils.nowUtc().plusSeconds(CLAIM_LEASE_SEC));
        if (claimed == 0) {
            return;
        }

        log.info("AutoRefundWorker.process.start",
                "處理自動退款: taskId=" + task.getTaskId() + ", orderNo=" + task.getOrderNo());

        try {
            String reason = "訂單取消自動退款: " + task.getOrderNo();
            String refundNo = paymentGrpcClient.createRefund(
                    task.getTransactionId(), task.getAmount(), reason,
                    "AUTO:" + task.getOrderNo() + ":" + task.getTransactionId());

            autoRefundTxHandler.completeAutoRefund(task, refundNo);
            log.info("AutoRefundWorker.process.done",
                    "自動退款完成: taskId=" + task.getTaskId() + ", refundNo=" + refundNo);
        } catch (Exception e) {
            log.error("AutoRefundWorker.process.error", "自動退款失敗", e);
            int retryDelay = calculateRetryDelay(task.getRetryCount());
            autoRefundTaskRepository.updateFailed(
                    task.getTaskId(),
                    OrderConstants.TASK_STATUS_FAILED,
                    e.getMessage(),
                    retryDelay
            );
        }
    }


    private int calculateRetryDelay(int retryCount) {
        return Math.min(60 * (1 << Math.min(retryCount, 6)), 3600);
    }
}
