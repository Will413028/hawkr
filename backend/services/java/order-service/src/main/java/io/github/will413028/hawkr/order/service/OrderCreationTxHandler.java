package io.github.will413028.hawkr.order.service;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.CodeMetaValidator;
import io.github.will413028.hawkr.common.utils.code.InvoiceCarrierSelectionMode;
import io.github.will413028.hawkr.common.utils.code.InvoiceDocumentKind;
import io.github.will413028.hawkr.common.utils.code.OrderItemStatus;
import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import io.github.will413028.hawkr.order.checkout.InvoiceIntakeResolver;
import io.github.will413028.hawkr.order.checkout.OrderPolicySnapshot;
import io.github.will413028.hawkr.order.config.OrderConfig;
import io.github.will413028.hawkr.order.constant.OrderConstants;
import io.github.will413028.hawkr.order.entity.OrderEntity;
import io.github.will413028.hawkr.order.entity.OrderInvoiceSnapshotEntity;
import io.github.will413028.hawkr.order.entity.OrderItemEntity;
import io.github.will413028.hawkr.order.repository.OrderInvoiceSnapshotRepository;
import io.github.will413028.hawkr.order.repository.OrderItemRepository;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import io.github.will413028.hawkr.order.repository.StockTxRecordRepository;
import io.github.will413028.hawkr.order.service.OrderService.CreateOrderItemParam;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.util.List;

/**
 * 建單的交易性處理（03 §5.1 階段「DB Transaction 建單」）。
 *
 * 從 OrderService 獨立出來，確保 Spring @Transactional proxy 正常運作
 * （原 OrderService.createOrder → this.createOrderInTx / createOrderInTxExtended 為
 * self-invocation，不經 proxy，@Transactional 實際失效——orders / order_items /
 * stock_tx_records 三寫入曾非原子）。⚠️ 結帳 hot path：僅改事務載體，邏輯原封搬移。
 */
@Service
public class OrderCreationTxHandler {

    private final OrderRepository orderRepository;
    private final OrderItemRepository orderItemRepository;
    private final StockTxRecordRepository stockTxRecordRepository;
    private final OrderInvoiceSnapshotRepository orderInvoiceSnapshotRepository;
    private final OrderNoGenerator orderNoGenerator;
    private final OrderConfig orderConfig;

    public OrderCreationTxHandler(OrderRepository orderRepository,
                                  OrderItemRepository orderItemRepository,
                                  StockTxRecordRepository stockTxRecordRepository,
                                  OrderInvoiceSnapshotRepository orderInvoiceSnapshotRepository,
                                  OrderNoGenerator orderNoGenerator,
                                  OrderConfig orderConfig) {
        this.orderRepository = orderRepository;
        this.orderItemRepository = orderItemRepository;
        this.stockTxRecordRepository = stockTxRecordRepository;
        this.orderInvoiceSnapshotRepository = orderInvoiceSnapshotRepository;
        this.orderNoGenerator = orderNoGenerator;
        this.orderConfig = orderConfig;
    }

    @Transactional
    public String createOrderInTx(
            long buyerId,
            long storeId,
            List<CreateOrderItemParam> items,
            long totalOriginalAmount,
            long totalActualAmount,
            String stockTxId,
            String checkoutId,
            String deliveryTemp,
            String shippingMethod,
            long shippingFee,
            String shippingFeePayer,
            OrderPolicySnapshot policySnapshot
    ) {
        String orderNo = orderNoGenerator.generateOrderNo();

        // 插入 orders
        OrderEntity order = new OrderEntity();
        order.setId(TsidUtil.generate());
        order.setOrderNo(orderNo);
        order.setBuyerId(buyerId);
        order.setStoreId(storeId);
        order.setTotalOriginalAmount(totalOriginalAmount);
        order.setTotalActualAmount(totalActualAmount);
        order.setStatus(OrderStatus.PENDING_PAYMENT.value());
        order.setCheckoutId(checkoutId);
        order.setDeliveryTemp(deliveryTemp);
        order.setShippingMethod(shippingMethod);
        order.setShippingFee(shippingFee);
        order.setShippingFeePayer(shippingFeePayer);
        order.setPaymentDeadline(DateTimeUtils.nowUtc().plusSeconds(orderConfig.getPaymentDeadlineSeconds()));
        order.setPaymentAttempts(0);
        applyPolicySnapshot(order, policySnapshot);
        orderRepository.insert(order);

        // 插入 order_items
        insertOrderItems(orderNo, items);

        // 58 號 §7.1.2 步驟 1：本路徑（購物車外建單，gRPC CreateOrder）沒有採集入口，
        // 一律落缺省快照。**不能不寫**：invoice-service 對「查無快照」是 fail-loud
        // （order.paid 重試三次進 DLQ），少了這一列，經本路徑建立的訂單每一張都會誤觸發它。
        insertInvoiceSnapshot(orderNo, InvoiceIntakeResolver.defaultSnapshot());

        // 更新協調紀錄
        stockTxRecordRepository.updateStatusAndOrderNo(stockTxId, OrderConstants.STOCK_TX_PHASE_DEDUCT, OrderConstants.STOCK_TX_STATUS_SUCCESS, orderNo);

        return orderNo;
    }

