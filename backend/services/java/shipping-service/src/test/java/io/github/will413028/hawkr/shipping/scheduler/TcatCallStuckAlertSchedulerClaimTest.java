package io.github.will413028.hawkr.shipping.scheduler;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import io.github.will413028.hawkr.shipping.mapper.ShipmentMapper;
import io.github.will413028.hawkr.shipping.mq.ShipmentAnomalyEventPublisher;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.springframework.test.util.ReflectionTestUtils;

import java.time.Duration;
import java.time.LocalDateTime;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * issue #2294 <b>L4</b>：{@link TcatCallStuckAlertScheduler} 的冷卻租約閘門。
 *
 * <p>原本<b>完全沒有搶佔</b>，而且單 pod 也每 10 分鐘把同一筆卡死重發一次告警。
 * 修法＝冷卻租約 on 新增的 {@code shipments.tcat_stuck_alerted_at}（V10）。
 *
 * <p>本檔最重要的一支是 {@link #cooldownIsBoundedByCandidateWindow()}：
 * 它釘住「租約 1 小時、不是 24 小時」——那正是 R4 抓到的假 lease。
 */
class TcatCallStuckAlertSchedulerClaimTest {

    /** {@code alert.tcat-stuck.max-age-hours} 預設值 ＝ 候選窗口寬度（小時）。 */
    private static final long CANDIDATE_WINDOW_HOURS = 24L;

    private ShipmentMapper shipmentMapper;
    private ShipmentAnomalyEventPublisher anomalyPublisher;
    private TcatCallStuckAlertScheduler scheduler;

    @BeforeEach
    void setUp() {
        shipmentMapper = mock(ShipmentMapper.class);
        anomalyPublisher = mock(ShipmentAnomalyEventPublisher.class);
        scheduler = new TcatCallStuckAlertScheduler(shipmentMapper, anomalyPublisher);
        ReflectionTestUtils.setField(scheduler, "batchSize", 30);
        ReflectionTestUtils.setField(scheduler, "requestedGraceMinutes", 3);
        ReflectionTestUtils.setField(scheduler, "maxAgeHours", (int) CANDIDATE_WINDOW_HOURS);
    }

    private static ShipmentEntity stuck(long id) {
        ShipmentEntity e = new ShipmentEntity();
        e.setId(id);
        e.setShipmentNo("SH" + id);
        e.setOrderNo("O" + id);
        e.setStoreId(100L);
        e.setBuyerId(555L);
        e.setShipTradeNo("ST" + id);
        e.setTcatCallRequestedAt(DateTimeUtils.nowUtc().minusHours(1));
        return e;
    }

    /** 反向斷言：Mockito 對 {@code int} 預設回 0，這一支確認 0 真的擋住了告警。 */
    @Test
    @DisplayName("claim 回 0（仍在冷卻中或他人持有）⇒ 不發告警")
    void claimLost_doesNotAlert() {
        when(shipmentMapper.findTcatStuckCandidates(any(), any(), any(), anyInt()))
                .thenReturn(List.of(stuck(1L)));
        when(shipmentMapper.claimTcatStuckAlert(anyLong(), anyString(), any(), any())).thenReturn(0);

        scheduler.run();

        verify(anomalyPublisher, never())
                .publishAnomalyDetected(any(), anyString(), anyString(), anyLong(), anyLong());
    }

    @Test
    @DisplayName("claim 回 1 ⇒ 照常發 TCAT_CALL_FAILED 告警")
    void claimWon_alerts() {
        when(shipmentMapper.findTcatStuckCandidates(any(), any(), any(), anyInt()))
                .thenReturn(List.of(stuck(1L)));
        when(shipmentMapper.claimTcatStuckAlert(anyLong(), anyString(), any(), any())).thenReturn(1);

        scheduler.run();

        verify(anomalyPublisher).publishAnomalyDetected(
                io.github.will413028.hawkr.common.utils.code.ShipmentAnomalyType.TCAT_CALL_FAILED,
                "O1", "1", 100L, 555L);
    }

    /**
     * 🚨 <b>本 issue 的 R4 P0：租約長度的上界</b>。
     *
     * <p>「租約是未來時間」對 24 小時<b>照樣成立</b>，所以只驗未來時間的斷言沒有鑑別力。
     * 真正被違反的是<b>候選窗口</b>：候選述詞含 {@code tcat_call_requested_at >= now − 24h}，
     * 所以每一筆的候選窗口寬度只有 24 小時。租約與窗口等寬 ⇒ 首次告警在
     * {@code requested_at + 3min} 蓋章、租約到期是 {@code +24h+3min}，而該列在 {@code +24h}
     * 就已掉出窗口 ⇒ <b>永遠只告警一次</b>，退化成 at-most-once；而
     * {@code publishAnomalyDetected} 在迴圈內無 try/catch，一拋例外就完全無聲。
     *
     * <p>本斷言要求租約 ≤ 窗口的 1/10（＝卡死期間至少能響 10 次），
     * 把 {@code ALERT_COOLDOWN_MINUTES} 改回 1440 會立刻轉紅。
     */
    @Test
    @DisplayName("R4：冷卻租約 ≤ 候選窗口的 1/10（改回 24 小時會轉紅）")
    void cooldownIsBoundedByCandidateWindow() {
        when(shipmentMapper.findTcatStuckCandidates(any(), any(), any(), anyInt()))
                .thenReturn(List.of(stuck(1L)));
        when(shipmentMapper.claimTcatStuckAlert(anyLong(), anyString(), any(), any())).thenReturn(1);

        LocalDateTime before = DateTimeUtils.nowUtc();
        scheduler.run();

        ArgumentCaptor<LocalDateTime> lease = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(shipmentMapper).claimTcatStuckAlert(anyLong(), anyString(), any(), lease.capture());
        long leaseMinutes = Duration.between(before, lease.getValue()).toMinutes();

        long windowMinutes = CANDIDATE_WINDOW_HOURS * 60;
        assertTrue(leaseMinutes > 0,
                "冷卻租約必須是未來時間（0 或負 ⇒ 每個 tick 都重發，等於沒有冷卻）");
        assertTrue(leaseMinutes * 10 <= windowMinutes,
                "冷卻租約實測 " + leaseMinutes + " 分，超過候選窗口(" + windowMinutes
                        + " 分)的 1/10。租約與窗口等寬 ⇒ 租約到期時該列早已掉出窗口 ⇒ "
                        + "永遠只告警一次 ⇒ at-most-once（這正是 24 小時那個值的錯誤）。");
        // 下界：必須明顯大於一個 tick（10 分）才有止噪效果
        assertTrue(leaseMinutes >= 10L * 3,
                "冷卻租約實測 " + leaseMinutes + " 分，不足一個 tick(10 分)的 3 倍，止噪效果不足");
        assertTrue(TcatCallStuckAlertScheduler.ALERT_COOLDOWN_MINUTES == 60L,
                "冷卻租約常數應為 60 分，實際 "
                        + TcatCallStuckAlertScheduler.ALERT_COOLDOWN_MINUTES);
    }

    /** 租約基準必須在迴圈內、claim 當下取（batch-size 預設 30）。 */
    @Test
    @DisplayName("租約基準取 claim 當下——第二筆嚴格晚於第一筆")
    void leaseBaselineIsPerItem() {
        when(shipmentMapper.findTcatStuckCandidates(any(), any(), any(), anyInt()))
                .thenReturn(List.of(stuck(1L), stuck(2L)));
        when(shipmentMapper.claimTcatStuckAlert(anyLong(), anyString(), any(), any())).thenReturn(1);
        org.mockito.Mockito.doAnswer(inv -> {
            Thread.sleep(40L);
            return null;
        }).when(anomalyPublisher)
                .publishAnomalyDetected(any(), anyString(), anyString(), anyLong(), anyLong());

        scheduler.run();

        ArgumentCaptor<LocalDateTime> leases = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(shipmentMapper, times(2))
                .claimTcatStuckAlert(anyLong(), anyString(), any(), leases.capture());
        long deltaMs = Duration
                .between(leases.getAllValues().get(0), leases.getAllValues().get(1)).toMillis();
        assertTrue(deltaMs >= 20L,
                "第二筆的租約必須明顯晚於第一筆（實測差 " + deltaMs + " ms）");
    }
}
