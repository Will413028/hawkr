package io.github.will413028.hawkr.aftersales.service;

import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.entity.RefundTaskEntity;
import io.github.will413028.hawkr.aftersales.exception.AfterSalesBusinessException;
import io.github.will413028.hawkr.aftersales.grpc.client.CouponGrpcClient;
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
import io.github.will413028.hawkr.common.utils.code.WorkerTaskStatus;
import io.github.will413028.hawkr.coupon.grpc.GrantSellerCompensationCouponResponse;
import io.github.will413028.hawkr.coupon.grpc.SellerCompensationCouponData;
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

    @InjectMocks private AfterSaleResolutionService service;

    private static final Long SELLER = 2683382637L;
    private static final Long BUYER = 2608240571L;
    private static final String CASE_NO = "A110159113";
    private static final Long CASE_ID = 2700000000001L;

    @BeforeAll
    static void initTsid() {
        try { TsidUtil.init(700, 0); } catch (IllegalStateException ignored) { }
    }

    private AfterSaleCaseEntity pendingCase() {
        AfterSaleCaseEntity c = new AfterSaleCaseEntity();
        c.setId(CASE_ID);
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
                anyString(), anyString(), anyString(), anyString(), anyString(), anyString(), anyString()))
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
                anyString(), anyString(), anyString(), anyString(), anyString(), anyString(), anyString()))
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

    // ---- D4：補償券的冪等鍵是售後單 id，必須送到 coupon ----
    @Test
    void couponCompensation_sendsTheCaseIdAsSourceId() {
        when(couponGrpcClient.grantSellerCompensationCoupon(
                anyString(), anyString(), anyString(), anyString(), anyString(), anyString(), anyString()))
                .thenReturn(GrantSellerCompensationCouponResponse.newBuilder()
                        .setCode(GrpcResponseCode.SUCCESS.code())
                        .setData(SellerCompensationCouponData.newBuilder().setTemplateId("9988").build())
                        .build());
        when(caseMapper.closeByCouponCompensation(anyString(), any())).thenReturn(1);

        service.resolve(SELLER, CASE_NO, AfterSaleResolutionType.COUPON_COMPENSATION.value(),
                null, null, null, null, 100L, 600L, "補償");

        verify(couponGrpcClient).grantSellerCompensationCoupon(
                eq(String.valueOf(SELLER)), eq(String.valueOf(BUYER)), eq("100"), eq("600"), eq("補償"),
                eq(CASE_NO), eq(String.valueOf(CASE_ID)));
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

    // ==================== 退款決議：建 PENDING 退款任務，交給 RefundTaskWorker ====================

    @Test
    @DisplayName("全額退款（免退貨）→ 建 PENDING 退款任務，不發額外的買家操作事件")
    void fullRefundNoReturn_insertsPendingRefundTask() {
        AfterSaleCaseEntity c = pendingCase();
        c.setTransactionId("TXN1");
        when(caseMapper.selectOne(any())).thenReturn(c);
        when(properties.getRefund()).thenReturn(new AfterSalesProperties.RefundConfig());
        when(caseMapper.markRefundProcessingFromResolution(CASE_NO)).thenReturn(1);

        service.resolve(SELLER, CASE_NO, AfterSaleResolutionType.FULL_REFUND_NO_RETURN.value(),
                null, null, null, null, null, null, null);

        ArgumentCaptor<RefundTaskEntity> taskCaptor = ArgumentCaptor.forClass(RefundTaskEntity.class);
        verify(refundTaskMapper).insert(taskCaptor.capture());
        assertEquals(CASE_NO, taskCaptor.getValue().getCaseNo());
        assertEquals("TXN1", taskCaptor.getValue().getTransactionId());
        assertEquals(WorkerTaskStatus.PENDING.value(), taskCaptor.getValue().getStatus());
    }
}
