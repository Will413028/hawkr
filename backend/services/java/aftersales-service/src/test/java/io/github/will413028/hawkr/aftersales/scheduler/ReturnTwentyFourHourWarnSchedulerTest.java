package io.github.will413028.hawkr.aftersales.scheduler;

import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.service.AfterSalesEventPublisher;
import io.github.will413028.hawkr.common.mq.MqConstants;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InOrder;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import java.time.LocalDateTime;
import java.util.List;

import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

/**
 * 2026-05-22 §5.3.10.3 ReturnTwentyFourHourWarnScheduler 測試。
 *
 * <p>2026-08-08 issue #2294 A1–A3 改寫：本排程改走《多副本排程搶佔規範》<b>例外 B</b>，
 * 順序由「stamp → publish」翻轉為 <b>publish → 成功後才 stamp</b>，
 * {@code warned_*_24h_at} 降為事後記錄、不再是冪等閘門。
 *
 * <p>覆蓋：
 * <ul>
 *   <li>store_pickup 候選命中 → publish RETURN_AT_STORE_24H → 成功後才 stamp（{@code InOrder} 驗序）</li>
 *   <li>confirm_receipt 候選命中 → publish RETURN_DELIVERED_24H → 成功後才 stamp</li>
 *   <li><b>publish 失敗（回 false）→ 不 stamp</b>（下一輪重發；這是 A1–A3 的整個修復目的）</li>
 *   <li>空候選 → 不 publish、不 stamp</li>
 * </ul>
 * <p>⚠️ 順序斷言（{@code InOrder}）是換順序後唯一還擋得住回歸的東西：沒有它，
 * 有人把順序改回 stamp→publish 時測試照樣綠。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class ReturnTwentyFourHourWarnSchedulerTest {

    @Mock private AfterSaleCaseMapper caseMapper;
    @Mock private AfterSalesEventPublisher eventPublisher;

    private ReturnTwentyFourHourWarnScheduler scheduler;

    @BeforeEach
    void setUp() {
        scheduler = new ReturnTwentyFourHourWarnScheduler(caseMapper, eventPublisher);
        // 三個分支預設空候選，個別測試再覆寫自己那一支
        when(caseMapper.findStorePickup24hWarnCandidates(any(), any(), eq(200))).thenReturn(List.of());
        when(caseMapper.findConfirmReceipt24hWarnCandidates(any(), any(), eq(200))).thenReturn(List.of());
        when(caseMapper.findShipBack24hWarnCandidates(any(), any(), eq(200))).thenReturn(List.of());
    }

    @Test
    void storePickup_candidateMatches_publishesThenStamps() {
        AfterSaleCaseEntity entity = caseWith("A001", "O100", "26", "3002");
        entity.setStorePickupDeadlineAt(LocalDateTime.of(2026, 5, 23, 1, 0));

        when(caseMapper.findStorePickup24hWarnCandidates(any(), any(), eq(200)))
                .thenReturn(List.of("A001"));
        when(caseMapper.selectOne(any())).thenReturn(entity);
        when(eventPublisher.tryPublishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(), anyMap()))
                .thenReturn(true);

        scheduler.checkWarnings();

        verify(eventPublisher).tryPublishCaseTransitionWithContext(
                eq("aftersales.case.return_at_store_24h"),
                eq(MqConstants.EVENT_TYPE_AFTERSALES_CASE_RETURN_AT_STORE_24H),
                eq("A001"), eq("O100"), eq(2001L), eq(1001L), eq("system"), anyMap());

        // 例外 B 的核心不變式：stamp 必須發生在 publish **之後**
        InOrder inOrder = inOrder(eventPublisher, caseMapper);
        inOrder.verify(eventPublisher).tryPublishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(), anyMap());
        inOrder.verify(caseMapper).stampStorePickup24hWarned(eq("A001"), any(LocalDateTime.class));
    }

    @Test
    void storePickup_publishFails_doesNotStamp() {
        // A1 的整個修復目的：publish 失敗 → 不 stamp → 下一輪重發
        // （舊順序下 publish 失敗＝該筆預警永久遺失，因為 stamp 已經寫進去了）
        AfterSaleCaseEntity entity = caseWith("A002", "O200", "26", "3002");
        entity.setStorePickupDeadlineAt(LocalDateTime.of(2026, 5, 23, 1, 0));

        when(caseMapper.findStorePickup24hWarnCandidates(any(), any(), eq(200)))
                .thenReturn(List.of("A002"));
        when(caseMapper.selectOne(any())).thenReturn(entity);
        when(eventPublisher.tryPublishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(), anyMap()))
                .thenReturn(false);

        scheduler.checkWarnings();

        verify(eventPublisher).tryPublishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(), anyMap());
        verify(caseMapper, never()).stampStorePickup24hWarned(anyString(), any());
    }

    @Test
    void confirmReceipt_candidateMatches_publishesThenStamps() {
        AfterSaleCaseEntity entity = caseWith("B001", "O200", "27", "3001");
        entity.setAutoConfirmAt(LocalDateTime.of(2026, 5, 23, 1, 0));

        when(caseMapper.findConfirmReceipt24hWarnCandidates(any(), any(), eq(200)))
                .thenReturn(List.of("B001"));
        when(caseMapper.selectOne(any())).thenReturn(entity);
        when(eventPublisher.tryPublishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(), anyMap()))
                .thenReturn(true);

        scheduler.checkWarnings();

        verify(eventPublisher).tryPublishCaseTransitionWithContext(
                eq("aftersales.case.return_delivered_24h"),
                eq(MqConstants.EVENT_TYPE_AFTERSALES_CASE_RETURN_DELIVERED_24H),
                eq("B001"), eq("O200"), eq(2001L), eq(1001L), eq("system"), anyMap());

        InOrder inOrder = inOrder(eventPublisher, caseMapper);
        inOrder.verify(eventPublisher).tryPublishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(), anyMap());
        inOrder.verify(caseMapper).stampConfirmReceipt24hWarned(eq("B001"), any(LocalDateTime.class));
    }

    @Test
    void confirmReceipt_publishFails_doesNotStamp() {
        AfterSaleCaseEntity entity = caseWith("B002", "O220", "27", "3001");
        entity.setAutoConfirmAt(LocalDateTime.of(2026, 5, 23, 1, 0));

        when(caseMapper.findConfirmReceipt24hWarnCandidates(any(), any(), eq(200)))
                .thenReturn(List.of("B002"));
        when(caseMapper.selectOne(any())).thenReturn(entity);
        when(eventPublisher.tryPublishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(), anyMap()))
                .thenReturn(false);

        scheduler.checkWarnings();

        verify(caseMapper, never()).stampConfirmReceipt24hWarned(anyString(), any());
    }

    @Test
    void shipBack_candidateMatches_publishesThenStamps() {
        // A3（2026-06-15 新增的第三個分支）先前完全沒有測試覆蓋
        AfterSaleCaseEntity entity = caseWith("C001", "O300", "20", "3001");
        entity.setShipBackDeadlineAt(LocalDateTime.of(2026, 6, 20, 1, 0));

        when(caseMapper.findShipBack24hWarnCandidates(any(), any(), eq(200)))
                .thenReturn(List.of("C001"));
        when(caseMapper.selectOne(any())).thenReturn(entity);
        when(eventPublisher.tryPublishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(), anyMap()))
                .thenReturn(true);

        scheduler.checkWarnings();

        verify(eventPublisher).tryPublishCaseTransitionWithContext(
                eq("aftersales.case.return_pending_24h"),
                eq(MqConstants.EVENT_TYPE_AFTERSALES_CASE_RETURN_PENDING_24H),
                eq("C001"), eq("O300"), eq(2001L), eq(1001L), eq("system"), anyMap());

        InOrder inOrder = inOrder(eventPublisher, caseMapper);
        inOrder.verify(eventPublisher).tryPublishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(), anyMap());
        inOrder.verify(caseMapper).stampShipBack24hWarned(eq("C001"), any(LocalDateTime.class));
    }

    @Test
    void shipBack_publishFails_doesNotStamp() {
        AfterSaleCaseEntity entity = caseWith("C002", "O320", "20", "3001");
        entity.setShipBackDeadlineAt(LocalDateTime.of(2026, 6, 20, 1, 0));

        when(caseMapper.findShipBack24hWarnCandidates(any(), any(), eq(200)))
                .thenReturn(List.of("C002"));
        when(caseMapper.selectOne(any())).thenReturn(entity);
        when(eventPublisher.tryPublishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(), anyMap()))
                .thenReturn(false);

        scheduler.checkWarnings();

        verify(caseMapper, never()).stampShipBack24hWarned(anyString(), any());
    }

    @Test
    void noCandidates_noPublishCalls() {
        scheduler.checkWarnings();

        verify(caseMapper, never()).stampStorePickup24hWarned(anyString(), any());
        verify(caseMapper, never()).stampConfirmReceipt24hWarned(anyString(), any());
        verify(caseMapper, never()).stampShipBack24hWarned(anyString(), any());
        verify(eventPublisher, never()).tryPublishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(), anyMap());
    }

    private AfterSaleCaseEntity caseWith(String caseNo, String orderNo, String status, String returnShippingMethod) {
        AfterSaleCaseEntity e = new AfterSaleCaseEntity();
        e.setCaseNo(caseNo);
        e.setOrderNo(orderNo);
        e.setStatus(status);
        e.setReturnShippingMethod(returnShippingMethod);
        e.setBuyerId(2001L);
        e.setStoreId(1001L);
        return e;
    }
}
