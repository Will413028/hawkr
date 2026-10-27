package io.github.will413028.hawkr.order.checkout;

import io.github.will413028.hawkr.buyer.grpc.AddressInfo;
import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.common.utils.code.ShippingFeePayer;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.common.utils.code.ShippingTemperature;
import io.github.will413028.hawkr.invoice.grpc.ValidateOrderInvoiceInputInternalResponse;
import io.github.will413028.hawkr.order.checkout.CheckoutValidator.AvailableOption;
import io.github.will413028.hawkr.order.checkout.CheckoutValidator.FreeShippingResult;
import io.github.will413028.hawkr.order.checkout.CheckoutValidator.ValidationResult;
import io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient;
import io.github.will413028.hawkr.order.grpc.CheckoutConfirmRequest;
import io.github.will413028.hawkr.order.grpc.CheckoutItem;
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
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyList;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * 結帳鏈的三支入口<b>各自實際餵給閘門的 context</b>——不是「有沒有呼叫閘門」，
 * 而是「呼叫時手上有沒有證據」。
 *
 * <h2>🔴 這一支要防的具體事故</h2>
 * 2026-09-12：買家在購物車<b>選得到</b> Uber（Buyer Web／App 都把選項畫出來了），
 * 而 {@code preview}／{@code confirm} 一律走 {@code Context.methodOnly} ⇒
 * {@code UberOrderEligibilityGuard} 依 deny-by-default 回 <b>9503</b>，<b>無條件</b>。
 * 那是一條<b>本次改動新開出來的可達死路</b>：選項看得到、按下去必定失敗。
 *
 * <p>當時沒有任何測試轉紅，因為既有的 {@code UberCheckoutWiringTest} 驗的是
 * 「{@code lookupSingleOption} 對 3003 會拋 9503」——那條在死路成立時<b>照樣是綠的</b>。
 * 缺的是這一支：<b>呼叫端有沒有把證據送進去</b>。
 *
 * <h2>三支入口的正確答案不一樣（所以要逐支驗，不能只驗一支）</h2>
 * <ul>
 *   <li>{@code preview}／{@code confirm}：收 {@code address_id} ⇒ <b>必須</b>帶完整 probe</li>
 *   <li>{@code shippingFee}：算的是級距費率表，Uber 沒有級距表 ⇒ <b>必須</b>是 {@code methodOnly}
 *       （這一格是正確行為不是缺口，見 {@code UberOrderEligibilityGuard} 的
 *       「誰供得起 probe」）</li>
 * </ul>
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class UberCheckoutProbeWiringTest {

    private static final long STORE_ID = 2685914682L;
    private static final long BUYER_ID = 2623900520L;
    private static final String SKU_ID = "5001";
    private static final String CHECKOUT_ID = "CHK-UBER-1";
    /** 硬寫字面值，不從 enum 取。 */
    private static final String UBER = "3003";
    private static final String TEMP = "3202";
    private static final String ADDRESS_ID = "2690000001";
    private static final String QUOTE_ID = "dqt_from_buyer";
    private static final String ORDER_NO = "O20260912001";

    @Mock private ProductGrpcClient productGrpcClient;
    @Mock private SellerGrpcClient sellerGrpcClient;
    @Mock private InvoiceGrpcClient invoiceGrpcClient;
    @Mock private BuyerGrpcClient buyerGrpcClient;
    @Mock private ShippingGrpcClient shippingGrpcClient;
    @Mock private CheckoutValidator checkoutValidator;
    @Mock private OrderService orderService;
    @Mock private StringRedisTemplate redisTemplate;
    @Mock private ValueOperations<String, String> valueOperations;

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
                                .setShippingMethod(UBER)
                                .setGoodsType(TEMP)
                                .setAvailable(true)
                                .setSenderPostalCode("106")
                                .setSenderCity("臺北市")
                                .setSenderDistrict("大安區")
                                .build())
                        // 賣家同時開通 3001：陰性案例（preview(3001) 不得組 probe）才走得到閘門那一行，
                        // 否則會在更前面的 requireSellerOnboarded 就被擋掉 ⇒ 那條測試對本案零鑑別力。
                        .addConfigs(SellerShippingConfigItem.newBuilder()
                                .setShippingMethod("3001")
                                .setGoodsType(TEMP)
                                .setAvailable(true)
                                .setSenderPostalCode("106")
                                .setSenderCity("臺北市")
                                .setSenderDistrict("大安區")
                                .build())
                        .build());
        when(sellerGrpcClient.getSellerMeta(anyLong())).thenReturn(SellerMeta.newBuilder()
                .setSellerName("測試賣家")
                .setPayuniMerId("HAWK" + STORE_ID)
                .setCanAcceptOrders(true)
                .build());
        // 🔴 買家地址簿是 dropoff 三欄的唯一來源。值刻意與「一整串 shipping_address」不同，
        //    這樣「有沒有真的走 buyer-service」驗得出來（若改成反解那串字串，縣市會是別的值）。
        when(buyerGrpcClient.getAddress(anyLong(), anyLong())).thenReturn(AddressInfo.newBuilder()
                .setId(ADDRESS_ID)
                .setReceiverName("王小明")
                .setReceiverPhone("0912345678")
                .setCity("臺北市")
                .setDistrict("信義區")
                .setDetailAddress("市府路1號5樓")
                .build());

        when(checkoutValidator.validate(anyList(), any()))
                .thenReturn(new ValidationResult(true, java.util.Set.of(TEMP), List.of(), List.of(), 0));
        when(checkoutValidator.lookupSingleOption(anyList(), any(), any(), any(), any()))
                .thenReturn(new AvailableOption(
                        ShippingMethod.UBER_INSTANT, ShippingTemperature.CHILLED,
                        "Uber 即時配送（冷藏）", 250L, 250L, ShippingFeePayer.BUYER.value(),
                        0L, false, "dqt_revalidated"));
        when(checkoutValidator.checkFreeShipping(anyList(), anyString(), anyString(), any()))
                .thenReturn(new FreeShippingResult(false, ShippingFeePayer.BUYER.value(), 0L));
        when(checkoutValidator.acceptUberQuote(any())).thenReturn(allowedDecision());

        when(redisTemplate.opsForValue()).thenReturn(valueOperations);
        when(orderService.createOrderFromCheckout(any())).thenReturn(ORDER_NO);
        when(invoiceGrpcClient.validateOrderInvoiceInput(any()))
                .thenReturn(ValidateOrderInvoiceInputInternalResponse.newBuilder().setCode(0).build());
        when(invoiceGrpcClient.resolveOrderInvoiceMode(any())).thenReturn(null);

        service = new CheckoutService(productGrpcClient, sellerGrpcClient, buyerGrpcClient,
                shippingGrpcClient, null, invoiceGrpcClient, checkoutValidator, orderService,
                new OrderPolicyResolver(), redisTemplate);

        OrderPolicySnapshot policy = new OrderPolicyResolver().resolve(UBER);
        when(valueOperations.getAndDelete(anyString())).thenReturn(
                service.buildPriceSnapshot(1000L, 250L, 0L, 1250L, false, false, "HAWK123", policy));
    }

    private static UberOrderEligibilityGuard.Decision allowedDecision() {
        return new UberOrderEligibilityGuard(
                probe -> UberOrderEligibilityGuard.Assessment.eligible(
                        "dqt_revalidated", "250", "TWD", "2026-09-12T05:00:00Z"))
                .evaluate(UberOrderEligibilityGuard.Context.of(UBER, probeForAssert()));
    }

    private static UberOrderEligibilityGuard.Probe probeForAssert() {
        return UberOrderEligibilityGuard.Probe.forCheckout(
                BUYER_ID, STORE_ID, TEMP, "王小明", "0912345678",
                "臺北市", "信義區", "市府路1號5樓", List.of(), "");
    }

    private UberOrderEligibilityGuard.Context capturedContext() {
        ArgumentCaptor<UberOrderEligibilityGuard.Context> captor =
                ArgumentCaptor.forClass(UberOrderEligibilityGuard.Context.class);
        verify(checkoutValidator).lookupSingleOption(anyList(), any(), any(), any(), captor.capture());
        return captor.getValue();
    }

    private static List<CheckoutItem> items() {
        return List.of(CheckoutItem.newBuilder()
                .setSkuId(SKU_ID).setProductId("9001").setQuantity("2").build());
    }

    private CheckoutConfirmRequest.Builder confirmRequest() {
        return CheckoutConfirmRequest.newBuilder()
                .setBuyerId(BUYER_ID)
                .setCheckoutId(CHECKOUT_ID)
                .addAllItems(items())
                .setShippingMethod(UBER)
                .setDeliveryTemp(TEMP)
                .setReceiverZipCode("110")
                .setRecipientName("王小明")
                .setRecipientPhone("0912345678")
                .setShippingAddress("臺北市信義區市府路1號5樓")
                .setAddressId(ADDRESS_ID)
                .setUberQuoteId(QUOTE_ID);
    }

    // ===== preview =====

    @Test
    @DisplayName("🔴 preview(3003) 帶 address_id：閘門收到的是完整 probe，不是 methodOnly")
    void previewFeedsFullProbe() {
        service.preview(items(), UBER, TEMP, "110", "", "", BUYER_ID, false, ADDRESS_ID, QUOTE_ID);

        UberOrderEligibilityGuard.Context context = capturedContext();
        assertThat(context.shippingMethodValue()).isEqualTo(UBER);
        assertThat(context.probe())
                .as("probe 為 null ⇒ 閘門 deny-by-default ⇒ 買家選得到卻一律 9503")
                .isNotNull();
        // dropoff 三欄逐欄比對：值來自 buyer-service，不是從 shipping_address 反解。
        assertThat(context.probe().county()).isEqualTo("臺北市");
        assertThat(context.probe().district()).isEqualTo("信義區");
        assertThat(context.probe().detailAddress()).isEqualTo("市府路1號5樓");
        assertThat(context.probe().receiverName()).isEqualTo("王小明");
        assertThat(context.probe().quoteId()).isEqualTo(QUOTE_ID);
        assertThat(context.probe().accept())
                .as("preview 不得落任何狀態")
                .isFalse();
        assertThat(context.probe().items()).hasSize(1);
    }

    @Test
    @DisplayName("🔴 preview(3003) 沒帶 address_id：退回 methodOnly（誠實拒絕，不得硬湊 dropoff）")
    void previewWithoutAddressIdFallsBackToMethodOnly() {
        service.preview(items(), UBER, TEMP, "110", "", "", BUYER_ID, false, "", QUOTE_ID);

        assertThat(capturedContext().probe()).isNull();
        verify(buyerGrpcClient, never()).getAddress(anyLong(), anyLong());
    }

    @Test
    @DisplayName("地址查不到（已被買家刪除）：一樣退回 methodOnly，不是拿空字串組 dropoff")
    void previewWithMissingAddressFallsBackToMethodOnly() {
        when(buyerGrpcClient.getAddress(anyLong(), anyLong())).thenReturn(null);

        service.preview(items(), UBER, TEMP, "110", "", "", BUYER_ID, false, ADDRESS_ID, QUOTE_ID);

        assertThat(capturedContext().probe()).isNull();
    }

    @Test
    @DisplayName("🔴 陰性：preview(3001) 即使帶了 address_id 也是 methodOnly——閘門不介入其他配送方式")
    void previewForNonUberNeverBuildsProbe() {
        service.preview(items(), "3001", TEMP, "110", "", "", BUYER_ID, false, ADDRESS_ID, QUOTE_ID);

        UberOrderEligibilityGuard.Context context = capturedContext();
        assertThat(context.shippingMethodValue()).isEqualTo("3001");
        assertThat(context.probe()).isNull();
        verify(buyerGrpcClient, never()).getAddress(anyLong(), anyLong());
    }

    // ===== shippingFee =====

    @Test
    @DisplayName("🔴 shippingFee(3003) 恆為 methodOnly：級距費率表不承載會過期的即時報價")
    void shippingFeeStaysMethodOnlyForUber() {
        service.shippingFee(items(), UBER, TEMP, "110", "");

        assertThat(capturedContext().probe()).isNull();
    }

    // ===== confirm =====

    @Test
    @DisplayName("🔴 confirm(3003)：建單成功後把報價 CAS 成 CHECKOUT_ACCEPTED（accept=true ＋ order_no）")
    void confirmAcceptsTheQuoteAfterOrderCreation() {
        service.confirm(confirmRequest().build());

        ArgumentCaptor<UberOrderEligibilityGuard.Probe> captor =
                ArgumentCaptor.forClass(UberOrderEligibilityGuard.Probe.class);
        verify(checkoutValidator).acceptUberQuote(captor.capture());
        UberOrderEligibilityGuard.Probe accepted = captor.getValue();

        assertThat(accepted.accept()).isTrue();
        assertThat(accepted.orderNo()).isEqualTo(ORDER_NO);
        // 🔴 鎖的是**重驗回來的**那一筆，不是買家送上來的那一筆（重驗允許重報，66 §7）。
        assertThat(accepted.quoteId())
                .isEqualTo("dqt_revalidated")
                .isNotEqualTo(QUOTE_ID);
        // 沿用同一份 dropoff：地址在這兩步之間被重組，鎖到的會是另一個 payload 的報價。
        assertThat(accepted.county()).isEqualTo("臺北市");
        assertThat(accepted.district()).isEqualTo("信義區");
    }

    @Test
    @DisplayName("🔴 陰性：confirm(3001) 不碰報價——不得對非 Uber 訂單做 CAS")
    void confirmDoesNotAcceptForNonUber() {
        when(checkoutValidator.lookupSingleOption(anyList(), any(), any(), any(), any()))
                .thenReturn(new AvailableOption(
                        ShippingMethod.TAQBIN, ShippingTemperature.CHILLED,
                        "黑貓宅配（冷藏）", 250L, 250L, ShippingFeePayer.BUYER.value(), 0L, false));
        when(shippingGrpcClient.getSellerShippingConfig(anyLong()))
                .thenReturn(GetSellerShippingConfigResponse.newBuilder()
                        .setCode(0)
                        .addConfigs(SellerShippingConfigItem.newBuilder()
                                .setShippingMethod("3001")
                                .setGoodsType(TEMP)
                                .setAvailable(true)
                                .setSenderPostalCode("106")
                                .build())
                        .build());
        OrderPolicySnapshot policy = new OrderPolicyResolver().resolve("3001");
        when(valueOperations.getAndDelete(anyString())).thenReturn(
                service.buildPriceSnapshot(1000L, 250L, 0L, 1250L, false, false, "HAWK123", policy));

        service.confirm(confirmRequest().setShippingMethod("3001").build());

        verify(checkoutValidator, never()).acceptUberQuote(any());
        verify(orderService).createOrderFromCheckout(any());
    }

    @Test
    @DisplayName("🔴 鎖定失敗（含 price_changed）：取消訂單 ＋ 拋 9503，不得留下一張建不出單的訂單")
    void confirmCancelsTheOrderWhenAcceptFails() {
        when(checkoutValidator.acceptUberQuote(any())).thenReturn(
                new UberOrderEligibilityGuard(
                        probe -> UberOrderEligibilityGuard.Assessment.unavailable(
                                UberOrderEligibilityGuard.ProviderReason.UBER_QUOTE_PRICE_CHANGED))
                        .evaluate(UberOrderEligibilityGuard.Context.of(UBER, probeForAssert())));

        Throwable thrown = org.assertj.core.api.Assertions
                .catchThrowable(() -> service.confirm(confirmRequest().build()));

        assertThat(thrown)
                .isInstanceOf(io.github.will413028.hawkr.order.exception.OrderBusinessException.class);
        assertThat(((io.github.will413028.hawkr.order.exception.OrderBusinessException) thrown).code())
                .isEqualTo(9503);
        verify(orderService).cancelOrder(eq(BUYER_ID), eq(ORDER_NO));
        verify(orderService).releaseStockAfterCancel(eq(ORDER_NO));
    }
}
