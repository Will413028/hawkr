package io.github.will413028.hawkr.order.worker;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.order.constant.OrderConstants;
import io.github.will413028.hawkr.order.entity.CouponConsumptionTaskEntity;
import io.github.will413028.hawkr.order.checkout.client.CheckoutCouponGrpcClient;
import io.github.will413028.hawkr.order.checkout.client.CouponNoVoucherForOrderException;
import io.github.will413028.hawkr.order.entity.OrderEntity;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import io.github.will413028.hawkr.order.repository.CouponConsumptionTaskRepository;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import java.util.List;

/**
 * 優惠券核銷補償 Worker
 *
 * 付款成功後非同步執行優惠券核銷、訂單取消後非同步釋放優惠券（task_type 區分），失敗時自動重試。
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
    private final OrderRepository orderRepository;

    public CouponConsumptionWorker(CouponConsumptionTaskRepository couponConsumptionTaskRepository,
                                   CheckoutCouponGrpcClient checkoutCouponGrpcClient,
                                   OrderRepository orderRepository) {
        this.couponConsumptionTaskRepository = couponConsumptionTaskRepository;
        this.checkoutCouponGrpcClient = checkoutCouponGrpcClient;
        this.orderRepository = orderRepository;
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
            runTask(task);

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

    /**
     * 依 task_type 執行：CONSUME 核銷、RELEASE 以 order_no 釋券（失敗拋出，由呼叫端走重試）。
     * 不認得的型別（含 null）拋出，不當成核銷：寫錯型別的任務重試到上限後轉人工，不會悄悄做錯事。
     */
    private void runTask(CouponConsumptionTaskEntity task) {
        String type = task.getTaskType();
        if (OrderConstants.COUPON_TASK_TYPE_CONSUME.equals(type)) {
            consume(task);
        } else if (OrderConstants.COUPON_TASK_TYPE_RELEASE.equals(type)) {
            checkoutCouponGrpcClient.releaseCouponByOrderNo(task.getOrderNo(), task.getCouponInstanceId());
        } else {
            throw new IllegalStateException("不認得的 coupon 任務型別：" + type);
        }
    }

    /**
     * 核銷。coupon 回「這筆訂單沒有持有券」（7009）時：訂單已取消（取消時券已釋放）是終態，任務視為完成；
     * 其他情況券遺失，照樣拋出走重試與轉人工。
     */
    private void consume(CouponConsumptionTaskEntity task) {
        try {
            checkoutCouponGrpcClient.consumeCoupon(task.getCouponInstanceId(), task.getBuyerId(), task.getOrderNo());
        } catch (CouponNoVoucherForOrderException e) {
            OrderEntity order = orderRepository.findByOrderNo(task.getOrderNo());
            if (order == null || !OrderStatus.CANCELLED.value().equals(order.getStatus())) {
                throw e;
            }
            log.info("CouponConsumptionWorker.consume.orderCancelled", "訂單已取消且券已釋放，核銷任務視為完成",
                    "orderNo", task.getOrderNo());
        }
    }

    private int calculateRetryDelay(int retryCount) {
        return Math.min(60 * (1 << Math.min(retryCount, 6)), 3600);
    }
}
