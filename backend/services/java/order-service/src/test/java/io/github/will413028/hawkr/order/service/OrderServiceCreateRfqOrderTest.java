package io.github.will413028.hawkr.order.service;

import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.order.checkout.OrderPolicySnapshot;
import io.github.will413028.hawkr.order.config.OrderConfig;
import io.github.will413028.hawkr.order.entity.OrderEntity;
import io.github.will413028.hawkr.order.exception.OrderBusinessException;
import io.github.will413028.hawkr.order.grpc.client.CartGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.ProductGrpcClient;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import io.github.will413028.hawkr.order.repository.StockTxRecordRepository;
import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;

import java.time.LocalDateTime;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

/**
 * EXTERNAL lane RFQ 建單（64 號 §6.4）。
 *
 * <h2>這支測試真正守的是什麼</h2>
 * 不是「方法會不會跑完」，是三件<b>錯了不會有人發現</b>的事：
 * <ol>
 *   <li><b>政策五欄的值</b>。用 {@code OrderPolicyResolver} 的預設值寫 RFQ 訂單
 *       <b>不會被任何 CHECK 擋下</b>——V14 的四條扣掉 V16 退場的那條、剩三條只驗五欄彼此一致，不驗它對不對這張
 *       訂單。錯的代價是撥款與開票的法源整組錯掉，而訂單、付款、帳本全部正常。</li>
 *   <li><b>不碰庫存</b>。這條 lane 的貨在站外業者手上；只要它退化成走
 *       {@code DeductStock}，平台會憑空扣掉一筆不存在的可售量。</li>
 *   <li><b>冪等</b>。命令由 outbox worker 投遞，重試是常態不是例外。</li>
 * </ol>
 *
 * <h2>🔴 2026-09-21：EXTERNAL lane 的受託代銷形狀退場</h2>
 * 本檔原本斷言 EXTERNAL lane 寫 {@code PLATFORM_CONSIGNMENT}／{@code PLATFORM_AGENT_MERCHANT}／
 * {@code PLATFORM_TO_BUYER}。那組值已退場：{@code OrderPolicyResolver} 的 javadoc 逐字寫著
 * 受託代銷「刻意尚未啟用，這不是還沒寫而是 fail-closed」（撥款費率未經契約確認、賣家→平台
 * 的進項發票軸未落地），而本方法曾是全系統唯一繞過那道閘門的地方。
 * ⇒ 兩條 lane 現在寫同一組賣家直營政策，差別只剩 {@code storeId}。
 *
 * <h2>M1 變異</h2>
 * 把政策五欄改成 {@code orderPolicyResolver.resolve(...)} ⇒ <b>不會</b>轉紅（現在同值），
 * 所以第一支改守「不得退回平台側四值」；拿掉冪等回查 ⇒ 冪等那支轉紅；
 * 拿掉空 MerID 的 fail-closed ⇒ {@code failsClosedWhenStoreHasNoPayuniMerId} 轉紅。
 */
class OrderServiceCreateRfqOrderTest {

    private OrderRepository orderRepository;
    private OrderCreationTxHandler txHandler;
    private ProductGrpcClient productGrpcClient;
    private CartGrpcClient cartGrpcClient;
    private StockTxRecordRepository stockTxRecordRepository;
    private OrderService orderService;
    private io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient sellerGrpcClient;

    private static final long ACCEPTANCE_ID = 7505210956461375488L;
    private static final long SETTLEMENT_STORE_ID = 9001L;

    @BeforeAll
    static void initTsid() {
        try {
            TsidUtil.init(301, 0);
        } catch (Exception ignored) {
            // 已初始化
        }
    }

