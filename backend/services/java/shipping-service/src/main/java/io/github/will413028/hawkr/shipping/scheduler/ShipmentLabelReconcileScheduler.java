package io.github.will413028.hawkr.shipping.scheduler;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.LabelPrintStatus;
import io.github.will413028.hawkr.common.utils.code.ShipmentAnomalyType;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import io.github.will413028.hawkr.shipping.mapper.ShipmentMapper;
import io.github.will413028.hawkr.shipping.mq.ShipmentAnomalyEventPublisher;
import io.github.will413028.hawkr.shipping.provider.FulfillmentContext;
import io.github.will413028.hawkr.shipping.provider.FulfillmentProviderRegistry;
import io.github.will413028.hawkr.shipping.provider.capability.LabelCapability;
import io.github.will413028.hawkr.shipping.service.ShippingService;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import java.time.LocalDateTime;
import java.util.List;
import java.util.Optional;

/**
 * §5.19.B 列印狀態機補收排程
 *
 * <p>對 {@code label_print_status IN ('4102','4104')} 且超過 N 分鐘的 shipment 主動向 provider
 * 取回標籤識別，若已產生則 transition 為 SUCCEEDED（與 Print Webhook 同一條 service path：
 * {@link ShippingService#processWebhookPrint}）。
 *
 * <p>用於補救 PayUni 列印 Webhook 漏收 / 同步呼叫失敗（O275613245 incident）。
 *
 * <h2>🔴 本類別<b>不再持有 provider client</b></h2>
 * 遷移前它自己打 {@code /api/logistics/query}，因而自己算一份 {@code LgsType}
 * （{@code deriveLgsType}）——那是 {@code AbstractPayUniAdapter.lgsTypeCode()} 之外的
 * <b>第二份 provider 參數推導</b>，兩者沒有任何東西保證同步。
 * 改成問 {@link LabelCapability#fetchLabelIdentifiers} 之後，那份副本連同它的字面分支一起消失，
 * 本類別只讀中性的 key→value。
 *
 * <p><b>搶佔語意（issue #2294 L3）</b>：原本<b>完全沒有搶佔</b>，多副本下同一張單會被兩個 pod
 * 各查一次、各發一次異常事件。修法＝<b>lease on 新增的 {@code shipments.label_reconcile_next_at}</b>
 * （V10 migration），claim 成功才 {@code reconcileOne}。
 *
 * <p>⚠️ 為什麼要新增欄位而不挪用既有的 {@code label_print_requested_at}：後者是業務時戳
 * （「何時送印」），推它等於竄改業務事實，而且它同時是 {@code requestedBefore} grace 與
 * {@code maxAgeCutoff} 兩個判準的基準——推它會把該列的候選壽命無限延長。
 *
 * <p>租約 10 分鐘（＝一個 tick）≪ 候選窗口 7 天（{@code label.max-age-days}）⇒ 窗口內仍有
 * 約 1000 次重試機會，不會退化成 at-most-once。
 *
 * 設計書：06 §2.4.5.1 / §5.19.3.B / §8.5
 */
@Component
@ConditionalOnProperty(name = "hawkr.shipping.reconcile.enabled", havingValue = "true", matchIfMissing = true)
public class ShipmentLabelReconcileScheduler {

    private static final ObsLogger log = ObsLogger.forClass(ShipmentLabelReconcileScheduler.class);

    /**
     * issue #2294 L3：租約長度（分）＝ 一個 tick（{@code label.interval-ms} 預設 600000）。
     *
     * <p>下界：單筆最壞耗時 = 一次 provider 查詢 ＋（成功時）一次 {@code processWebhookPrint}，
     * 遠小於 10 分。上界：必須 ≪ 候選窗口 7 天，且不該長於一個 tick——租約是「pod 崩潰後多久
     * 才會有人重試」的窗口，取得比 tick 大只會讓補收更慢，卻不會讓它更安全。
     */
    static final long LEASE_MINUTES = 10L;

    private final ShipmentMapper shipmentMapper;
    private final ShippingService shippingService;
    private final ShipmentAnomalyEventPublisher anomalyEventPublisher;
    private final FulfillmentProviderRegistry fulfillmentProviders;

    @Value("${hawkr.shipping.reconcile.label.batch-size:50}")
    private int batchSize;

    @Value("${hawkr.shipping.reconcile.label.max-age-days:7}")
    private int maxAgeDays;

    /**
     * REQUESTED 過久才掃 — 給同步 webhook 一個窗口先處理
     */
    @Value("${hawkr.shipping.reconcile.label.requested-grace-minutes:5}")
    private int requestedGraceMinutes;

    /** §5.21 CVS_LABEL_UNPRINTED 異常通知門檻（小時） */
    @Value("${hawkr.shipping.anomaly.cvs-label-stuck-hours:48}")
    private int cvsLabelStuckHours;

