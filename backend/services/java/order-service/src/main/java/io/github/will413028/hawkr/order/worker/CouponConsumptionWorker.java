package io.github.will413028.hawkr.order.worker;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.order.constant.OrderConstants;
import io.github.will413028.hawkr.order.entity.CouponConsumptionTaskEntity;
import io.github.will413028.hawkr.order.checkout.client.CheckoutCouponGrpcClient;
import io.github.will413028.hawkr.order.repository.CouponConsumptionTaskRepository;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import java.util.List;

/**
 * 優惠券核銷補償 Worker
 *
 * 付款成功後非同步執行優惠券核銷，失敗時自動重試。
 * 避免 Coupon Service 暫時不可用導致優惠券永久鎖定。
 *
 * <p><b>搶佔語意（issue #2294 C5）</b>：claim 是 <b>lease on {@code next_retry_time}</b>
 * （{@link #CLAIM_LEASE_SEC} 秒），不是 {@code SET status=PROCESSING} 的永久旗標。
 * ⚠️ 這一支曾被 #2294 §1-4 的「不納入」清單明文判為「已正確」（理由是「閘門在副作用之前」
 * ＋「副作用是 gRPC 不是 publish」）——兩個理由都不成立：閘門位置正確不代表<b>搶佔語意</b>正確
 * （§二 通則 0），而 C2／C3 的副作用同樣是 gRPC 卻被納入。零 migration。
 */
@Component
public class CouponConsumptionWorker {

    private static final ObsLogger log = ObsLogger.forClass(CouponConsumptionWorker.class);
    private static final int BATCH_SIZE = 20;

    /**
     * issue #2294 C5：claim 租約長度（秒）。
     *
     * <p><b>上界</b>：候選述詞無時間上界 ⇒ 窗口寬度 <b>∞</b> ⇒ 任何租約都合格；
     * tick 45 秒遠小於窗口。
     *
     * <p><b>下界</b>（《多副本排程搶佔規範》§三第 5 步）：claim 之後唯一的長工作是
     * {@link CheckoutCouponGrpcClient#consumeCoupon}（coupon 端本地 DB 寫入），其最壞耗時由
     * {@link CheckoutCouponGrpcClient#INTERNAL_READ_DEADLINE_SEC}（10 秒）硬性封頂
     * （在此之前該呼叫<b>沒有任何 deadline</b>）。取 <b>135 秒</b> = tick 45 秒 × 3，
     * 同時是最壞耗時的 13 倍。
     */
    static final long CLAIM_LEASE_SEC = 135L;

    private final CouponConsumptionTaskRepository couponConsumptionTaskRepository;
    private final CheckoutCouponGrpcClient checkoutCouponGrpcClient;

    public CouponConsumptionWorker(CouponConsumptionTaskRepository couponConsumptionTaskRepository,
                                   CheckoutCouponGrpcClient checkoutCouponGrpcClient) {
        this.couponConsumptionTaskRepository = couponConsumptionTaskRepository;
        this.checkoutCouponGrpcClient = checkoutCouponGrpcClient;
    }

    /**
     * 每 45 秒掃描一次
     */
    @Scheduled(fixedDelay = 45000, initialDelay = 20000)
    public void process() {
        List<CouponConsumptionTaskEntity> tasks = couponConsumptionTaskRepository.findRetryable(BATCH_SIZE);
        for (CouponConsumptionTaskEntity task : tasks) {
            processTask(task);
        }
    }

    private void processTask(CouponConsumptionTaskEntity task) {
        // 檢查是否超過最大重試次數
        if (task.getRetryCount() >= task.getMaxRetryCount()) {
            couponConsumptionTaskRepository.markManualIntervention(task.getTaskId());
            log.info("CouponConsumptionWorker.manualIntervention", "超過最大重試次數，轉人工處理",
                    "taskId", String.valueOf(task.getTaskId()));
            return;
        }

        // Claim 任務（lease）。claim 回 0 ⇒ 另一個 pod 持有租約 ⇒ 一個副作用都不做。
        int claimed = couponConsumptionTaskRepository.claimTask(
                task.getTaskId(), DateTimeUtils.nowUtc().plusSeconds(CLAIM_LEASE_SEC));
        if (claimed == 0) {
            return;
        }

        log.info("CouponConsumptionWorker.process.start", "執行優惠券核銷",
                "taskId", String.valueOf(task.getTaskId()));

        try {
            checkoutCouponGrpcClient.consumeCoupon(
                    task.getCouponInstanceId(),
                    task.getBuyerId(),
                    task.getOrderNo()
            );

            couponConsumptionTaskRepository.markDone(task.getTaskId());
            log.info("CouponConsumptionWorker.process.done", "優惠券核銷成功",
                    "taskId", String.valueOf(task.getTaskId()));
        } catch (Exception e) {
            log.error("CouponConsumptionWorker.process.error", "優惠券核銷失敗，將重試", e);
            int retryDelay = calculateRetryDelay(task.getRetryCount());
            couponConsumptionTaskRepository.updateFailed(
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
