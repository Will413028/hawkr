package io.github.will413028.hawkr.order.checkout;

import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.common.utils.code.ShippingFeePayer;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.common.utils.code.ShippingTemperature;
import io.github.will413028.hawkr.order.checkout.CheckoutValidator.AvailableOption;
import io.github.will413028.hawkr.order.checkout.CheckoutValidator.FreeShippingResult;
import io.github.will413028.hawkr.order.checkout.CheckoutValidator.ValidationResult;
import io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient;
import io.github.will413028.hawkr.order.config.OrderConfig;
import io.github.will413028.hawkr.order.entity.OrderEntity;
import io.github.will413028.hawkr.order.grpc.CheckoutConfirmRequest;
import io.github.will413028.hawkr.order.grpc.CheckoutItem;
import io.github.will413028.hawkr.order.grpc.client.BuyerGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.InvoiceGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.ProductGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.ShippingGrpcClient;
import io.github.will413028.hawkr.order.repository.OrderOutboxMapper;
import io.github.will413028.hawkr.order.mq.OrderEventPublisher;
import io.github.will413028.hawkr.order.repository.AutoRefundTaskRepository;
import io.github.will413028.hawkr.order.repository.CartCleanupRetryTaskRepository;
import io.github.will413028.hawkr.order.repository.FinancialLogRepository;
import io.github.will413028.hawkr.order.repository.OrderInvoiceSnapshotRepository;
import io.github.will413028.hawkr.order.repository.OrderItemRepository;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import io.github.will413028.hawkr.order.repository.RefundRequestRepository;
import io.github.will413028.hawkr.order.repository.StockTxRecordRepository;
import io.github.will413028.hawkr.order.service.OrderCompletionTxHandler;
import io.github.will413028.hawkr.order.service.OrderCreationTxHandler;
import io.github.will413028.hawkr.order.service.OrderService;
import io.github.will413028.hawkr.order.service.OrderNoGenerator;
import io.github.will413028.hawkr.invoice.grpc.ValidateOrderInvoiceInputInternalResponse;
import io.github.will413028.hawkr.product.grpc.BatchGetProductForCheckoutResponse;
import io.github.will413028.hawkr.product.grpc.ProductForCheckoutInfo;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import io.github.will413028.hawkr.shipping.grpc.GetSellerShippingConfigResponse;
import io.github.will413028.hawkr.shipping.grpc.SellerShippingConfigItem;
import io.github.will413028.hawkr.order.grpc.client.CartGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.AfterSalesGrpcClient;
import org.junit.jupiter.api.BeforeAll;
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
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * <b>訂單政策快照的跨層契約守衛（R3）—— 不需 Docker，跑在一般 CI。</b>
 *
 * <h2>它守的是哪一段</h2>
 * B1 的失效不是「某一層寫錯」，而是<b>四段接力中任何一棒掉了都完全無聲</b>：
 * <pre>
 *   ① OrderPolicyResolver.resolve      算出政策
 *   ② CheckoutService.buildPriceSnapshot  放進 Redis 快照（JSON 字串）
 *   ③ CheckoutService.extractPolicySnapshot  confirm 讀回
 *   ④ OrderService.createOrderFromCheckout → OrderCreationTxHandler → OrderEntity
 * </pre>
 * 掉棒的症狀是 {@code OrderEntity} 的政策欄為 null；而 MyBatis-Plus 的
 * {@code insertStrategy=NOT_NULL} 會把 null 欄位<b>整個排除在 INSERT 之外</b>
 * ⇒ DB DEFAULT 生效 ⇒ 每張新單都拿到 {@code v0-legacy}／{@code SELLER_MANAGED}，
 * 而且沒有任何錯誤、沒有任何 log。這正是 Independent QA 找到的第一個 Foundation blocker。
 *
 * <h2>為什麼既有的兩支測試補不上這個洞</h2>
 * <ul>
 *   <li>{@code OrderPolicyPersistenceOnCreationPathsTest} 直接把 policy 物件餵進 handler
 *       ——<b>整段跳過 ②③</b>，快照掉欄它照樣全綠。</li>
 *   <li>{@code CheckoutConfirmInvoiceIntakeTest} 把 {@code OrderService} mock 掉
 *       ——<b>斷在 ④ 之前</b>，{@code params.policySnapshot()} 有沒有真的傳給 handler 沒人驗。</li>
 * </ul>
 * 兩支各覆蓋一半、中間那條縫兩邊都綠——那是「自己跟自己對」的典型形狀。
 * 本檔用<b>真的</b> {@code OrderService} ＋ 真的 {@code OrderCreationTxHandler}，
 * 一路走到 {@code orderRepository.insert} 的那個 entity 上斷言。
 *
 * <h2>與真 PostgreSQL IT 的分工</h2>
 * {@code OrderPolicySnapshotPersistenceIT}（Testcontainers ＋ 完整 Flyway）仍是最強證據，
 * 但它被 {@code excludedGroups=integration} 排除在 CI 之外，而理由是基建面的：
 * {@code java-tests.yml} 檔頭寫著 {@code postgres:15/17-alpine 未鏡像到 ghcr}。
 * ⇒ 兩層並存：<b>本檔守跨層傳遞（CI 每次跑）、IT 守 DB 行為（本地／未來 CI 跑）</b>。
 * 本檔驗不到的是「MyBatis 對 null 欄位的實際 INSERT 行為」——那是 IT 的職責，
 * 這裡只能驗它的<b>前提</b>：送進 mapper 的 entity 五欄皆非 null。
 *
 * <h2>期望值一律硬寫字面值</h2>
 * 不從 {@code OrderPolicyResolver} 取期望值——那會讓斷言跟著被測物一起變，
 * 把 +400 改成 +399 測試照樣綠。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class OrderPolicySnapshotChainContractTest {

    private static final long STORE_ID = 2685914682L;
    private static final long BUYER_ID = 2623900520L;
    private static final String SKU_ID = "5001";
    private static final String CHECKOUT_ID = "CHK-POLICY-CHAIN";
    private static final String METHOD = ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value();
    private static final String TEMP = ShippingTemperature.CHILLED.value();

    @Mock private ProductGrpcClient productGrpcClient;
    @Mock private SellerGrpcClient sellerGrpcClient;
    @Mock private InvoiceGrpcClient invoiceGrpcClient;
    @Mock private BuyerGrpcClient buyerGrpcClient;
    @Mock private ShippingGrpcClient shippingGrpcClient;
    @Mock private CheckoutValidator checkoutValidator;
    @Mock private StringRedisTemplate redisTemplate;
    @Mock private ValueOperations<String, String> valueOperations;
    @Mock private OrderRepository orderRepository;

    private OrderService orderService;
    private OrderCreationTxHandler txHandler;
    private CheckoutService service;

    @BeforeAll
    static void initTsid() {
        try {
            io.github.will413028.hawkr.common.utils.TsidUtil.init(300, 0);
        } catch (Exception ignored) {
            // 同一個 JVM 內別的測試已經初始化過（surefire 不 fork per class）
        }
    }

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

        // 發票前置驗證預設放行（code=0）。要測「被擋下」的分支就在該支測試裡改 stub。
        when(invoiceGrpcClient.validateOrderInvoiceInput(any()))
                .thenReturn(ValidateOrderInvoiceInputInternalResponse.newBuilder().setCode(0).build());

        // 🔴 真 OrderCreationTxHandler ＋ 真 OrderService：本測試要驗的正是它們之間那條縫。
        OrderConfig orderConfig = mock(OrderConfig.class);
        when(orderConfig.getPaymentDeadlineSeconds()).thenReturn(259200L);
        OrderNoGenerator orderNoGenerator = mock(OrderNoGenerator.class);
        when(orderNoGenerator.generateOrderNo()).thenReturn("O20260910001");
        txHandler = new OrderCreationTxHandler(
                orderRepository, mock(OrderItemRepository.class), mock(StockTxRecordRepository.class),
                mock(OrderInvoiceSnapshotRepository.class), orderNoGenerator, orderConfig);
        orderService = new OrderService(
                orderRepository, mock(OrderItemRepository.class), mock(RefundRequestRepository.class),
                mock(FinancialLogRepository.class), mock(StockTxRecordRepository.class),
                mock(CartCleanupRetryTaskRepository.class), mock(AutoRefundTaskRepository.class),
                orderNoGenerator, stockDeductingProductClient(), mock(CartGrpcClient.class),
                orderConfig, mock(OrderEventPublisher.class), mock(SellerGrpcClient.class),
                mock(BuyerGrpcClient.class), mock(AfterSalesGrpcClient.class),
                mock(OrderCompletionTxHandler.class), txHandler, mock(OrderOutboxMapper.class),
                new OrderPolicyResolver(), new UberOrderEligibilityGuard(),
                new io.micrometer.core.instrument.simple.SimpleMeterRegistry());

        service = new CheckoutService(productGrpcClient, sellerGrpcClient, buyerGrpcClient,
                shippingGrpcClient, null, invoiceGrpcClient, checkoutValidator, orderService, new OrderPolicyResolver(), redisTemplate);
    }


    /**
     * 🔴 <b>主測試：四段接力一次走完，在 INSERT 的那個 entity 上斷言。</b>
     *
     * <p>快照用<b>產線那一支</b> {@code buildPriceSnapshot} 組（不手寫 JSON），
     * 再原樣經過 Redis 的 String 往返餵回 confirm——那就是真實路徑，
     * Redis 存的本來就是字串。
     */
    @Test
    @DisplayName("preview → Redis 快照 → confirm → 建單：政策五欄一路到 INSERT 的 entity 上")
    void policySurvivesTheWholeChainFromPreviewToInsert() {
        OrderEntity inserted = confirmAndCaptureInsertedOrder(METHOD);

        // 🔴 五欄逐一硬寫字面值。任何一欄為 null ⇒ MyBatis insertStrategy=NOT_NULL 會把它
        //    排除在 INSERT 之外 ⇒ DB DEFAULT 生效 ⇒ B1 原樣重演，而且完全無聲。
        assertThat(inserted.getCommerceModel())
                .as("commerce_model 掉棒 ⇒ INSERT 省略該欄 ⇒ 吃 DB DEFAULT").isEqualTo("SELLER_DIRECT");
        assertThat(inserted.getCollectionModel())
                .as("collection_model 掉棒 ⇒ 收款主體判定失去權威來源").isEqualTo("SELLER_MERCHANT");
        assertThat(inserted.getFulfillmentMode())
                .as("3002 是平台物流；吃到 DB DEFAULT 會變成 SELLER_MANAGED").isEqualTo("PLATFORM_LOGISTICS");
        assertThat(inserted.getInvoiceModel()).isEqualTo("SELLER_TO_BUYER");
        assertThat(inserted.getPolicyVersion())
                .as("policy_version 掉棒 ⇒ 新單與 DB DEFAULT 的 v0-legacy 無法區分").isEqualTo("v1-foundation");
    }

    /**
     * 陰性對照：換一個配送方式，結果必須<b>不同</b>。
     *
     * <p>沒有這一支的話，「政策一律回同一組常數」也會讓上面那支全綠——
     * 那時測到的是「有值」而不是「值對」。
     */
    @Test
    @DisplayName("陰性對照：3005 商家自送走完同一條鏈得到 SELLER_MANAGED")
    void merchantDeliveryTakesADifferentPathThroughTheSameChain() {
        OrderEntity inserted = confirmAndCaptureInsertedOrder(ShippingMethod.MERCHANT_DELIVERY.value());
        assertThat(inserted.getFulfillmentMode()).isEqualTo("SELLER_MANAGED");
        assertThat(inserted.getPolicyVersion()).isEqualTo("v1-foundation");
    }

    /**
     * 🔴 <b>fail-closed 的那一半</b>：快照缺政策時<b>不得</b>靜默用預設值建單。
     *
     * <p>B1 的驗收條件裡「不得用 provider heuristic 事後重新推導」與這一條是同一件事的兩面：
     * 讀不到權威快照時唯一正確的行為是中止，而不是猜一組看起來合理的值填進去。
     */
    @Test
    @DisplayName("快照缺政策區塊 ⇒ confirm 中止，不得用預設值建單")
    void missingPolicyInSnapshotAbortsInsteadOfDefaulting() {
        when(valueOperations.getAndDelete(anyString()))
                .thenReturn("{\"subtotal\":1000,\"shippingFee\":60,\"totalAmount\":1060,\"paymentMerId\":\"HAWK123\"}");

        try {
            service.confirm(baseRequest(METHOD).build());
            org.junit.jupiter.api.Assertions.fail("快照缺政策卻建單成功——那正是 fail-closed 要擋的");
        } catch (io.github.will413028.hawkr.order.exception.OrderBusinessException expected) {
            // 期望路徑
        }
        verify(orderRepository, org.mockito.Mockito.never()).insert(any());
    }

    // ==================== harness ====================

    private OrderEntity confirmAndCaptureInsertedOrder(String shippingMethod) {
        // setUp 的預設 stub 綁死 3002；陰性對照要換配送方式，這裡覆寫成當次的那一個。
        // （賣家開通與可選方案都是 confirm 的前置檢查，不是本測試要驗的東西。）
        when(shippingGrpcClient.getSellerShippingConfig(anyLong()))
                .thenReturn(GetSellerShippingConfigResponse.newBuilder()
                        .setCode(0)
                        .addConfigs(SellerShippingConfigItem.newBuilder()
                                .setShippingMethod(shippingMethod)
                                .setGoodsType(TEMP)
                                .setAvailable(true)
                                // 宅配／自送會回頭讀寄件郵遞區號（3002 走不到這條路徑）
                                .setSenderPostalCode("100")
                                .build())
                        .build());
        when(checkoutValidator.lookupSingleOption(anyList(), any(), any(), any(), any()))
                .thenReturn(new AvailableOption(
                        ShippingMethod.fromValue(shippingMethod), ShippingTemperature.CHILLED,
                        "測試方案", 60L, 60L, ShippingFeePayer.BUYER.value(), 0L, false));

        OrderPolicySnapshot policy = new OrderPolicyResolver().resolve(shippingMethod);
        // ②：產線的快照組裝，不手寫 JSON——欄名改了要在這裡就爆
        String snapshot = service.buildPriceSnapshot(
                1000L, 60L, 0L, 1060L, false, false, "HAWK" + STORE_ID, policy);
        // Redis 的往返就是字串往返
        when(valueOperations.getAndDelete(anyString())).thenReturn(snapshot);

        service.confirm(baseRequest(shippingMethod).build());

        ArgumentCaptor<OrderEntity> captor = ArgumentCaptor.forClass(OrderEntity.class);
        verify(orderRepository).insert(captor.capture());
        return captor.getValue();
    }

    private CheckoutConfirmRequest.Builder baseRequest(String shippingMethod) {
        CheckoutConfirmRequest.Builder b = CheckoutConfirmRequest.newBuilder()
                .setBuyerId(BUYER_ID)
                .setCheckoutId(CHECKOUT_ID)
                .addItems(CheckoutItem.newBuilder()
                        .setSkuId(SKU_ID).setProductId("9001").setQuantity("2").build())
                .setShippingMethod(shippingMethod)
                .setDeliveryTemp(TEMP)
                .setRecipientName("王小明")
                .setRecipientPhone("0912345678");
        // 超商取貨要門市代號、宅配／自送要地址——兩者是不同的必填集合。
        if (ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value().equals(shippingMethod)) {
            b.setCvsStoreId("252698").setCvsStoreName("測試門市");
        } else {
            // 位址驗證只收 CJK ＋ 半形數字 ＋ - ＋ 之（InputValidator.ADDRESS_ALLOWED_CHARS）
            b.setShippingAddress("台北市中正區忠孝東路一段1號");
        }
        return b;
    }

    /**
     * 真 {@code OrderService} 會在建單前扣庫存；本測試不驗庫存，給一個恆成功的 stub。
     * 🔴 但不是 {@code mock(...)} 的裸物件——那會回 null 而 OrderService 直接 NPE，
     * 失敗訊息會指向庫存而不是本測試要驗的政策鏈。
     */
    private ProductGrpcClient stockDeductingProductClient() {
        ProductGrpcClient c = mock(ProductGrpcClient.class);
        when(c.deductStock(anyString(), anyList()))
                .thenReturn(io.github.will413028.hawkr.product.grpc.StockOperationResponse.newBuilder()
                        .setStatus(io.github.will413028.hawkr.common.utils.code.StockOperationStatus.SUCCESS.value())
                        .build());
        return c;
    }
}
