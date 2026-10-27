package io.github.will413028.hawkr.order.service;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.order.checkout.OrderPolicySnapshot;
import io.github.will413028.hawkr.order.config.OrderConfig;
import io.github.will413028.hawkr.order.entity.OrderEntity;
import io.github.will413028.hawkr.order.entity.OrderItemEntity;
import io.github.will413028.hawkr.order.repository.OrderInvoiceSnapshotRepository;
import io.github.will413028.hawkr.order.repository.OrderItemRepository;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import io.github.will413028.hawkr.order.repository.StockTxRecordRepository;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;

import java.time.Duration;
import java.time.LocalDateTime;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * RFQ 建單在持久層的兩件事：<b>non-catalog 明細</b>與<b>自己的付款期限</b>。
 *
 * <h2>兩個變異</h2>
 * <ul>
 *   <li>把 {@code createRfqOrderInTx} 的付款期限改回讀 {@code getPaymentDeadlineSeconds}
 *       ⇒ {@link #rfqOrderUsesItsOwnTwelveHourPaymentDeadline} 轉紅。這條錯了<b>不會有人發現</b>：
 *       訂單照建、付款照收，只是買家多拿到 12 小時，而 64 號 §6.4.11 的依據是賣家把貨
 *       留給這筆成交的時間。</li>
 *   <li>在 {@code insertOrderItems} 給 {@code sku_id} 補一個預設值（例如 0）⇒
 *       {@link #rfqLinePersistsNullSkuId} 轉紅。</li>
 * </ul>
 */
class OrderCreationTxHandlerRfqLineTest {

    private static final long CATALOG_DEADLINE_SECONDS = 86400L;
    private static final long RFQ_DEADLINE_SECONDS = 43200L;

    private OrderRepository orderRepository;
    private OrderItemRepository orderItemRepository;
    private OrderCreationTxHandler handler;

    @BeforeAll
    static void initTsid() {
        try {
            TsidUtil.init(302, 0);
        } catch (IllegalStateException ignored) {
            // 已初始化
        }
    }

    @BeforeEach
    void setUp() {
        orderRepository = mock(OrderRepository.class);
        orderItemRepository = mock(OrderItemRepository.class);
        OrderNoGenerator orderNoGenerator = mock(OrderNoGenerator.class);
        when(orderNoGenerator.generateOrderNo()).thenReturn("RFQ20260921001");

        // 兩個值刻意不同：相同的話，「讀錯設定」這個變異就測不出來。
        OrderConfig orderConfig = mock(OrderConfig.class);
        when(orderConfig.getPaymentDeadlineSeconds()).thenReturn(CATALOG_DEADLINE_SECONDS);
        when(orderConfig.getRfqPaymentDeadlineSeconds()).thenReturn(RFQ_DEADLINE_SECONDS);

        handler = new OrderCreationTxHandler(orderRepository, orderItemRepository,
                mock(StockTxRecordRepository.class), mock(OrderInvoiceSnapshotRepository.class),
                orderNoGenerator, orderConfig);
    }

    private OrderEntity createRfq() {
        // ⛔ 走 canonical constructor 而不是 CreateOrderItemParam.catalog：這一列就是
        //    non-catalog line，工廠方法的 primitive long 傳不進 null。
        List<OrderService.CreateOrderItemParam> items = List.of(
                new OrderService.CreateOrderItemParam(
                        null, "文旦柚 200 公斤", 1, 30000L, 30000L, 0L, "", null, null));
        return handler.createRfqOrderInTx(
                100L, 9001L, 7505210956461375488L, items,
                30800L, 800L, "3201", "3005", "3101", "HAWKAGENT01",
                new OrderPolicySnapshot("PLATFORM_CONSIGNMENT", "PLATFORM_AGENT_MERCHANT",
                        "SELLER_MANAGED", "PLATFORM_TO_BUYER", "v1-foundation"));
    }

    @Test
    @DisplayName("RFQ 訂單的付款期限是建單 +12h，不是結帳路徑的 24h（64 號 §6.4.11）")
    void rfqOrderUsesItsOwnTwelveHourPaymentDeadline() {
        LocalDateTime before = DateTimeUtils.nowUtc();

        OrderEntity order = createRfq();

        LocalDateTime after = DateTimeUtils.nowUtc();
        // 斷言成區間而不是精確等值：nowUtc() 在受測程式碼裡呼叫，測試取不到同一個瞬間。
        assertThat(order.getPaymentDeadline())
                .isAfterOrEqualTo(before.plusSeconds(RFQ_DEADLINE_SECONDS))
                .isBeforeOrEqualTo(after.plusSeconds(RFQ_DEADLINE_SECONDS));
        // 鑑別力：12h 與 24h 相差 12 小時，遠大於上面那個區間的寬度 ⇒ 讀錯設定一定落在區間外。
        assertThat(Duration.between(before, order.getPaymentDeadline()))
                .isLessThan(Duration.ofSeconds(CATALOG_DEADLINE_SECONDS));
    }

    @Test
    @DisplayName("non-catalog 明細：sku_id 一路落到 order_items 都是 NULL，沒有被補成 0")
    void rfqLinePersistsNullSkuId() {
        createRfq();

        ArgumentCaptor<OrderItemEntity> captor = ArgumentCaptor.forClass(OrderItemEntity.class);
        verify(orderItemRepository).insert(captor.capture());
        OrderItemEntity item = captor.getValue();
        assertThat(item.getSkuId()).isNull();
        // 品名仍是真品名：放寬的是參照，不是 sku_name 的禁 sentinel 規則（64 號 §6.4.3）。
        assertThat(item.getSkuName()).isEqualTo("文旦柚 200 公斤");
    }
}
