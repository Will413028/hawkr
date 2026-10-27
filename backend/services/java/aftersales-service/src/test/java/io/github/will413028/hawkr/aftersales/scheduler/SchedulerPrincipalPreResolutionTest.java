package io.github.will413028.hawkr.aftersales.scheduler;

import com.baomidou.mybatisplus.core.MybatisConfiguration;
import com.baomidou.mybatisplus.core.metadata.TableInfoHelper;
import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.service.AfterSalesEventPublisher;
import io.github.will413028.hawkr.aftersales.service.CaseAutoRejectService;
import io.github.will413028.hawkr.aftersales.service.CaseEscalationService;
import io.github.will413028.hawkr.aftersales.service.EventPrincipals;
import io.github.will413028.hawkr.aftersales.service.PrincipalResolutionFailedException;
import io.github.will413028.hawkr.aftersales.service.ReturnPendingTimeoutCloseService;
import org.apache.ibatis.builder.MapperBuilderAssistant;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InOrder;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import java.util.List;

import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.inOrder;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * issue #2294 P1-2：身分反解（兩次<b>無 deadline</b> 的 gRPC）必須發生在<b>進 tx 之前</b>，
 * 且失敗時「該筆本輪不做、下輪重試」。
 *
 * <p>要擋的退化有兩種，兩種都在這裡被機械化：
 * <ol>
 *   <li><b>反解被搬回 tx 內</b> —— 由「worker 先呼叫 {@code resolveEventPrincipals}、
 *       再呼叫 tx service」的 {@link InOrder} 斷言擋住。</li>
 *   <li><b>反解失敗被當成可降級</b> —— 由「解析拋例外時 tx service 一次都沒被呼叫、
 *       且同批的下一筆照常處理」的斷言擋住。tx 沒開始 ⇒ 沒有 CAS ⇒ 候選述詞未變 ⇒
 *       下一 tick 必然重撈同一筆（這就是「重試」的全部機制，不需要額外設施）。</li>
 * </ol>
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class SchedulerPrincipalPreResolutionTest {

    @Mock private AfterSaleCaseMapper caseMapper;
    @Mock private CaseAutoRejectService autoRejectService;
    @Mock private CaseEscalationService escalationService;
    @Mock private ReturnPendingTimeoutCloseService closeService;
    @Mock private AfterSalesEventPublisher eventPublisher;

    private static final EventPrincipals OK = new EventPrincipals(42L, 55L);

    @BeforeAll
    static void initLambdaCache() {
        // ReturnTimeoutWorker 用 LambdaQueryWrapper，需要 MyBatis-Plus 的 lambda column cache
        MybatisConfiguration config = new MybatisConfiguration();
        MapperBuilderAssistant assistant = new MapperBuilderAssistant(config, "");
        TableInfoHelper.initTableInfo(assistant, AfterSaleCaseEntity.class);
    }

    // ==================== B2 CaseProcessTimeoutWorker（AUTO_REJECT） ====================

    @Test
    @DisplayName("B2：反解失敗 → 不進 tx（autoReject 一次都沒被呼叫），同批下一筆照常處理")
    void autoReject_principalResolutionFails_skipsCaseWithoutEnteringTx() {
        CaseProcessTimeoutWorker worker = processTimeoutWorker(AfterSalesProperties.TimeoutPolicy.AUTO_REJECT);
        when(caseMapper.selectProcessTimeoutCandidates(anyInt()))
                .thenReturn(List.of(candidate("BAD"), candidate("GOOD")));
        when(eventPublisher.resolveEventPrincipals(eq(1001L), eq(2001L)))
                .thenThrow(new PrincipalResolutionFailedException("store_id -> seller account_id", 1001L));
        when(eventPublisher.resolveEventPrincipals(eq(1002L), eq(2002L))).thenReturn(OK);

        worker.scanProcessTimeouts();

        // 沒進 tx ⇒ 沒有 CAS、沒有 audit、沒有事件；候選述詞未變 ⇒ 下一 tick 重撈
        verify(autoRejectService, never())
                .autoReject(eq("BAD"), any(), anyInt(), any(), any(), any(), any());
        // 單筆失敗不得中斷整批
        verify(autoRejectService)
                .autoReject(eq("GOOD"), any(), anyInt(), any(), any(), any(), eq(OK));
    }

    @Test
    @DisplayName("B2：反解必須先於 tx service 發生（防止被搬回 tx 內）")
    void autoReject_resolvesPrincipalsBeforeEnteringTx() {
        CaseProcessTimeoutWorker worker = processTimeoutWorker(AfterSalesProperties.TimeoutPolicy.AUTO_REJECT);
        when(caseMapper.selectProcessTimeoutCandidates(anyInt())).thenReturn(List.of(candidate("GOOD")));
        when(eventPublisher.resolveEventPrincipals(any(), any())).thenReturn(OK);

        worker.scanProcessTimeouts();

        InOrder order = inOrder(eventPublisher, autoRejectService);
        order.verify(eventPublisher).resolveEventPrincipals(1002L, 2002L);
        order.verify(autoRejectService)
                .autoReject(eq("GOOD"), any(), anyInt(), any(), any(), any(), eq(OK));
    }

    // ==================== B3 CaseProcessTimeoutWorker（AUTO_ESCALATE） ====================

    @Test
    @DisplayName("B3：反解失敗 → 不進 tx（escalate 一次都沒被呼叫），同批下一筆照常處理")
    void escalate_principalResolutionFails_skipsCaseWithoutEnteringTx() {
        CaseProcessTimeoutWorker worker = processTimeoutWorker(AfterSalesProperties.TimeoutPolicy.AUTO_ESCALATE);
        when(caseMapper.selectProcessTimeoutCandidates(anyInt()))
                .thenReturn(List.of(candidate("BAD"), candidate("GOOD")));
        when(eventPublisher.resolveEventPrincipals(eq(1001L), eq(2001L)))
                .thenThrow(new PrincipalResolutionFailedException("buyer_id -> notify account_id", 2001L));
        when(eventPublisher.resolveEventPrincipals(eq(1002L), eq(2002L))).thenReturn(OK);

        worker.scanProcessTimeouts();

        verify(escalationService, never()).escalate(eq("BAD"), any(), any(), any(), any(), any());
        verify(escalationService).escalate(eq("GOOD"), any(), any(), any(), any(), eq(OK));
    }

    // ==================== B1 ReturnTimeoutWorker ====================

    @Test
    @DisplayName("B1：反解失敗 → 不進 tx（closeExpiredCase 一次都沒被呼叫），同批下一筆照常處理")
    void returnTimeout_principalResolutionFails_skipsCaseWithoutEnteringTx() {
        ReturnTimeoutWorker worker = new ReturnTimeoutWorker(caseMapper, closeService, eventPublisher);
        AfterSaleCaseEntity bad = candidate("BAD");
        AfterSaleCaseEntity good = candidate("GOOD");
        when(caseMapper.selectList(any())).thenReturn(List.of(bad, good));
        when(eventPublisher.resolveEventPrincipals(eq(1001L), eq(2001L)))
                .thenThrow(new PrincipalResolutionFailedException("store_id -> seller account_id", 1001L));
        when(eventPublisher.resolveEventPrincipals(eq(1002L), eq(2002L))).thenReturn(OK);

        worker.checkReturnTimeout();

        // status 仍是 '20'、ship_back_deadline_at 仍到期 ⇒ 下一 tick（cron 10 分鐘）必然重撈
        verify(closeService, never()).closeExpiredCase(eq(bad), any(), any());
        verify(closeService).closeExpiredCase(eq(good), any(), eq(OK));
    }

    @Test
    @DisplayName("B1：反解必須先於 tx service 發生（防止被搬回 tx 內）")
    void returnTimeout_resolvesPrincipalsBeforeEnteringTx() {
        ReturnTimeoutWorker worker = new ReturnTimeoutWorker(caseMapper, closeService, eventPublisher);
        AfterSaleCaseEntity good = candidate("GOOD");
        when(caseMapper.selectList(any())).thenReturn(List.of(good));
        when(eventPublisher.resolveEventPrincipals(any(), any())).thenReturn(OK);

        worker.checkReturnTimeout();

        InOrder order = inOrder(eventPublisher, closeService);
        order.verify(eventPublisher).resolveEventPrincipals(1002L, 2002L);
        order.verify(closeService).closeExpiredCase(eq(good), any(), eq(OK));
    }

    // ==================== fixtures ====================

    private CaseProcessTimeoutWorker processTimeoutWorker(AfterSalesProperties.TimeoutPolicy policy) {
        AfterSalesProperties properties = new AfterSalesProperties();
        properties.getCaseConfig().setProcessDeadlineHours(48);
        properties.getCaseConfig().setTimeoutPolicy(policy);
        return new CaseProcessTimeoutWorker(
                caseMapper, autoRejectService, escalationService, properties, eventPublisher);
    }

    /** "BAD" 的 storeId/buyerId 綁反解失敗，"GOOD" 綁反解成功。 */
    private AfterSaleCaseEntity candidate(String caseNo) {
        AfterSaleCaseEntity e = new AfterSaleCaseEntity();
        e.setCaseNo(caseNo);
        e.setOrderNo("O-" + caseNo);
        e.setStatus("20");
        boolean bad = "BAD".equals(caseNo);
        e.setStoreId(bad ? 1001L : 1002L);
        e.setBuyerId(bad ? 2001L : 2002L);
        e.setReturnShippingMethod("3001");
        e.setShipBackDeadlineAt(java.time.LocalDateTime.of(2026, 8, 8, 2, 0));
        return e;
    }
}
