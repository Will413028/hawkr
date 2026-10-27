package io.github.will413028.hawkr.aftersales.service;

import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleAppealEventMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleResolutionMapper;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import org.junit.jupiter.api.BeforeAll;
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
 * issue #2294 B2／B3：賣家處理逾時代決的 publish 已從 worker 移進 service 既有 {@code @Transactional}
 * （出路 4 短 tx），且改用會傳遞失敗的 publish 變體。
 *
 * <p>兩支 service 在本 issue 之前皆<b>零測試</b>。四條分支都是新覆蓋：
 * <ul>
 *   <li>autoReject：CAS 成功 → publish 成功 → true</li>
 *   <li>autoReject：publish 失敗 → 拋 {@link EventPublishFailedException}（tx rollback，下一 tick 重來）</li>
 *   <li>escalate：CAS 成功 → publish 成功 → true</li>
 *   <li>escalate：publish 失敗 → 拋 {@link EventPublishFailedException}</li>
 * </ul>
 * 另補 CAS affected=0 時<b>不得 publish</b>（否則買家會收到「賣家逾時未回應、系統代拒」，
 * 而案件其實被賣家正常處理了——那正是不能走例外 B 的理由）。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class CaseTimeoutPublishInTxTest {

    @Mock private AfterSaleCaseMapper caseMapper;
    @Mock private AfterSaleResolutionMapper resolutionMapper;
    @Mock private AfterSaleAppealEventMapper appealEventMapper;
    @Mock private AfterSaleAuditService auditService;
    @Mock private AfterSalesEventPublisher eventPublisher;

    private CaseAutoRejectService autoRejectService;
    private CaseEscalationService escalationService;

    private final LocalDateTime now = LocalDateTime.of(2026, 8, 8, 3, 0);

    /** issue #2294 P1-2：身分欄位由 worker 在 tx 外解析好後傳入，service 內不再有 gRPC。 */
    private final EventPrincipals principals = new EventPrincipals(4001L, 5001L);

    @BeforeAll
    static void initTsid() {
        try { TsidUtil.init(700, 0); } catch (IllegalStateException ignored) { /* 已初始化 */ }
    }

    @BeforeEach
    void setUp() {
        AfterSalesProperties properties = new AfterSalesProperties();
        properties.getCaseConfig().setProcessDeadlineHours(48);
        autoRejectService = new CaseAutoRejectService(
                caseMapper, resolutionMapper, auditService, eventPublisher);
        escalationService = new CaseEscalationService(
                caseMapper, appealEventMapper, properties, eventPublisher);
    }

    // ==================== B2 autoReject ====================

    @Test
    void autoReject_casAndPublishSucceed_returnsTrue() {
        when(caseMapper.markAutoRejected(eq("A001"), any())).thenReturn(1);
        when(eventPublisher.tryPublishCaseAutoRejected(anyString(), anyString(), any(), any(), anyInt(), any()))
                .thenReturn(true);

        assertTrue(autoRejectService.autoReject("A001", now, 48, "O100", 2001L, 1001L, principals));

        verify(eventPublisher).tryPublishCaseAutoRejected("A001", "O100", 2001L, 1001L, 48, principals);
    }

    @Test
    void autoReject_publishFails_throwsSoTxRollsBack() {
        when(caseMapper.markAutoRejected(eq("A002"), any())).thenReturn(1);
        when(eventPublisher.tryPublishCaseAutoRejected(anyString(), anyString(), any(), any(), anyInt(), any()))
                .thenReturn(false);

        assertThrows(EventPublishFailedException.class,
                () -> autoRejectService.autoReject("A002", now, 48, "O200", 2001L, 1001L, principals));
    }

    @Test
    void autoReject_casAffectedZero_doesNotPublish() {
        when(caseMapper.markAutoRejected(eq("A003"), any())).thenReturn(0);

        assertThrows(CaseAutoRejectService.OptimisticLockLostException.class,
                () -> autoRejectService.autoReject("A003", now, 48, "O300", 2001L, 1001L, principals));

        verify(eventPublisher, never()).tryPublishCaseAutoRejected(
                anyString(), anyString(), any(), any(), anyInt(), any());
    }

    // ==================== B3 escalate ====================

    @Test
    void escalate_casAndPublishSucceed_returnsTrue() {
        when(caseMapper.markAutoEscalated(eq("B001"), any())).thenReturn(1);
        when(eventPublisher.tryPublishCaseAutoEscalated(anyString(), anyString(), any(), any(), anyInt(), any()))
                .thenReturn(true);

        assertTrue(escalationService.escalate("B001", now, "O100", 2001L, 1001L, principals));

        verify(eventPublisher).tryPublishCaseAutoEscalated("B001", "O100", 2001L, 1001L, 48, principals);
    }

    @Test
    void escalate_publishFails_throwsSoTxRollsBack() {
        when(caseMapper.markAutoEscalated(eq("B002"), any())).thenReturn(1);
        when(eventPublisher.tryPublishCaseAutoEscalated(anyString(), anyString(), any(), any(), anyInt(), any()))
                .thenReturn(false);

        assertThrows(EventPublishFailedException.class,
                () -> escalationService.escalate("B002", now, "O200", 2001L, 1001L, principals));
    }

    @Test
    void escalate_casAffectedZero_returnsFalseWithoutPublish() {
        when(caseMapper.markAutoEscalated(eq("B003"), any())).thenReturn(0);

        assertFalse(escalationService.escalate("B003", now, "O300", 2001L, 1001L, principals));

        verify(appealEventMapper, never()).insert(any());
        verify(eventPublisher, never()).tryPublishCaseAutoEscalated(
                anyString(), anyString(), any(), any(), anyInt(), any());
    }
}
