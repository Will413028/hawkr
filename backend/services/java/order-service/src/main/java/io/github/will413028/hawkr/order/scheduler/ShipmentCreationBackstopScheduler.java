package io.github.will413028.hawkr.order.scheduler;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.order.config.OrderConfig;
import io.github.will413028.hawkr.order.entity.OrderEntity;
import io.github.will413028.hawkr.order.mq.OrderEventPublisher;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import java.time.LocalDateTime;
import java.util.List;

/**
 * 付款後建單兜底排程（03 §8.11）。
 *
 * <p>業務 SLA 兜底（非 reconciliation/drift；等同 PaymentDeadlineScheduler 對未付款的逾期取消）：
 * 付款後超過寬限期仍無物流單（{@code shipment_no} 為空）者，先重試建單，
 * 超過重試上限仍失敗則系統取消 + 退款。
 *
 * <p>兩階段：先以單筆 CAS UPDATE 搶佔（自動提交），搶到才於事務外 publish，
 * 避免事件在事務內發送。
 *
 * <p><b>搶佔語意（issue #2294 H4）</b>：兩個分支各有一道
 * <b>lease on {@code orders.shipment_create_next_retry_at}</b> 閘門，都在 publish <b>之前</b>，
 * 租約長度<b>同為</b> {@code shipmentBackstopBackoffMinutes}（預設 30 分）。兩者都<b>不是</b>
 * 永久旗標：租約到期該列自動回到候選集合。{@code orders.shipment_backstop_given_up_at} 只是
 * <b>觀測欄</b>（供 sql_exporter 告警），不進候選述詞。詳見
 * {@code OrderRepository.bumpShipmentCreateAttempt} / {@code claimShipmentBackstopGiveUp}
 * 與 {@code docs/開發規範/多副本排程搶佔規範.md}。
 *
 * <p><b>租約長度的上界（issue #2294 R4-P1-1，2026-08-08 修正）</b>：
 * give-up 租約原為獨立設定 {@code shipmentBackstopGiveUpLeaseMinutes = 1440}（1 天）。
 * 那個值只論證過<b>下界</b>（「1 天遠大於一次 MQ publish」）與「候選窗口 = ∞ 故上界無條件成立」——
 * 但那是<b>述詞</b>層的上界，不是業務層的上界，而租約長度在這裡同時是
 * 「{@link OrderEventPublisher#publishShipmentCreationFailed} 靜默失敗後的救援窗口」：
 * 該 publish 走 {@code OrderEventPublisher.publish}，<b>吞掉所有例外</b>（03 §8.13「publisher
 * 失敗語意」明列，本 issue 裁決不改），所以 claim 已寫入、事件卻沒發出去時，呼叫端拿不到任何訊號，
 * 該訂單只能等租約到期才會再被掃到。1440 分 ⇒ <b>買家已付款、無物流單、無取消、無退款、無告警</b>
 * 的狀態要維持 24 小時；而修復前的舊行為（give-up 分支不寫任何欄位）是每個 tick 重發，
 * 恢復窗口 5 分鐘 ⇒ 該值等於把救援延遲放大 <b>288 倍</b>，相對現況是退化。
 *
 * <p>因此上界取<b>同一支排程的重試節奏</b> {@code shipmentBackstopBackoffMinutes}（30 分）：
 * give-up 是這條鏈路的<b>終局救援</b>，它的救援延遲沒有理由比非終局的重試分支更長
 * ⇒ 不變式 <b>{@code giveUpLease ≤ backoffMinutes}</b>，用「共用同一個值」讓它結構上恆成立
 * （少一個可以被改回 1440 的旋鈕）。下界仍然成立：單筆工作只有一次本地 MQ publish（毫秒級）。
 * 30 分鐘的重複成本可接受——下游 {@code cancelShipmentBySystem} 本來就冪等
 * （{@code uk_seller_cancel_order_no}），原缺陷「每 5 分鐘 × 每 pod 各一次取消+退款」已被消滅。
 */
@Component
@ConditionalOnProperty(name = "hawkr.order.shipment-backstop-enabled", havingValue = "true", matchIfMissing = true)
public class ShipmentCreationBackstopScheduler {

    private static final ObsLogger log = ObsLogger.forClass(ShipmentCreationBackstopScheduler.class);
    private static final int BATCH_SIZE = 100;

    private final OrderRepository orderRepository;
    private final OrderEventPublisher orderEventPublisher;
    private final OrderConfig orderConfig;

    public ShipmentCreationBackstopScheduler(OrderRepository orderRepository,
                                             OrderEventPublisher orderEventPublisher,
                                             OrderConfig orderConfig) {
        this.orderRepository = orderRepository;
        this.orderEventPublisher = orderEventPublisher;
        this.orderConfig = orderConfig;
    }

