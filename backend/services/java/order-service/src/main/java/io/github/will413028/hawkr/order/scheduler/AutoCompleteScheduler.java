package io.github.will413028.hawkr.order.scheduler;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.order.config.OrderConfig;
import io.github.will413028.hawkr.order.constant.OrderConstants;
import io.github.will413028.hawkr.order.entity.OrderEntity;
import io.github.will413028.hawkr.order.grpc.client.AfterSalesGrpcClient;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import io.github.will413028.hawkr.order.service.OrderService;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import java.util.List;

/**
 * 訂單自動完成排程
 *
 * 定期掃描已送達且到達自動完成時間的訂單，自動標記為 COMPLETED。
 * 排程間隔透過 hawkr.order.auto-complete-interval-ms 配置（預設每小時）。
 */
@Component
public class AutoCompleteScheduler {

    private static final ObsLogger log = ObsLogger.forClass(AutoCompleteScheduler.class);

    private final OrderRepository orderRepository;
    private final OrderService orderService;
    private final OrderConfig orderConfig;
    private final AfterSalesGrpcClient afterSalesGrpcClient;

    public AutoCompleteScheduler(
            OrderRepository orderRepository,
            OrderService orderService,
            OrderConfig orderConfig,
            AfterSalesGrpcClient afterSalesGrpcClient
    ) {
        this.orderRepository = orderRepository;
        this.orderService = orderService;
        this.orderConfig = orderConfig;
        this.afterSalesGrpcClient = afterSalesGrpcClient;
    }

    /**
     * 掃描到期的已送達訂單，逐筆自動完成
     */
    @Scheduled(fixedDelayString = "${hawkr.order.auto-complete-interval-ms:3600000}", initialDelay = 60_000)
    public void autoCompleteDeliveredOrders() {
        List<OrderEntity> orders = orderRepository.findAutoCompleteOrders(orderConfig.getAutoCompleteBatchSize());

        if (orders.isEmpty()) {
            return;
        }

        log.info("AutoCompleteScheduler.scan.found",
                "發現到期已送達訂單",
                "count", String.valueOf(orders.size()));

        int successCount = 0;
        int skippedActiveCase = 0;
        for (OrderEntity order : orders) {
            try {
                // #1228 收斂點縱深防禦：完成當下反查活躍售後 case。countdown_paused_at IS NULL 只能擋
                // 「已正確 pause 的訂單」，無法擋「pause / resume 事件漏發或亂序」殘留窗口；在完成前對
                // 活躍 case 做最終一次反查，擋掉所有此類情境（hasActiveCase 失敗時保守回 true → skip）。
                // 正常情況此反查恆 false（倒數暫停已先攔下），僅作縱深防禦兜底。
                if (afterSalesGrpcClient.hasActiveCase(order.getOrderNo())) {
                    skippedActiveCase++;
                    log.info("AutoCompleteScheduler.complete.skipActiveCase",
                            "訂單有活躍售後 case，跳過自動完成（不撥款）", "orderNo", order.getOrderNo());
                    continue;
                }
                boolean completed = orderService.completeOrder(order.getOrderNo(), OrderConstants.CONFIRM_TYPE_AUTO);
                if (completed) {
                    successCount++;
                }
            } catch (Exception e) {
                log.error("AutoCompleteScheduler.complete.error",
                        "自動完成訂單失敗: " + order.getOrderNo(), e);
            }
        }

        log.info("AutoCompleteScheduler.scan.done",
                "自動完成批次處理完畢，total=" + orders.size() + ", success=" + successCount
                        + ", skippedActiveCase=" + skippedActiveCase);
    }
}
