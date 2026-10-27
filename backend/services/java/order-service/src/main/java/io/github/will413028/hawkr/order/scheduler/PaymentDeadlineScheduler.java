package io.github.will413028.hawkr.order.scheduler;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.OrderCancelReason;
import io.github.will413028.hawkr.order.config.OrderConfig;
import io.github.will413028.hawkr.order.entity.OrderEntity;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import io.github.will413028.hawkr.order.service.OrderExpiryCancellationService;
import org.springframework.scheduling.annotation.SchedulingConfigurer;
import org.springframework.scheduling.config.ScheduledTaskRegistrar;
import org.springframework.stereotype.Component;

import java.util.List;

/**
 * 付款期限排程任務
 *
 * 定期掃描已超過 payment_deadline 的待付款訂單，自動取消並釋放庫存、釋放優惠券。
 *
 * 實際的「取消前反查付款 SoT 收斂 / TX 內悲觀鎖 CAS / TX 外 best-effort 補償」三段式防護
 * 已抽到 {@link OrderExpiryCancellationService}（與賣家停權批次取消共用，03 §5.1a.2），
 * 本排程只負責掃描 + 逐筆呼叫。
 *
 * 禁止在取消流程中呼叫 ClosePaymentByOrderNo：
 * 付款交易由 Payment Service ExpiredTransactionScheduler 根據 expire_at 自然過期。
 * 若取消後立即關閉交易，遲到的 PayUni Webhook 會因版本衝突無法處理，導致客戶扣款但無退款。
 */
@Component
public class PaymentDeadlineScheduler implements SchedulingConfigurer {

    private static final ObsLogger log = ObsLogger.forClass(PaymentDeadlineScheduler.class);

    private final OrderRepository orderRepository;
    private final OrderExpiryCancellationService orderExpiryCancellationService;
    private final OrderConfig orderConfig;

    public PaymentDeadlineScheduler(
            OrderRepository orderRepository,
            OrderExpiryCancellationService orderExpiryCancellationService,
            OrderConfig orderConfig
    ) {
        this.orderRepository = orderRepository;
        this.orderExpiryCancellationService = orderExpiryCancellationService;
        this.orderConfig = orderConfig;
    }

    @Override
    public void configureTasks(ScheduledTaskRegistrar taskRegistrar) {
        taskRegistrar.addFixedDelayTask(this::cancelExpiredOrders, orderConfig.getCancelScanIntervalMs());
    }

    /**
     * 掃描超時待付款訂單（間隔由 OrderConfig.cancelScanIntervalMs 控制，預設 15 秒）
     */
    public void cancelExpiredOrders() {
        List<OrderEntity> expiredOrders = orderRepository.findExpiredPendingOrders();

        if (expiredOrders.isEmpty()) {
            return;
        }

        log.info("PaymentDeadlineScheduler.scan.found",
                "發現超時待付款訂單",
                "count", String.valueOf(expiredOrders.size()));

        for (OrderEntity order : expiredOrders) {
            try {
                boolean cancelled = orderExpiryCancellationService.cancelPendingOrder(
                        order.getOrderNo(), OrderCancelReason.PAYMENT_TIMEOUT.value());
                if (cancelled) {
                    log.info("PaymentDeadlineScheduler.cancel.success",
                            "超時自動取消訂單",
                            "orderNo", order.getOrderNo());
                }
            } catch (Exception e) {
                log.error("PaymentDeadlineScheduler.cancel.error",
                        "取消超時訂單失敗: " + order.getOrderNo(), e);
            }
        }
    }
}
