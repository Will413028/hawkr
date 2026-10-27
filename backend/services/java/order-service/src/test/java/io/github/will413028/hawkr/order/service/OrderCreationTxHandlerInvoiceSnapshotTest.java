package io.github.will413028.hawkr.order.service;

import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.InvoiceCarrierSelectionMode;
import io.github.will413028.hawkr.common.utils.code.InvoiceDocumentKind;
import io.github.will413028.hawkr.order.checkout.InvoiceIntakeResolver;
import io.github.will413028.hawkr.order.config.OrderConfig;
import io.github.will413028.hawkr.order.entity.OrderEntity;
import io.github.will413028.hawkr.order.entity.OrderInvoiceSnapshotEntity;
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
import org.mockito.InOrder;

import java.lang.reflect.Method;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.inOrder;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * 發票快照必須與 {@code orders} / {@code order_items} <b>同一個建單交易</b>寫入
 * （58 號 §6.2.4 order-service ③⑤、§7.1.2 步驟 1）。
 *
 * <p><b>為什麼「同一交易」是靠 {@code @Transactional} 的載體位置守的</b>：
 * {@link OrderCreationTxHandler#createOrderInTxExtended} 是那個載體
 * （{@code OrderService.createOrder} 走 bean 呼叫，不是 self-invocation，§11.1）。
 * 只要 insert 發生在這個方法內，它就與 orders 同生共死；插到方法外、或搬到付款成功之後，
 * 就變成兩件可以各自成立／失敗的事。
 *
 * <p><b>鑑別力</b>（本檔要抓的兩個變異）：
 * <ul>
 *   <li>把 {@code insertInvoiceSnapshot} 從 {@code createOrderInTxExtended} 拿掉、搬到
 *       {@code PaymentEventHandler}（＝付款之後）⇒ {@link #snapshotIsWrittenInsideTheOrderCreationTransaction}
 *       與 {@link #legacyCreateOrderPathAlsoWritesADefaultSnapshot} 一起轉紅</li>
 *   <li>把 {@code @Transactional} 從 {@code createOrderInTxExtended} 拿掉 ⇒
 *       {@link #bothCreationPathsCarryTheTransactionalAnnotation} 轉紅（那會讓三張表的寫入非原子）</li>
 * </ul>
 */
class OrderCreationTxHandlerInvoiceSnapshotTest {

    private OrderRepository orderRepository;
    private OrderItemRepository orderItemRepository;
    private StockTxRecordRepository stockTxRecordRepository;
    private OrderInvoiceSnapshotRepository invoiceSnapshotRepository;
    private OrderCreationTxHandler handler;

    @BeforeAll
    static void initTsid() {
        try {
            TsidUtil.init(300, 0);
        } catch (IllegalStateException ignored) {
            // 已初始化
        }
    }

    @BeforeEach
    void setUp() {
        orderRepository = mock(OrderRepository.class);
        orderItemRepository = mock(OrderItemRepository.class);
        stockTxRecordRepository = mock(StockTxRecordRepository.class);
        invoiceSnapshotRepository = mock(OrderInvoiceSnapshotRepository.class);
        OrderNoGenerator orderNoGenerator = mock(OrderNoGenerator.class);
        when(orderNoGenerator.generateOrderNo()).thenReturn("O20260830001");
        OrderConfig orderConfig = mock(OrderConfig.class);
        when(orderConfig.getPaymentDeadlineSeconds()).thenReturn(1800L);

        handler = new OrderCreationTxHandler(orderRepository, orderItemRepository,
                stockTxRecordRepository, invoiceSnapshotRepository, orderNoGenerator, orderConfig);
    }

    private static List<OrderService.CreateOrderItemParam> items() {
        return List.of(new OrderService.CreateOrderItemParam(
                5001L, "Apple", 2, 500L, 500L, 0L, "key/a.jpg", 9001L, "Apple Box"));
    }

    private String createExtended(InvoiceIntakeResolver.Snapshot snapshot) {
        return handler.createOrderInTxExtended(
                100L, 2001L, items(), 1000L, 1060L, "STX1", "CHK1",
                "3201", "3001", 60L, "BUYER",
                "王小明", "0912345678", "台北市中正區忠孝東路一段 1 號", "",
                "", "", "", "TAIPEI", "台北市", "中正區",
                0, "HAWK2001", 0L, snapshot,
                new io.github.will413028.hawkr.order.checkout.OrderPolicyResolver().resolve("3001"));
    }

    @Test
    @DisplayName("快照與 orders / order_items 在同一個 @Transactional 方法內寫入，且順序在建單之後、協調紀錄之前")
    void snapshotIsWrittenInsideTheOrderCreationTransaction() {
        createExtended(InvoiceIntakeResolver.defaultSnapshot());

        InOrder ord = inOrder(orderRepository, orderItemRepository,
                invoiceSnapshotRepository, stockTxRecordRepository);
        ord.verify(orderRepository).insert(org.mockito.ArgumentMatchers.any(OrderEntity.class));
        ord.verify(orderItemRepository).insert(org.mockito.ArgumentMatchers.any(OrderItemEntity.class));
        ord.verify(invoiceSnapshotRepository).insert(org.mockito.ArgumentMatchers.any(OrderInvoiceSnapshotEntity.class));
        ord.verify(stockTxRecordRepository).updateStatusAndOrderNo(
                org.mockito.ArgumentMatchers.eq("STX1"),
                org.mockito.ArgumentMatchers.anyString(),
                org.mockito.ArgumentMatchers.anyString(),
                org.mockito.ArgumentMatchers.eq("O20260830001"));
    }

    @Test
    @DisplayName("落庫的欄位逐項對上 resolver 的輸出（version 恆 1；本服務沒有遞增它的路徑，58 I-50）")
    void snapshotColumnsMatchResolverOutput() {
        createExtended(new InvoiceIntakeResolver.Snapshot(
                InvoiceDocumentKind.B2B.value(), InvoiceCarrierSelectionMode.NONE.value(),
                null, "12345678", "範例生鮮股份有限公司", null, "buyer@example.com"));

        ArgumentCaptor<OrderInvoiceSnapshotEntity> captor =
                ArgumentCaptor.forClass(OrderInvoiceSnapshotEntity.class);
        verify(invoiceSnapshotRepository).insert(captor.capture());
        OrderInvoiceSnapshotEntity row = captor.getValue();

        assertThat(row.getOrderNo()).isEqualTo("O20260830001");
        assertThat(row.getSnapshotVersion()).isEqualTo(1);
        assertThat(row.getInvoiceKind()).isEqualTo(InvoiceDocumentKind.B2B.value());
        assertThat(row.getCarrierSelectionMode()).isEqualTo(InvoiceCarrierSelectionMode.NONE.value());
        assertThat(row.getBuyerTaxId()).isEqualTo("12345678");
        assertThat(row.getBuyerTitle()).isEqualTo("範例生鮮股份有限公司");
        assertThat(row.getDonationCode()).isNull();
        assertThat(row.getBuyerInvoicePreferenceId()).isNull();
        assertThat(row.getNotifyEmail())
                .as("notify_email 是 §5.4.3 DDL 的漏列；漏了它 invoice_documents.recipient_email 恆空、"
                        + "開票通知寄不出去而不報錯")
                .isEqualTo("buyer@example.com");
        assertThat(row.getId()).as("TSID（@TableId(type = INPUT)）").isNotNull().isPositive();
    }

    @Test
    @DisplayName("購物車外建單（gRPC CreateOrder）也落一列缺省快照——否則那條路徑的每張訂單都會誤觸發 invoice 端的 fail-loud")
    void legacyCreateOrderPathAlsoWritesADefaultSnapshot() {
        handler.createOrderInTx(100L, 2001L, items(), 1000L, 1000L,
                "STX2", "CHK2", "3201", "3001", 0L, "BUYER",
                new io.github.will413028.hawkr.order.checkout.OrderPolicyResolver().resolve("3001"));

        ArgumentCaptor<OrderInvoiceSnapshotEntity> captor =
                ArgumentCaptor.forClass(OrderInvoiceSnapshotEntity.class);
        verify(invoiceSnapshotRepository).insert(captor.capture());

        assertThat(captor.getValue().getInvoiceKind()).isEqualTo(InvoiceDocumentKind.B2C.value());
        assertThat(captor.getValue().getCarrierSelectionMode())
                .isEqualTo(InvoiceCarrierSelectionMode.USE_BUYER_DEFAULT.value());
    }

    @Test
    @DisplayName("兩條建單路徑都必須帶 @Transactional（拿掉就不再是同一交易，而三張表的寫入會各自 auto-commit）")
    void bothCreationPathsCarryTheTransactionalAnnotation() throws Exception {
        for (String name : new String[]{"createOrderInTx", "createOrderInTxExtended"}) {
            Method m = java.util.Arrays.stream(OrderCreationTxHandler.class.getDeclaredMethods())
                    .filter(x -> x.getName().equals(name))
                    .findFirst()
                    .orElseThrow(() -> new AssertionError("找不到方法：" + name));
            assertThat(m.getAnnotation(org.springframework.transaction.annotation.Transactional.class))
                    .as("%s 少了 @Transactional", name)
                    .isNotNull();
        }
    }
}
