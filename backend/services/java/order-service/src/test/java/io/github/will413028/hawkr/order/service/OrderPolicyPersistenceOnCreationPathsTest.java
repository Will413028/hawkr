package io.github.will413028.hawkr.order.service;

import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.order.checkout.InvoiceIntakeResolver;
import io.github.will413028.hawkr.order.checkout.OrderPolicyResolver;
import io.github.will413028.hawkr.order.checkout.OrderPolicySnapshot;
import io.github.will413028.hawkr.order.config.OrderConfig;
import io.github.will413028.hawkr.order.entity.OrderEntity;
import io.github.will413028.hawkr.order.repository.OrderInvoiceSnapshotRepository;
import io.github.will413028.hawkr.order.repository.OrderItemRepository;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import io.github.will413028.hawkr.order.repository.StockTxRecordRepository;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * 政策五欄在<b>兩條建單路徑</b>上都真的被 setter 設過。
 *
 * <h2>本檔與 {@code OrderPolicySnapshotPersistenceIT} 的分工（兩支缺一不可）</h2>
 * <ul>
 *   <li>IT 驗的是「entity 上有值 ⇒ DB row 有值」——那是 MyBatis-Plus
 *       {@code insertStrategy=NOT_NULL} 那一段，只有真 DB 看得到</li>
 *   <li><b>本檔</b>驗的是「產線路徑會把值放上 entity」——IT 自己呼叫 setter，
 *       所以把 {@link OrderCreationTxHandler} 的寫入拿掉，<b>IT 不會紅</b>。
 *       這一段的鑑別力只有本檔有</li>
 * </ul>
 *
 * <h2>M1 變異</h2>
 * 拿掉 {@code applyPolicySnapshot(order, policySnapshot)} 任一處呼叫（或刪掉該方法裡的任一行）
 * ⇒ 對應那一支測試轉紅。<b>兩條路徑各有一支</b>，因為只修 checkout 路徑會留下
 * gRPC {@code CreateOrder}（{@code POST /api/v1/app/orders}，路由現存）繼續產出吃 DB DEFAULT 的列。
 */
class OrderPolicyPersistenceOnCreationPathsTest {

    private OrderRepository orderRepository;
    private OrderCreationTxHandler handler;

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
        orderRepository = mock(OrderRepository.class);
        OrderConfig orderConfig = mock(OrderConfig.class);
        when(orderConfig.getPaymentDeadlineSeconds()).thenReturn(259200L);
        handler = new OrderCreationTxHandler(
                orderRepository,
                mock(OrderItemRepository.class),
                mock(StockTxRecordRepository.class),
                mock(OrderInvoiceSnapshotRepository.class),
                mock(OrderNoGenerator.class),
                orderConfig);
    }

    @Test
    @DisplayName("結帳路徑（createOrderInTxExtended）：五欄全部設在送進 insert 的那個 entity 上")
    void checkoutPathWritesAllSixPolicyColumns() {
        OrderPolicySnapshot policy =
                new OrderPolicyResolver().resolve(ShippingMethod.TAQBIN.value());

        handler.createOrderInTxExtended(
                100L, 2001L, items(), 1000L, 1060L, "STX1", "CHK1",
                "3201", ShippingMethod.TAQBIN.value(), 60L, "BUYER",
                "王小明", "0912345678", "台北市中正區忠孝東路一段 1 號", "",
                "", "", "", "TAIPEI", "台北市", "中正區",
                0, "HAWK2001", 0L, InvoiceIntakeResolver.defaultSnapshot(), policy);

        assertSixColumns(capturedOrder(), "PLATFORM_LOGISTICS");
    }

    @Test
    @DisplayName("gRPC CreateOrder 路徑（createOrderInTx）：同樣五欄——只修結帳那一條等於修一半")
    void legacyGrpcPathWritesAllSixPolicyColumns() {
        OrderPolicySnapshot policy =
                new OrderPolicyResolver().resolve(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value());

        handler.createOrderInTx(100L, 2001L, items(), 1000L, 1000L,
                "STX2", "CHK2", "3201", ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value(),
                0L, "BUYER", policy);

        assertSixColumns(capturedOrder(), "PLATFORM_LOGISTICS");
    }

    @Test
    @DisplayName("陰性對照：3005 商家自送寫 SELLER_MANAGED——證明本檔讀得到差異，不是恆等斷言")
    void merchantDeliveryWritesSellerManaged() {
        OrderPolicySnapshot policy =
                new OrderPolicyResolver().resolve(ShippingMethod.MERCHANT_DELIVERY.value());

        handler.createOrderInTx(100L, 2001L, items(), 1000L, 1000L,
                "STX3", "CHK3", "3201", ShippingMethod.MERCHANT_DELIVERY.value(),
                0L, "BUYER", policy);

        assertSixColumns(capturedOrder(), "SELLER_MANAGED");
    }

    // ==================== helpers ====================

    private OrderEntity capturedOrder() {
        ArgumentCaptor<OrderEntity> captor = ArgumentCaptor.forClass(OrderEntity.class);
        verify(orderRepository).insert(captor.capture());
        return captor.getValue();
    }

    /**
     * 期望值一律硬寫字面字串。
     * 用 {@code policy.fulfillmentMode()} 當期望值會讓 resolver 與斷言一起變，
     * 那種寫法對「resolver 被改壞」零鑑別力。
     */
    private static void assertSixColumns(OrderEntity order, String expectedFulfillmentMode) {
        assertThat(order.getCommerceModel()).isEqualTo("SELLER_DIRECT");
        assertThat(order.getCollectionModel()).isEqualTo("SELLER_MERCHANT");
        assertThat(order.getFulfillmentMode()).isEqualTo(expectedFulfillmentMode);
        assertThat(order.getInvoiceModel()).isEqualTo("SELLER_TO_BUYER");
        assertThat(order.getPolicyVersion()).isEqualTo("v1-foundation");
    }

    private static List<OrderService.CreateOrderItemParam> items() {
        return List.of(new OrderService.CreateOrderItemParam(
                5001L, "Apple", 2, 500L, 500L, 0L, "key/a.jpg", 9001L, "Apple Box"));
    }
}
