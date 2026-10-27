package io.github.will413028.hawkr.aftersales.scheduler;

import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.service.EventPrincipals;
import io.github.will413028.hawkr.aftersales.service.PrincipalResolutionFailedException;
import io.github.will413028.hawkr.aftersales.service.ReturnReceiptService;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InOrder;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.time.LocalDateTime;
import java.util.List;

import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

/**
 * 2026-05-22 §5.3.10.2 StorePickupTimeoutScheduler 測試。
 *
 * <p>覆蓋：
 * <ul>
 *   <li>mapper 返回 case list → 每筆呼 autoCompleteFromAtStore</li>
 *   <li>單筆 throw exception 不中斷後續批次</li>
 *   <li>空 list 不呼叫 service</li>
 * </ul>
 */
@ExtendWith(MockitoExtension.class)
class StorePickupTimeoutSchedulerTest {

    @Mock private AfterSaleCaseMapper caseMapper;
    @Mock private ReturnReceiptService returnReceiptService;

    /** 排程在 tx 外預解好的身分欄位（issue #2294 B5/B6）。 */
    private static final EventPrincipals OK = new EventPrincipals(42L, 55L);

    private AfterSalesProperties properties;
    private StorePickupTimeoutScheduler scheduler;

    @BeforeEach
    void setUp() {
        properties = new AfterSalesProperties();
        properties.getReturnConfig().setStorePickupTimeoutHours(72);
        scheduler = new StorePickupTimeoutScheduler(caseMapper, returnReceiptService, properties);
    }

    @Test
    void emptyResult_skipsServiceCall() {
        when(caseMapper.findStorePickupExpired(any(LocalDateTime.class), eq(200)))
                .thenReturn(List.of());

        scheduler.checkStorePickupTimeout();

        verify(returnReceiptService, never()).autoCompleteFromAtStore(anyString(), any());
    }

    @Test
    void multipleCandidates_callsAutoCompleteForEach() {
        when(caseMapper.findStorePickupExpired(any(LocalDateTime.class), eq(200)))
                .thenReturn(List.of("A001", "A002", "A003"));
        when(returnReceiptService.resolvePrincipalsForCaseStrict(anyString())).thenReturn(OK);
        when(returnReceiptService.autoCompleteFromAtStore(anyString(), any(EventPrincipals.class))).thenReturn(true);

        scheduler.checkStorePickupTimeout();

        verify(returnReceiptService).autoCompleteFromAtStore("A001", OK);
        verify(returnReceiptService).autoCompleteFromAtStore("A002", OK);
        verify(returnReceiptService).autoCompleteFromAtStore("A003", OK);
    }

    @Test
    void singleFailureDoesNotInterruptBatch() {
        when(caseMapper.findStorePickupExpired(any(LocalDateTime.class), eq(200)))
                .thenReturn(List.of("A001", "A002", "A003"));
        when(returnReceiptService.resolvePrincipalsForCaseStrict(anyString())).thenReturn(OK);
        when(returnReceiptService.autoCompleteFromAtStore("A001", OK))
                .thenThrow(new RuntimeException("DB connection lost"));
        when(returnReceiptService.autoCompleteFromAtStore("A002", OK)).thenReturn(true);
        when(returnReceiptService.autoCompleteFromAtStore("A003", OK)).thenReturn(false);

        // 第一筆 throw 不應中斷 — A002 / A003 仍要被處理
        scheduler.checkStorePickupTimeout();

        verify(returnReceiptService).autoCompleteFromAtStore("A001", OK);
        verify(returnReceiptService).autoCompleteFromAtStore("A002", OK);
        verify(returnReceiptService).autoCompleteFromAtStore("A003", OK);
    }

    // ==================== issue #2294 B6：反解必須在 tx 之外、且走 strict ====================

    @Test
    void resolvesPrincipalsBeforeEnteringTx() {
        when(caseMapper.findStorePickupExpired(any(LocalDateTime.class), eq(200)))
                .thenReturn(List.of("A001"));
        when(returnReceiptService.resolvePrincipalsForCaseStrict("A001")).thenReturn(OK);
        when(returnReceiptService.autoCompleteFromAtStore("A001", OK)).thenReturn(true);

        scheduler.checkStorePickupTimeout();

        InOrder order = inOrder(returnReceiptService);
        order.verify(returnReceiptService).resolvePrincipalsForCaseStrict("A001");
        order.verify(returnReceiptService).autoCompleteFromAtStore("A001", OK);
    }

    @Test
    void principalResolutionFails_skipsCaseWithoutEnteringTx() {
        // strict：不進 tx ⇒ status='26' 與 store_pickup_deadline_at 未變 ⇒ 下一 tick 重撈
        when(caseMapper.findStorePickupExpired(any(LocalDateTime.class), eq(200)))
                .thenReturn(List.of("BAD", "A002"));
        when(returnReceiptService.resolvePrincipalsForCaseStrict("BAD"))
                .thenThrow(new PrincipalResolutionFailedException("buyer_id -> notify account_id", 2001L));
        when(returnReceiptService.resolvePrincipalsForCaseStrict("A002")).thenReturn(OK);
        when(returnReceiptService.autoCompleteFromAtStore("A002", OK)).thenReturn(true);

        scheduler.checkStorePickupTimeout();

        verify(returnReceiptService, never()).autoCompleteFromAtStore(eq("BAD"), any());
        verify(returnReceiptService).autoCompleteFromAtStore("A002", OK);
    }
}
