package io.github.will413028.hawkr.order.service;

import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.order.checkout.OrderPolicyResolver;
import io.github.will413028.hawkr.order.checkout.UberOrderEligibilityGuard;
import io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient;
import io.github.will413028.hawkr.order.config.OrderConfig;
import io.github.will413028.hawkr.order.exception.OrderBusinessException;
import io.github.will413028.hawkr.order.grpc.client.AfterSalesGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.BuyerGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.CartGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.ProductGrpcClient;
import io.github.will413028.hawkr.order.mq.OrderEventPublisher;
import io.github.will413028.hawkr.order.repository.AutoRefundTaskRepository;
import io.github.will413028.hawkr.order.repository.CartCleanupRetryTaskRepository;
import io.github.will413028.hawkr.order.repository.FinancialLogRepository;
import io.github.will413028.hawkr.order.repository.OrderItemRepository;
import io.github.will413028.hawkr.order.repository.OrderOutboxMapper;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import io.github.will413028.hawkr.order.repository.RefundRequestRepository;
import io.github.will413028.hawkr.order.repository.StockTxRecordRepository;
import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.catchThrowableOfType;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * {@link UberOrderEligibilityGuard} 在<b>建單路徑</b>上真的被接上了——最後防線。
 *
 * <h2>為什麼結帳鏈擋過了還要擋一次</h2>
 * {@code OrderService.createOrder} 有<b>兩個多載</b>，而它們的來源不同：
 * <ul>
 *   <li>簡版（{@code buyerId…shippingFeePayer} 十參數）：唯一 caller 是
 *       {@code OrderGrpcService.createOrder}，也就是 gRPC {@code CreateOrder}
 *       （{@code POST /api/v1/app/orders}）——<b>完全不經過 {@code CheckoutService}</b>，
 *       結帳鏈那兩道對它是不存在的</li>
 *   <li>擴展版：{@code createOrderFromCheckout} 的下游（結帳主路徑）</li>
 * </ul>
 * 兩者沒有共同的收斂方法（各自呼叫 {@code OrderCreationTxHandler} 的不同方法），
 * 所以<b>兩個入口各接一次</b>，本檔也就有兩支對稱的測試。
 *
 * <h2>🔴 位置的斷言比「有沒有拋」更重要</h2>
 * 每一支都同時斷言 {@code cartGrpcClient.checkoutLock} <b>從未被呼叫</b>：
 * 閘門若放在扣庫存或鎖車之後，拋出來的錯誤一樣是 9503，但已經留下要補償的副作用。
 * 只驗「拋 9503」的測試對那個 bug 零鑑別力。
 */
class UberOrderCreationGuardWiringTest {

    /** 硬寫字面值。 */
    private static final String UBER = "3003";
    private static final String TAQBIN = "3001";
    private static final String CHECKOUT_ID = "CK-UBER-GUARD-TEST";

    private CartGrpcClient cartGrpcClient;
    private ProductGrpcClient productGrpcClient;
    private OrderService orderService;

    @BeforeAll
    static void initTsid() {
        try {
            TsidUtil.init(300, 0);
        } catch (Exception ignored) {
            // 已初始化
        }
    }

    @BeforeEach
    void setUp() {
        cartGrpcClient = mock(CartGrpcClient.class);
        productGrpcClient = mock(ProductGrpcClient.class);
        // 鎖車成功（0）：讓「非 3003 會往下走」這件事在陰性測試裡真的走得到，
        // 否則陰性測試會因為鎖車失敗而提早拋，看起來也像被擋住。
        when(cartGrpcClient.checkoutLock(anyLong(), anyString(), any())).thenReturn(0);

        orderService = new OrderService(
                mock(OrderRepository.class),
                mock(OrderItemRepository.class),
                mock(RefundRequestRepository.class),
                mock(FinancialLogRepository.class),
                mock(StockTxRecordRepository.class),
                mock(CartCleanupRetryTaskRepository.class),
                mock(AutoRefundTaskRepository.class),
                mock(OrderNoGenerator.class),
                productGrpcClient,
                cartGrpcClient,
                mock(OrderConfig.class),
                mock(OrderEventPublisher.class),
                mock(SellerGrpcClient.class),
                mock(BuyerGrpcClient.class),
                mock(AfterSalesGrpcClient.class),
                mock(OrderCompletionTxHandler.class),
                mock(OrderCreationTxHandler.class),
                mock(OrderOutboxMapper.class),
                new OrderPolicyResolver(),
                new UberOrderEligibilityGuard(),
                new SimpleMeterRegistry());
    }

