package io.github.will413028.hawkr.payment.scheduler;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.payment.config.ReconcileProperties;
import io.github.will413028.hawkr.payment.domain.model.PaymentTransaction;
import io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository;
import io.micrometer.core.instrument.MeterRegistry;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import java.time.LocalDateTime;
import java.util.EnumMap;
import java.util.List;
import java.util.Map;

/**
 * L2 補救路徑排程器
 *
 * 設計參考：docs/詳細設計書/05-支付服務（Payment Service）詳細設計書.md §5.7.1
 *
 * 每 5 分鐘（可由 hawkr.payment.reconcile.pending.fixed-delay-ms 調整）
 * 撈出 reconcile 候選交易，逐筆呼叫 PendingPaymentReconcileService 處理。
 *
 * 候選條件參考 PaymentTransactionMapper.findReconcileCandidates。
 */
@Component
@ConditionalOnProperty(prefix = "hawkr.payment.reconcile.pending", name = "enabled", havingValue = "true", matchIfMissing = true)
public class PendingPaymentReconcileScheduler {

    private static final ObsLogger log = ObsLogger.forClass(PendingPaymentReconcileScheduler.class);

    // issue #1369：reconcile outcome 業務 metric 名（Prometheus 對 Counter 自動補 _total →
    // 暴露為 hawkr_payment_reconcile_outcome_total{outcome=...}，供 PaymentReconcileConflict 告警）
    private static final String METRIC_RECONCILE_OUTCOME = "hawkr_payment_reconcile_outcome";

    private final PaymentTransactionRepository transactionRepository;
    private final PendingPaymentReconcileService reconcileService;
    private final ReconcileProperties properties;
    private final MeterRegistry meterRegistry;

    public PendingPaymentReconcileScheduler(
            PaymentTransactionRepository transactionRepository,
            PendingPaymentReconcileService reconcileService,
            ReconcileProperties properties,
            MeterRegistry meterRegistry
    ) {
        this.transactionRepository = transactionRepository;
        this.reconcileService = reconcileService;
        this.properties = properties;
        this.meterRegistry = meterRegistry;
    }

    @Scheduled(fixedDelayString = "${hawkr.payment.reconcile.pending.fixed-delay-ms:300000}")
    public void runReconcile() {
        LocalDateTime now = DateTimeUtils.nowUtc();
        LocalDateTime initialDelayBefore = now.minusMinutes(properties.getInitialDelayMinutes());
        LocalDateTime graceFloor = now.minusMinutes(properties.getGraceMinutes());
        LocalDateTime retryBefore = now.minusMinutes(properties.getRetryIntervalMinutes());

        List<PaymentTransaction> candidates = transactionRepository.findReconcileCandidates(
                initialDelayBefore, graceFloor, retryBefore, properties.getBatchSize());

        if (candidates.isEmpty()) {
            return;
        }

        log.info("PendingPaymentReconcileScheduler.tick.start",
                "reconcile 開始 count=" + candidates.size());

        Map<PendingPaymentReconcileService.ReconcileOutcome, Integer> stats =
                new EnumMap<>(PendingPaymentReconcileService.ReconcileOutcome.class);

        for (PaymentTransaction candidate : candidates) {
            // issue #2294 H6：搶佔閘門必須在任何副作用之前——reconcileSingle 內含 PayUni HTTP 查詢、
            // rescue audit log INSERT、管道 B 的重複付款處理（三者皆無冪等鍵）。claim 之後只剩一個
            // pod 能進，連 PayUni 查詢都不會重複發。閘門為 lease on last_reconciled_at（述詞＝候選
            // SQL 第 4 條），退避間隔到期該列自動回到候選集合。
            //
            // ⚠️ R4-P1-2（2026-08-08）：stamp（＝租約基準）取 **claim 當下**而非 tick 起點的 now。
            // 本排程的有效租約 = stamp + retryIntervalMinutes(5 分)，而單筆最壞耗時是一次 PayUni
            // 同步查詢（PayUniHttpClient connect+write+read 各 30 秒 ⇒ 90 秒）＋ 可能一次退款；
            // batchSize 預設 50 ⇒ 批次上界 ≈ 75 分鐘 ≫ 5 分鐘租約。拿 tick 起點當基準時，
            // 跑到第 4 筆之後租約就已過期，另一個 pod 會合法搶走同一筆 —— 而 reconcileSingle 內的
            // PayUni 查詢／rescue audit INSERT／管道 B 重複付款處理三者皆無冪等鍵。
            // retryBefore（述詞）維持 tick 起點：那個方向讓候選集合更小，是保守的。
            if (transactionRepository.claimForReconcile(
                    candidate.tradeNo(), DateTimeUtils.nowUtc(), retryBefore) == 0) {
                continue;
            }
            try {
                var outcome = reconcileService.reconcileSingle(candidate);
                stats.merge(outcome, 1, Integer::sum);
                recordOutcomeMetric(outcome);
            } catch (Exception e) {
                stats.merge(PendingPaymentReconcileService.ReconcileOutcome.FAILED, 1, Integer::sum);
                recordOutcomeMetric(PendingPaymentReconcileService.ReconcileOutcome.FAILED);
                log.error("PendingPaymentReconcileScheduler.item.failed",
                        "reconcile 單筆處理拋例外: " + e.getMessage() + ", trade_no=" + candidate.tradeNo(), e);
            }

            // 對 PayUni 友善
            if (properties.getPerItemSleepMs() > 0) {
                try {
                    Thread.sleep(properties.getPerItemSleepMs());
                } catch (InterruptedException ie) {
                    Thread.currentThread().interrupt();
                    break;
                }
            }
        }

        log.info("PendingPaymentReconcileScheduler.tick.done",
                String.format("reconcile 完成 scanned=%d updated=%d rescue_success=%d rescue_manual=%d rescue_failed=%d duplicate_detected=%d failed=%d",
                        candidates.size(),
                        stats.getOrDefault(PendingPaymentReconcileService.ReconcileOutcome.UPDATED, 0),
                        stats.getOrDefault(PendingPaymentReconcileService.ReconcileOutcome.RESCUE_SUCCESS, 0),
                        stats.getOrDefault(PendingPaymentReconcileService.ReconcileOutcome.RESCUE_MANUAL_REQUIRED, 0),
                        stats.getOrDefault(PendingPaymentReconcileService.ReconcileOutcome.RESCUE_FAILED, 0),
                        stats.getOrDefault(PendingPaymentReconcileService.ReconcileOutcome.RESCUE_DUPLICATE_DETECTED, 0),
                        stats.getOrDefault(PendingPaymentReconcileService.ReconcileOutcome.FAILED, 0)));
    }

    /**
     * issue #1369：把單筆 reconcile 結果匯出為 tagged Counter，供 Prometheus 監測。
     * RESCUE_MANUAL_REQUIRED / RESCUE_FAILED 表「金流成功但本地未收斂、需人工介入或退款失敗」，
     * 由 business-alerts.yml PaymentReconcileConflict 規則告警。
     */
    private void recordOutcomeMetric(PendingPaymentReconcileService.ReconcileOutcome outcome) {
        meterRegistry.counter(METRIC_RECONCILE_OUTCOME, "outcome", outcome.name()).increment();
    }
}
