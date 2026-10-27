package io.github.will413028.hawkr.order.worker;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.order.constant.OrderConstants;
import io.github.will413028.hawkr.order.entity.CartCleanupRetryTaskEntity;
import io.github.will413028.hawkr.order.grpc.client.CartGrpcClient;
import io.github.will413028.hawkr.order.repository.CartCleanupRetryTaskRepository;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import java.util.List;

/**
 * 購物車清理重試 Worker
 *
 * 付款成功後非同步清理購物車中已購買的商品，失敗時自動重試。
 * 最終防線：即使所有重試皆失敗，Cart Service 背景排程（5 分鐘）
 * 會自動解鎖過期鎖定的商品。
 *
 * <p><b>搶佔語意（issue #2294 C3）</b>：claim 是 <b>lease on {@code next_retry_time}</b>
 * （{@link #CLAIM_LEASE_SEC} 秒），不是 {@code SET status=PROCESSING} 的永久旗標。
 * ⚠️ 上面那條「最終防線」<b>救不了本缺陷</b>：cart 的背景排程只解鎖商品鎖，
 * <b>購物車項目本身仍永久不清</b>。零 migration。
 * 詳見 {@code CartCleanupRetryTaskRepository.claimTask}。
 */
@Component
public class CartCleanupRetryWorker {

    private static final ObsLogger log = ObsLogger.forClass(CartCleanupRetryWorker.class);
    private static final int BATCH_SIZE = 20;

    /**
     * issue #2294 C3：claim 租約長度（秒）。
     *
     * <p><b>上界</b>：候選述詞無時間上界 ⇒ 窗口寬度 <b>∞</b> ⇒ 任何租約都合格；
     * tick 30 秒遠小於窗口。
     *
     * <p><b>下界</b>（《多副本排程搶佔規範》§三第 5 步）：claim 之後唯一的長工作是
     * {@link CartGrpcClient#removeItemsByCheckoutId}（cart 端本地 DB 寫入），其最壞耗時由
     * {@link CartGrpcClient#INTERNAL_DEADLINE_SEC}（10 秒）硬性封頂
     * （在此之前該呼叫<b>沒有任何 deadline</b>）。取 <b>90 秒</b> = tick 30 秒 × 3，
     * 同時是最壞耗時的 9 倍。
     */
    static final long CLAIM_LEASE_SEC = 90L;

    private final CartCleanupRetryTaskRepository cartCleanupRetryTaskRepository;
    private final CartGrpcClient cartGrpcClient;

    public CartCleanupRetryWorker(CartCleanupRetryTaskRepository cartCleanupRetryTaskRepository,
                                  CartGrpcClient cartGrpcClient) {
        this.cartCleanupRetryTaskRepository = cartCleanupRetryTaskRepository;
        this.cartGrpcClient = cartGrpcClient;
    }

    /**
     * 每 30 秒掃描一次
     */
    @Scheduled(fixedDelay = 30000, initialDelay = 25000)
    public void process() {
        List<CartCleanupRetryTaskEntity> tasks = cartCleanupRetryTaskRepository.findRetryable(BATCH_SIZE);
        for (CartCleanupRetryTaskEntity task : tasks) {
            processTask(task);
        }
    }

    private void processTask(CartCleanupRetryTaskEntity task) {
        // 檢查是否超過最大重試次數
        if (task.getRetryCount() >= task.getMaxRetryCount()) {
            cartCleanupRetryTaskRepository.markManualIntervention(task.getTaskId());
            log.info("CartCleanupRetryWorker.manualIntervention", "超過最大重試次數，轉人工處理",
                    "taskId", String.valueOf(task.getTaskId()));
            return;
        }

        // Claim 任務（lease）。claim 回 0 ⇒ 另一個 pod 持有租約 ⇒ 一個副作用都不做。
        int claimed = cartCleanupRetryTaskRepository.claimTask(
                task.getTaskId(), DateTimeUtils.nowUtc().plusSeconds(CLAIM_LEASE_SEC));
        if (claimed == 0) {
            return;
        }

        log.info("CartCleanupRetryWorker.process.start", "清理購物車",
                "taskId", String.valueOf(task.getTaskId()));

        try {
            cartGrpcClient.removeItemsByCheckoutId(task.getBuyerId(), task.getCheckoutId());

            cartCleanupRetryTaskRepository.markDone(task.getTaskId());
            log.info("CartCleanupRetryWorker.process.done", "購物車清理成功",
                    "taskId", String.valueOf(task.getTaskId()));
        } catch (Exception e) {
            log.error("CartCleanupRetryWorker.process.error", "購物車清理失敗，將重試", e);
            int retryDelay = calculateRetryDelay(task.getRetryCount());
            cartCleanupRetryTaskRepository.updateFailed(
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