    private List<OrderService.CreateOrderItemParam> items() {
        return List.of(new OrderService.CreateOrderItemParam(
                1L, "sku-1", 1, 500L, 500L, 0L, "img", 1L, "product-1"));
    }

    // ===== 入口 1／2：gRPC CreateOrder（購物車外建單）=====

    @Test
    @DisplayName("🔴 簡版 createOrder(3003)：拋 9503，且在鎖車／扣庫存之前")
    void simpleCreateOrderRejectsUberBeforeSideEffects() {
        OrderBusinessException ex = catchThrowableOfType(
                () -> orderService.createOrder(
                        2623900520L, 2685914682L, items(), 500L, 500L,
                        CHECKOUT_ID, "3201", UBER, 0L, "3101"),
                OrderBusinessException.class);

        assertThat(ex).isNotNull();
        assertThat(ex.code()).isEqualTo(9503);
        verify(cartGrpcClient, never()).checkoutLock(anyLong(), anyString(), any());
        verify(productGrpcClient, never()).deductStock(anyString(), any());
    }

    @Test
    @DisplayName("🔴 陰性：簡版 createOrder(3001) 不被閘門擋，會走到鎖車")
    void simpleCreateOrderAllowsNonUber() {
        // 後續會因為其餘 collaborator 都是 no-op mock 而失敗，本測試只關心「有沒有走到鎖車」——
        // 那是「閘門沒有誤擋」的可指認證據。
        try {
            orderService.createOrder(
                    2623900520L, 2685914682L, items(), 500L, 500L,
                    CHECKOUT_ID, "3201", TAQBIN, 0L, "3101");
        } catch (Exception ignored) {
            // 預期：mock 不足以跑完整條建單流程
        }
        verify(cartGrpcClient).checkoutLock(anyLong(), anyString(), any());
    }

    // ===== 入口 2／2：結帳主路徑 createOrderFromCheckout → 擴展版 createOrder =====

    @Test
    @DisplayName("🔴 擴展版 createOrder(3003)：拋 9503，且在鎖車／扣庫存之前")
    void extendedCreateOrderRejectsUberBeforeSideEffects() {
        OrderBusinessException ex = catchThrowableOfType(
                () -> orderService.createOrder(
                        2623900520L, 2685914682L, items(), 500L, 500L,
                        CHECKOUT_ID, "3201", UBER, 0L, "3101",
                        "王小明", "0912345678", "台北市中正區測試路1號", "",
                        "", "", "", "1", "台北市", "中正區",
                        30, "HAWK2685914682", 0L, null,
                        new OrderPolicyResolver().resolve(UBER)),
                OrderBusinessException.class);

        assertThat(ex).isNotNull();
        assertThat(ex.code()).isEqualTo(9503);
        verify(cartGrpcClient, never()).checkoutLock(anyLong(), anyString(), any());
        verify(productGrpcClient, never()).deductStock(anyString(), any());
    }

    @Test
    @DisplayName("🔴 擴展版 createOrder(3003)：閘門在 recipientName 驗證之前——"
            + "收件人姓名合法時仍必須拋 9503，不能只在姓名也剛好不合法時才擋")
    void extendedCreateOrderRejectsUberEvenWithValidRecipient() {
        OrderBusinessException ex = catchThrowableOfType(
                () -> orderService.createOrder(
                        2623900520L, 2685914682L, items(), 500L, 500L,
                        CHECKOUT_ID, "3201", UBER, 0L, "3101",
                        "陳大文", "0987654321", "新北市板橋區測試街9號", "",
                        "", "", "", "1", "新北市", "板橋區",
                        30, "HAWK2685914682", 0L, null,
                        new OrderPolicyResolver().resolve(UBER)),
                OrderBusinessException.class);

        assertThat(ex).isNotNull();
        assertThat(ex.code()).isEqualTo(9503);
    }

    @Test
    @DisplayName("🔴 陰性：擴展版 createOrder(3001) 不被閘門擋，會走到鎖車")
    void extendedCreateOrderAllowsNonUber() {
        try {
            orderService.createOrder(
                    2623900520L, 2685914682L, items(), 500L, 500L,
                    CHECKOUT_ID, "3201", TAQBIN, 0L, "3101",
                    "王小明", "0912345678", "台北市中正區測試路1號", "",
                    "", "", "", "1", "台北市", "中正區",
                    30, "HAWK2685914682", 0L, null,
                    new OrderPolicyResolver().resolve(TAQBIN));
        } catch (Exception ignored) {
            // 預期：mock 不足以跑完整條建單流程
        }
        verify(cartGrpcClient).checkoutLock(anyLong(), anyString(), any());
    }
}