    @BeforeEach
    void setUp() {
        orderRepository = mock(OrderRepository.class);
        txHandler = mock(OrderCreationTxHandler.class);
        productGrpcClient = mock(ProductGrpcClient.class);
        cartGrpcClient = mock(CartGrpcClient.class);
        stockTxRecordRepository = mock(StockTxRecordRepository.class);
        sellerGrpcClient = mock(io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient.class);
        when(sellerGrpcClient.getSellerMeta(anyLong())).thenReturn(
                io.github.will413028.hawkr.seller.grpc.SellerMeta.newBuilder().setPayuniMerId("HAWK2001").build());

        orderService = new OrderService(
                orderRepository, mock(io.github.will413028.hawkr.order.repository.OrderItemRepository.class),
                mock(io.github.will413028.hawkr.order.repository.RefundRequestRepository.class),
                mock(io.github.will413028.hawkr.order.repository.FinancialLogRepository.class),
                stockTxRecordRepository,
                mock(io.github.will413028.hawkr.order.repository.CartCleanupRetryTaskRepository.class),
                mock(io.github.will413028.hawkr.order.repository.AutoRefundTaskRepository.class),
                mock(OrderNoGenerator.class), productGrpcClient, cartGrpcClient,
                mock(OrderConfig.class), mock(io.github.will413028.hawkr.order.mq.OrderEventPublisher.class),
                sellerGrpcClient,
                mock(io.github.will413028.hawkr.order.grpc.client.BuyerGrpcClient.class),
                mock(io.github.will413028.hawkr.order.grpc.client.AfterSalesGrpcClient.class),
                mock(OrderCompletionTxHandler.class), txHandler,
                mock(io.github.will413028.hawkr.order.repository.OrderOutboxMapper.class),
                new io.github.will413028.hawkr.order.checkout.OrderPolicyResolver(),
                mock(io.github.will413028.hawkr.order.checkout.UberOrderEligibilityGuard.class),
                new SimpleMeterRegistry());

        OrderEntity created = new OrderEntity();
        created.setOrderNo("RFQ20260914001");
        created.setPaymentDeadline(LocalDateTime.of(2026, 9, 17, 10, 0));
        when(txHandler.createRfqOrderInTx(anyLong(), anyLong(), anyLong(), any(), anyLong(), anyLong(),
                any(), any(), any(), any(), any())).thenReturn(created);
    }

    @Test
    @DisplayName("EXTERNAL lane 走賣家直營：結算 store 就是賣方，⛔ 不得退回受託代銷四值")
    void externalLaneWritesSellerDirectPolicy() {
        orderService.createRfqOrder(100L, SETTLEMENT_STORE_ID, ACCEPTANCE_ID,
                30800L, 800L, null, "文旦柚 200 公斤", 30000L, "3201", "EXTERNAL");

        // 期望值一律硬寫字面字串：拿 CodeMeta 常數當期望值會讓值域與斷言一起變。
        OrderPolicySnapshot policy = capturedPolicy();
        assertThat(policy.commerceModel())
                .as("⛔ 退回 PLATFORM_CONSIGNMENT ＝ 又一次繞過 OrderPolicyResolver 刻意關掉的閘門："
                        + "撥款指示費的費率未經契約確認、賣家→平台的進項發票軸未落地，"
                        + "而三條 CHECK 只驗五欄彼此一致、不會擋下它")
                .isEqualTo("SELLER_DIRECT");
        assertThat(policy.collectionModel()).isEqualTo("SELLER_MERCHANT");
        assertThat(policy.invoiceModel())
                .as("開票 issuer 必須與收款主體一致（chk_orders_invoice_collection）")
                .isEqualTo("SELLER_TO_BUYER");
        assertThat(policy.fulfillmentMode()).isEqualTo("SELLER_MANAGED");
    }

    @Test
    @DisplayName("EXTERNAL lane 的 mer_id 取結算 store 自己的 PayUni 子商店，不是平台代理商戶")
    void externalLaneUsesSettlementStoreMerId() {
        orderService.createRfqOrder(100L, SETTLEMENT_STORE_ID, ACCEPTANCE_ID,
                30800L, 800L, null, "文旦柚 200 公斤", 30000L, "3201", "EXTERNAL");

        verify(sellerGrpcClient).getSellerMeta(SETTLEMENT_STORE_ID);
        verify(txHandler).createRfqOrderInTx(anyLong(), anyLong(), anyLong(), any(),
                anyLong(), anyLong(), any(), anyString(), anyString(),
                org.mockito.ArgumentMatchers.eq("HAWK2001"), any());
    }

    @Test
    @DisplayName("不碰庫存、不碰購物車：站外的貨不在平台手上")
    void neverTouchesStockOrCart() {
        orderService.createRfqOrder(100L, SETTLEMENT_STORE_ID, ACCEPTANCE_ID,
                30800L, 800L, null, "文旦柚 200 公斤", 30000L, "3201", "EXTERNAL");

        verifyNoInteractions(productGrpcClient);
        verifyNoInteractions(cartGrpcClient);
        verifyNoInteractions(stockTxRecordRepository);
    }

    @Test
    @DisplayName("單一品項、quantity 恆為 1：INT 表達不了賣家報價的小數數量")
    void writesExactlyOneItemWithQuantityOne() {
        orderService.createRfqOrder(100L, SETTLEMENT_STORE_ID, ACCEPTANCE_ID,
                30800L, 800L, null, "文旦柚 200 公斤", 30000L, "3201", "EXTERNAL");

        List<OrderService.CreateOrderItemParam> items = capturedItems();
        assertThat(items).hasSize(1);
        OrderService.CreateOrderItemParam item = items.get(0);
        assertThat(item.quantity()).isEqualTo(1);
        // unitPrice ＝ goods + tax；運費另計在 shipping_fee，對帳式因此成立。
        assertThat(item.unitPrice()).isEqualTo(30000L);
        assertThat(item.listPrice()).isEqualTo(30000L);
        assertThat(item.shareDiscount()).isEqualTo(0L);
        // 真實數量與單位寫在品名裡（⛔ 不得使用 sentinel）。
        assertThat(item.skuName()).isEqualTo("文旦柚 200 公斤");
    }

