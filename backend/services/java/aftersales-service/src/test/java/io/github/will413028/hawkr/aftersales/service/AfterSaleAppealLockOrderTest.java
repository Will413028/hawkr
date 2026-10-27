package io.github.will413028.hawkr.aftersales.service;

import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.exception.AfterSalesBusinessException;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleAppealEventMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSalePlatformDecisionMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleResolutionMapper;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAppealState;
import io.github.will413028.hawkr.common.utils.code.AfterSaleCaseStatus;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InOrder;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import java.time.LocalDateTime;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.inOrder;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * backend#3262 P0 <b>生產端</b>的協定守衛：{@code submitAppeal} 必須
 * 「先持鎖、再判定資格、然後才做任何耗時的事」。
 *
 * <p>撥款閘門那一半（消費端）由 {@code AfterSalePayoutHoldServiceTest} /
 * {@code AfterSalePayoutHoldBoundaryTest} / {@code AfterSalePayoutHoldLockProtocolIT} 守著。
 * 兩半缺一，順序保證就不成立：
 * <ul>
 *   <li>鎖<b>晚於</b>資格判定 ⇒ 判定通過到上鎖之間仍有窗口，閘門探不到 ⇒ P0 復活</li>
 *   <li>資格判定用的是<b>非持鎖</b>那一次讀 ⇒ 判定基於可能已過期的值，且與鎖無因果關係</li>
 *   <li>媒體 RPC 排在鎖<b>之前</b>再重檢 ⇒ 交易自己跑得慢就把買家合法取得的申訴權利打掉（禁止）</li>
 * </ul>
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class AfterSaleAppealLockOrderTest {

    private static final Long BUYER_ID = 1001L;
    private static final String CASE_NO = "AS-ORDER";

    @Mock private AfterSaleCaseMapper caseMapper;
    @Mock private AfterSaleCaseService caseService;
    @Mock private AfterSaleAppealEventMapper appealEventMapper;
    @Mock private AfterSalePlatformDecisionMapper platformDecisionMapper;
    @Mock private AfterSaleResolutionMapper resolutionMapper;
    @Mock private EvidenceService evidenceService;
    @Mock private AfterSalesEventPublisher eventPublisher;

    private AfterSaleAppealService service;

    @BeforeEach
    void setUp() {
        service = new AfterSaleAppealService(caseMapper, caseService, appealEventMapper,
                platformDecisionMapper, resolutionMapper, evidenceService, eventPublisher,
                new AfterSalesProperties());
    }

    private AfterSaleCaseEntity aCase(String appealState, LocalDateTime lastRejectedAt) {
        AfterSaleCaseEntity c = new AfterSaleCaseEntity();
        c.setCaseNo(CASE_NO);
        c.setOrderNo("O-ORDER");
        c.setBuyerId(BUYER_ID);
        c.setStoreId(2001L);
        c.setStatus(AfterSaleCaseStatus.REJECTED.value());
        c.setAppealState(appealState);
        c.setLastRejectedAt(lastRejectedAt);
        return c;
    }

    /** 擁有權那一次讀（非持鎖）永遠回「看起來完全合格」，以突顯資格判定實際採信的是哪一份。 */
    private void ownershipReadLooksEligible() {
        when(caseService.getCaseForBuyer(eq(BUYER_ID), eq(CASE_NO)))
                .thenReturn(aCase(AfterSaleAppealState.NONE.value(),
                        DateTimeUtils.nowUtc().minusHours(1)));
    }

    // ==================== 資格判定必須採信「持鎖那一份」 ====================

    @Test
    @DisplayName("持鎖重讀顯示已申訴 ⇒ 拒絕（證明 A2 採信的是持鎖讀，不是擁有權那次讀）")
    void lockedReadIsAuthoritative_alreadyAppealed() {
        ownershipReadLooksEligible();
        when(caseMapper.selectByCaseNoForUpdate(CASE_NO))
                .thenReturn(aCase(AfterSaleAppealState.APPEALED.value(),
                        DateTimeUtils.nowUtc().minusHours(1)));

        AfterSalesBusinessException ex = assertThrows(AfterSalesBusinessException.class,
                () -> service.submitAppeal(BUYER_ID, CASE_NO, "R", "D", List.of()));
        assertEquals(GrpcResponseCode.AFTERSALE_APPEAL_ALREADY_SUBMITTED.code(), ex.code());
        verify(caseMapper, never()).markAppealed(anyString(), any(), anyString(), anyString());
    }

    @Test
    @DisplayName("持鎖重讀顯示已過期 ⇒ 拒絕（證明 A3 採信的是持鎖讀）")
    void lockedReadIsAuthoritative_deadlineExceeded() {
        ownershipReadLooksEligible();
        when(caseMapper.selectByCaseNoForUpdate(CASE_NO))
                .thenReturn(aCase(AfterSaleAppealState.NONE.value(),
                        DateTimeUtils.nowUtc().minusHours(100)));   // QA 重現用的時序

        AfterSalesBusinessException ex = assertThrows(AfterSalesBusinessException.class,
                () -> service.submitAppeal(BUYER_ID, CASE_NO, "R", "D", List.of()));
        assertEquals(GrpcResponseCode.AFTERSALE_APPEAL_DEADLINE_EXCEEDED.code(), ex.code());
        verify(caseMapper, never()).markAppealed(anyString(), any(), anyString(), anyString());
        verify(evidenceService, never()).finalizeAndInsert(anyString(), anyLong(), any(), anyString());
    }

    @Test
    @DisplayName("持鎖重讀取不到該列（極端 race）⇒ 拒絕，不得當作合格繼續")
    void lockedReadMissing_rejected() {
        ownershipReadLooksEligible();
        when(caseMapper.selectByCaseNoForUpdate(CASE_NO)).thenReturn(null);

        AfterSalesBusinessException ex = assertThrows(AfterSalesBusinessException.class,
                () -> service.submitAppeal(BUYER_ID, CASE_NO, "R", "D", List.of()));
        assertEquals(GrpcResponseCode.AFTERSALE_NOT_FOUND.code(), ex.code());
    }

    // ==================== 順序：鎖 → 判定 → 耗時工作 → CAS ====================

    @Test
    @DisplayName("順序不變式：持鎖 → （資格判定）→ media RPC → CAS，media 絕不得排在持鎖之前")
    void lockIsTakenBeforeAnyExpensiveWork() {
        ownershipReadLooksEligible();
        AfterSaleCaseEntity locked = aCase(AfterSaleAppealState.NONE.value(),
                DateTimeUtils.nowUtc().minusHours(1));
        when(caseMapper.selectByCaseNoForUpdate(CASE_NO)).thenReturn(locked);
        when(caseMapper.markAppealed(eq(CASE_NO), any(), anyString(), anyString())).thenReturn(1);

        service.submitAppeal(BUYER_ID, CASE_NO, "R", "D", List.of("tok1"));

        InOrder order = inOrder(caseMapper, evidenceService);
        // 鎖必須第一個發生：它之後的一切（含無界的 media RPC）都被這把鎖保護著，
        // 撥款閘門在這段期間探測會取不到鎖 ⇒ fail-closed。
        order.verify(caseMapper).selectByCaseNoForUpdate(CASE_NO);
        order.verify(evidenceService).finalizeAndInsert(eq(CASE_NO), eq(BUYER_ID), any(), anyString());
        order.verify(caseMapper).markAppealed(eq(CASE_NO), any(), anyString(), anyString());
    }

    @Test
    @DisplayName("買家權利不得縮減：窗內通過判定後，即使沒有證據照片也照樣成立（無時間性二次否決）")
    void withinWindow_appealSucceeds() {
        ownershipReadLooksEligible();
        when(caseMapper.selectByCaseNoForUpdate(CASE_NO))
                .thenReturn(aCase(AfterSaleAppealState.NONE.value(),
                        DateTimeUtils.nowUtc().minusHours(71).minusMinutes(59)));   // 剩約 1 分鐘
        when(caseMapper.markAppealed(eq(CASE_NO), any(), anyString(), anyString())).thenReturn(1);

        assertEquals(CASE_NO, service.submitAppeal(BUYER_ID, CASE_NO, "R", "D", List.of()));
        verify(caseMapper).markAppealed(eq(CASE_NO), any(), anyString(), anyString());
    }
}
