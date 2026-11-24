package io.github.will413028.hawkr.order.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.order.checkout.client.CheckoutCouponGrpcClient;
import io.github.will413028.hawkr.order.repository.CouponConsumptionTaskRepository;
import org.springframework.stereotype.Service;

/**
 * 取消與補償路徑的釋券（D3）。
 *
 * <p>釋券是 {@code coupon_consumption_tasks} 的 RELEASE 任務（以 order_no 為鍵、同一筆訂單只有一筆）。
 * 取消的 transaction 內呼叫 {@link #enqueueInTx}，任務與取消狀態一起提交；提交後呼叫 {@link #release}
 * 當場嘗試一次，成功就把任務標成完成，失敗不拋出，任務留給 {@code CouponConsumptionWorker} 指數退避重試，
 * 超過次數轉人工。鎖沒有到期時間，漏送的釋放不補就永遠卡在 locked。
 *
 * <p>結帳的鎖券失敗補償走 {@link #releaseWithLateLockRecheck}：鎖券請求可能在 coupon 端還沒提交，
 * 任務延後到 reserve 的 deadline 之後再跑一次，不在當場成功時標成完成。
 */
@Service
public class CouponReleaseService {

    private static final ObsLogger log = ObsLogger.forClass(CouponReleaseService.class);

    /**
     * 晚到的鎖券的複查延遲（秒）：reserve 的 gRPC deadline 加上 coupon 端提交與時鐘誤差的餘裕。
     * deadline 之後 client 已放棄等待，但 server 端仍可能在這之後才提交 Lock。
     */
    public static final int LATE_LOCK_RECHECK_DELAY_SEC =
            (int) CheckoutCouponGrpcClient.INTERNAL_READ_DEADLINE_SEC + 20;

    private final CouponConsumptionTaskRepository taskRepository;
    private final CheckoutCouponGrpcClient couponClient;

    public CouponReleaseService(CouponConsumptionTaskRepository taskRepository,
                                CheckoutCouponGrpcClient couponClient) {
        this.taskRepository = taskRepository;
        this.couponClient = couponClient;
    }

    /**
     * 在呼叫端的 transaction 內寫入釋券任務，與取消狀態同時提交或回滾。寫入失敗會拋出，讓取消一起回滾。
     */
    public void enqueueInTx(String orderNo, String couponInstanceId) {
        taskRepository.enqueueRelease(TsidUtil.generate(), orderNo, couponInstanceId);
    }

    /**
     * 取消提交後釋放這筆訂單持有的券（任務已由 {@link #enqueueInTx} 寫入）。不拋出：失敗時任務由 worker 重試。
     */
    public void release(String orderNo, String couponInstanceId) {
        try {
            couponClient.releaseCouponByOrderNo(orderNo, couponInstanceId);
        } catch (Exception e) {
            log.warn("CouponReleaseService.release.deferred", "當場釋券失敗，交由任務重試，orderNo=" + orderNo);
            return;
        }
        markDone(orderNo);
    }

    /**
     * 鎖券失敗的補償：寫一筆延後 {@link #LATE_LOCK_RECHECK_DELAY_SEC} 秒的任務，並當場釋放一次，但不標成完成。
     * 當場釋放時 coupon 端的 Lock 可能還沒提交，延後的那一次才能釋放晚到的鎖。不拋出。
     */
    public void releaseWithLateLockRecheck(String orderNo) {
        try {
            taskRepository.enqueueReleaseAfter(TsidUtil.generate(), orderNo, null, LATE_LOCK_RECHECK_DELAY_SEC);
        } catch (Exception e) {
            log.error("CouponReleaseService.enqueueDelayed.error", "寫入延後釋券任務失敗，仍嘗試當場釋放", e);
        }
        try {
            couponClient.releaseCouponByOrderNo(orderNo, null);
        } catch (Exception e) {
            log.warn("CouponReleaseService.release.deferred", "當場釋券失敗，交由任務重試，orderNo=" + orderNo);
        }
    }

    private void markDone(String orderNo) {
        try {
            taskRepository.markReleaseDone(orderNo);
        } catch (Exception e) {
            log.warn("CouponReleaseService.markDone.error", "釋券已完成但任務標記失敗，worker 會重送（冪等），orderNo=" + orderNo);
        }
    }
}