    @Test
    @DisplayName("RFQ 明細是 non-catalog line：sku_id 為 NULL，不指向任何 product_sku")
    void writesNonCatalogLineWithNullSkuId() {
        orderService.createRfqOrder(100L, SETTLEMENT_STORE_ID, ACCEPTANCE_ID,
                30800L, 800L, null, "文旦柚 200 公斤", 30000L, "3201", "EXTERNAL");

        OrderService.CreateOrderItemParam item = capturedItems().get(0);
        // ⛔ 判準是 null，不是 0：0 在 TSID 定義域外看起來安全，但它會讓「沒有 SKU」與
        //    「解析失敗」共用同一個值（64 號 §6.4.15 對哨兵的立場）。
        assertThat(item.skuId()).isNull();
        // 品名仍必須是真品名——放寬的是參照，不是 sku_name 的禁 sentinel 規則。
        assertThat(item.skuName()).isEqualTo("文旦柚 200 公斤");
    }

    @Test
    @DisplayName("wire 上真的帶了 sku_id 就照帶：放寬不等於忽略")
    void honoursSkuIdWhenCallerProvidesOne() {
        orderService.createRfqOrder(100L, SETTLEMENT_STORE_ID, ACCEPTANCE_ID,
                30800L, 800L, 5150L, "文旦柚 200 公斤", 30000L, "3201", "EXTERNAL");

        assertThat(capturedItems().get(0).skuId()).isEqualTo(5150L);
    }

    @Test
    @DisplayName("配送走 3005／運費買方負擔，且 acceptance_id 有落到命令上")
    void writesMerchantDeliveryAndBuyerBorneShipping() {
        orderService.createRfqOrder(100L, SETTLEMENT_STORE_ID, ACCEPTANCE_ID,
                30800L, 800L, null, "文旦柚 200 公斤", 30000L, "3201", "EXTERNAL");

        verify(txHandler).createRfqOrderInTx(
                org.mockito.ArgumentMatchers.eq(100L),
                org.mockito.ArgumentMatchers.eq(SETTLEMENT_STORE_ID),
                org.mockito.ArgumentMatchers.eq(ACCEPTANCE_ID),
                any(),
                org.mockito.ArgumentMatchers.eq(30800L),
                org.mockito.ArgumentMatchers.eq(800L),
                org.mockito.ArgumentMatchers.eq("3201"),
                org.mockito.ArgumentMatchers.eq("3005"),
                org.mockito.ArgumentMatchers.eq("3101"),
                org.mockito.ArgumentMatchers.eq("HAWK2001"),
                any());
    }

    @Test
    @DisplayName("冪等：已有同一個 acceptance 的訂單就回既有那張，不建第二張")
    void replaysExistingOrderForSameAcceptance() {
        OrderEntity existing = new OrderEntity();
        existing.setOrderNo("RFQ20260913009");
        existing.setPaymentDeadline(LocalDateTime.of(2026, 9, 16, 9, 0));
        when(orderRepository.findByAcceptanceId(ACCEPTANCE_ID)).thenReturn(existing);

        OrderService.RfqOrderResult result = orderService.createRfqOrder(
                100L, SETTLEMENT_STORE_ID, ACCEPTANCE_ID,
                30800L, 800L, null, "文旦柚 200 公斤", 30000L, "3201", "EXTERNAL");

        assertThat(result.replayed()).isTrue();
        assertThat(result.orderNo()).isEqualTo("RFQ20260913009");
        verify(txHandler, never()).createRfqOrderInTx(anyLong(), anyLong(), anyLong(), any(),
                anyLong(), anyLong(), any(), any(), any(), any(), any());
    }

    @Test
    @DisplayName("fail-closed：賣方 store 沒有 PayUni 子商店時不得建單（空 mer_id 會等到付款才炸）")
    void failsClosedWhenStoreHasNoPayuniMerId() {
        // 這一格接手 EXTERNAL lane 原本那道 PLATFORM_COLLECTION_NOT_CONFIGURED：
        // 收款主體換人之後，風險從「沒設代理商 MerID」變成「這個 store 沒有 PayUni 子商店」。
        when(sellerGrpcClient.getSellerMeta(anyLong())).thenReturn(
                io.github.will413028.hawkr.seller.grpc.SellerMeta.newBuilder().setPayuniMerId("").build());

        assertThatThrownBy(() -> orderService.createRfqOrder(100L, SETTLEMENT_STORE_ID, ACCEPTANCE_ID,
                30800L, 800L, null, "文旦柚 200 公斤", 30000L, "3201", "EXTERNAL"))
                .isInstanceOf(OrderBusinessException.class);

        verify(txHandler, never()).createRfqOrderInTx(anyLong(), anyLong(), anyLong(), any(),
                anyLong(), anyLong(), any(), any(), any(), any(), any());
    }

