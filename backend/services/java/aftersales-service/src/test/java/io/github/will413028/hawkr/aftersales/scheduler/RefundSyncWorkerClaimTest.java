package io.github.will413028.hawkr.aftersales.scheduler;

import io.github.will413028.hawkr.aftersales.entity.RefundTaskEntity;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.mapper.RefundTaskMapper;
import io.github.will413028.hawkr.aftersales.mq.AfterSalesEventPublisherImpl;
import io.github.will413028.hawkr.common.utils.code.AfterSalePlatformDecisionOutcome;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.common.utils.code.WorkerTaskStatus;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import java.time.Duration;
import java.time.LocalDateTime;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

/**
 * issue #2294 L5：{@code RefundSyncWorker}（補發遺漏 MQ 通知）加上
 * <b>lease on {@code refund_tasks.resync_at}</b>——本 worker 原本<b>完全沒有搶佔</b>，
 * prod {@code replicas=2} 下同一則補發事件會被兩個 pod 各發一次。
 *
 * <p>本 worker 在本 issue 之前零測試，claim 閘門兩條分支都是新覆蓋。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class RefundSyncWorkerClaimTest {

    @Mock private RefundTaskMapper refundTaskMapper;
    @Mock private AfterSaleCaseMapper caseMapper;
    @Mock private AfterSalesEventPublisherImpl eventPublisher;

    private RefundSyncWorker worker;

    @BeforeEach
    void setUp() {
        worker = new RefundSyncWorker(refundTaskMapper, caseMapper, eventPublisher);
        when(refundTaskMapper.selectList(any())).thenReturn(List.of(doneTask()));
        when(caseMapper.findStoreIdByCaseNo("A001")).thenReturn(1001L);
        when(caseMapper.findBuyerIdByCaseNo("A001")).thenReturn(2001L);
    }

    @Test
    void claimSucceeds_republishesRefundCompleted() {
        when(refundTaskMapper.claimForResync(eq(1001L), eq(WorkerTaskStatus.DONE.value()), any(), any()))
                .thenReturn(1);

        worker.syncRefundEvents();

        verify(eventPublisher).publishRefundCompleted(
                eq("A001"), eq("O100"), eq(1001L), eq(2001L),
                eq(AfterSalePlatformDecisionOutcome.FULL_REFUND), eq(500L), anyString(), any());
    }

    @Test
    void claimLost_doesNotRepublish() {
        when(refundTaskMapper.claimForResync(anyLong(), anyString(), any(), any())).thenReturn(0);

        worker.syncRefundEvents();

        verify(eventPublisher, never()).publishRefundCompleted(
                anyString(), anyString(), any(), any(), any(), any(), anyString(), any());
        verify(eventPublisher, never()).publishCancelCompleted(anyString(), any());
    }

    /**
     * issue #2294 <b>R4-P1-2</b>：租約基準必須在<b>迴圈內、claim 當下</b>取，
     * 不能像原實作那樣在 {@code syncRefundEvents()} 開頭（tick 起點）只算一次。
     *
     * <p>只算一次 ⇒ 全批共用同一個到期時間 ⇒ 第 k 筆的<b>有效</b>租約
     * = {@code RESYNC_LEASE_HOURS} − (前 k−1 筆的耗時)，不變式退化成
     * 「批次總耗時 &lt; 租約」——那條會隨 {@code LIMIT 50} 這個批量一起破，
     * 是參數巧合不是結構保證。基準改在 claim 當下取之後，不變式降為
     * 「<b>單筆</b>最壞耗時 &lt; 租約」，與批量無關。
     *
     * <p><b>鑑別方式</b>：讓第一筆 claim 之後的工作（{@code findStoreIdByCaseNo}）
     * 耗掉一段可觀測的時間，再比較兩筆送進 claim 的租約。
     * 基準在迴圈內 ⇒ 第二筆嚴格晚於第一筆；基準在迴圈外 ⇒ 兩者<b>完全相等</b>。
     */
    @Test
    @DisplayName("R4-P1-2：租約基準取 claim 當下——後一筆的租約嚴格晚於前一筆（迴圈外只算一次會轉紅）")
    void leaseBaselineIsTakenPerItemNotPerTick() {
        when(refundTaskMapper.selectList(any()))
                .thenReturn(List.of(doneTask(1001L, "A_SLOW"), doneTask(1002L, "A_NEXT")));
        when(refundTaskMapper.claimForResync(anyLong(), anyString(), any(), any())).thenReturn(1);
        when(caseMapper.findStoreIdByCaseNo(anyString())).thenAnswer(inv -> {
            Thread.sleep(40L);
            return 1001L;
        });
        when(caseMapper.findBuyerIdByCaseNo(anyString())).thenReturn(2001L);

        worker.syncRefundEvents();

        ArgumentCaptor<LocalDateTime> leases = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(refundTaskMapper, times(2))
                .claimForResync(anyLong(), anyString(), any(), leases.capture());
        long deltaMs = Duration
                .between(leases.getAllValues().get(0), leases.getAllValues().get(1)).toMillis();

        assertTrue(deltaMs >= 20L,
                "第二筆的租約必須明顯晚於第一筆（實測差 " + deltaMs + " ms）。"
                        + "兩者相等 ⇒ 租約基準在迴圈外只算一次 ⇒ 不變式退化成「批次總耗時 < 租約」，"
                        + "會隨 LIMIT 50 這個批量一起破。");
    }

    private RefundTaskEntity doneTask() {
        return doneTask(1001L, "A001");
    }

    private RefundTaskEntity doneTask(long id, String caseNo) {
        RefundTaskEntity t = new RefundTaskEntity();
        t.setId(id);
        t.setCaseNo(caseNo);
        t.setOrderNo("O100");
        t.setRefundAmount(500L);
        t.setRefundOutcome(AfterSalePlatformDecisionOutcome.FULL_REFUND.value());
        t.setPaymentMethod(PaymentMethod.CREDIT_CARD.value());
        t.setStatus(WorkerTaskStatus.DONE.value());
        return t;
    }
}