    @Transactional
    public String createOrderInTxExtended(
            long buyerId, long storeId,
            List<CreateOrderItemParam> items,
            long totalOriginalAmount, long totalActualAmount,
            String stockTxId, String checkoutId,
            String deliveryTemp, String shippingMethod,
            long shippingFee, String shippingFeePayer,
            String recipientName, String recipientPhone,
            String shippingAddress, String shippingNote,
            String orderRemark, String cvsStoreId, String cvsStoreName,
            String shippingArea, String senderCity, String senderDistrict,
            int dimensionSum, String merId,
            long discountAmount,
            InvoiceIntakeResolver.Snapshot invoiceSnapshot,
            OrderPolicySnapshot policySnapshot
    ) {
        String orderNo = orderNoGenerator.generateOrderNo();

        OrderEntity order = new OrderEntity();
        order.setId(TsidUtil.generate());
        order.setOrderNo(orderNo);
        order.setBuyerId(buyerId);
        order.setStoreId(storeId);
        order.setTotalOriginalAmount(totalOriginalAmount);
        order.setTotalActualAmount(totalActualAmount);
        order.setStatus(OrderStatus.PENDING_PAYMENT.value());
        order.setCheckoutId(checkoutId);
        order.setDeliveryTemp(deliveryTemp);
        order.setShippingMethod(shippingMethod);
        order.setShippingFee(shippingFee);
        order.setShippingFeePayer(shippingFeePayer);
        // 結帳模組新增欄位
        order.setRecipientName(recipientName);
        order.setRecipientPhone(recipientPhone);
        order.setShippingAddress(shippingAddress);
        order.setShippingNote(shippingNote);
        order.setOrderRemark(orderRemark);
        order.setCvsStoreId(cvsStoreId);
        order.setCvsStoreName(cvsStoreName);
        order.setShippingArea(shippingArea);
        order.setSenderCity(senderCity);
        order.setSenderDistrict(senderDistrict);
        if (dimensionSum > 0) {
            order.setDimensionSum(dimensionSum);
        }
        order.setMerId(merId);
        // couponInstanceId 不在建單時寫入，由 CheckoutService.reserveCoupon 成功後 updateCouponInstanceId 回寫
        order.setDiscountAmount(discountAmount);
        order.setPaymentDeadline(DateTimeUtils.nowUtc().plusSeconds(orderConfig.getPaymentDeadlineSeconds()));
        order.setPaymentAttempts(0);
        applyPolicySnapshot(order, policySnapshot);
        orderRepository.insert(order);

        insertOrderItems(orderNo, items);

        // 58 號 §7.1.2 步驟 1：發票快照與 orders / order_items 同一交易。
        // 🔴 這裡是「付款之前」——結帳確認建的是 PENDING_PAYMENT 訂單，付款成功要等
        // payment.success webhook。發票資料若晚於付款才落地，invoice-service 的
        // order.paid consumer 會讀到空快照，而發票必須在付款成功即開立（§6.2.4 ③）。
        insertInvoiceSnapshot(orderNo, invoiceSnapshot);

        stockTxRecordRepository.updateStatusAndOrderNo(stockTxId,
                OrderConstants.STOCK_TX_PHASE_DEDUCT, OrderConstants.STOCK_TX_STATUS_SUCCESS, orderNo);

        return orderNo;
    }

