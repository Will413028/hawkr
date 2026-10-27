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
import io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient;
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
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * {@code CheckoutConfirm} 端的發票採集（58 號 §6.1.2(b)、§7.1.2 步驟 1）。
 *
 * <p>🔴 <b>本檔第一支測試是「不要打爛線上 App」的機械保證</b>：
 * {@code hawkr_buyer} 已上架、直接呼叫 {@code /app/checkout/confirm}，且它送出的 body
 * 完全沒有 {@code invoice} 這個欄位
 * （{@code lib/features/cart/presentation/widgets/checkout_payment_sheet.dart}）。
 * 把 invoice 區塊改成必填 ⇒ {@link #confirmWithoutInvoiceBlockStillCreatesTheOrder} 轉紅。
 *
 * <p>fixture 走 7-11 店到店（3002）：{@code getSenderPostalCode} 對超商短路回空字串、
 * {@code resolveRoute} 固定 DOMESTIC，省掉與本案無關的郵遞區號推導；
 * Redis 快照由產線的 {@code buildPriceSnapshot} 組出（含政策五欄）——confirm 對缺政策的快照 fail-closed。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class CheckoutConfirmInvoiceIntakeTest {

    private static final long STORE_ID = 2685914682L;
    private static final long BUYER_ID = 2623900520L;
    private static final String SKU_ID = "5001";
    private static final String CHECKOUT_ID = "CHK-INVOICE-1";
    private static final String METHOD = ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value();
    private static final String TEMP = ShippingTemperature.CHILLED.value();

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
                                .setShippingMethod(METHOD)
                                .setGoodsType(TEMP)
                                .setAvailable(true)
                                .build())
                        .build());
        when(sellerGrpcClient.getSellerMeta(anyLong())).thenReturn(SellerMeta.newBuilder()
                .setSellerName("測試賣家")
                .setPayuniMerId("HAWK" + STORE_ID)
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
                shippingGrpcClient, null, invoiceGrpcClient, checkoutValidator, orderService, new OrderPolicyResolver(), redisTemplate);

        // 🔴 快照用**產線那一支** buildPriceSnapshot 組，不手寫 JSON。
        //    舊版這裡餵的是 "1"（舊格式哨兵），而 confirm 現在對「快照沒有政策五欄」一律 fail-closed
        //    ——那正是要防的東西，所以測試必須餵一份真的快照，不是繞過檢查。
        OrderPolicySnapshot policy =
                new OrderPolicyResolver().resolve(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value());
        when(valueOperations.getAndDelete(anyString())).thenReturn(
                service.buildPriceSnapshot(1000L, 60L, 0L, 1060L, false, false, "HAWK123", policy));
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
                .setCvsStoreName("測試門市");
    }

    private OrderService.CreateOrderParams confirmAndCaptureParams(CheckoutConfirmRequest request) {
        service.confirm(request);
        ArgumentCaptor<OrderService.CreateOrderParams> captor =
                ArgumentCaptor.forClass(OrderService.CreateOrderParams.class);
        verify(orderService).createOrderFromCheckout(captor.capture());
        return captor.getValue();
    }

    /**
     * 🔴 <b>「preview 算出的政策要原封不動傳到建單」——這一行本身的守衛。</b>
     *
     * <p>BLOCKER 1 的第一句驗收就是「preview → order creation 不得遺失」，而
     * {@code CheckoutService.confirm} 把 {@code policySnapshot} 放進 {@code CreateOrderParams}
     * 的那一行<b>就是</b>那個「不遺失」的實體。
     *
     * <p>⚠️ 這一格在 2026-09-10 的對抗驗證中被指出**零守衛**，並實測坐實：
     * 把那一行換成寫死的 {@code SELLER_MANAGED} 政策，order-service 的 <b>287 支測試全綠</b>
     * ——既有的兩支新測試都繞過它（一支直呼 {@code OrderCreationTxHandler}、
     * 一支直接呼叫六個 setter），而 DB 的
     * {@code chk_orders_fulfillment_mode_consistent} 只約束 {@code shipping_method='3005'}
     * ⇒ 「3002 配 SELLER_MANAGED」是合法列，PostgreSQL 不會叫。
     *
     * <p>期望值一律硬寫字面字串——用 {@code new OrderPolicyResolver().resolve(...)} 當期望值，
     * resolver 被改壞時測試會跟著一起變，那種寫法零鑑別力。
     */
    @Test
    @DisplayName("🔴 confirm 必須把 preview 凍結的政策原封不動傳進建單參數（BLOCKER 1 的那一行）")
    void confirmPassesTheFrozenPolicyIntoOrderCreation() {
        OrderService.CreateOrderParams params = confirmAndCaptureParams(baseRequest().build());

        assertThat(params.policySnapshot())
                .as("政策快照必須傳下去，null 代表這一段被拿掉了")
                .isNotNull();
        assertThat(params.policySnapshot().fulfillmentMode())
                .as("本測試的配送方式是 3002（平台物流）——寫死成 SELLER_MANAGED 會在這裡被抓到")
                .isEqualTo("PLATFORM_LOGISTICS");
        assertThat(params.policySnapshot().commerceModel()).isEqualTo("SELLER_DIRECT");
        assertThat(params.policySnapshot().collectionModel()).isEqualTo("SELLER_MERCHANT");
        assertThat(params.policySnapshot().invoiceModel()).isEqualTo("SELLER_TO_BUYER");
        assertThat(params.policySnapshot().policyVersion()).isEqualTo("v1-foundation");
    }

    @Test
    @DisplayName("🔴 完全不帶 invoice 區塊（線上 App 的實際行為）→ 照常建單，快照走 USE_BUYER_DEFAULT")
    void confirmWithoutInvoiceBlockStillCreatesTheOrder() {
        OrderService.CreateOrderParams params = confirmAndCaptureParams(baseRequest().build());

        assertThat(params.invoiceSnapshot()).isNotNull();
        assertThat(params.invoiceSnapshot().invoiceKind()).isEqualTo(InvoiceDocumentKind.B2C.value());
        assertThat(params.invoiceSnapshot().carrierSelectionMode())
                .isEqualTo(InvoiceCarrierSelectionMode.USE_BUYER_DEFAULT.value());
    }

    @Test
    @DisplayName("帶統編 → 快照為 B2B ＋ NONE，抬頭與統編原樣落到建單參數")
    void confirmWithTaxIdProducesB2bSnapshot() {
        OrderService.CreateOrderParams params = confirmAndCaptureParams(baseRequest()
                .setInvoice(CheckoutInvoiceRequest.newBuilder()
                        .setBuyerTaxId("12345678")
                        .setBuyerTitle("範例生鮮股份有限公司")
                        .setNotifyEmail("buyer@example.com"))
                .build());

        assertThat(params.invoiceSnapshot().invoiceKind()).isEqualTo(InvoiceDocumentKind.B2B.value());
        assertThat(params.invoiceSnapshot().carrierSelectionMode())
                .isEqualTo(InvoiceCarrierSelectionMode.NONE.value());
        assertThat(params.invoiceSnapshot().buyerTaxId()).isEqualTo("12345678");
        assertThat(params.invoiceSnapshot().buyerTitle()).isEqualTo("範例生鮮股份有限公司");
        assertThat(params.invoiceSnapshot().notifyEmail()).isEqualTo("buyer@example.com");
    }

    @Test
    @DisplayName("帶常用資料 id → EXPLICIT_PREFERENCE，id 原樣帶下去（歸屬驗證是 invoice-service 的事）")
    void confirmWithProfileIdProducesExplicitPreference() {
        OrderService.CreateOrderParams params = confirmAndCaptureParams(baseRequest()
                .setInvoice(CheckoutInvoiceRequest.newBuilder().setProfileId("2649605557"))
                .build());

        assertThat(params.invoiceSnapshot().carrierSelectionMode())
                .isEqualTo(InvoiceCarrierSelectionMode.EXPLICIT_PREFERENCE.value());
        assertThat(params.invoiceSnapshot().buyerInvoicePreferenceId()).isEqualTo(2649605557L);
    }

    @Test
    @DisplayName("發票輸入不合法 → 在建單之前就以 9306 失敗，不得先建單再回錯")
    void invalidInvoiceInputFailsBeforeOrderCreation() {
        CheckoutConfirmRequest request = baseRequest()
                .setInvoice(CheckoutInvoiceRequest.newBuilder()
                        .setBuyerTaxId("12345678").setBuyerTitle("X").setDonationCode("5238"))
                .build();

        assertThatThrownBy(() -> service.confirm(request))
                .isInstanceOf(OrderBusinessException.class)
                .satisfies(e -> assertThat(((OrderBusinessException) e).code())
                        .isEqualTo(GrpcResponseCode.INVOICE_TARGET_CONFLICT.code()));

        verify(orderService, org.mockito.Mockito.never()).createOrderFromCheckout(any());
    }

    /**
     * §6.2.4 order ③：invoice-service 回非 0 業務碼時**必須擋下建單**。
     *
     * <p>今天真正會走到這裡的只有 9326（待審接單雙上限）——其餘四格都要買家送出
     * {@code invoice} 區塊才觸發，而已上架 App 不帶、網頁 B02 尚未實作。
     *
     * <p>🔴 這一支守的是「有沒有真的接上」，不是「helper 寫得對不對」：接線漏掉時
     * 訂單會照常建出去，而沒有任何東西會報錯。
     */
    @Test
    void confirmIsBlockedWhenInvoiceServiceRejects() {
        when(invoiceGrpcClient.validateOrderInvoiceInput(any()))
                .thenReturn(ValidateOrderInvoiceInputInternalResponse.newBuilder()
                        .setCode(GrpcResponseCode.SELLER_INVOICE_INTAKE_LIMIT_REACHED.code())
                        .setMessage("待審期間接單已達上限")
                        .build());

        assertThatThrownBy(() -> service.confirm(baseRequest().build()))
                .isInstanceOf(OrderBusinessException.class)
                .extracting(e -> ((OrderBusinessException) e).code())
                .isEqualTo(GrpcResponseCode.SELLER_INVOICE_INTAKE_LIMIT_REACHED.code());
        verify(orderService, never()).createOrderFromCheckout(any());
    }

    /**
     * invoice-service 不可用（client 回 null）時**放行**。
     *
     * <p>fail-closed 會讓 invoice-service 變成整條金流的硬依賴——發票正確性的保護在
     * {@code order.paid} 那一側，不在這一格。
     */
    @Test
    void confirmProceedsWhenInvoiceServiceUnavailable() {
        when(invoiceGrpcClient.validateOrderInvoiceInput(any())).thenReturn(null);

        service.confirm(baseRequest().build());

        verify(orderService).createOrderFromCheckout(any());
    }
}
