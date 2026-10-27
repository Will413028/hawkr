package io.github.will413028.hawkr.shipping.scheduler;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.shipping.entity.SellerShipmentSettingEntity;
import io.github.will413028.hawkr.shipping.mapper.SellerShipmentSettingMapper;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import java.time.LocalDateTime;
import java.util.List;

/**
 * §5.19.3.D 建店送審 REQUESTED 卡死 ops 告警排程
 *
 * <p>掃描 {@code seller_shipment_settings.payuni_submit_status='4302'} 且
 * {@code payuni_submit_requested_at} 過久。
 *
 * <p><b>不主動對帳</b>：PayUni 建店設定無查詢 API。本通道僅累積筆數觸發告警，
 * 由人工後台確認後手動 transition 到 SUCCEEDED 或 FAILED。常態下此通道應為 0。
 *
 * <p>設計書：06 §5.19.3.D
 */
@Component
@ConditionalOnProperty(name = "hawkr.shipping.alert.submit-stuck.enabled", havingValue = "true", matchIfMissing = true)
public class PayUniSubmitStuckAlertScheduler {

    private static final ObsLogger log = ObsLogger.forClass(PayUniSubmitStuckAlertScheduler.class);

    private final SellerShipmentSettingMapper sellerShipmentSettingMapper;

    @Value("${hawkr.shipping.alert.submit-stuck.batch-size:20}")
    private int batchSize;

    @Value("${hawkr.shipping.alert.submit-stuck.requested-grace-minutes:5}")
    private int requestedGraceMinutes;

    @Value("${hawkr.shipping.alert.submit-stuck.max-age-hours:24}")
    private int maxAgeHours;

    public PayUniSubmitStuckAlertScheduler(SellerShipmentSettingMapper sellerShipmentSettingMapper) {
        this.sellerShipmentSettingMapper = sellerShipmentSettingMapper;
    }

    @Scheduled(fixedDelayString = "${hawkr.shipping.alert.submit-stuck.interval-ms:600000}")
    public void run() {
        LocalDateTime now = DateTimeUtils.nowUtc();
        LocalDateTime requestedBefore = now.minusMinutes(requestedGraceMinutes);
        LocalDateTime maxAgeCutoff = now.minusHours(maxAgeHours);

        List<SellerShipmentSettingEntity> candidates = sellerShipmentSettingMapper.findSubmitStuckCandidates(
                requestedBefore, maxAgeCutoff, batchSize);

        if (candidates.isEmpty()) {
            log.info("PayUniSubmitStuckAlertScheduler.empty",
                    "本輪無建店送審 REQUESTED 卡死候選");
            return;
        }

        for (SellerShipmentSettingEntity entity : candidates) {
            log.warn("PayUniSubmitStuckAlertScheduler.itemAlert",
                    String.format("建店送審 REQUESTED 卡死，需 ops 介入: storeId=%s, shippingType=%s, requestedAt=%s",
                            entity.getStoreId(), entity.getShippingType(), entity.getPayuniSubmitRequestedAt()),
                    "settingId", String.valueOf(entity.getId()));
        }

        log.warn("PayUniSubmitStuckAlertScheduler.summary",
                "本輪累積建店送審 REQUESTED 卡死筆數",
                "count", String.valueOf(candidates.size()));
    }
}
