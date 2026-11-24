package io.github.will413028.hawkr.order.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.order.checkout.client.CheckoutCouponGrpcClient;
import io.github.will413028.hawkr.order.repository.CouponConsumptionTaskRepository;
import org.springframework.stereotype.Service;

/**
 * 取消與補償路徑的釋券（D3）。
 *
 * <p>先把釋券寫成 {@code coupon_consumption_tasks} 的 RELEASE 任務（以 order_no 為鍵、同一筆訂單只有一筆），
 * 再當場嘗試一次；當場失敗不拋出，任務留給 {@code CouponConsumptionWorker} 指數退避重試，超過次數轉人工。
 * 這取代原本「呼叫一次、失敗只 log、靠 Coupon AutoReleaseJob 兜底」的寫法：AutoReleaseJob 隨 B7 刪除，
 * 鎖沒有到期時間，漏送的釋放不補就永遠卡在 locked。
 *
 * <p>呼叫端都在取消的 transaction 之外（best-effort 補償），本類別不開 transaction。
 */
@Service
public class CouponReleaseService {

    private static final ObsLogger log = ObsLogger.forClass(CouponReleaseService.class);

    private final CouponConsumptionTaskRepository taskRepository;
    private final CheckoutCouponGrpcClient couponClient;

    public CouponReleaseService(CouponConsumptionTaskRepository taskRepository,
                                CheckoutCouponGrpcClient couponClient) {
        this.taskRepository = taskRepository;
        this.couponClient = couponClient;
    }

    /**
     * 釋放這筆訂單持有的券。不拋出：寫任務失敗與當場呼叫失敗各自記錄，兩者都失敗時才會漏掉（DB 與 coupon 同時不可用）。
     */
    public void release(String orderNo) {
        try {
            taskRepository.enqueueRelease(TsidUtil.generate(), orderNo);
        } catch (Exception e) {
            log.error("CouponReleaseService.enqueue.error", "寫入釋券任務失敗，仍嘗試當場釋放", e);
        }
        try {
            couponClient.releaseCouponByOrderNo(orderNo);
        } catch (Exception e) {
            log.warn("CouponReleaseService.release.deferred", "當場釋券失敗，交由任務重試，orderNo=" + orderNo);
            return;
        }
        try {
            taskRepository.markReleaseDone(orderNo);
        } catch (Exception e) {
            log.warn("CouponReleaseService.markDone.error", "釋券已完成但任務標記失敗，worker 會重送（冪等），orderNo=" + orderNo);
        }
    }
}
