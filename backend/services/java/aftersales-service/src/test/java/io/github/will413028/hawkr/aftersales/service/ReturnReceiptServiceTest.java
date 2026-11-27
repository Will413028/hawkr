package io.github.will413028.hawkr.aftersales.service;

import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleResolutionEntity;
import io.github.will413028.hawkr.aftersales.entity.RefundTaskEntity;
import io.github.will413028.hawkr.aftersales.exception.AfterSalesBusinessException;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleResolutionMapper;
import io.github.will413028.hawkr.aftersales.mapper.RefundTaskMapper;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.AfterSaleCaseStatus;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.MockitoAnnotations;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import java.time.LocalDateTime;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

/**
 * 2026-05-22 逆物流退貨政策修訂 — ReturnReceiptService 三條路徑（賣家手動 / SYSTEM 雙逾時 / Admin force）測試。
 *
 * <p>覆蓋（對應 commit 46b2f068 / 設計書 §5.3.7 / §5.3.10 / §5.3.12）：
 * <ul>
 *   <li>autoConfirmReceipt(27→30) 走 markAutoCompletedFromDelivered + reason=CONFIRM_RECEIPT_TIMEOUT</li>
 *   <li>autoConfirmReceipt 前置 status≠27 skip return false</li>
 *   <li>autoCompleteFromAtStore(26→30) 走 markAutoCompletedFromAtStore + reason=STORE_PICKUP_TIMEOUT</li>
 *   <li>autoCompleteFromAtStore 前置 status≠26 skip return false</li>
 *   <li>confirmReceipt(賣家) 前置 status=26 拒絕（AFTERSALE_INVALID_STATUS）</li>
 *   <li>forceConfirmReceiptByAdmin status=26 允許（前次擴增 ADMIN_FORCE_ALLOWED_STATUSES）</li>
 * </ul>
 *
 * <p><b>issue #2294 B5/B6/B7 追加</b>：四條路徑的 tx 內<b>不得</b>出現會自己打 gRPC 的 publish 多載。
 * 每個 happy path 都附一組 {@code never()} 斷言鎖住這件事——把任何一處退回不帶
 * {@link EventPrincipals} 的多載，對應測試立刻轉紅。
 * 「那些多載真的零 gRPC」由 {@code EventPrincipalPreResolutionTest} 在 publisher 層面驗證，
 * 兩層合起來才是完整的機械保證。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class ReturnReceiptServiceTest {

    @Mock private AfterSaleCaseMapper caseMapper;
    @Mock private AfterSaleResolutionMapper resolutionMapper;
    @Mock private RefundTaskMapper refundTaskMapper;
    @Mock private AfterSaleAuditService auditService;
    @Mock private AfterSalesEventPublisher eventPublisher;
    @Mock private EvidenceService evidenceService;
    @Mock private AfterSaleCaseService caseService;

    private AfterSalesProperties properties;
    private ReturnReceiptService service;

    /**
     * 呼叫端在 tx 外解好的身分欄位。刻意與 {@code caseWith(...)} 的 storeId(1001)／buyerId(2001)
     * 都不同 —— 同值就分不出「有沒有真的用預解值」。
     */
    private static final EventPrincipals PRINCIPALS = new EventPrincipals(42L, 55L);

    @BeforeAll
    static void initTsid() {
        try { TsidUtil.init(700, 0); } catch (IllegalStateException ignored) {  }
    }

    @BeforeEach
    void setUp() {
        MockitoAnnotations.openMocks(this);
        properties = new AfterSalesProperties();
        properties.getReturnConfig().setAutoConfirmReceiptHours(48);
        properties.getReturnConfig().setStorePickupTimeoutHours(72);
        properties.getRefund().setMaxRetryCount(10);
        service = new ReturnReceiptService(
                caseMapper, resolutionMapper, refundTaskMapper, auditService,
                properties, eventPublisher, evidenceService, caseService);
        when(eventPublisher.tryPublishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(),
                any(), any(EventPrincipals.class)))
                .thenReturn(true);
    }

    /**
     * issue #2294 B5/B6/B7：斷言 tx 內<b>沒有</b>用到任何「自己會打 gRPC」的 publish 多載。
     * 這四個是全部的候選——publisher 介面上不帶 {@link EventPrincipals} 的 case 事件出口。
     */
    private void assertNoGrpcDoingPublishOverloadUsed() {
        verify(eventPublisher, never()).publishCaseTransition(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString());
        verify(eventPublisher, never()).publishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(),
                any());
        verify(eventPublisher, never()).tryPublishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(),
                any());
    }


    @Test
    void autoConfirmReceipt_status27_marksFromDeliveredAndPublishesTimeoutEvent() {
        AfterSaleCaseEntity entity = caseWith("A001", "O100", "27", "3001",
                PaymentMethod.CREDIT_CARD.value());
        entity.setAutoConfirmAt(LocalDateTime.of(2026, 5, 22, 1, 0));
        entity.setReturnDeliveredAt(LocalDateTime.of(2026, 5, 20, 1, 0));
        stubLoadCase(entity);
        stubResolutionAmount("A001", 1000L);
        when(caseMapper.markAutoCompletedFromDelivered(eq("A001"), any())).thenReturn(1);
        when(caseMapper.markRefundProcessingFromReceived("A001")).thenReturn(1);

        boolean ok = service.autoConfirmReceipt("A001", PRINCIPALS);

        assertTrue(ok);
        verify(caseMapper).markAutoCompletedFromDelivered(eq("A001"), any(LocalDateTime.class));
        verify(caseMapper, never()).markAutoCompletedFromAtStore(anyString(), any());

        @SuppressWarnings("unchecked")
        ArgumentCaptor<Map<String, Object>> extraCap = ArgumentCaptor.forClass(Map.class);
        verify(eventPublisher).tryPublishCaseTransitionWithContext(
                eq("aftersales.case.auto_completed_confirm_receipt_timeout"),
                eq(MqConstants.EVENT_TYPE_AFTERSALES_CASE_AUTO_COMPLETED_CONFIRM_RECEIPT_TIMEOUT),
                eq("A001"), eq("O100"), eq(2001L), eq(1001L), eq("system"),
                extraCap.capture(), eq(PRINCIPALS));
        Map<String, Object> extra = extraCap.getValue();
        assertEquals("3001", extra.get("shipping_method"));
        assertNotNull(extra.get("deadline_at"));
        assertNoGrpcDoingPublishOverloadUsed();
    }

    @Test
    void autoConfirmReceipt_statusNot27_returnsFalseWithoutSideEffects() {
        AfterSaleCaseEntity entity = caseWith("A002", "O200", "30", "3001",
                PaymentMethod.CREDIT_CARD.value());
        stubLoadCase(entity);

        boolean ok = service.autoConfirmReceipt("A002", PRINCIPALS);

        assertFalse(ok);
        verify(caseMapper, never()).markAutoCompletedFromDelivered(anyString(), any());
        verify(caseMapper, never()).markAutoCompletedFromAtStore(anyString(), any());
        verify(eventPublisher, never()).tryPublishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(), any(), any(EventPrincipals.class));
    }

    @Test
    void autoConfirmReceipt_raceLost_returnsFalse() {
        AfterSaleCaseEntity entity = caseWith("A003", "O300", "27", "3001",
                PaymentMethod.CREDIT_CARD.value());
        stubLoadCase(entity);
        stubResolutionAmount("A003", 500L);
        when(caseMapper.markAutoCompletedFromDelivered(eq("A003"), any())).thenReturn(0);

        boolean ok = service.autoConfirmReceipt("A003", PRINCIPALS);

        assertFalse(ok);
        verify(caseMapper, never()).markRefundProcessingFromReceived(anyString());
        verify(eventPublisher, never()).tryPublishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(), any(), any(EventPrincipals.class));
    }


    @Test
    void autoCompleteFromAtStore_status26_marksFromAtStoreAndPublishesTimeoutEvent() {
        AfterSaleCaseEntity entity = caseWith("A004", "O400", "26", "3002",
                PaymentMethod.CREDIT_CARD.value());
        entity.setStoreArrivedAt(LocalDateTime.of(2026, 5, 19, 1, 0));
        entity.setStorePickupDeadlineAt(LocalDateTime.of(2026, 5, 22, 1, 0));
        stubLoadCase(entity);
        stubResolutionAmount("A004", 800L);
        when(caseMapper.markAutoCompletedFromAtStore(eq("A004"), any())).thenReturn(1);
        when(caseMapper.markRefundProcessingFromReceived("A004")).thenReturn(1);

        boolean ok = service.autoCompleteFromAtStore("A004", PRINCIPALS);

        assertTrue(ok);
        verify(caseMapper).markAutoCompletedFromAtStore(eq("A004"), any(LocalDateTime.class));
        verify(caseMapper, never()).markAutoCompletedFromDelivered(anyString(), any());

        @SuppressWarnings("unchecked")
        ArgumentCaptor<Map<String, Object>> extraCap = ArgumentCaptor.forClass(Map.class);
        verify(eventPublisher).tryPublishCaseTransitionWithContext(
                eq("aftersales.case.auto_completed_store_pickup_timeout"),
                eq(MqConstants.EVENT_TYPE_AFTERSALES_CASE_AUTO_COMPLETED_STORE_PICKUP_TIMEOUT),
                eq("A004"), eq("O400"), eq(2001L), eq(1001L), eq("system"),
                extraCap.capture(), eq(PRINCIPALS));
        Map<String, Object> extra = extraCap.getValue();
        assertEquals("3002", extra.get("shipping_method"));
        assertNotNull(extra.get("deadline_at"));

        assertNoGrpcDoingPublishOverloadUsed();
    }

    @Test
    void autoCompleteFromAtStore_statusNot26_returnsFalseWithoutSideEffects() {
        AfterSaleCaseEntity entity = caseWith("A005", "O500", "27", "3002",
                PaymentMethod.CREDIT_CARD.value());
        stubLoadCase(entity);

        boolean ok = service.autoCompleteFromAtStore("A005", PRINCIPALS);

        assertFalse(ok);
        verify(caseMapper, never()).markAutoCompletedFromAtStore(anyString(), any());
        verify(caseMapper, never()).markAutoCompletedFromDelivered(anyString(), any());
        verify(eventPublisher, never()).tryPublishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(), any(), any(EventPrincipals.class));
    }

    @Test
    void autoCompleteFromAtStore_raceLost_returnsFalse() {
        AfterSaleCaseEntity entity = caseWith("A006", "O600", "26", "3002",
                PaymentMethod.CREDIT_CARD.value());
        stubLoadCase(entity);
        stubResolutionAmount("A006", 800L);
        when(caseMapper.markAutoCompletedFromAtStore(eq("A006"), any())).thenReturn(0);

        boolean ok = service.autoCompleteFromAtStore("A006", PRINCIPALS);

        assertFalse(ok);
        verify(caseMapper, never()).markRefundProcessingFromReceived(anyString());
        verify(eventPublisher, never()).tryPublishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(), any(), any(EventPrincipals.class));
    }


    @Test
    void autoConfirmReceipt_publishFails_throwsSoTxRollsBack() {
        AfterSaleCaseEntity entity = caseWith("A010", "O1000", "27", "3001",
                PaymentMethod.CREDIT_CARD.value());
        entity.setAutoConfirmAt(LocalDateTime.of(2026, 5, 22, 1, 0));
        stubLoadCase(entity);
        stubResolutionAmount("A010", 1000L);
        when(caseMapper.markAutoCompletedFromDelivered(eq("A010"), any())).thenReturn(1);
        when(caseMapper.markRefundProcessingFromReceived("A010")).thenReturn(1);
        when(eventPublisher.tryPublishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(), any(), any(EventPrincipals.class)))
                .thenReturn(false);

        assertThrows(EventPublishFailedException.class, () -> service.autoConfirmReceipt("A010", PRINCIPALS));
    }

    @Test
    void autoCompleteFromAtStore_publishFails_throwsSoTxRollsBack() {
        AfterSaleCaseEntity entity = caseWith("A011", "O1100", "26", "3002",
                PaymentMethod.CREDIT_CARD.value());
        entity.setStorePickupDeadlineAt(LocalDateTime.of(2026, 5, 22, 1, 0));
        stubLoadCase(entity);
        stubResolutionAmount("A011", 800L);
        when(caseMapper.markAutoCompletedFromAtStore(eq("A011"), any())).thenReturn(1);
        when(caseMapper.markRefundProcessingFromReceived("A011")).thenReturn(1);
        when(eventPublisher.tryPublishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(), any(), any(EventPrincipals.class)))
                .thenReturn(false);

        assertThrows(EventPublishFailedException.class, () -> service.autoCompleteFromAtStore("A011", PRINCIPALS));
    }


    @Test
    void confirmReceipt_status26_isRejected() {
        AfterSaleCaseEntity entity = caseWith("A007", "O700", "26", "3002",
                PaymentMethod.CREDIT_CARD.value());
        stubLoadCase(entity);

        AfterSalesBusinessException ex = assertThrows(
                AfterSalesBusinessException.class,
                () -> service.confirmReceipt(1001L, "A007", null, null, null, null, PRINCIPALS));
        assertEquals(GrpcResponseCode.AFTERSALE_INVALID_STATUS.code(), ex.code());

        verify(caseMapper, never()).markReturnReceived(anyString());
        verify(caseMapper, never()).markAutoCompletedFromAtStore(anyString(), any());
        verify(eventPublisher, never()).publishCaseTransition(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString());
    }


    @Test
    void confirmReceipt_callerIdDiffersFromCaseStoreId_throwsNotOwner() {
        AfterSaleCaseEntity entity = caseWith("A099", "O990", "27", "3001",
                PaymentMethod.CREDIT_CARD.value());
        stubLoadCase(entity);

        AfterSalesBusinessException ex = assertThrows(
                AfterSalesBusinessException.class,
                () -> service.confirmReceipt(99999L, "A099", null, null, null, null, PRINCIPALS));
        assertEquals(GrpcResponseCode.AFTERSALE_NOT_OWNER.code(), ex.code());

        verify(caseMapper, never()).markReturnReceived(anyString());
    }


    @Test
    void forceConfirmReceiptByAdmin_status26_allowedWithExtendedAllowList() {
        AfterSaleCaseEntity entity = caseWith("A008", "O800", "26", "3002",
                PaymentMethod.CREDIT_CARD.value());
        stubLoadCase(entity);
        stubResolutionAmount("A008", 1200L);
        when(caseMapper.markReturnReceivedByAdmin("A008")).thenReturn(1);
        when(caseMapper.markRefundProcessingFromReceived("A008")).thenReturn(1);

        ReturnReceiptService.ForceConfirmResult result =
                service.forceConfirmReceiptByAdmin(9999L, "A008", "ops 介入", null, null, PRINCIPALS);

        assertEquals("26", result.priorStatus());
        assertEquals(1200L, result.finalRefundAmount());
        assertNotNull(result.refundTaskId());
        verify(caseMapper).markReturnReceivedByAdmin("A008");
        verify(eventPublisher).tryPublishCaseTransitionWithContext(
                eq("aftersales.case.return_received"),
                eq(MqConstants.EVENT_TYPE_AFTERSALES_CASE_RETURN_RECEIVED),
                eq("A008"), eq("O800"), eq(2001L), eq(1001L), eq("admin"),
                isNull(), eq(PRINCIPALS));
        assertNoGrpcDoingPublishOverloadUsed();
    }

    @Test
    void forceConfirmReceiptByAdmin_transitionPublishFails_stillSucceeds() {
        AfterSaleCaseEntity entity = caseWith("A014", "O1400", "27", "3001",
                PaymentMethod.CREDIT_CARD.value());
        stubLoadCase(entity);
        stubResolutionAmount("A014", 1200L);
        when(caseMapper.markReturnReceivedByAdmin("A014")).thenReturn(1);
        when(caseMapper.markRefundProcessingFromReceived("A014")).thenReturn(1);
        when(eventPublisher.tryPublishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(),
                any(), any(EventPrincipals.class)))
                .thenReturn(false);

        assertNotNull(service.forceConfirmReceiptByAdmin(
                9999L, "A014", "ops 介入", null, null, PRINCIPALS).refundTaskId());
    }

    @Test
    void confirmReceipt_transitionPublishFails_stillSucceeds() {
        AfterSaleCaseEntity entity = caseWith("A015", "O1500", "27", "3001",
                PaymentMethod.CREDIT_CARD.value());
        stubLoadCase(entity);
        stubResolutionAmount("A015", 1200L);
        when(caseMapper.markReturnReceived("A015")).thenReturn(1);
        when(caseMapper.markRefundProcessingFromReceived("A015")).thenReturn(1);
        when(eventPublisher.tryPublishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(),
                any(), any(EventPrincipals.class)))
                .thenReturn(false);

        assertDoesNotThrow(
                () -> service.confirmReceipt(1001L, "A015", null, null, null, null, PRINCIPALS));
        assertNoGrpcDoingPublishOverloadUsed();
    }


    @Test
    void resolvePrincipalsForCaseStrict_delegatesToStrictResolver() {
        AfterSaleCaseEntity entity = caseWith("A016", "O1600", "27", "3001",
                PaymentMethod.CREDIT_CARD.value());
        stubLoadCase(entity);
        when(eventPublisher.resolveEventPrincipals(1001L, 2001L))
                .thenThrow(new PrincipalResolutionFailedException("store_id -> seller account_id", 1001L));

        assertThrows(PrincipalResolutionFailedException.class,
                () -> service.resolvePrincipalsForCaseStrict("A016"));
        verify(eventPublisher, never()).resolveEventPrincipalsBestEffort(any(), any());
    }

    @Test
    void resolvePrincipalsForCaseBestEffort_delegatesToBestEffortResolver() {
        AfterSaleCaseEntity entity = caseWith("A017", "O1700", "27", "3001",
                PaymentMethod.CREDIT_CARD.value());
        stubLoadCase(entity);
        when(eventPublisher.resolveEventPrincipalsBestEffort(1001L, 2001L))
                .thenReturn(new EventPrincipals(null, 0L));

        EventPrincipals p = service.resolvePrincipalsForCaseBestEffort("A017");

        assertNull(p.sellerAccountId());
        assertEquals(0L, p.notifyBuyerAccountId());
        verify(eventPublisher, never()).resolveEventPrincipals(any(), any());
    }

    @Test
    void forceConfirmReceiptByAdmin_statusOutsideAllowList_isRejected() {
        AfterSaleCaseEntity entity = caseWith("A009", "O900", "50", "3002",
                PaymentMethod.CREDIT_CARD.value());
        stubLoadCase(entity);

        AfterSalesBusinessException ex = assertThrows(
                AfterSalesBusinessException.class,
                () -> service.forceConfirmReceiptByAdmin(9999L, "A009", "嘗試", null, null, PRINCIPALS));
        assertEquals(GrpcResponseCode.AFTERSALE_INVALID_STATUS.code(), ex.code());
        verify(caseMapper, never()).markReturnReceivedByAdmin(anyString());
    }


    private AfterSaleCaseEntity caseWith(String caseNo, String orderNo, String status,
                                          String returnShippingMethod, String paymentMethod) {
        AfterSaleCaseEntity e = new AfterSaleCaseEntity();
        e.setCaseNo(caseNo);
        e.setOrderNo(orderNo);
        e.setStatus(status);
        e.setReturnShippingMethod(returnShippingMethod);
        e.setBuyerId(2001L);
        e.setStoreId(1001L);
        e.setPaymentMethod(paymentMethod);
        e.setTransactionId("TXN-" + caseNo);
        e.setMerId("HAWK9");
        return e;
    }


    private void stubLoadCase(AfterSaleCaseEntity entity) {
        when(caseMapper.selectOne(any())).thenReturn(entity);
    }

    private void stubResolutionAmount(String caseNo, Long amount) {
        AfterSaleResolutionEntity resolution = new AfterSaleResolutionEntity();
        resolution.setCaseNo(caseNo);
        resolution.setRefundAmount(amount);
        when(resolutionMapper.selectOne(any())).thenReturn(resolution);
    }
}
