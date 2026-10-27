package io.github.will413028.hawkr.aftersales.service;

import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import java.time.LocalDateTime;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

/**
 * issue #2294 B1：待寄回逾時自動關案的短 tx（出路 4）。
 *
 * <p>本 class 在本 issue 之前不存在（邏輯散在 {@code ReturnTimeoutWorker} 的 private 方法內、
 * 完全沒有事務、也沒有任何測試），三條分支都是新覆蓋：
 * <ul>
 *   <li>CAS 成功 ＋ publish 成功 → 關案（audit ＋ event 都發生）</li>
 *   <li>CAS affected=0（買家壓哨寄出 20→25）→ 回 false，<b>不寫 audit、不發 event</b></li>
 *   <li>publish 失敗 → 拋 {@link EventPublishFailedException} 讓整個 tx rollback（下一 tick 重來）</li>
 * </ul>
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class ReturnPendingTimeoutCloseServiceTest {

    @Mock private AfterSaleCaseMapper caseMapper;
    @Mock private AfterSaleAuditService auditService;
    @Mock private AfterSalesEventPublisher eventPublisher;

    private ReturnPendingTimeoutCloseService service;
    private final LocalDateTime now = LocalDateTime.of(2026, 8, 8, 3, 0);

    /** issue #2294 P1-2：身分欄位由 worker 在 tx 外解析好後傳入，service 內不再有 gRPC。 */
    private final EventPrincipals principals = new EventPrincipals(4001L, 5001L);

    @BeforeEach
    void setUp() {
        AfterSalesProperties properties = new AfterSalesProperties();
        properties.getReturnConfig().setTimeoutDays(7);
        service = new ReturnPendingTimeoutCloseService(caseMapper, auditService, eventPublisher, properties);
    }

    @Test
    void casSucceedsAndPublishSucceeds_closesCase() {
        when(caseMapper.markReturnPendingTimeoutAutoClosed(eq("A001"), any())).thenReturn(1);
        when(eventPublisher.tryPublishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(), anyMap(), any()))
                .thenReturn(true);

        assertTrue(service.closeExpiredCase(expiredCase("A001"), now, principals));

        verify(auditService).log(eq("A001"), isNull(), isNull(), anyString(), anyString(), any());
        verify(eventPublisher).tryPublishCaseTransitionWithContext(
                eq("aftersales.case.auto_completed_return_pending_timeout"),
                anyString(), eq("A001"), eq("O100"), eq(2001L), eq(1001L), eq("system"), anyMap(),
                eq(principals));
    }

    @Test
    void casAffectedZero_returnsFalseWithoutSideEffects() {
        // 競態：買家壓哨寄出 20→25 搶先 ⇒ 不得寫 audit、不得發事件（否則買家會收到「逾時未寄回」）
        when(caseMapper.markReturnPendingTimeoutAutoClosed(eq("A002"), any())).thenReturn(0);

        assertFalse(service.closeExpiredCase(expiredCase("A002"), now, principals));

        verify(auditService, never()).log(anyString(), any(), any(), anyString(), anyString(), any());
        verify(eventPublisher, never()).tryPublishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(), anyMap(), any());
    }

    @Test
    void publishFails_throwsSoTxRollsBack() {
        // status='33' 是終態：不 rollback 的話該列永久離開候選集合、事件永久遺失
        when(caseMapper.markReturnPendingTimeoutAutoClosed(eq("A003"), any())).thenReturn(1);
        when(eventPublisher.tryPublishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(), anyMap(), any()))
                .thenReturn(false);

        assertThrows(EventPublishFailedException.class,
                () -> service.closeExpiredCase(expiredCase("A003"), now, principals));
    }

    private AfterSaleCaseEntity expiredCase(String caseNo) {
        AfterSaleCaseEntity e = new AfterSaleCaseEntity();
        e.setCaseNo(caseNo);
        e.setOrderNo("O100");
        e.setStatus("20");
        e.setBuyerId(2001L);
        e.setStoreId(1001L);
        e.setReturnShippingMethod("3001");
        e.setShipBackDeadlineAt(now.minusHours(1));
        return e;
    }
}
