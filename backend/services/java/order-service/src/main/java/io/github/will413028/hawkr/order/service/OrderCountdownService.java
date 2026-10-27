package io.github.will413028.hawkr.order.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.order.grpc.client.AfterSalesGrpcClient;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import org.springframework.stereotype.Service;

/**
 * 訂單自動完成倒數的暫停 / 恢復（03 §5.8）。
 *
 * 設計原則：
 * - 暫停冪等：SQL 帶 {@code countdown_paused_at IS NULL} 守門
 * - 恢復冪等：SQL 帶 {@code countdown_paused_at IS NOT NULL} 守門
 * - 多 case 並存：恢復前必須反查 AfterSales 確認該訂單已無活躍 case
 */
@Service
public class OrderCountdownService {

    private static final ObsLogger log = ObsLogger.forClass(OrderCountdownService.class);

    private final OrderRepository orderRepository;
    private final AfterSalesGrpcClient afterSalesGrpcClient;

    public OrderCountdownService(OrderRepository orderRepository,
                                 AfterSalesGrpcClient afterSalesGrpcClient) {
        this.orderRepository = orderRepository;
        this.afterSalesGrpcClient = afterSalesGrpcClient;
    }

    /**
     * 嘗試暫停倒數。冪等：若訂單非 status=40 或已暫停則 affected=0。
     */
    public void pause(String orderNo, String reason) {
        int affected = orderRepository.pauseCountdown(orderNo);
        if (affected > 0) {
            log.info("OrderCountdownService.pause.success",
                    "暫停自動完成倒數成功 reason=" + reason, "orderNo", orderNo);
        } else {
            log.info("OrderCountdownService.pause.skip",
                    "訂單非 status=40 或已暫停，跳過 reason=" + reason, "orderNo", orderNo);
        }
    }

    /**
     * 嘗試恢復倒數。先反查 AfterSales 確認無活躍 case 才放行。
     */
    public void tryResume(String orderNo, String reason) {
        if (afterSalesGrpcClient.hasActiveCase(orderNo)) {
            log.info("OrderCountdownService.resume.skip.activeCase",
                    "訂單仍有活躍 case，不恢復倒數 reason=" + reason, "orderNo", orderNo);
            return;
        }
        int affected = orderRepository.resumeCountdown(orderNo);
        if (affected > 0) {
            log.info("OrderCountdownService.resume.success",
                    "恢復自動完成倒數成功 reason=" + reason, "orderNo", orderNo);
        } else {
            log.info("OrderCountdownService.resume.skip",
                    "訂單非 status=40 或未暫停，跳過 reason=" + reason, "orderNo", orderNo);
        }
    }
}
