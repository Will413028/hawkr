package io.github.will413028.hawkr.shipping.scheduler;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.ShipmentAnomalyType;
import io.github.will413028.hawkr.common.utils.code.TcatCallStatus;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import io.github.will413028.hawkr.shipping.mapper.ShipmentMapper;
import io.github.will413028.hawkr.shipping.mq.ShipmentAnomalyEventPublisher;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import java.time.LocalDateTime;
import java.util.List;

/**
 * §5.19.3.C 黑貓叫件 REQUESTED 卡死 ops 告警排程
 *
 * <p>掃描 {@code shipments.tcat_call_status='4202'} 且 {@code tcat_call_requested_at}
 * 過久（callTcat 為純同步呼叫，REQUESTED 過久代表 JVM crash 或 PayUni 例外被吞噬）。
 *
 * <p><b>不主動對帳</b>：PayUni 黑貓 callTcat 為內部記帳，沒有專屬可查詢的對外狀態
 * （詳見設計書 §5.19.3.C「設計決策：不從 PayUni 物流貨態反推」）。本通道僅累積筆數
 * 觸發告警，由人工 SSH 進 PayUni 後台確認黑貓系統是否實際收到此件，再手動
 * transition DB 到 SUCCEEDED 或 FAILED。常態下此通道應為 0。
 *
 * <p><b>搶佔語意（issue #2294 L4）</b>：原本<b>完全沒有搶佔</b>，而且單 pod 也每 10 分鐘就把
 * 同一筆重發一次告警。修法＝<b>冷卻租約 on 新增的 {@code shipments.tcat_stuck_alerted_at}</b>
 * （V10 migration），claim 成功才 {@code publishAnomalyDetected}。
 *
 * <p>⚠️ <b>不得走「只發一次」的例外 B</b>：卡死狀態沒解除時再發一次對收件人<b>仍有新資訊</b>
 * （「還卡著」），沒人處理就該持續有聲音。所以這是會過期的租約，不是 {@code IS NULL} 的永久旗標。
 *
 * <p>🚨 <b>租約 = 1 小時，不是 24 小時</b>：候選述詞的
 * {@code tcat_call_requested_at >= #{maxAgeCutoff}}（{@code max-age-hours} 預設 24）
 * 使<b>候選窗口寬度只有 24 小時</b>。租約與窗口等寬 ⇒ 首次告警在 {@code requested_at + 3min}
 * 蓋章、租約到期是 {@code +24h+3min}，但該列在 {@code +24h} 就已掉出窗口 ⇒ 永遠只告警一次
 * ⇒ 假 lease（行為上等同永久旗標）；而 {@code publishAnomalyDetected} 在迴圈內<b>無 try/catch</b>，
 * 它一拋例外就是零告警、完全無聲。1 小時 = tick（10 分）× 6 足以止噪，且 ≪ 24h ⇒ 卡死期間會響約 23 次。
 *
 * <p>設計書：06 §5.19.3.C / §8.5
 */
@Component
@ConditionalOnProperty(name = "hawkr.shipping.alert.tcat-stuck.enabled", havingValue = "true", matchIfMissing = true)
public class TcatCallStuckAlertScheduler {

    private static final ObsLogger log = ObsLogger.forClass(TcatCallStuckAlertScheduler.class);

    /**
     * issue #2294 L4：告警冷卻租約長度（分）。
     *
     * <p><b>1 小時，不是 24 小時。</b> 上界由候選窗口決定：候選述詞含
     * {@code tcat_call_requested_at >= now − maxAgeHours}（預設 24h），租約若與窗口等寬，
     * 到期時該列早已掉出窗口 ⇒ 退化成 at-most-once。取窗口的 1/24 讓卡死期間仍能響約 23 次。
     * <p>下界：必須明顯大於一個 tick（10 分）才有止噪效果，也必須大於單筆最壞耗時
     * （一次 MQ publish）。
     */
    static final long ALERT_COOLDOWN_MINUTES = 60L;

    private final ShipmentMapper shipmentMapper;
    private final ShipmentAnomalyEventPublisher anomalyEventPublisher;

    @Value("${hawkr.shipping.alert.tcat-stuck.batch-size:30}")
    private int batchSize;

    @Value("${hawkr.shipping.alert.tcat-stuck.requested-grace-minutes:3}")
    private int requestedGraceMinutes;

    @Value("${hawkr.shipping.alert.tcat-stuck.max-age-hours:24}")
    private int maxAgeHours;

    public TcatCallStuckAlertScheduler(ShipmentMapper shipmentMapper,
                                       ShipmentAnomalyEventPublisher anomalyEventPublisher) {
        this.shipmentMapper = shipmentMapper;
        this.anomalyEventPublisher = anomalyEventPublisher;
    }

    @Scheduled(fixedDelayString = "${hawkr.shipping.alert.tcat-stuck.interval-ms:600000}")
    public void run() {
        LocalDateTime now = DateTimeUtils.nowUtc();
        LocalDateTime requestedBefore = now.minusMinutes(requestedGraceMinutes);
        LocalDateTime maxAgeCutoff = now.minusHours(maxAgeHours);

        List<ShipmentEntity> candidates = shipmentMapper.findTcatStuckCandidates(
                requestedBefore, maxAgeCutoff, now, batchSize);

        if (candidates.isEmpty()) {
            log.info("TcatCallStuckAlertScheduler.empty",
                    "本輪無黑貓叫件 REQUESTED 卡死候選");
            return;
        }

        int alerted = 0;
        for (ShipmentEntity entity : candidates) {
            // issue #2294 L4：冷卻租約 claim —— 搶到才告警（0 = 另一個 pod 已搶走或仍在冷卻中）。
            // ⚠️ 租約基準取 **claim 當下** 而非 tick 起點的 now：取 tick 起點 ⇒ 全批共用同一個
            // 到期時間 ⇒ 不變式退化成「批次總耗時 < 租約」。改在這裡取之後，不變式降為
            // 「**單筆**最壞耗時 < 租約」，與 batch-size（預設 30）無關。
            LocalDateTime leaseUntil = DateTimeUtils.nowUtc().plusMinutes(ALERT_COOLDOWN_MINUTES);
            int claimed = shipmentMapper.claimTcatStuckAlert(
                    entity.getId(), TcatCallStatus.REQUESTED.value(), now, leaseUntil);
            if (claimed == 0) {
                log.info("TcatCallStuckAlertScheduler.claim.skip",
                        "租約由他人持有或仍在冷卻中，skip",
                        "shipmentNo", entity.getShipmentNo());
                continue;
            }
            alerted++;

            log.warn("TcatCallStuckAlertScheduler.itemAlert",
                    String.format("黑貓叫件 REQUESTED 卡死，需 ops 介入: shipTradeNo=%s, requestedAt=%s",
                            entity.getShipTradeNo(), entity.getTcatCallRequestedAt()),
                    "shipmentNo", entity.getShipmentNo());

            // §5.21 配送異常賣家通知：發 TCAT_CALL_FAILED event（fire-and-forget）
            anomalyEventPublisher.publishAnomalyDetected(
                    ShipmentAnomalyType.TCAT_CALL_FAILED,
                    entity.getOrderNo(),
                    entity.getId() == null ? "" : String.valueOf(entity.getId()),
                    entity.getStoreId(),
                    entity.getBuyerId()
            );
        }

        log.warn("TcatCallStuckAlertScheduler.summary",
                "本輪黑貓叫件 REQUESTED 卡死：本 pod 搶到並告警 " + alerted + " 筆",
                "count", String.valueOf(candidates.size()));
    }
}