    @Test
    @DisplayName("同一道 fail-closed 也蓋 PLATFORM lane：兩條 lane 的 mer_id 現在同源")
    void platformLaneAlsoFailsClosedWhenStoreHasNoPayuniMerId() {
        when(sellerGrpcClient.getSellerMeta(anyLong())).thenReturn(
                io.github.will413028.hawkr.seller.grpc.SellerMeta.newBuilder().setPayuniMerId("").build());

        assertThatThrownBy(() -> orderService.createRfqOrder(100L, 7489859548420286732L, ACCEPTANCE_ID,
                30800L, 800L, null, "文旦柚 200 公斤", 30000L, "3201", "PLATFORM"))
                .isInstanceOf(OrderBusinessException.class);

        verify(txHandler, never()).createRfqOrderInTx(anyLong(), anyLong(), anyLong(), any(),
                anyLong(), anyLong(), any(), any(), any(), any(), any());
    }

    @Test
    @DisplayName("PLATFORM lane：政策五欄是賣家直營，不是 MoR——賣方就是那個平台賣家")
    void platformLaneWritesSellerDirectPolicy() {
        orderService.createRfqOrder(100L, 7489859548420286732L, ACCEPTANCE_ID,
                30800L, 800L, null, "文旦柚 200 公斤", 30000L, "3201", "PLATFORM");

        OrderPolicySnapshot policy = capturedPolicy();
        assertThat(policy.commerceModel()).isEqualTo("SELLER_DIRECT");
        assertThat(policy.collectionModel()).isEqualTo("SELLER_MERCHANT");
        assertThat(policy.invoiceModel()).isEqualTo("SELLER_TO_BUYER");
        assertThat(policy.fulfillmentMode()).isEqualTo("SELLER_MANAGED");
    }

    @Test
    @DisplayName("PLATFORM lane 一樣不扣庫存：賣家回價不綁 SKU，沒有可扣的可售量")
    void platformLaneAlsoNeverTouchesStock() {
        orderService.createRfqOrder(100L, 7489859548420286732L, ACCEPTANCE_ID,
                30800L, 800L, null, "文旦柚 200 公斤", 30000L, "3201", "PLATFORM");

        verifyNoInteractions(productGrpcClient);
        verifyNoInteractions(stockTxRecordRepository);
    }

    @Test
    @DisplayName("PLATFORM lane 的 mer_id 取賣家子商店，不是平台代理商戶")
    void platformLaneUsesSellerMerId() {
        orderService.createRfqOrder(100L, 7489859548420286732L, ACCEPTANCE_ID,
                30800L, 800L, null, "文旦柚 200 公斤", 30000L, "3201", "PLATFORM");

        verify(txHandler).createRfqOrderInTx(anyLong(), anyLong(), anyLong(), any(),
                anyLong(), anyLong(), any(), anyString(), anyString(),
                org.mockito.ArgumentMatchers.eq("HAWK2001"), any());
    }

    @Test
    @DisplayName("未知 lane fail-closed：不猜賣方身分，因為猜錯不會有人發現")
    void unknownLaneFailsClosed() {
        assertThatThrownBy(() -> orderService.createRfqOrder(100L, SETTLEMENT_STORE_ID, ACCEPTANCE_ID,
                30800L, 800L, null, "文旦柚 200 公斤", 30000L, "3201", "WISH_POOL"))
                .isInstanceOf(OrderBusinessException.class);

        verify(txHandler, never()).createRfqOrderInTx(anyLong(), anyLong(), anyLong(), any(),
                anyLong(), anyLong(), any(), any(), any(), any(), any());
    }

    // ==================== helpers ====================

    @SuppressWarnings("unchecked")
    private List<OrderService.CreateOrderItemParam> capturedItems() {
        ArgumentCaptor<List<OrderService.CreateOrderItemParam>> captor = ArgumentCaptor.forClass(List.class);
        verify(txHandler).createRfqOrderInTx(anyLong(), anyLong(), anyLong(), captor.capture(),
                anyLong(), anyLong(), any(), anyString(), anyString(), anyString(), any());
        return captor.getValue();
    }

    private OrderPolicySnapshot capturedPolicy() {
        ArgumentCaptor<OrderPolicySnapshot> captor = ArgumentCaptor.forClass(OrderPolicySnapshot.class);
        verify(txHandler).createRfqOrderInTx(anyLong(), anyLong(), anyLong(), any(),
                anyLong(), anyLong(), any(), anyString(), anyString(), anyString(), captor.capture());
        return captor.getValue();
    }
}