    @Scheduled(fixedDelayString = "${hawkr.order.shipment-backstop-interval-ms:300000}", initialDelay = 90_000)
    public void scanStuckPaidOrders() {
        LocalDateTime now = DateTimeUtils.nowUtc();
        LocalDateTime graceCutoff = now.minusMinutes(orderConfig.getShipmentBackstopGraceMinutes());

        List<OrderEntity> orders = orderRepository.findPaidOrdersWithoutShipmentPastGrace(now, graceCutoff, BATCH_SIZE);
        if (orders.isEmpty()) {
            return;
        }

        log.info("ShipmentCreationBackstopScheduler.scan.found",
                "發現付款後逾期無物流單訂單 count=" + orders.size());

        int maxAttempts = orderConfig.getShipmentBackstopMaxAttempts();
        long backoffMinutes = orderConfig.getShipmentBackstopBackoffMinutes();

        for (OrderEntity order : orders) {
            try {
                handleOne(order, now, maxAttempts, backoffMinutes);
            } catch (Exception e) {
                log.error("ShipmentCreationBackstopScheduler.handle.error",
                        "處理逾期無物流單訂單失敗: " + order.getOrderNo(), e);
            }
        }
    }

    /**
     * 處理單筆逾期無物流單訂單。
     *
     * <p><b>issue #2294 H4</b>：<b>兩個分支各一道 lease 閘門</b>，都在 publish 之前。
     * claim 回 0 ⇒ 另一個 pod 已認領（或訂單已離開 PAID）⇒ 一個副作用都不做。
     */
    private void handleOne(OrderEntity order, LocalDateTime now, int maxAttempts,
                           long backoffMinutes) {
        String orderNo = order.getOrderNo();
        int attempts = order.getShipmentCreateAttempts() == null ? 0 : order.getShipmentCreateAttempts();

        if (attempts < maxAttempts) {
            // 重試建單：先以 lease CAS 搶佔（同時累加 attempts + 推進下次重試時間），搶到才 publish
            //
            // ⚠️ R4-P1-2（2026-08-08）：租約基準取 **claim 當下**而非 tick 起點的 now
            //（與 OrderOutboxPublisherScheduler / StockCompensationWorker /
            //  PendingPaymentReconcileScheduler / payuni-webhook OutboxPublisherScheduler 一致）。
            // 取 tick 起點 ⇒ 全批共用同一個到期時間 ⇒ 第 k 筆的**有效**租約
            // = backoffMinutes − (前 k−1 筆的耗時)，不變式退化成「批次總耗時 < 租約」而非
            // 「**單筆**最壞耗時 < 租約」。本排程今天不會觸發（BATCH_SIZE=100、單筆只有一次
            // 本地 MQ publish ⇒ 100 × 毫秒 ≪ 30 分），但那是**參數巧合不是結構保證**：
            // 往這個迴圈裡加一次 gRPC（例如查快照組 payload）就會破。
            // 述詞用的 now 維持 tick 起點：那個方向讓候選集合更小，是保守的。
            int claimed = orderRepository.bumpShipmentCreateAttempt(
                    orderNo, DateTimeUtils.nowUtc().plusMinutes(backoffMinutes), now);
            if (claimed == 0) {
                return;
            }
            orderEventPublisher.publishShipmentCreateRequested(orderNo);
            log.info("ShipmentCreationBackstopScheduler.retry",
                    "重試建單 attempts=" + (attempts + 1) + "/" + maxAttempts, "orderNo", orderNo);
        } else {
            // 超過上限：請 AfterSales 系統取消 + 退款（冪等）。
            // give-up 同樣先搶佔——但用的是**租約**不是永久旗標：取消一旦生效訂單就離開 PAID、
            // 自然退出候選；租約只在「取消沒生效」或「publish 靜默失敗」時才會再響一次
            // （可觀測 > 不可觀測）。租約長度與重試分支同值，上界推導見類 javadoc。
            // 租約基準同樣取 claim 當下（理由見重試分支的註解）。
            int claimed = orderRepository.claimShipmentBackstopGiveUp(
                    orderNo, DateTimeUtils.nowUtc().plusMinutes(backoffMinutes), now);
            if (claimed == 0) {
                return;
            }
            orderEventPublisher.publishShipmentCreationFailed(orderNo);
            log.warn("ShipmentCreationBackstopScheduler.giveUp",
                    "建單重試上限耗盡，觸發取消退款 attempts=" + attempts, "orderNo", orderNo);
        }
    }
}
