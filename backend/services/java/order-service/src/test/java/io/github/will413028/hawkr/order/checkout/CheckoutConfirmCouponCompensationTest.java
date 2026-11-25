package io.github.will413028.hawkr.order.checkout;

import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.InvoiceCarrierSelectionMode;
import io.github.will413028.hawkr.common.utils.code.InvoiceDocumentKind;
import io.github.will413028.hawkr.common.utils.code.ShippingFeePayer;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.common.utils.code.ShippingTemperature;
import io.github.will413028.hawkr.order.checkout.CheckoutValidator.AvailableOption;
import io.github.will413028.hawkr.order.checkout.CheckoutValidator.FreeShippingResult;
import io.github.will413028.hawkr.order.checkout.CheckoutValidator.ValidationResult;
import io.github.will413028.hawkr.coupon.grpc.CouponValidateResult;
import io.github.will413028.hawkr.order.checkout.client.CheckoutCouponGrpcClient;
import io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient;
import io.github.will413028.hawkr.order.service.CouponReleaseService;
import io.github.will413028.hawkr.order.exception.OrderBusinessException;
import io.github.will413028.hawkr.order.grpc.CheckoutInvoiceRequest;
import io.github.will413028.hawkr.order.grpc.CheckoutItem;
import io.github.will413028.hawkr.order.grpc.CheckoutConfirmRequest;
import io.github.will413028.hawkr.invoice.grpc.ValidateOrderInvoiceInputInternalResponse;
import io.github.will413028.hawkr.order.grpc.client.BuyerGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.InvoiceGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.ProductGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.ShippingGrpcClient;
import io.github.will413028.hawkr.order.service.OrderService;
import io.github.will413028.hawkr.product.grpc.BatchGetProductForCheckoutResponse;
import io.github.will413028.hawkr.product.grpc.ProductForCheckoutInfo;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import io.github.will413028.hawkr.shipping.grpc.GetSellerShippingConfigResponse;
import io.github.will413028.hawkr.shipping.grpc.SellerShippingConfigItem;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.data.redis.core.ValueOperations;

