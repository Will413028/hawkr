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
 * 2026-05-22 §5.3.10.1 AutoConfirmReceiptScheduler 測試。
 *
 * <p>覆蓋：
 * <ul>
 *   <li>mapper 返回 case list → 每筆呼 autoConfirmReceipt</li>
 *   <li>單筆 throw exception 不中斷後續批次</li>
 *   <li>空 list 不呼叫 service</li>
 * </ul>
 */
@ExtendWith(MockitoExtension.class)
class AutoConfirmReceiptSchedulerTest {

    @Mock private AfterSaleCaseMapper caseMapper;
    @Mock private ReturnReceiptService returnReceiptService;

    /** 排程在 tx 外預解好的身分欄位（issue #2294 B5/B6）。 */
    private static final EventPrincipals OK = new EventPrincipals(42L, 55L);

    private AfterSalesProperties properties;
    private AutoConfirmReceiptScheduler scheduler;

    @BeforeEach
    void setUp() {
        properties = new AfterSalesProperties();
        properties.getReturnConfig().setAutoConfirmReceiptHours(48);
        scheduler = new AutoConfirmReceiptScheduler(caseMapper, returnReceiptService, properties);
    }

    @Test
    void emptyResult_skipsServiceCall() {
        when(caseMapper.findAutoConfirmExpired(any(LocalDateTime.class), eq(200)))
                .thenReturn(List.of());

        scheduler.checkAutoConfirm();

        verify(returnReceiptService, never()).autoConfirmReceipt(anyString(), any());
    }

    @Test
    void multipleCandidates_callsAutoConfirmForEach() {
        when(caseMapper.findAutoConfirmExpired(any(LocalDateTime.class), eq(200)))
                .thenReturn(List.of("B001", "B002", "B003"));
        when(returnReceiptService.resolvePrincipalsForCaseStrict(anyString())).thenReturn(OK);
        when(returnReceiptService.autoConfirmReceipt(anyString(), any(EventPrincipals.class))).thenReturn(true);

        scheduler.checkAutoConfirm();

        verify(returnReceiptService).autoConfirmReceipt("B001", OK);
        verify(returnReceiptService).autoConfirmReceipt("B002", OK);
        verify(returnReceiptService).autoConfirmReceipt("B003", OK);
    }

    @Test
    void singleFailureDoesNotInterruptBatch() {
        when(caseMapper.findAutoConfirmExpired(any(LocalDateTime.class), eq(200)))
                .thenReturn(List.of("B001", "B002", "B003"));
        when(returnReceiptService.resolvePrincipalsForCaseStrict(anyString())).thenReturn(OK);
        when(returnReceiptService.autoConfirmReceipt("B001", OK))
                .thenThrow(new RuntimeException("transient DB error"));
        when(returnReceiptService.autoConfirmReceipt("B002", OK)).thenReturn(true);
        when(returnReceiptService.autoConfirmReceipt("B003", OK)).thenReturn(false);

        scheduler.checkAutoConfirm();

        verify(returnReceiptService).autoConfirmReceipt("B001", OK);
        verify(returnReceiptService).autoConfirmReceipt("B002", OK);
        verify(returnReceiptService).autoConfirmReceipt("B003", OK);
    }

    // ==================== issue #2294 B5：反解必須在 tx 之外、且走 strict ====================

    @Test
    void resolvesPrincipalsBeforeEnteringTx() {
        // 防止反解被搬回 @Transactional 內（那會讓短 tx 在下游掛住時無限期持有 row lock）
        when(caseMapper.findAutoConfirmExpired(any(LocalDateTime.class), eq(200)))
                .thenReturn(List.of("B001"));
        when(returnReceiptService.resolvePrincipalsForCaseStrict("B001")).thenReturn(OK);
        when(returnReceiptService.autoConfirmReceipt("B001", OK)).thenReturn(true);

        scheduler.checkAutoConfirm();

        InOrder order = inOrder(returnReceiptService);
        order.verify(returnReceiptService).resolvePrincipalsForCaseStrict("B001");
        order.verify(returnReceiptService).autoConfirmReceipt("B001", OK);
    }

    @Test
    void principalResolutionFails_skipsCaseWithoutEnteringTx() {
        // strict 語意：反解失敗 ⇒ 不進 tx ⇒ 沒有 CAS ⇒ status='27' 與 auto_confirm_at 未變
        // ⇒ 下一 tick 必然重撈。降級成 best-effort（照發帶 0 的事件）會讓本測試轉綠 = 鑑別點。
        when(caseMapper.findAutoConfirmExpired(any(LocalDateTime.class), eq(200)))
                .thenReturn(List.of("BAD", "B002"));
        when(returnReceiptService.resolvePrincipalsForCaseStrict("BAD"))
                .thenThrow(new PrincipalResolutionFailedException("store_id -> seller account_id", 1001L));
        when(returnReceiptService.resolvePrincipalsForCaseStrict("B002")).thenReturn(OK);
        when(returnReceiptService.autoConfirmReceipt("B002", OK)).thenReturn(true);

        scheduler.checkAutoConfirm();

        verify(returnReceiptService, never()).autoConfirmReceipt(eq("BAD"), any());
        // 單筆失敗不得中斷整批
        verify(returnReceiptService).autoConfirmReceipt("B002", OK);
    }
}