    /**
     * EXTERNAL lane RFQ 成交建單（64 號 §6.4）。
     *
     * <p><b>為什麼是另一支而不是給 {@code createOrderInTx} 加參數</b>：兩者的差別不是
     * 欄位多寡，是<b>有沒有庫存協調這件事</b>。既有兩支的結尾都必然
     * {@code stockTxRecordRepository.updateStatusAndOrderNo(stockTxId, …)}——那不是可選
     * 的一步，是「這張訂單扣過庫存」這個事實的落點。RFQ 沒有那個事實（貨在站外業者
     * 手上，平台不持有可售數量），塞一個 null 進去只會讓那一行變成一個要靠讀者記得
     * 的例外。三個私有 helper（{@code applyPolicySnapshot}／{@code insertOrderItems}／
     * {@code insertInvoiceSnapshot}）照樣共用，重複的只有欄位賦值。
     *
     * <p>⛔ {@code checkoutId} 恆為 null：RFQ 沒有購物車批次，而那一欄非空會在上游
     * 觸發購物車鎖。冪等鍵是 {@code acceptanceId}（{@code uk_orders_acceptance_id}）。
     *
     * @return 建好的訂單（呼叫端需要 {@code orderNo} 與 {@code paymentDeadline}）
     */
    @Transactional
    public OrderEntity createRfqOrderInTx(
            long buyerId, long storeId, long acceptanceId,
            List<CreateOrderItemParam> items,
            long totalAmount, long shippingFee,
            String deliveryTemp, String shippingMethod, String shippingFeePayer,
            String merId,
            OrderPolicySnapshot policySnapshot
    ) {
        String orderNo = orderNoGenerator.generateOrderNo();

        OrderEntity order = new OrderEntity();
        order.setId(TsidUtil.generate());
        order.setOrderNo(orderNo);
        order.setBuyerId(buyerId);
        order.setStoreId(storeId);
        order.setAcceptanceId(acceptanceId);
        // RFQ 沒有優惠券：原始金額 ＝ 實付金額，discount_amount 留 DB 預設 0。
        order.setTotalOriginalAmount(totalAmount);
        order.setTotalActualAmount(totalAmount);
        order.setStatus(OrderStatus.PENDING_PAYMENT.value());
        order.setDeliveryTemp(deliveryTemp);
        order.setShippingMethod(shippingMethod);
        order.setShippingFee(shippingFee);
        order.setShippingFeePayer(shippingFeePayer);
        order.setMerId(merId);
        // 64 號 §6.4.11：RFQ 的付款期限是建單 +12h，不是結帳路徑的 24h。
        order.setPaymentDeadline(DateTimeUtils.nowUtc().plusSeconds(orderConfig.getRfqPaymentDeadlineSeconds()));
        order.setPaymentAttempts(0);
        applyPolicySnapshot(order, policySnapshot);
        orderRepository.insert(order);

        insertOrderItems(orderNo, items);

        // 58 號 §7.1.2 步驟 1：**不能不寫**。invoice-service 對「查無快照」是 fail-loud
        // （order.paid 重試三次進 DLQ），少了這一列，經本路徑建立的訂單每一張都會誤觸發它。
        // RFQ 沒有發票採集入口（買家還沒進結帳），一律落缺省快照。
        insertInvoiceSnapshot(orderNo, InvoiceIntakeResolver.defaultSnapshot());

        return order;
    }