    public ShipmentLabelReconcileScheduler(
            ShipmentMapper shipmentMapper,
            ShippingService shippingService,
            ShipmentAnomalyEventPublisher anomalyEventPublisher,
            FulfillmentProviderRegistry fulfillmentProviders) {
        this.shipmentMapper = shipmentMapper;
        this.shippingService = shippingService;
        this.anomalyEventPublisher = anomalyEventPublisher;
        this.fulfillmentProviders = fulfillmentProviders;
    }

    @Scheduled(fixedDelayString = "${hawkr.shipping.reconcile.label.interval-ms:600000}")
    public void run() {
        LocalDateTime now = DateTimeUtils.nowUtc();
        LocalDateTime requestedBefore = now.minusMinutes(requestedGraceMinutes);
        LocalDateTime maxAgeCutoff = now.minusDays(maxAgeDays);

        List<ShipmentEntity> candidates = shipmentMapper.findLabelReconcileCandidates(
                requestedBefore, maxAgeCutoff, now, batchSize);

        if (candidates.isEmpty()) {
            return;
        }

        log.info("ShipmentLabelReconcileScheduler.found",
                "找到 label 待補收候選 " + candidates.size() + " 筆");

        int processed = 0;
        for (ShipmentEntity entity : candidates) {
            LocalDateTime leaseUntil = DateTimeUtils.nowUtc().plusMinutes(LEASE_MINUTES);
            int claimed = shipmentMapper.claimLabelReconcile(
                    entity.getId(),
                    LabelPrintStatus.REQUESTED.value(),
                    LabelPrintStatus.FAILED.value(),
                    now,
                    leaseUntil);
            if (claimed == 0) {
                log.info("ShipmentLabelReconcileScheduler.claim.skip",
                        "租約由他人持有或狀態已變更，skip",
                        "shipmentNo", entity.getShipmentNo());
                continue;
            }
            try {
                if (reconcileOne(entity)) {
                    processed++;
                }
            } catch (Exception e) {
                log.warn("ShipmentLabelReconcileScheduler.itemError",
                        "處理單筆失敗: " + e.getMessage(),
                        "shipmentNo", entity.getShipmentNo());
            }
        }

        log.info("ShipmentLabelReconcileScheduler.done",
                "label 補收完成 processed=" + processed + " total=" + candidates.size());
    }

    /**
     * @return true 表已成功補拉到標籤識別並 transition；false 表 provider 端尚未產生
     */
    private boolean reconcileOne(ShipmentEntity entity) {
        Optional<LabelCapability> label = labelCapabilityOf(entity);
        if (label.isEmpty()) {
            log.warn("ShipmentLabelReconcileScheduler.noLabelCapability",
                    "候選的 provider 沒有標籤能力，skip",
                    "shipmentNo", entity.getShipmentNo());
            return false;
        }

        LabelCapability.LabelIdentifierLookup lookup = label.get().fetchLabelIdentifiers(entity);

        if (!lookup.querySucceeded()) {
            log.warn("ShipmentLabelReconcileScheduler.queryFailed",
                    "provider 查詢失敗: " + lookup.providerMessage(),
                    "shipmentNo", entity.getShipmentNo());
            maybePublishCvsLabelUnprinted(entity);
            return false;
        }

        if (!lookup.identifiersReady()) {
            if (label.get().usesThreeSegmentCode()) {
                anomalyEventPublisher.publishAnomalyDetected(
                        ShipmentAnomalyType.PAYUNI_LGS_NO_CODE,
                        entity.getOrderNo(),
                        entity.getId() == null ? "" : String.valueOf(entity.getId()),
                        entity.getStoreId(),
                        entity.getBuyerId()
                );
            }
            maybePublishCvsLabelUnprinted(entity);
            return false;
        }

        shippingService.processWebhookPrint(entity.getShipTradeNo(), lookup.segmentCodes());

        log.info("ShipmentLabelReconcileScheduler.reconciled",
                "已補拉 label 識別並 transition SUCCEEDED",
                "shipmentNo", entity.getShipmentNo());
        return true;
    }

    /**
     * §5.21 CVS_LABEL_UNPRINTED：label_print_status='4102' 且超過設定門檻時發 event。
     */
    private void maybePublishCvsLabelUnprinted(ShipmentEntity entity) {
        if (!LabelPrintStatus.REQUESTED.value().equals(entity.getLabelPrintStatus())) {
            return;
        }
        if (entity.getLabelPrintRequestedAt() == null) {
            return;
        }
        LocalDateTime threshold = DateTimeUtils.nowUtc().minusHours(cvsLabelStuckHours);
        if (entity.getLabelPrintRequestedAt().isBefore(threshold)) {
            anomalyEventPublisher.publishAnomalyDetected(
                    ShipmentAnomalyType.CVS_LABEL_UNPRINTED,
                    entity.getOrderNo(),
                    entity.getId() == null ? "" : String.valueOf(entity.getId()),
                    entity.getStoreId(),
                    entity.getBuyerId()
            );
        }
    }

    private Optional<LabelCapability> labelCapabilityOf(ShipmentEntity entity) {
        return fulfillmentProviders.find(entity.getShippingMethod())
                .flatMap(adapter -> adapter.label(FulfillmentContext.forShipment(entity)));
    }
}
