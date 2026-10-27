package io.github.will413028.hawkr.aftersales.grpc;

import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.grpc.client.BuyerGrpcClient;
import io.github.will413028.hawkr.aftersales.grpc.client.OrderGrpcClient;
import io.github.will413028.hawkr.aftersales.grpc.client.PaymentGrpcClient;
import io.github.will413028.hawkr.aftersales.grpc.client.SellerGrpcClient;
import io.github.will413028.hawkr.aftersales.grpc.client.ShippingGrpcClient;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseItemMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleEvidenceMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleResolutionMapper;
import io.github.will413028.hawkr.aftersales.mapper.RefundTaskMapper;
import io.github.will413028.hawkr.aftersales.mapper.SellerCancelRequestMapper;
import io.github.will413028.hawkr.aftersales.service.AfterSaleAppealService;
import io.github.will413028.hawkr.aftersales.service.AfterSaleCaseService;
import io.github.will413028.hawkr.aftersales.service.AfterSaleLabelRetryService;
import io.github.will413028.hawkr.aftersales.service.AfterSaleResolutionService;
import io.github.will413028.hawkr.aftersales.service.PlatformDecisionService;
import io.github.will413028.hawkr.aftersales.service.RefundPageService;
import io.github.will413028.hawkr.aftersales.service.ReturnReceiptService;
import io.github.will413028.hawkr.aftersales.service.SellerCancelService;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import java.lang.reflect.Method;
import java.time.LocalDateTime;
import java.util.List;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.when;

/**
 * 2026-05-22 逆物流退貨政策修訂 — AfterSalesGrpcService 欄位映射測試。
 *
 * <p>覆蓋 case detail proto 的逆物流時間戳欄位映射：
 * <ul>
 *   <li>storeArrivedAt / storePickupDeadlineAt / sellerPickedUpAt</li>
 * </ul>
 * （autoCompleteReason / autoCompleteReasonLabel / canConfirmReceipt 已於 549889c1 status 拆分重構移除）
 *
 * <p>用 reflection 呼叫 private {@code buildDetailView}，避免拉 SpringBoot context（gRPC server / Nacos）。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class AfterSalesGrpcServiceFieldMappingTest {

    @Mock private AfterSaleCaseService caseService;
    @Mock private AfterSaleResolutionService resolutionService;
    @Mock private RefundPageService refundPageService;
    @Mock private ReturnReceiptService returnReceiptService;
    @Mock private SellerCancelService sellerCancelService;
    @Mock private AfterSaleLabelRetryService labelRetryService;
    @Mock private AfterSaleAppealService appealService;
    @Mock private PlatformDecisionService platformDecisionService;
    @Mock private AfterSaleCaseMapper caseMapper;
    @Mock private AfterSaleCaseItemMapper caseItemMapper;
    @Mock private AfterSaleEvidenceMapper evidenceMapper;
    @Mock private AfterSaleResolutionMapper resolutionMapper;
    @Mock private RefundTaskMapper refundTaskMapper;
    @Mock private SellerCancelRequestMapper cancelMapper;
    @Mock private PaymentGrpcClient paymentGrpcClient;
    @Mock private BuyerGrpcClient buyerGrpcClient;
    @Mock private SellerGrpcClient sellerGrpcClient;
    @Mock private ShippingGrpcClient shippingGrpcClient;
    @Mock private OrderGrpcClient orderGrpcClient;
    @Mock private io.github.will413028.hawkr.aftersales.service.AtmRefundReconciler atmRefundReconciler;

    private AfterSalesProperties properties;
    private AfterSalesGrpcService service;
    private Method buildDetailView;

    @BeforeEach
    void setUp() throws Exception {
        properties = new AfterSalesProperties();
        properties.getCaseConfig().setProcessDeadlineHours(72);

        // buildDetailView 中所有非新欄位 side-effect 的 mock 預設：
        when(caseItemMapper.selectList(any())).thenReturn(List.of());
        when(evidenceMapper.selectList(any())).thenReturn(List.of());
        when(resolutionMapper.selectList(any())).thenReturn(List.of());
        when(refundTaskMapper.selectOne(any())).thenReturn(null);
        when(appealService.computeAppealDeadline(any())).thenReturn(null);
        when(appealService.canAppeal(any())).thenReturn(false);
        // orderGrpcClient 拋出例外是合法的（buildDetailView 內部 try-catch，降級為空陣列）
        when(orderGrpcClient.getOrderForAfterSales(any()))
                .thenThrow(new RuntimeException("test stub: skip shipment photos"));

        service = new AfterSalesGrpcService(
                caseService, resolutionService, refundPageService, returnReceiptService,
                sellerCancelService, labelRetryService, appealService, platformDecisionService,
                org.mockito.Mockito.mock(io.github.will413028.hawkr.aftersales.service.AfterSalePayoutHoldService.class),
                caseMapper, caseItemMapper, evidenceMapper, resolutionMapper, refundTaskMapper,
                cancelMapper, properties,
                paymentGrpcClient, buyerGrpcClient, sellerGrpcClient, shippingGrpcClient, orderGrpcClient,
                atmRefundReconciler,
                org.mockito.Mockito.mock(org.springframework.transaction.PlatformTransactionManager.class));

        buildDetailView = AfterSalesGrpcService.class
                .getDeclaredMethod("buildDetailView", AfterSaleCaseEntity.class);
        buildDetailView.setAccessible(true);
    }

    // ==================== 6 個新欄位映射 ====================

    @Test
    @DisplayName("detail 寫入 storeArrivedAt / storePickupDeadlineAt / sellerPickedUpAt 三個 timestamp")
    void detail_writesNewTimestampFields() throws Exception {
        AfterSaleCaseEntity entity = baseEntity("A001", "27");
        entity.setStoreArrivedAt(LocalDateTime.of(2026, 5, 22, 1, 0));
        entity.setStorePickupDeadlineAt(LocalDateTime.of(2026, 5, 25, 1, 0));
        entity.setSellerPickedUpAt(LocalDateTime.of(2026, 5, 23, 9, 30));

        Object detail = buildDetailView.invoke(service, entity);

        assertNotEquals("", invokeGetter(detail, "getStoreArrivedAt"));
        assertNotEquals("", invokeGetter(detail, "getStorePickupDeadlineAt"));
        assertNotEquals("", invokeGetter(detail, "getSellerPickedUpAt"));
    }

    // ==================== helpers ====================

    private AfterSaleCaseEntity baseEntity(String caseNo, String status) {
        AfterSaleCaseEntity e = new AfterSaleCaseEntity();
        e.setCaseNo(caseNo);
        e.setOrderNo("O-" + caseNo);
        e.setBuyerId(2001L);
        e.setStoreId(1001L);
        e.setStatus(status);
        e.setReason("BROKEN");
        e.setDescription("test");
        e.setRefundableAmount(1000L);
        e.setCreatedAt(LocalDateTime.of(2026, 5, 22, 0, 0));
        return e;
    }

    /** 透過 reflection 取 proto getter（避免顯式 import proto class 增加耦合） */
    private Object invokeGetter(Object detail, String getterName) throws Exception {
        Method m = detail.getClass().getDeclaredMethod(getterName);
        m.setAccessible(true);
        return m.invoke(detail);
    }
}
