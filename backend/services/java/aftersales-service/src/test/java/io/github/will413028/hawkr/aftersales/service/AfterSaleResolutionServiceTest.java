package io.github.will413028.hawkr.aftersales.service;

import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.exception.AfterSalesBusinessException;
import io.github.will413028.hawkr.aftersales.grpc.client.CouponGrpcClient;
import io.github.will413028.hawkr.aftersales.grpc.client.PaymentGrpcClient;
import io.github.will413028.hawkr.aftersales.grpc.client.ShippingGrpcClient;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleResolutionMapper;
import io.github.will413028.hawkr.aftersales.mapper.RefundTaskMapper;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.AfterSaleCaseStatus;
import io.github.will413028.hawkr.common.utils.code.AfterSaleResolutionType;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.coupon.grpc.GrantSellerCompensationCouponResponse;
import io.github.will413028.hawkr.coupon.grpc.SellerCompensationCouponData;
import io.github.will413028.hawkr.payment.grpc.CreateRefundResponse;
import io.github.will413028.hawkr.payment.grpc.RefundDetail;
import io.github.will413028.hawkr.shipping.grpc.CreateReturnShipmentResponse;
import io.github.will413028.hawkr.shipping.grpc.ReturnShipmentData;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

/**
 * #1150：售後決議 envelope-code 檢查（依《Java gRPC 錯誤處理規範》§7.0）。
 *
 * <p>覆蓋：
 * <ul>
 *   <li>handleCouponCompensation：coupon grant 回 code≠0（如 amount=0 被擋）→ throw + 不 insert resolution + 不關案</li>
 *   <li>handleCouponCompensation：grant 成功 → 正常 insert + 關案（success path）</li>
 *   <li>handleReturnRefund：shipping 建單回 code≠0 → throw + 不 insert resolution + 不推進 RETURN_PENDING</li>
 * </ul>
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class AfterSaleResolutionServiceTest {

    @Mock private AfterSaleCaseMapper caseMapper;
    @Mock private AfterSaleResolutionMapper resolutionMapper;
    @Mock private RefundTaskMapper refundTaskMapper;
    @Mock private ShippingGrpcClient shippingGrpcClient;
    @Mock private CouponGrpcClient couponGrpcClient;
    @Mock private AfterSaleAuditService auditService;
    @Mock private AfterSalesProperties properties;
    @Mock private AfterSalesEventPublisher afterSalesEventPublisher;
    @Mock private AfterSaleCaseService caseService;
    @Mock private PaymentGrpcClient paymentGrpcClient;

    @InjectMocks private AfterSaleResolutionService service;

    private static final Long SELLER = 2683382637L;
    private static final Long BUYER = 2608240571L;
    private static final String CASE_NO = "A110159113";

    @BeforeAll
    static void initTsid() {
        try { TsidUtil.init(700, 0); } catch (IllegalStateException ignored) { }
    }

    private AfterSaleCaseEntity pendingCase() {
        AfterSaleCaseEntity c = new AfterSaleCaseEntity();
        c.setCaseNo(CASE_NO);
        c.setOrderNo("O400280260");
        c.setStoreId(SELLER);
        c.setBuyerId(BUYER);
        c.setStatus(AfterSaleCaseStatus.PENDING.value());
        c.setPaymentMethod(PaymentMethod.CREDIT_CARD.value());
        c.setShippingMethod(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value());
        c.setRefundableAmount(500L);
        c.setMerId("HAWK2683382637");
        c.setDeliveryTemp("NORMAL");
        c.setBuyerName("測試買家");
        c.setBuyerPhone("0912345678");
        return c;
    }

    @BeforeEach
    void setup() {
        when(caseMapper.selectOne(any())).thenReturn(pendingCase());
    }

    // ---- #1150 觸發點：補償券 grant 失敗（amount=0 被 coupon-service 擋）----
    @Test
    void couponCompensation_grantFailed_throwsAndDoesNotCloseCase() {
        when(couponGrpcClient.grantSellerCompensationCoupon(
                anyString(), anyString(), anyString(), anyString(), anyString(), anyString()))
                .thenReturn(GrantSellerCompensationCouponResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("補償金額必須大於 0")
                        .build());  // envelope-code 失敗，data 空

        AfterSalesBusinessException ex = assertThrows(AfterSalesBusinessException.class, () ->
                service.resolve(SELLER, CASE_NO, AfterSaleResolutionType.COUPON_COMPENSATION.value(),
                        null, null, null, null, 0L, 500L, "補償"));

        assertEquals(GrpcResponseCode.AFTERSALE_COUPON_COMPENSATION_FAILED.code(), ex.code());
        // 關鍵：不 insert resolution、不關案
        verify(resolutionMapper, never()).insert(any());
        verify(caseMapper, never()).closeByCouponCompensation(anyString(), any());
        verify(afterSalesEventPublisher, never()).publishCaseTransition(any(), any(), any(), any(), any(), any(), any());
    }

    // ---- success path：grant 成功正常關案 ----
    @Test
    void couponCompensation_grantSuccess_insertsAndClosesCase() {
        when(couponGrpcClient.grantSellerCompensationCoupon(
                anyString(), anyString(), anyString(), anyString(), anyString(), anyString()))
                .thenReturn(GrantSellerCompensationCouponResponse.newBuilder()
                        .setCode(GrpcResponseCode.SUCCESS.code())
                        .setData(SellerCompensationCouponData.newBuilder().setTemplateId("9988").build())
                        .build());
        when(caseMapper.closeByCouponCompensation(anyString(), any())).thenReturn(1);

        service.resolve(SELLER, CASE_NO, AfterSaleResolutionType.COUPON_COMPENSATION.value(),
                null, null, null, null, 100L, 600L, "補償");

        verify(resolutionMapper, times(1)).insert(any());
        verify(caseMapper, times(1)).closeByCouponCompensation(eq(CASE_NO), any());
    }

    // ---- #1150 同類：退貨建單失敗不推進 ----
    @Test
    void returnRefund_shipmentFailed_throwsAndDoesNotAdvance() {
        when(shippingGrpcClient.createReturnCvsShipment(
                anyString(), anyString(), anyString(), anyString(), anyString(), anyString(), anyString()))
                .thenReturn(CreateReturnShipmentResponse.newBuilder()
                        .setCode(GrpcResponseCode.INTERNAL_ERROR.code())
                        .setMessage("PayUni 物流建單失敗 SHIP02080")
                        .build());  // envelope-code 失敗

        AfterSalesBusinessException ex = assertThrows(AfterSalesBusinessException.class, () ->
                service.resolve(SELLER, CASE_NO, AfterSaleResolutionType.RETURN_REFUND.value(),
                        null, ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value(), null, null, null, null, null));

        assertEquals(GrpcResponseCode.RETURN_SHIPMENT_FAILED.code(), ex.code());
        verify(resolutionMapper, never()).insert(any());
        verify(caseMapper, never()).markReturnPending(anyString(), any(), anyString(), any(), any());
    }

    // ==================== Plan C Phase 3: account_id vs store_id 判別 ====================

    @Test
    void resolve_callerIdDiffersFromCaseStoreId_throwsNotOwner() {
        // pendingCase() 的 store_id = SELLER(2683382637L)；呼叫端傳入明顯不同的 id
        // 模擬賣家重新上架後 store_id 已換發、呼叫端誤傳舊/其他 id 的情境。
        Long wrongCallerId = 9999999999L;

        AfterSalesBusinessException ex = assertThrows(
                AfterSalesBusinessException.class,
                () -> service.resolve(wrongCallerId, CASE_NO, AfterSaleResolutionType.FULL_REFUND_NO_RETURN.value(),
                        null, null, null, null, null, null, null));
        assertEquals(GrpcResponseCode.AFTERSALE_NOT_OWNER.code(), ex.code());
    }

    // ==================== issue #1626: publishAwaitingBuyerRefundPage 參數位置 ====================
    //
    // publishAwaitingBuyerRefundPage(orderNo, caseNo, refundTaskId, buyerId) 的第 4 個參數是
    // 這次新加的 buyerId，與第 3 個 refundTaskId 同為 Long，寫反照樣編譯過。下面兩支測試把
    // ATM 決議路徑（handleFullRefundNoReturn / handlePartialRefund 兩個 call site）跑到底，
    // 綁死第 4 個參數 = caseEntity.getBuyerId()。BUYER / SELLER / refundAmount 三者值皆不同，
    // 任何錯位都會 fail。

    private AfterSaleCaseEntity atmPendingCase() {
        AfterSaleCaseEntity c = pendingCase();
        c.setPaymentMethod(PaymentMethod.ATM.value());
        c.setTransactionId("TXN1");
        return c;
    }

    private void stubAtmRefundPrebuildSuccess() {
        when(caseMapper.selectOne(any())).thenReturn(atmPendingCase());
        when(paymentGrpcClient.createRefund(any(), any(), any(), any()))
                .thenReturn(CreateRefundResponse.newBuilder()
                        .setCode(0)
                        .setData(RefundDetail.newBuilder().setRefundNo("RFD_1").build())
                        .build());
        when(properties.getRefund()).thenReturn(new AfterSalesProperties.RefundConfig());
        when(caseMapper.markRefundProcessingFromResolution(CASE_NO)).thenReturn(1);
    }

    private void assertAwaitingBuyerRefundPagePublishedWithBuyerId() {
        ArgumentCaptor<String> orderNoCaptor = ArgumentCaptor.forClass(String.class);
        ArgumentCaptor<String> caseNoCaptor = ArgumentCaptor.forClass(String.class);
        ArgumentCaptor<Long> refundTaskIdCaptor = ArgumentCaptor.forClass(Long.class);
        ArgumentCaptor<Long> buyerIdCaptor = ArgumentCaptor.forClass(Long.class);
        verify(afterSalesEventPublisher).publishAwaitingBuyerRefundPage(
                orderNoCaptor.capture(), caseNoCaptor.capture(),
                refundTaskIdCaptor.capture(), buyerIdCaptor.capture());

        assertEquals("O400280260", orderNoCaptor.getValue());
        assertEquals(CASE_NO, caseNoCaptor.getValue());
        assertNotNull(refundTaskIdCaptor.getValue());
        assertEquals(BUYER, buyerIdCaptor.getValue(),
                "第 4 個參數必須是 caseEntity.buyerId，不可與 storeId/refundTaskId 錯位");
        assertNotEquals(SELLER, buyerIdCaptor.getValue());
        assertNotEquals(refundTaskIdCaptor.getValue(), buyerIdCaptor.getValue());
    }

    @Test
    @DisplayName("ATM 全額退款（免退貨）→ publishAwaitingBuyerRefundPage 第 4 參數 = caseEntity.buyerId")
    void fullRefundNoReturn_atm_publishesAwaitingBuyerRefundPageWithBuyerId() {
        stubAtmRefundPrebuildSuccess();

        service.resolve(SELLER, CASE_NO, AfterSaleResolutionType.FULL_REFUND_NO_RETURN.value(),
                null, null, null, null, null, null, null);

        assertAwaitingBuyerRefundPagePublishedWithBuyerId();
    }

    @Test
    @DisplayName("ATM 部分退款 → publishAwaitingBuyerRefundPage 第 4 參數 = caseEntity.buyerId")
    void partialRefund_atm_publishesAwaitingBuyerRefundPageWithBuyerId() {
        stubAtmRefundPrebuildSuccess();

        service.resolve(SELLER, CASE_NO, AfterSaleResolutionType.PARTIAL_REFUND.value(),
                300L, null, null, null, null, null, null);

        assertAwaitingBuyerRefundPagePublishedWithBuyerId();
    }

    /**
     * 對照組：§2.3.1 INV-5——ATM createRefund 預建失敗時 task 直接 FAILED，
     * 不該發等待頁事件（守住 {@code isAtm && status==AWAITING_BUYER_REFUND_PAGE} 這個 and 條件）。
     */
    @Test
    @DisplayName("ATM createRefund 預建失敗 → 不發 publishAwaitingBuyerRefundPage")
    void fullRefundNoReturn_atmPrebuildFailed_doesNotPublishAwaitingBuyerRefundPage() {
        when(caseMapper.selectOne(any())).thenReturn(atmPendingCase());
        when(paymentGrpcClient.createRefund(any(), any(), any(), any()))
                .thenReturn(CreateRefundResponse.newBuilder()
                        .setCode(GrpcResponseCode.INTERNAL_ERROR.code())
                        .setMessage("PayUni 預建退款失敗")
                        .build());
        when(properties.getRefund()).thenReturn(new AfterSalesProperties.RefundConfig());
        when(caseMapper.markRefundProcessingFromResolution(CASE_NO)).thenReturn(1);

        service.resolve(SELLER, CASE_NO, AfterSaleResolutionType.FULL_REFUND_NO_RETURN.value(),
                null, null, null, null, null, null, null);

        verify(afterSalesEventPublisher, never())
                .publishAwaitingBuyerRefundPage(any(), any(), any(), any());
    }
}
