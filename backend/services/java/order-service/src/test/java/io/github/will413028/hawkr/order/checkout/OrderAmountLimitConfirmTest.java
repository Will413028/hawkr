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
 * 訂單金額上限（{@link OrderAmountLimit#ORDER_MAX_AMOUNT}）：confirm 對超過者回 ORDER_AMOUNT_EXCEEDS_LIMIT，恰好等於上限放行。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class OrderAmountLimitConfirmTest {

    private static final long STORE_ID = 2685914682L;
    private static final long BUYER_ID = 2623900520L;
    private static final String SKU_ID = "5001";
    private static final String CHECKOUT_ID = "CHK-LIMIT-1";
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
    private OrderPolicySnapshot snapshotPolicy;

    @BeforeEach
    void setUp() {
        when(productGrpcClient.batchGetProductForCheckout(anyList()))
                .thenReturn(BatchGetProductForCheckoutResponse.newBuilder()
                        .putData(SKU_ID, ProductForCheckoutInfo.newBuilder()
                                .setSkuName("大")
                                .setProductName("蘋果禮盒")
                                .setStoreId(String.valueOf(STORE_ID))
                                .setListPrice("199939")
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
        snapshotPolicy = policy;
    }

    private static CheckoutConfirmRequest.Builder baseRequest(String quantity) {
        return CheckoutConfirmRequest.newBuilder()
                .setBuyerId(BUYER_ID)
                .setCheckoutId(CHECKOUT_ID)
                .addItems(CheckoutItem.newBuilder()
                        .setSkuId(SKU_ID).setProductId("9001").setQuantity(quantity).build())
                .setShippingMethod(METHOD)
                .setDeliveryTemp(TEMP)
                .setRecipientName("王小明")
                .setRecipientPhone("0912345678")
                .setCvsStoreId("252698")
                .setCvsStoreName("測試門市");
    }

    @Test
    @DisplayName("訂單金額等於上限（199,999）→ 放行建單")
    void amountEqualToLimitPasses() {
        // 單價 199,939 × 1 + 運費 60 = 199,999
        when(valueOperations.getAndDelete(anyString())).thenReturn(
                service.buildPriceSnapshot(199_939L, 60L, 0L, 199_999L, false, false, snapshotPolicy));

        service.confirm(baseRequest("1").build());

        verify(orderService).createOrderFromCheckout(any());
    }

    @Test
    @DisplayName("訂單金額超過上限（200,000）→ ORDER_AMOUNT_EXCEEDS_LIMIT，不建單")
    void amountAboveLimitIsRejected() {
        // 單價 199,939 × 1 + 運費 60 = 199,999；數量 1 → 改成運費 61 讓總額剛好 200,000
        when(checkoutValidator.lookupSingleOption(anyList(), any(), any(), any(), any()))
                .thenReturn(new AvailableOption(
                        ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP, ShippingTemperature.CHILLED,
                        "7-11店到店（冷藏）", 61L, 61L, ShippingFeePayer.BUYER.value(), 0L, false));
        when(valueOperations.getAndDelete(anyString())).thenReturn(
                service.buildPriceSnapshot(199_939L, 61L, 0L, 200_000L, false, false, snapshotPolicy));

        assertThatThrownBy(() -> service.confirm(baseRequest("1").build()))
                .isInstanceOf(OrderBusinessException.class)
                .satisfies(e -> assertThat(((OrderBusinessException) e).code())
                        .isEqualTo(GrpcResponseCode.ORDER_AMOUNT_EXCEEDS_LIMIT.code()));

        verify(orderService, never()).createOrderFromCheckout(any());
    }

    @Test
    @DisplayName("上限常數是 199,999（invoice-service A20 MaxAmount 與它一致）")
    void limitConstantMatchesInvoiceMaxAmount() {
        assertThat(OrderAmountLimit.ORDER_MAX_AMOUNT).isEqualTo(199_999L);
    }
}
