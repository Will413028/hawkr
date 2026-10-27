package io.github.will413028.hawkr.shipping.scheduler;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.ShipmentAnomalyType;
import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import io.github.will413028.hawkr.common.utils.code.helper.ShipmentAnomalyRules;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import io.github.will413028.hawkr.shipping.mapper.ShipmentMapper;
import io.github.will413028.hawkr.shipping.mq.ShipmentAnomalyEventPublisher;
import io.github.will413028.hawkr.shipping.service.ShippingService;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import java.time.LocalDateTime;
import java.util.List;

/**
 * §5.19 物流貨態補收排程（L2 補救路徑）
 *
 * 主動向 PayUni 查詢未終態物流單貨態，補救 Webhook 漏收。
 *
 * 關鍵設計：
 * - 路徑唯一匯流：呼叫 {@link ShippingService#queryAndSyncStatus(String)}，與 §5.5 Webhook、§5.7 按需查詢共用同一條主路徑
 * - 退避控制：由 service 層 {@code touchLastReconciledAt} 統一處理，scheduler 不重複 touch
 * - 終態與 Uber 已透過 SQL 部分索引在掃描階段排除
 *
 * <p><b>搶佔語意（issue #2294 L2）</b>：原本<b>完全沒有搶佔</b>，多副本下同一張單會被兩個 pod
 * 各打一次 PayUni 查詢、各發一次異常事件。修法＝<b>lease on 既有的
 * {@code shipments.last_reconciled_at}</b>（候選述詞本來就含這個租約形狀的條件，零 migration）：
 * claim 把它推到 claim 當下，該列就退出候選集合 {@code backoff-minutes}（預設 15 分）＝有效租約，
 * 到期自動回來。
 *
 * <p>⚠️ 刻意<b>另加</b>一支 {@code claimForReconcile}，而不是把既有的 {@code touchLastReconciledAt}
 * 改成 CAS：後者的唯一寫入點在 {@link ShippingService#queryAndSyncStatus} 的 finally，
 * 而該方法同時被 §5.7 按需查詢 API 共用——改成 CAS 會讓 API 路徑的退避刷新靜默失效。
 * claim 與 finally 兩次寫入都是往前推，不互相衝突。
 *
 * 設計書：docs/詳細設計書/06-物流微服務（Shipping Service）詳細設計書.md §5.19 / §8.5
 */
@Component
@ConditionalOnProperty(name = "hawkr.shipping.reconcile.enabled", havingValue = "true", matchIfMissing = true)
public class ShipmentStatusReconcileScheduler {

    private static final ObsLogger log = ObsLogger.forClass(ShipmentStatusReconcileScheduler.class);

    private final ShipmentMapper shipmentMapper;
    private final ShippingService shippingService;
    private final ShipmentAnomalyEventPublisher anomalyEventPublisher;

    @Value("${hawkr.shipping.reconcile.batch-size:100}")
    private int batchSize;

    @Value("${hawkr.shipping.reconcile.max-age-days:7}")
    private int maxAgeDays;

    @Value("${hawkr.shipping.reconcile.backoff-minutes:15}")
    private int backoffMinutes;

    /** §5.21 PACKAGE_LOST（status=32 卡）門檻天數 */
    @Value("${hawkr.shipping.anomaly.package-lost-days:7}")
    private int packageLostDays;

    public ShipmentStatusReconcileScheduler(ShipmentMapper shipmentMapper,
                                            ShippingService shippingService,
                                            ShipmentAnomalyEventPublisher anomalyEventPublisher) {
        this.shipmentMapper = shipmentMapper;
        this.shippingService = shippingService;
        this.anomalyEventPublisher = anomalyEventPublisher;
    }

