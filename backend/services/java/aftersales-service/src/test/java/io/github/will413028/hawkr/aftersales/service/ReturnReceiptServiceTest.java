package io.github.will413028.hawkr.aftersales.service;

import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleResolutionEntity;
import io.github.will413028.hawkr.aftersales.entity.RefundTaskEntity;
import io.github.will413028.hawkr.aftersales.exception.AfterSalesBusinessException;
import io.github.will413028.hawkr.aftersales.grpc.client.PaymentGrpcClient;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleResolutionMapper;
import io.github.will413028.hawkr.aftersales.mapper.RefundTaskMapper;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.AfterSaleCaseStatus;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.payment.grpc.CreateRefundResponse;
import io.github.will413028.hawkr.payment.grpc.RefundDetail;
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
    @Mock private PaymentGrpcClient paymentGrpcClient;

    private AfterSalesProperties properties;
    private ReturnReceiptService service;

    /**
     * 呼叫端在 tx 外解好的身分欄位。刻意與 {@code caseWith(...)} 的 storeId(1001)／buyerId(2001)
     * 都不同 —— 同值就分不出「有沒有真的用預解值」。
     */
    private static final EventPrincipals PRINCIPALS = new EventPrincipals(42L, 55L);

    @BeforeAll
    static void initTsid() {
        // TsidUtil 在 ReturnReceiptService 中用於生成 RefundTaskEntity.id；測試環境直接初始化避免 IllegalStateException。
        try { TsidUtil.init(700, 0); } catch (IllegalStateException ignored) { /* 已初始化 */ }
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
                properties, eventPublisher, evidenceService, caseService, paymentGrpcClient);
        // issue #2294 B5/B6/B7：publish 改用會傳遞失敗的變體，Mockito 對 boolean 預設回 false
        // ⇒ 不 stub 的話每個 happy path 都會被判為「publish 失敗」而拋例外。預設補成成功。
        // （帶 EventPrincipals 的零 gRPC 多載；不帶的那些留給 never() 斷言，刻意不 stub）
        when(eventPublisher.tryPublishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(),
                any(), any(EventPrincipals.class)))
                .thenReturn(true);
        when(eventPublisher.tryPublishAwaitingBuyerRefundPage(
                any(), any(), any(), any(), any(EventPrincipals.class)))
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
        verify(eventPublisher, never()).publishAwaitingBuyerRefundPage(any(), any(), any(), any());
    }

    // ==================== autoConfirmReceipt（27→30，CONFIRM_RECEIPT_TIMEOUT）====================

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

        // 非 ATM 不應該呼叫 publishAwaitingBuyerRefundPage（含 try 變體）
        verify(eventPublisher, never()).publishAwaitingBuyerRefundPage(any(), any(), any(), any());
        verify(eventPublisher, never()).tryPublishAwaitingBuyerRefundPage(
                any(), any(), any(), any(), any());
        // B5：tx 內零身分反解 gRPC —— 退回不帶 principals 的多載即轉紅
        // （範圍限定見 EventPrincipalPreResolutionTest 的同名註解）
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
        // 競態：load 時 status=27，但 markAutoCompletedFromDelivered 卻 affected=0
        // （表示賣家剛搶先手動 confirm 或 Admin force completed）
        AfterSaleCaseEntity entity = caseWith("A003", "O300", "27", "3001",
                PaymentMethod.CREDIT_CARD.value());
        stubLoadCase(entity);
        stubResolutionAmount("A003", 500L);
        when(caseMapper.markAutoCompletedFromDelivered(eq("A003"), any())).thenReturn(0);

        boolean ok = service.autoConfirmReceipt("A003", PRINCIPALS);

        assertFalse(ok);
        // race lost：不應該繼續推進到 markRefundProcessingFromReceived
        verify(caseMapper, never()).markRefundProcessingFromReceived(anyString());
        verify(eventPublisher, never()).tryPublishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(), any(), any(EventPrincipals.class));
    }

    // ==================== autoCompleteFromAtStore（26→30，STORE_PICKUP_TIMEOUT）====================

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

        // B6：tx 內零身分反解 gRPC
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
        // 競態：load 時 status=26，但 markAutoCompletedFromAtStore affected=0
        // （表示賣家剛從超商取貨觸發 26→27 或 Admin force completed）
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

    // ==================== issue #2294 B5/B6/B7：publish 失敗必須讓短 tx rollback ====================

    @Test
    void autoConfirmReceipt_publishFails_throwsSoTxRollsBack() {
        // B5：status='32' 是永久轉換，publish 失敗若不 rollback ⇒ 事件永久遺失（row 已離開候選集合）。
        // 契約：tryPublish 回 false ⇒ 拋 EventPublishFailedException ⇒ @Transactional rollback ⇒ 下一 tick 重來。
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
        // B6：與 B5 逐字同形（status='31'）。兩處必須一起改，否則同一支 service 兩種保證等級。
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
    void atmRefundPagePublishFails_throwsSoTxRollsBack() {
        // B7：ATM 路徑的 awaiting_buyer_refund_page 遺失＝買家永遠拿不到退款頁連結、退款卡死。
        // ⚠️ 這個 helper 被兩條排程 ＋ 兩條 API 共用 ⇒ 本斷言同時鎖住「API 側接受 fail-closed」的裁決（09 §5.3.7）。
        AfterSaleCaseEntity entity = caseWith("A012", "O1200", "27", "3001",
                PaymentMethod.ATM.value());
        entity.setAutoConfirmAt(LocalDateTime.of(2026, 5, 22, 1, 0));
        stubLoadCase(entity);
        stubResolutionAmount("A012", 900L);
        when(caseMapper.markAutoCompletedFromDelivered(eq("A012"), any())).thenReturn(1);
        when(caseMapper.markRefundProcessingFromReceived("A012")).thenReturn(1);
        when(paymentGrpcClient.createRefund(anyString(), anyString(), anyString(), anyString()))
                .thenReturn(atmCreateRefundOk());
        when(eventPublisher.tryPublishAwaitingBuyerRefundPage(
                any(), any(), any(), any(), any(EventPrincipals.class)))
                .thenReturn(false);

        assertThrows(EventPublishFailedException.class, () -> service.autoConfirmReceipt("A012", PRINCIPALS));
    }

    @Test
    void atmRefundPagePublishSucceeds_usesFailurePropagatingVariant() {
        // 正面對照：ATM happy path 走的必須是 try 變體（不是舊的吞噬版），否則上面那支測試永遠不會紅
        AfterSaleCaseEntity entity = caseWith("A013", "O1300", "27", "3001",
                PaymentMethod.ATM.value());
        entity.setAutoConfirmAt(LocalDateTime.of(2026, 5, 22, 1, 0));
        stubLoadCase(entity);
        stubResolutionAmount("A013", 900L);
        when(caseMapper.markAutoCompletedFromDelivered(eq("A013"), any())).thenReturn(1);
        when(caseMapper.markRefundProcessingFromReceived("A013")).thenReturn(1);
        when(paymentGrpcClient.createRefund(anyString(), anyString(), anyString(), anyString()))
                .thenReturn(atmCreateRefundOk());

        assertTrue(service.autoConfirmReceipt("A013", PRINCIPALS));

        // B7：帶 principals 的零 gRPC 多載 ＋ 預解值真的被帶下去（55 ≠ buyerId 2001）
        verify(eventPublisher).tryPublishAwaitingBuyerRefundPage(
                eq("O1300"), eq("A013"), any(), eq(2001L), eq(PRINCIPALS));
        verify(eventPublisher, never()).publishAwaitingBuyerRefundPage(any(), any(), any(), any());
        assertNoGrpcDoingPublishOverloadUsed();
    }

    // ==================== confirmReceipt（賣家手動，前置必須 status=27）====================

    @Test
    void confirmReceipt_status26_isRejected() {
        // §5.3.7：賣家確認收貨前置必須 status=27（已送達）；26 RETURN_AT_STORE 屬於到店待取，禁止 confirm
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

    // ==================== Plan C Phase 3: account_id vs store_id 判別 ====================

    @Test
    void confirmReceipt_callerIdDiffersFromCaseStoreId_throwsNotOwner() {
        // caseWith(...) 的 store_id 固定為 1001L（helper 內建）；呼叫端傳入不同的 id
        // 模擬賣家重新上架後 store_id 已換發、呼叫端誤傳舊 id 的情境，必須被拒絕。
        AfterSaleCaseEntity entity = caseWith("A099", "O990", "27", "3001",
                PaymentMethod.CREDIT_CARD.value());
        stubLoadCase(entity);

        AfterSalesBusinessException ex = assertThrows(
                AfterSalesBusinessException.class,
                () -> service.confirmReceipt(99999L, "A099", null, null, null, null, PRINCIPALS));
        assertEquals(GrpcResponseCode.AFTERSALE_NOT_OWNER.code(), ex.code());

        verify(caseMapper, never()).markReturnReceived(anyString());
    }

    // ==================== forceConfirmReceiptByAdmin（Admin force，允許 status=26）====================

    @Test
    void forceConfirmReceiptByAdmin_status26_allowedWithExtendedAllowList() {
        // 2026-05-22 ADMIN_FORCE_ALLOWED_STATUSES 已擴增 26 RETURN_AT_STORE
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
        // issue #2294 B7：Admin 路徑同樣走零 gRPC 多載（語意仍是 best-effort，回傳值不影響流程）
        verify(eventPublisher).tryPublishCaseTransitionWithContext(
                eq("aftersales.case.return_received"),
                eq(MqConstants.EVENT_TYPE_AFTERSALES_CASE_RETURN_RECEIVED),
                eq("A008"), eq("O800"), eq(2001L), eq(1001L), eq("admin"),
                isNull(), eq(PRINCIPALS));
        assertNoGrpcDoingPublishOverloadUsed();
    }

    @Test
    void forceConfirmReceiptByAdmin_transitionPublishFails_stillSucceeds() {
        // API 路徑的 CASE_RETURN_RECEIVED 維持 best-effort：MQ 故障只是少一則通知，
        // 不該讓 Admin 的強制確認整筆失敗（與同 tx 內 ATM 退款頁事件的 fail-closed 是兩種等級，
        // 差別在「遺失的後果是少一則通知」vs「錢退不出去且沒人知道」）。
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
        // 同上，賣家手動路徑。
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

    // ==================== issue #2294 B5/B6/B7：tx 外的身分反解 ====================

    @Test
    void resolvePrincipalsForCaseStrict_delegatesToStrictResolver() {
        // 排程路徑：反解失敗必須拋（該筆本輪不做、下一 tick 重試），不得降級
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
        // API 路徑：走 best-effort（永不拋）——反解抖動不得升級成使用者可見的失敗
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
        // CLOSED(50) 不在 ADMIN_FORCE_ALLOWED_STATUSES 內
        AfterSaleCaseEntity entity = caseWith("A009", "O900", "50", "3002",
                PaymentMethod.CREDIT_CARD.value());
        stubLoadCase(entity);

        AfterSalesBusinessException ex = assertThrows(
                AfterSalesBusinessException.class,
                () -> service.forceConfirmReceiptByAdmin(9999L, "A009", "嘗試", null, null, PRINCIPALS));
        assertEquals(GrpcResponseCode.AFTERSALE_INVALID_STATUS.code(), ex.code());
        verify(caseMapper, never()).markReturnReceivedByAdmin(anyString());
    }

    // ==================== helpers ====================

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


    private CreateRefundResponse atmCreateRefundOk() {
        return CreateRefundResponse.newBuilder()
                .setCode(0)
                .setData(RefundDetail.newBuilder().setRefundNo("RF-TEST").build())
                .build();
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
