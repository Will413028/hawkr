package io.github.will413028.hawkr.coupon.scheduler;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.CouponInstanceStatus;
import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import io.github.will413028.hawkr.coupon.entity.CouponInstanceEntity;
import io.github.will413028.hawkr.coupon.grpc.client.OrderGrpcClient;
import io.github.will413028.hawkr.coupon.service.CouponInstanceService;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import java.time.LocalDateTime;
import java.util.List;

/**
 * 優惠券對帳 Job（前身：AutoReleaseJob）
 *
 * 依 §2.6.1 業務規則：券的狀態變更完全由訂單事件驅動，本 Job 為「兜底對帳」用途。
 *
 * 掃出長時間仍處於 RESERVED 的 instance（reserved_at 早於 grace 秒數），
 * 逐筆呼叫 order-service 查詢訂單真實狀態，依結果觸發補正動作：
 *
 * <pre>
 * 訂單狀態           動作
 * -------------------------------------------------------
 * PENDING_PAYMENT   → 留著（訂單還在等付款，正常情況）
 * PAID              → consumeCoupon（補：core consume worker 漏網）
 * CANCELLED         → releaseCoupon（補：cancel 路徑同步 release 漏網）
 * 出貨後狀態         → consumeCoupon（補；正常 PAID 應已被處理）
 * 查無訂單           → 跳過 + WARN（不主動標終局，避免誤殺）
 * </pre>
 *
 * 不再使用 reserve_expire_at timer。
 */
@Component
public class AutoReleaseJob {

    private static final ObsLogger log = ObsLogger.forClass(AutoReleaseJob.class);

    @Value("${coupon.scheduler.auto-release.enabled:true}")
    private boolean enabled;

    /** 對帳延遲：reserved_at 早於 NOW - graceMinutes 才會被掃，避免打擾正常等付款訂單 */
    @Value("${coupon.scheduler.auto-release.grace-minutes:5}")
    private int graceMinutes;

    @Value("${coupon.scheduler.auto-release.batch-size:200}")
    private int batchSize;

    private final CouponInstanceService instanceService;
    private final OrderGrpcClient orderGrpcClient;

    public AutoReleaseJob(CouponInstanceService instanceService, OrderGrpcClient orderGrpcClient) {
        this.instanceService = instanceService;
        this.orderGrpcClient = orderGrpcClient;
    }

    @Scheduled(cron = "${coupon.scheduler.auto-release.cron:0 */5 * * * ?}")
    public void execute() {
        if (!enabled) {
            return;
        }
        try {
            LocalDateTime threshold = DateTimeUtils.nowUtc().minusMinutes(graceMinutes);
            List<CouponInstanceEntity> stale = instanceService.findStaleReserved(threshold, batchSize);
            if (stale.isEmpty()) {
                return;
            }

            int released = 0;
            int consumed = 0;
            int leftAlone = 0;
            int orphan = 0;

            for (CouponInstanceEntity inst : stale) {
                String orderNo = inst.getReservedOrderNo();
                if (orderNo == null || orderNo.isBlank()) {
                    orphan++;
                    log.warn("AutoReleaseJob.orphanInstance",
                            "RESERVED instance 無 reserved_order_no, instanceId=" + inst.getInstanceId());
                    continue;
                }

                String orderStatus = orderGrpcClient.getOrderStatus(orderNo);
                if (orderStatus == null) {
                    orphan++;
                    continue;
                }

                if (OrderStatus.PENDING_PAYMENT.value().equals(orderStatus)) {
                    leftAlone++;
                    continue;
                }

                if (OrderStatus.CANCELLED.value().equals(orderStatus)) {
                    boolean ok = instanceService.releaseCoupon(inst.getInstanceId(), orderNo);
                    if (ok) released++;
                    continue;
                }

                // 其餘狀態（PAID / PENDING_SHIPMENT / IN_TRANSIT / DELIVERED / COMPLETED 等）
                // 都代表訂單已付款，券應該為 USED；補打 consumeCoupon。
                boolean ok = instanceService.consumeCoupon(inst.getInstanceId(), orderNo);
                if (ok) consumed++;
            }

            log.info("AutoReleaseJob.execute.done",
                    "對帳完成 scanned=" + stale.size()
                            + " released=" + released
                            + " consumed=" + consumed
                            + " leftAlone=" + leftAlone
                            + " orphan=" + orphan);
        } catch (Exception e) {
            log.error("AutoReleaseJob.execute.failed", "對帳排程異常", e);
        }
    }
}