    /**
     * 寫入 {@code order_invoice_snapshots}（{@code snapshot_version = 1}）。
     *
     * <p>{@code invoiceSnapshot} 恆非 null（{@link InvoiceIntakeResolver#resolve} 對「未帶
     * invoice 區塊」回 {@code defaultSnapshot()}）⇒ <b>每一張經結帳建立的訂單都有一列快照</b>。
     * 這是刻意的：沒有快照與「買家選了無載具」在 invoice-service 端分不開，而
     * {@code GetOrderInvoiceSnapshotInternal} 對查無快照必須回非 0 讓事件進 DLQ
     * ——若舊 App 的訂單都沒有快照，那條 fail-loud 就會對每一張訂單誤觸發。
     */
    private void insertInvoiceSnapshot(String orderNo, InvoiceIntakeResolver.Snapshot snapshot) {
        // §8：對應 CodeMeta Enum 的欄位寫入 DB 前必須走共用 validator。值雖然由
        // InvoiceIntakeResolver 以 enum 產生，但這裡是 order_invoice_snapshots 的**唯一寫入點**，
        // 驗證放在收斂點才擋得住未來新增的呼叫端。
        CodeMetaValidator.requireValidValue(InvoiceDocumentKind.class, snapshot.invoiceKind(), "invoiceKind");
        CodeMetaValidator.requireValidValue(InvoiceCarrierSelectionMode.class,
                snapshot.carrierSelectionMode(), "carrierSelectionMode");

        OrderInvoiceSnapshotEntity entity = new OrderInvoiceSnapshotEntity();
        entity.setId(TsidUtil.generate());
        entity.setOrderNo(orderNo);
        entity.setSnapshotVersion(1);
        entity.setInvoiceKind(snapshot.invoiceKind());
        entity.setCarrierSelectionMode(snapshot.carrierSelectionMode());
        entity.setBuyerInvoicePreferenceId(snapshot.buyerInvoicePreferenceId());
        entity.setBuyerTaxId(snapshot.buyerTaxId());
        entity.setBuyerTitle(snapshot.buyerTitle());
        entity.setDonationCode(snapshot.donationCode());
        entity.setNotifyEmail(snapshot.notifyEmail());
        orderInvoiceSnapshotRepository.insert(entity);
    }

    /** 兩條建單路徑共用的 order_items 寫入（邏輯與原 OrderService 逐行一致）。 */
    private void insertOrderItems(String orderNo, List<CreateOrderItemParam> items) {
        for (CreateOrderItemParam item : items) {
            OrderItemEntity itemEntity = new OrderItemEntity();
            itemEntity.setId(TsidUtil.generate());
            itemEntity.setOrderNo(orderNo);
            itemEntity.setProductId(item.productId());
            itemEntity.setProductName(item.productName());
            itemEntity.setSkuId(item.skuId());
            itemEntity.setSkuName(item.skuName());
            itemEntity.setQuantity(item.quantity());
            itemEntity.setListPrice(item.listPrice());
            itemEntity.setUnitPrice(item.unitPrice());
            // 03 §3.2.1：actual_pay_amount = unit_price × qty − share_discount
            itemEntity.setShareDiscount(item.shareDiscount());
            itemEntity.setActualPayAmount(item.unitPrice() * item.quantity() - item.shareDiscount());
            itemEntity.setRefundStatus(OrderItemStatus.NORMAL.value());
            itemEntity.setImageObjectKey(item.imageObjectKey());
            orderItemRepository.insert(itemEntity);
        }
    }

    /**
     * 訂單政策五欄的<b>唯一寫入點</b>——兩條建單路徑都經過這裡。
     *
     * <p>🔴 <b>為什麼要抽成一支</b>：兩條路徑各自 setter 一次，等於兩份會漂移的清單；
     * 而漏掉其中一欄是完全無聲的（MyBatis-Plus 的 {@code insertStrategy=NOT_NULL}
     * 讓 null 欄位根本不進 INSERT ⇒ 直接吃 DB DEFAULT，沒有錯誤、沒有 CHECK 會擋）。
     * 新增第六個政策欄時，改這一支就是全部。
     *
     * <p>五欄恆非 null——來源是 {@code OrderPolicyResolver}，它對算不出來的輸入直接拋。
     */
    private static void applyPolicySnapshot(OrderEntity order, OrderPolicySnapshot policy) {
        order.setCommerceModel(policy.commerceModel());
        order.setCollectionModel(policy.collectionModel());
        order.setFulfillmentMode(policy.fulfillmentMode());
        order.setInvoiceModel(policy.invoiceModel());
        order.setPolicyVersion(policy.policyVersion());
    }
}