import java.util.List;
import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyList;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * 結帳鎖券失敗的補償（D3）：reserve 在 coupon 端可能已成功而訂單沒記下 instance id，補償要以 order_no 釋券。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class CheckoutConfirmCouponCompensationTest {

    private static final long STORE_ID = 2685914682L;
    private static final long BUYER_ID = 2623900520L;
    private static final String SKU_ID = "5001";
    private static final String CHECKOUT_ID = "CHK-COUPON-1";
    private static final String METHOD = ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value();
    private static final String TEMP = ShippingTemperature.CHILLED.value();

    @Mock private ProductGrpcClient productGrpcClient;
    @Mock private SellerGrpcClient sellerGrpcClient;
    @Mock private InvoiceGrpcClient invoiceGrpcClient;
    @Mock private BuyerGrpcClient buyerGrpcClient;
    @Mock private ShippingGrpcClient shippingGrpcClient;
    @Mock private CheckoutValidator checkoutValidator;
    @Mock private OrderService orderService;
    @Mock private CheckoutCouponGrpcClient couponGrpcClient;
    @Mock private CouponReleaseService couponReleaseService;
    @Mock private StringRedisTemplate redisTemplate;
    @Mock private ValueOperations<String, String> valueOperations;

    private static final String ORDER_NO = "O20260830001";

    private CheckoutService service;

    @BeforeEach
    void setUp() {
        when(productGrpcClient.batchGetProductForCheckout(anyList()))
                .thenReturn(BatchGetProductForCheckoutResponse.newBuilder()
                        .putData(SKU_ID, ProductForCheckoutInfo.newBuilder()
                                .setSkuName("大")
                                .setProductName("蘋果禮盒")
                                .setStoreId(String.valueOf(STORE_ID))
                                .setListPrice("500")
                                .setStockQty("10")
                                .setIsActive(true)
                                .build())
                        .build());
        when(sellerGrpcClient.resolvePrincipals(anyList()))
                .thenReturn(PrincipalResolution.allResolved(Map.of(STORE_ID, STORE_ID)));
        when(buyerGrpcClient.resolvePrincipals(anyList()))
                .thenReturn(PrincipalResolution.allResolved(Map.of(BUYER_ID, BUYER_ID)));
        when(shippingGrpcClient.getSellerShippingConfig(anyLong()))
                .thenReturn(GetSellerShippingConfigResponse.newBuilder()
                        .setCode(0)
                        .addConfigs(SellerShippingConfigItem.newBuilder()
                                .setShippingMethod(METHOD)
                                .setGoodsType(TEMP)
                                .setAvailable(true)
                                .build())
                        .build());
        when(sellerGrpcClient.getSellerMeta(anyLong())).thenReturn(SellerMeta.newBuilder()
                .setSellerName("測試賣家")
                .setCanAcceptOrders(true)
                .build());
        when(checkoutValidator.validate(anyList(), any()))
                .thenReturn(new ValidationResult(true, java.util.Set.of(TEMP), List.of(), List.of(), 0));
        when(checkoutValidator.lookupSingleOption(anyList(), any(), any(), any(), any()))
                .thenReturn(new AvailableOption(
                        ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP, ShippingTemperature.CHILLED,
                        "7-11店到店（冷藏）", 60L, 60L, ShippingFeePayer.BUYER.value(), 0L, false));
        when(checkoutValidator.checkFreeShipping(anyList(), anyString(), anyString(), any()))
                .thenReturn(new FreeShippingResult(false, ShippingFeePayer.BUYER.value(), 0L));
        when(redisTemplate.opsForValue()).thenReturn(valueOperations);
        when(orderService.createOrderFromCheckout(any())).thenReturn("O20260830001");

        // 發票前置驗證預設放行（code=0）。要測「被擋下」的分支就在該支測試裡改 stub。
        when(invoiceGrpcClient.validateOrderInvoiceInput(any()))
                .thenReturn(ValidateOrderInvoiceInputInternalResponse.newBuilder().setCode(0).build());

        service = new CheckoutService(productGrpcClient, sellerGrpcClient, buyerGrpcClient,
                shippingGrpcClient, couponGrpcClient, invoiceGrpcClient, checkoutValidator, orderService, new OrderPolicyResolver(), redisTemplate, couponReleaseService, null);

        // 🔴 快照用**產線那一支** buildPriceSnapshot 組，不手寫 JSON。
        //    舊版這裡餵的是 "1"（舊格式哨兵），而 confirm 現在對「快照沒有政策五欄」一律 fail-closed
        //    ——那正是要防的東西，所以測試必須餵一份真的快照，不是繞過檢查。
        OrderPolicySnapshot policy =
                new OrderPolicyResolver().resolve(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value());
        when(valueOperations.getAndDelete(anyString())).thenReturn(
                service.buildPriceSnapshot(1000L, 60L, 0L, 1060L, false, false, policy));
    }

    private static CheckoutConfirmRequest.Builder baseRequest() {
        return CheckoutConfirmRequest.newBuilder()
                .setBuyerId(BUYER_ID)
                .setCheckoutId(CHECKOUT_ID)
                .addItems(CheckoutItem.newBuilder()
                        .setSkuId(SKU_ID).setProductId("9001").setQuantity("2").build())
                .setShippingMethod(METHOD)
                .setDeliveryTemp(TEMP)
                .setRecipientName("王小明")
                .setRecipientPhone("0912345678")
                .setCvsStoreId("252698")
                .setCvsStoreName("測試門市")
                .setCouponTemplateId("TPL-1");
    }


    @Test
    @DisplayName("鎖券失敗（含回應遺失、回寫 instance id 失敗）→ 取消訂單後必須以 order_no 釋券（D3），訂單沒記 instance id 也要釋")
    void reserveFailureReleasesTheCouponByOrderNo() {
        when(couponGrpcClient.validateCoupon(anyString(), anyLong(), anyLong(), anyLong()))
                .thenReturn(CouponValidateResult.newBuilder().setValid(true).setDiscountAmount("0").build());
        when(couponGrpcClient.reserveCoupon(anyString(), anyLong(), anyString(), anyLong(), anyLong()))
                .thenThrow(new RuntimeException("response lost"));

        assertThatThrownBy(() -> service.confirm(baseRequest().build()))
                .isInstanceOf(OrderBusinessException.class)
                .satisfies(e -> assertThat(((OrderBusinessException) e).code())
                        .isEqualTo(GrpcResponseCode.COUPON_LOCK_FAILED.code()));

        verify(orderService).cancelOrder(BUYER_ID, ORDER_NO);
        verify(couponReleaseService).releaseWithLateLockRecheck(ORDER_NO);
    }

    @Test
    @DisplayName("回寫 instance id 失敗（reserve 在 coupon 端已成功）→ 同樣以 order_no 釋券")
    void instanceIdWriteBackFailureReleasesTheCouponByOrderNo() {
        when(couponGrpcClient.validateCoupon(anyString(), anyLong(), anyLong(), anyLong()))
                .thenReturn(CouponValidateResult.newBuilder().setValid(true).setDiscountAmount("0").build());
        when(couponGrpcClient.reserveCoupon(anyString(), anyLong(), anyString(), anyLong(), anyLong())).thenReturn("INSTANCE-1");
        org.mockito.Mockito.doThrow(new RuntimeException("db down"))
                .when(orderService).updateCouponInstanceId(ORDER_NO, "INSTANCE-1");

        assertThatThrownBy(() -> service.confirm(baseRequest().build()))
                .isInstanceOf(OrderBusinessException.class);

        verify(couponReleaseService).releaseWithLateLockRecheck(ORDER_NO);
    }

    @Test
    @DisplayName("鎖券成功 → 不釋券")
    void reserveSuccessDoesNotRelease() {
        when(couponGrpcClient.validateCoupon(anyString(), anyLong(), anyLong(), anyLong()))
                .thenReturn(CouponValidateResult.newBuilder().setValid(true).setDiscountAmount("0").build());
        when(couponGrpcClient.reserveCoupon(anyString(), anyLong(), anyString(), anyLong(), anyLong())).thenReturn("INSTANCE-1");

        service.confirm(baseRequest().build());

        verify(couponReleaseService, never()).releaseWithLateLockRecheck(anyString());
    }

    @Test
    @DisplayName("鎖券帶上驗券用的店家與訂單小計，coupon 端 Lock 才能重驗範圍與門檻（D10）")
    void reserveSendsTheSameStoreAndSubtotalAsValidate() {
        when(couponGrpcClient.validateCoupon(anyString(), anyLong(), anyLong(), anyLong()))
                .thenReturn(CouponValidateResult.newBuilder().setValid(true).setDiscountAmount("0").build());
        when(couponGrpcClient.reserveCoupon(anyString(), anyLong(), anyString(), anyLong(), anyLong())).thenReturn("INSTANCE-1");

        service.confirm(baseRequest().build());

        org.mockito.ArgumentCaptor<Long> validateStore = org.mockito.ArgumentCaptor.forClass(Long.class);
        org.mockito.ArgumentCaptor<Long> validateAmount = org.mockito.ArgumentCaptor.forClass(Long.class);
        verify(couponGrpcClient).validateCoupon(eq("TPL-1"), eq(BUYER_ID), validateStore.capture(), validateAmount.capture());
        verify(couponGrpcClient).reserveCoupon("TPL-1", BUYER_ID, ORDER_NO, validateStore.getValue(), validateAmount.getValue());
        assertThat(validateStore.getValue()).isPositive();
        assertThat(validateAmount.getValue()).isPositive();
    }
}