    /**
     * 預設每 10 分鐘執行一次，可由 SHIPPING_RECONCILE_INTERVAL_MS 覆寫
     */
    @Scheduled(fixedDelayString = "${hawkr.shipping.reconcile.interval-ms:600000}")
    public void reconcile() {
        LocalDateTime now = DateTimeUtils.nowUtc();
        LocalDateTime maxAgeCutoff = now.minusDays(maxAgeDays);
        LocalDateTime backoffCutoff = now.minusMinutes(backoffMinutes);

        List<ShipmentEntity> candidates = shipmentMapper.findReconcileCandidates(maxAgeCutoff, backoffCutoff, batchSize);

        if (candidates.isEmpty()) {
            return;
        }

        int processed = 0;
        int failed = 0;
        for (ShipmentEntity entity : candidates) {
            // issue #2294 L2：lease claim —— 搶到才打 PayUni 查詢（0 = 另一個 pod 已搶走）。
            // ⚠️ 租約基準取 **claim 當下**（DateTimeUtils.nowUtc()）而非 tick 起點的 now：
            // 取 tick 起點 ⇒ 全批共用同一個到期時間 ⇒ 第 k 筆的**有效**租約
            // = backoffMinutes − (前 k−1 筆的耗時)，不變式退化成「批次總耗時 < 租約」，
            // 而 batch-size 預設 100、單筆是一次 PayUni HTTP ⇒ 那條會被 batchSize 直接打破。
            // 改在這裡取之後，不變式降為「**單筆**最壞耗時 < 租約」，與 batchSize 無關。
            // 述詞用的 backoffCutoff 維持 tick 起點：那個方向讓可搶佔集合更小，是保守的。
            int claimed = shipmentMapper.claimForReconcile(
                    entity.getId(), DateTimeUtils.nowUtc(), backoffCutoff);
            if (claimed == 0) {
                log.info("ShipmentStatusReconcileScheduler.claim.skip",
                        "租約由他人持有，skip shipmentNo=" + entity.getShipmentNo());
                continue;
            }
            try {
                shippingService.queryAndSyncStatus(entity.getShipmentNo());
                processed++;
            } catch (Exception e) {
                failed++;
                log.error("ShipmentStatusReconcileScheduler.process.failed",
                        "reconcile 失敗 shipmentNo=" + entity.getShipmentNo(), e);
                // 失敗時 service 層 finally 已 touch，下一輪會自然退避；此處不再額外 touch
            }
            // §5.21 配送異常賣家通知：reload row 後依當下 status 推導異常並發 event（fire-and-forget）
            try {
                ShipmentEntity refreshed = (entity.getId() != null) ? shipmentMapper.selectById(entity.getId()) : null;
                if (refreshed != null) {
                    publishAnomaliesIfAny(refreshed);
                }
            } catch (Exception e) {
                log.warn("ShipmentStatusReconcileScheduler.anomaly.publish.failed",
                        "shipmentNo=" + entity.getShipmentNo() + " err=" + e.getMessage());
            }
        }

        log.info("ShipmentStatusReconcileScheduler.process.done",
                "貨態補收完成 candidates=" + candidates.size() + " processed=" + processed + " failed=" + failed);
    }

    /**
     * §5.21 依當下 shipments.status 推導 anomaly 並發 event。
     * <ul>
     *   <li>status ∈ {33, 41, 44} → 44 視為 PACKAGE_LOST、其他 PROVIDER_STATUS_ABNORMAL</li>
     *   <li>status=32 且 updated_at &lt; NOW() - 7d → PACKAGE_LOST</li>
     *   <li>expected_delivery_date &lt; CURRENT_DATE 且 status NOT IN (11, 41, 44)
     *       且 provider_status_time IS NULL 或 &gt; 3 天無更新 → DELIVERY_OVERDUE
     *       （ETA 過但軌跡近期仍活躍視為仍在正常移動，不打異常；對應 Mapper CASE_ANOMALY_TYPE）</li>
     * </ul>
     */
    private void publishAnomaliesIfAny(ShipmentEntity entity) {
        String status = entity.getStatus();
        if (status == null) {
            return;
        }

        // 終態（11=待付款返貨/41=已取消）排除 DELIVERY_OVERDUE，但允許 33/44 異常通知
        if (ShipmentAnomalyRules.abnormalShipmentStatusValues().contains(status)) {
            ShipmentAnomalyType type = ShipmentAnomalyRules.providerStatusAnomalyOf(status);
            anomalyEventPublisher.publishAnomalyDetected(
                    type,
                    entity.getOrderNo(),
                    String.valueOf(entity.getId()),
                    entity.getStoreId(),
                    entity.getBuyerId()
            );
        }

        // WAITING_PICKUP（卡 N 天）→ PACKAGE_LOST
        if (ShippingStatus.WAITING_PICKUP.value().equals(status) && entity.getUpdatedAt() != null) {
            LocalDateTime threshold = DateTimeUtils.nowUtc().minusDays(packageLostDays);
            if (entity.getUpdatedAt().isBefore(threshold)) {
                anomalyEventPublisher.publishAnomalyDetected(
                        ShipmentAnomalyType.PACKAGE_LOST,
                        entity.getOrderNo(),
                        String.valueOf(entity.getId()),
                        entity.getStoreId(),
                        entity.getBuyerId()
                );
            }
        }

        // DELIVERY_OVERDUE（排除已取貨 / 已取消 / 已遺失終態，
        // 且要求物流商超過 3 天無狀態更新；對應 Mapper CASE_ANOMALY_TYPE 收緊規則）
        if (!ShippingStatus.PICKED_UP.value().equals(status)
                && !ShippingStatus.CANCELLED.value().equals(status)
                && !ShippingStatus.PACKAGE_LOST.value().equals(status)
                && entity.getExpectedDeliveryDate() != null
                && entity.getExpectedDeliveryDate().isBefore(java.time.LocalDate.now(java.time.ZoneOffset.UTC))
                && (entity.getProviderStatusTime() == null
                    || entity.getProviderStatusTime().isBefore(DateTimeUtils.nowUtc().minusDays(3)))) {
            anomalyEventPublisher.publishAnomalyDetected(
                    ShipmentAnomalyType.DELIVERY_OVERDUE,
                    entity.getOrderNo(),
                    String.valueOf(entity.getId()),
                    entity.getStoreId(),
                    entity.getBuyerId()
            );
        }
    }
}
