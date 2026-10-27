package io.github.will413028.hawkr.order.grpc.converter;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import io.github.will413028.hawkr.common.utils.proto.ProtoConversionUtils;
import io.github.will413028.hawkr.order.entity.*;
import io.github.will413028.hawkr.order.grpc.*;
import io.github.will413028.hawkr.payment.grpc.PaymentDetail;
import io.github.will413028.hawkr.buyer.grpc.BuyerDisplay;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import io.github.will413028.hawkr.shipping.grpc.ShipmentDetailView;

import java.time.LocalDateTime;
import java.time.format.DateTimeFormatter;
import java.util.List;

/**
 * Entity <-> Proto 轉換器
 */
public final class OrderProtoConverter {

    private OrderProtoConverter() {}

    /** 評論期限天數（與 review-service domain.ReviewWindowDays 保持一致） */
    private static final int REVIEW_WINDOW_DAYS = 30;

    // ==================== OrderSummaryView ====================

    public static OrderSummaryView toSummaryView(OrderEntity order, List<OrderItemEntity> items,
                                                   SellerMeta sellerMeta, BuyerDisplay buyerDisplay,
                                                   String cdnBaseUrl, boolean isSellerView) {
        return toSummaryView(order, items, sellerMeta, buyerDisplay, cdnBaseUrl, isSellerView, null);
    }

    /**
     * 賣家列表頁專用：傳入即時 shipment（避免快照延遲）
     *
     * @param liveShipment 由賣家列表 handler 透過 batch RPC 取得，null 時 fallback 到快照
     */
    public static OrderSummaryView toSummaryView(OrderEntity order, List<OrderItemEntity> items,
                                                   SellerMeta sellerMeta, BuyerDisplay buyerDisplay,
                                                   String cdnBaseUrl, boolean isSellerView,
                                                   ShipmentDetailView liveShipment) {
        OrderSummaryView.Builder builder = OrderSummaryView.newBuilder()
                .setOrderNo(order.getOrderNo())
                .setCreateTime(formatIso8601(order.getCreatedAt()))
                .setStatus(order.getStatus())
                .setTotalActualAmount(ProtoConversionUtils.formatLong(order.getTotalActualAmount()))
                .setItemCount(ProtoConversionUtils.formatInt(items.size()))
                .setStoreId(ProtoConversionUtils.formatLong(order.getStoreId()))
                .setShippingMethod(order.getShippingMethod() != null ? order.getShippingMethod() : "")
                .setDeliveryTemp(order.getDeliveryTemp() != null ? order.getDeliveryTemp() : "")
                .setPickupStoreName(order.getCvsStoreName() != null ? order.getCvsStoreName() : "")
                // 付款方式（PaymentMethod value 2001-2004；未付款為空字串。admin 交易列表顯示用）
                .setPaymentMethod(order.getPaymentMethod() != null ? order.getPaymentMethod() : "")
                // 狀態摘要
                .setShippingStatus(order.getShippingStatus() != null
                        ? order.getShippingStatus() : "")
                .setShippingStatusDesc(order.getShippingStatusDesc() != null ? order.getShippingStatusDesc() : "")
                .setStatusTime(resolveStatusTime(order))
                .setStatusSummary(resolveStatusSummary(order))
                // 賣家訂單列表擴充欄位
                .setRecipientName(order.getRecipientName() != null ? order.getRecipientName() : "")
                .setShippingAddress(order.getShippingAddress() != null ? order.getShippingAddress() : "")
                .setBuyerId(ProtoConversionUtils.formatLong(order.getBuyerId()));

        // 物流出貨資訊（僅賣家/管理員視角，買家不可見）
        if (isSellerView) {
            String liveLabelKey = liveShipment != null ? liveShipment.getLabelKey() : "";
            String livePartnerId = liveShipment != null ? liveShipment.getPartnerId() : "";
            String liveOdno = liveShipment != null ? liveShipment.getOdno() : "";
            String liveValidationNo = liveShipment != null ? liveShipment.getValidationNo() : "";

            // 49 §6.6：只填 objectKey，Gateway 組 labelUrl（order.labelUrl 欄快照值為 key，見 ShippingEventConsumer）
            builder.setLabelKey(firstNonBlank(liveLabelKey, order.getLabelUrl()))
                    .setLabelExpireAt(order.getLabelExpireAt() != null ? order.getLabelExpireAt().toString() : "")
                    .setPartnerId(firstNonBlank(livePartnerId, order.getPartnerId()))
                    .setOdno(firstNonBlank(liveOdno, order.getOdno()))
                    .setValidationNo(firstNonBlank(liveValidationNo, order.getValidationNo()));
        }

        // 出貨照片：賣家視角優先用 liveShipment（即時，避免賣家剛上傳的延遲），其餘走快照。
        // proto shipment_photo_urls 欄名沿用、值一律 objectKey（liveShipment 與快照皆已改帶 key，49 §6.2）；Gateway 組顯示 URL
        if (isSellerView && liveShipment != null && liveShipment.getShipmentPhotoKeysCount() > 0) {
            builder.addAllShipmentPhotoKeys(liveShipment.getShipmentPhotoKeysList());
        } else if (order.getShipmentPhotoKeys() != null && !order.getShipmentPhotoKeys().isEmpty()) {
            builder.addAllShipmentPhotoKeys(order.getShipmentPhotoKeys());
        }

        // 賣家資訊聚合（即時從 Seller Service 取得）
        if (sellerMeta != null) {
            builder.setSellerName(sellerMeta.getSellerName())
                    .setSellerAvatarKey(sellerMeta.getLogoKey()) // 49 §6.6 S9：只填 key，Gateway 組 sellerAvatarUrl
                    // 賣家頁 canonical 網址走 store_code；值已在同一次 batchGetSellerMeta 回應裡
                    .setStoreCode(sellerMeta.getStoreCode())
                    .setSellerCity(order.getSenderCity() != null ? order.getSenderCity() : "")
                    .setSellerDistrict(order.getSenderDistrict() != null ? order.getSenderDistrict() : "");
        }

        // 買家資訊聚合（賣家/管理員視角，即時從 Buyer Service 取得）
        if (buyerDisplay != null) {
            builder.setBuyerNickname(buyerDisplay.getDisplayName())
                    .setBuyerAvatarKey(buyerDisplay.getAvatarKey()); // 49 §6.6 S2+S9：只填 key，Gateway 組 buyerAvatarUrl
        }

        // need_refund_page 標記（ATM 退款等待買家前往 PayUni 頁面）
        if (order.getNeedRefundPage() != null && order.getNeedRefundPage()) {
            builder.setNeedRefundPage(true);
        }

        // is_reviewed 標記（任一商品已評論即 true）
        if (order.getIsReviewed() != null && order.getIsReviewed()) {
            builder.setIsReviewed(true);
        }

        // 評論截止時間 & 可否評論（僅 COMPLETED(50) 時計算）
        if (OrderStatus.COMPLETED.value().equals(order.getStatus())) {
            LocalDateTime completedAt = order.getConfirmedAt() != null
                    ? order.getConfirmedAt() : order.getCreatedAt();
            LocalDateTime deadline = completedAt.plusDays(REVIEW_WINDOW_DAYS);
            builder.setReviewDeadline(formatIso8601(deadline));
            builder.setCanReview(DateTimeUtils.nowUtc().isBefore(deadline));
        }

        // 自動完成期限（僅 DELIVERED(40) 時有值）
        if (order.getAutoCompleteAt() != null) {
            builder.setAutoCompleteAt(formatIso8601(order.getAutoCompleteAt()));
        }

        // 倒數暫停時刻（03 §5.8；NULL=倒數中=空字串）
        if (order.getCountdownPausedAt() != null) {
            builder.setCountdownPausedAt(formatIso8601(order.getCountdownPausedAt()));
        }

        // §5.9 賣家處理期限（賣家未處理 + 配送方式非 Uber + 訂單未取消時有值）
        if (order.getProcessingDeadline() != null) {
            builder.setProcessingDeadline(formatIso8601(order.getProcessingDeadline()));
        }

        // 03 §8.5 付款期限（僅 PENDING_PAYMENT(0) 時對外暴露；買家列表「待付款倒數」卡片用，免逐筆打詳情 API）
        if (OrderStatus.PENDING_PAYMENT.value().equals(order.getStatus()) && order.getPaymentDeadline() != null) {
            builder.setPaymentDeadline(formatIso8601(order.getPaymentDeadline()));
        }

        // 03 §5.3：取消資訊（僅 status=80 時有值；buyer 自願取消無 reason）
        if (order.getCancelReason() != null) {
            builder.setCancelReason(order.getCancelReason());
        }
        if (order.getCancelRemark() != null) {
            builder.setCancelRemark(order.getCancelRemark());
        }
        if (order.getCancelByRole() != null) {
            builder.setCancelByRole(order.getCancelByRole());
        }

        for (OrderItemEntity item : items) {
            OrderItemView.Builder itemBuilder = OrderItemView.newBuilder()
                    .setId(ProtoConversionUtils.formatLong(item.getId()))
                    .setSkuId(ProtoConversionUtils.formatLong(item.getSkuId()))
                    .setSkuName(item.getSkuName())
                    .setQuantity(ProtoConversionUtils.formatInt(item.getQuantity()))
                    .setUnitPrice(ProtoConversionUtils.formatLong(item.getUnitPrice()))
                    .setListPrice(ProtoConversionUtils.formatLong(item.getListPrice()))
                    .setImageKey(item.getImageObjectKey() != null ? item.getImageObjectKey() : ""); // 49 §6.6：只填 key，Gateway 組 imageUrl
            if (item.getProductId() != null) {
                itemBuilder.setProductId(ProtoConversionUtils.formatLong(item.getProductId()));
            }
            if (item.getProductName() != null) {
                itemBuilder.setProductName(item.getProductName());
            }
            builder.addItems(itemBuilder.build());
        }
        return builder.build();
    }

    // ==================== OrderDetailView ====================

    public static OrderDetailView toDetailView(OrderEntity order, List<OrderItemEntity> items,
                                                List<RefundRequestEntity> refunds, ShipmentDetailView shipment,
                                                SellerMeta sellerMeta, BuyerDisplay buyerDisplay,
                                                String cdnBaseUrl, boolean isSellerView) {
        OrderDetailView.Builder builder = OrderDetailView.newBuilder()
                .setOrderNo(order.getOrderNo())
                .setCreateTime(formatIso8601(order.getCreatedAt()))
                .setPayTime(formatIso8601(order.getPayTime()))
                .setShipTime(formatIso8601(order.getShipTime()))
                .setStatus(order.getStatus())
                .setTotalOriginalAmount(ProtoConversionUtils.formatLong(order.getTotalOriginalAmount()))
                .setTotalActualAmount(ProtoConversionUtils.formatLong(order.getTotalActualAmount()))
                .setShipmentNo(order.getShipmentNo() != null ? order.getShipmentNo() : "")
                .setTransactionId(order.getTransactionId() != null ? order.getTransactionId() : "")
                .setBuyerId(ProtoConversionUtils.formatLong(order.getBuyerId()))
                .setStoreId(ProtoConversionUtils.formatLong(order.getStoreId()))
                .setDeliveryTemp(order.getDeliveryTemp() != null ? order.getDeliveryTemp() : "")
                .setShippingMethod(order.getShippingMethod() != null ? order.getShippingMethod() : "")
                .setShippingFee(ProtoConversionUtils.formatAmount(order.getShippingFee()))
                .setShippingFeePayer(order.getShippingFeePayer() != null ? order.getShippingFeePayer() : "")
                // 收件人資訊快照（field 18-21）
                .setRecipientName(order.getRecipientName() != null ? order.getRecipientName() : "")
                .setRecipientPhone(order.getRecipientPhone() != null ? order.getRecipientPhone() : "")
                .setShippingAddress(order.getShippingAddress() != null ? order.getShippingAddress() : "")
                .setShippingNote(order.getShippingNote() != null ? order.getShippingNote() : "")
                // 訂單備註（field 48）
                .setOrderRemark(order.getOrderRemark() != null ? order.getOrderRemark() : "")
                // 超商取貨快照（field 22-23）
                .setCvsStoreId(order.getCvsStoreId() != null ? order.getCvsStoreId() : "")
                .setPickupStoreName(order.getCvsStoreName() != null ? order.getCvsStoreName() : "")
                // 結帳模組新增（field 24-26）
                .setShippingArea(order.getShippingArea() != null ? order.getShippingArea() : "")
                .setCouponInstanceId(order.getCouponInstanceId() != null ? order.getCouponInstanceId() : "")
                .setDiscountAmount(ProtoConversionUtils.formatAmount(order.getDiscountAmount()))
                // 物流狀態（field 27-28，Order 快照 fallback）
                .setShippingStatus(order.getShippingStatus() != null
                        ? order.getShippingStatus() : "")
                .setShippingStatusDesc(order.getShippingStatusDesc() != null ? order.getShippingStatusDesc() : "")
                .setLabelKey(order.getLabelUrl() != null ? order.getLabelUrl() : "") // 49 §6.6：labelUrl 欄快照值為 key，Gateway 組
                // 付款方式 code（field 32，來自 Order DB payment_method，如 CREDIT_CARD / ATM）
                .setPaymentMethod(order.getPaymentMethod() != null ? order.getPaymentMethod() : "")
                // 付款 PayUni MerID（field 61，給 Web/Mobile Google Pay 重試動態取得 gatewayMerchantId，20 §4.13）
                .setPaymentMerId(order.getMerId() != null ? order.getMerId() : "");

        // 物流資訊聚合（以 Shipping Service 即時資料為準，覆蓋 Order 快照）
        // shippingStatusDesc 不從 shipment 覆蓋，統一用 orders 快照值（MQ 事件已即時更新）
        if (shipment != null) {
            builder.setShippingStatus(shipment.getStatus());
            // 出貨單號 / PDF / 超商出貨便代碼僅賣家視角填充（不給買家看）
            if (isSellerView) {
                builder.setLabelKey(shipment.getLabelKey())
                        .setPartnerId(shipment.getPartnerId())
                        .setOdno(shipment.getOdno())
                        .setValidationNo(shipment.getValidationNo());
            }
        }

        // 出貨照片（雙方詳情頁可見，統一從 orders 快照取，不再走 shipment 即時值）
        // proto 欄名沿用、值為 objectKey；Gateway 組顯示 URL（49 §6.2）
        if (order.getShipmentPhotoKeys() != null && !order.getShipmentPhotoKeys().isEmpty()) {
            builder.addAllShipmentPhotoKeys(order.getShipmentPhotoKeys());
        }

        // 買家收貨照片（雙方詳情頁可見，於 ConfirmReceipt 寫入快照；值為 objectKey）
        if (order.getReceiptPhotoKeys() != null && !order.getReceiptPhotoKeys().isEmpty()) {
            builder.addAllReceiptPhotoKeys(order.getReceiptPhotoKeys());
        }

        // ATM 付款資訊快照（建單時寫入 Order DB，僅 ATM 付款時有值）
        if (order.getAtmBankCode() != null) {
            builder.setAtmBankCode(order.getAtmBankCode());
        }
        if (order.getAtmVirtualAccount() != null) {
            builder.setAtmVirtualAccount(order.getAtmVirtualAccount());
        }
        if (order.getAtmExpireDate() != null) {
            builder.setAtmExpireDate(order.getAtmExpireDate());
        }

        // 賣家資訊聚合（即時從 Seller Service 取得）
        if (sellerMeta != null) {
            builder.setSellerName(sellerMeta.getSellerName())
                    .setSellerAvatarKey(sellerMeta.getLogoKey()) // 49 §6.6 S9：只填 key，Gateway 組 sellerAvatarUrl
                    // 賣家頁 canonical 網址走 store_code；值已在同一次 batchGetSellerMeta 回應裡
                    .setStoreCode(sellerMeta.getStoreCode())
                    .setSellerCity(order.getSenderCity() != null ? order.getSenderCity() : "")
                    .setSellerDistrict(order.getSenderDistrict() != null ? order.getSenderDistrict() : "");
        }

        // 買家資訊聚合（賣家/管理員視角，即時從 Buyer Service 取得）
        if (buyerDisplay != null) {
            builder.setBuyerNickname(buyerDisplay.getDisplayName())
                    .setBuyerAvatarKey(buyerDisplay.getAvatarKey()); // 49 §6.6 S2+S9：只填 key，Gateway 組 buyerAvatarUrl
        }

        for (OrderItemEntity item : items) {
            OrderItemDetail.Builder itemBuilder = OrderItemDetail.newBuilder()
                    .setId(ProtoConversionUtils.formatLong(item.getId()))
                    .setSkuId(ProtoConversionUtils.formatLong(item.getSkuId()))
                    .setSkuName(item.getSkuName())
                    .setQuantity(ProtoConversionUtils.formatInt(item.getQuantity()))
                    .setUnitPrice(ProtoConversionUtils.formatLong(item.getUnitPrice()))
                    .setListPrice(ProtoConversionUtils.formatLong(item.getListPrice()))
                    .setActualPayAmount(ProtoConversionUtils.formatLong(item.getActualPayAmount()))
                    .setRefundStatus(item.getRefundStatus())
                    .setShareDiscount(ProtoConversionUtils.formatAmount(item.getShareDiscount()))
                    .setImageKey(item.getImageObjectKey() != null ? item.getImageObjectKey() : ""); // 49 §6.6：只填 key，Gateway 組 imageUrl
            if (item.getProductId() != null) {
                itemBuilder.setProductId(ProtoConversionUtils.formatLong(item.getProductId()));
            }
            if (item.getProductName() != null) {
                itemBuilder.setProductName(item.getProductName());
            }
            builder.addItems(itemBuilder.build());
        }

        // need_refund_page 標記（ATM 退款等待買家前往 PayUni 頁面）
        if (order.getNeedRefundPage() != null && order.getNeedRefundPage()) {
            builder.setNeedRefundPage(true);
        }

        // is_reviewed 標記（任一商品已評論即 true）
        if (order.getIsReviewed() != null && order.getIsReviewed()) {
            builder.setIsReviewed(true);
        }

        // 評論截止時間 & 可否評論（僅 COMPLETED(50) 時計算）
        if (OrderStatus.COMPLETED.value().equals(order.getStatus())) {
            LocalDateTime completedAt = order.getConfirmedAt() != null
                    ? order.getConfirmedAt() : order.getCreatedAt();
            LocalDateTime deadline = completedAt.plusDays(REVIEW_WINDOW_DAYS);
            builder.setReviewDeadline(formatIso8601(deadline));
            builder.setCanReview(DateTimeUtils.nowUtc().isBefore(deadline));
        }

        // 自動完成期限（僅 DELIVERED(40) 時有值）
        if (order.getAutoCompleteAt() != null) {
            builder.setAutoCompleteAt(formatIso8601(order.getAutoCompleteAt()));
        }

        // 倒數暫停時刻（03 §5.8；NULL=倒數中=空字串）
        if (order.getCountdownPausedAt() != null) {
            builder.setCountdownPausedAt(formatIso8601(order.getCountdownPausedAt()));
        }

        // §5.9 賣家處理期限（賣家未處理 + 配送方式非 Uber + 訂單未取消時有值）
        if (order.getProcessingDeadline() != null) {
            builder.setProcessingDeadline(formatIso8601(order.getProcessingDeadline()));
        }

        // 03 §8.5 付款期限（僅 PENDING_PAYMENT(0) 時對外暴露；買家詳情頁顯示「待付款倒數」用）
        if (OrderStatus.PENDING_PAYMENT.value().equals(order.getStatus()) && order.getPaymentDeadline() != null) {
            builder.setPaymentDeadline(formatIso8601(order.getPaymentDeadline()));
        }

        // 03 §5.3：取消資訊（僅 status=80 時有值；buyer 自願取消無 reason）
        if (order.getCancelReason() != null) {
            builder.setCancelReason(order.getCancelReason());
        }
        if (order.getCancelRemark() != null) {
            builder.setCancelRemark(order.getCancelRemark());
        }
        if (order.getCancelByRole() != null) {
            builder.setCancelByRole(order.getCancelByRole());
        }

        // 結案節點時間（訂單時間軸的最後一格）。formatIso8601 對 null 回空字串，故不另加守衛。
        // completed_at 取 confirmed_at：40→50 的轉換一次寫入 status 與 confirmed_at
        // （completeDeliveredOrderByStatus），全 repo 也只有那一處寫它 ⇒ 非 null 即等於已完成。
        builder.setCompletedAt(formatIso8601(order.getConfirmedAt()))
                .setCancelledAt(formatIso8601(order.getCancelledAt()))
                .setDeliveredAt(formatIso8601(order.getDeliveredAt()));

        for (RefundRequestEntity refund : refunds) {
            builder.addRefundHistory(RefundRequestView.newBuilder()
                    .setRequestNo(refund.getRequestNo())
                    .setOrderItemId(ProtoConversionUtils.formatLong(refund.getOrderItemId()))
                    .setStatus(refund.getStatus())
                    .setRefundAmount(ProtoConversionUtils.formatLong(refund.getRefundAmount()))
                    .setReason(refund.getReason() != null ? refund.getReason() : "")
                    .setCreateTime(formatIso8601(refund.getCreatedAt()))
                    .build());
        }

        // 訂單層級評論快照（訂單詳情單一評論顯示，取最近一筆商品評論；未評論時不設定）
        applyOrderReview(builder, order.getReview());

        return builder.build();
    }

    // ==================== OrderAfterSalesView ====================

    /**
     * 訂單售後查詢視圖（AfterSales Service 用）
     *
     * buyer_name/buyer_phone 等欄位映射自 orders 表的 recipient 欄位。
     * shipping_address 是完整地址字串，city/district/postal_code 在 orders 表中未拆分儲存，
     * 故 buyer_city/buyer_district/buyer_postal_code 回傳空字串，由 AfterSales 端自行判斷。
     */
    public static OrderAfterSalesView toAfterSalesView(OrderEntity order, List<OrderItemEntity> items,
                                                        String cdnBaseUrl) {
        OrderAfterSalesView.Builder builder = OrderAfterSalesView.newBuilder()
                .setOrderNo(order.getOrderNo())
                .setBuyerId(ProtoConversionUtils.formatLong(order.getBuyerId()))
                .setStoreId(ProtoConversionUtils.formatLong(order.getStoreId()))
                .setStatus(order.getStatus())
                .setTotalActualAmount(ProtoConversionUtils.formatLong(order.getTotalActualAmount()))
                .setShippingFee(ProtoConversionUtils.formatAmount(order.getShippingFee()))
                .setShippingFeePayer(order.getShippingFeePayer() != null ? order.getShippingFeePayer() : "")
                .setPaymentMethod(order.getPaymentMethod() != null ? order.getPaymentMethod() : "")
                .setTransactionId(order.getTransactionId() != null ? order.getTransactionId() : "")
                .setDeliveredAt(formatIso8601(order.getDeliveredAt()))
                .setShippingMethod(order.getShippingMethod() != null ? order.getShippingMethod() : "")
                .setDeliveryTemp(order.getDeliveryTemp() != null ? order.getDeliveryTemp() : "")
                .setMerId(order.getMerId() != null ? order.getMerId() : "")
                // 買家收件資訊（映射自 orders 表的 recipient 欄位）
                .setBuyerName(order.getRecipientName() != null ? order.getRecipientName() : "")
                .setBuyerPhone(order.getRecipientPhone() != null ? order.getRecipientPhone() : "")
                .setBuyerAddressDetail(order.getShippingAddress() != null ? order.getShippingAddress() : "")
                .setNeedRefundPage(order.getNeedRefundPage() != null && order.getNeedRefundPage());

        // 出貨照片快照（值為 objectKey；aftersales 透傳、Gateway 組顯示 URL）
        if (order.getShipmentPhotoKeys() != null && !order.getShipmentPhotoKeys().isEmpty()) {
            builder.addAllShipmentPhotoKeys(order.getShipmentPhotoKeys());
        }

        // 訂單明細
        for (OrderItemEntity item : items) {
            builder.addItems(OrderItemForAfterSales.newBuilder()
                    .setId(ProtoConversionUtils.formatLong(item.getId()))
                    .setSkuId(ProtoConversionUtils.formatLong(item.getSkuId()))
                    .setSkuName(item.getSkuName())
                    .setQuantity(ProtoConversionUtils.formatInt(item.getQuantity()))
                    .setUnitPrice(ProtoConversionUtils.formatLong(item.getUnitPrice()))
                    .setListPrice(ProtoConversionUtils.formatLong(item.getListPrice()))
                    .setActualPayAmount(ProtoConversionUtils.formatLong(item.getActualPayAmount()))
                    .setRefundStatus(item.getRefundStatus())
                    .setImageObjectKey(item.getImageObjectKey() != null ? item.getImageObjectKey() : "")
                    .build());
        }

        return builder.build();
    }

    // ==================== FinancialLogView ====================

    public static List<FinancialLogView> toFinancialLogViews(List<FinancialLogEntity> logs) {
        return logs.stream()
                .map(l -> FinancialLogView.newBuilder()
                        .setType(l.getType())
                        .setAmount(ProtoConversionUtils.formatLong(l.getAmount()))
                        .setExternalTxnId(l.getExternalTxnId())
                        .setCreateTime(formatIso8601(l.getCreatedAt()))
                        .build())
                .toList();
    }

    // ==================== PageMeta ====================

    public static PageMeta toPageMeta(int page, int size, long total) {
        int totalPages = size > 0 ? (int) Math.ceil((double) total / size) : 0;
        return PageMeta.newBuilder()
                .setCurrentPage(ProtoConversionUtils.formatInt(page))
                .setTotalPages(ProtoConversionUtils.formatInt(totalPages))
                .setTotalItems(ProtoConversionUtils.formatLong(total))
                .build();
    }

    // ==================== GetOrderForReview ====================

    public static OrderForReviewData toOrderForReviewData(OrderEntity order) {
        return OrderForReviewData.newBuilder()
                .setOrderNo(order.getOrderNo())
                .setBuyerId(ProtoConversionUtils.formatLong(order.getBuyerId()))
                .setStoreId(ProtoConversionUtils.formatLong(order.getStoreId()))
                .setStatus(order.getStatus())
                .setCompletedAt(formatIso8601(
                        order.getConfirmedAt() != null ? order.getConfirmedAt() : order.getCreatedAt()))
                .build();
    }

    public static List<OrderItemForReview> toOrderItemsForReview(List<OrderItemEntity> items, String cdnBaseUrl) {
        return items.stream()
                .map(item -> OrderItemForReview.newBuilder()
                        .setOrderItemId(ProtoConversionUtils.formatLong(item.getId()))
                        .setSkuName(item.getSkuName())
                        // image_url 相容欄位（組好的 CDN URL）；image_object_key 供 review-service 直接落庫免剝殼（49 §5.3）
                        .setImageUrl(buildCdnUrl(cdnBaseUrl, item.getImageObjectKey()))
                        .setImageObjectKey(item.getImageObjectKey() != null ? item.getImageObjectKey() : "")
                        .setPrice(ProtoConversionUtils.formatLong(item.getUnitPrice()))
                        .setProductId(ProtoConversionUtils.formatLong(item.getProductId()))
                        .build())
                .toList();
    }

    public static List<OrderItemDetail> toItemDetails(List<OrderItemEntity> items, String cdnBaseUrl) {
        return items.stream()
                .map(item -> {
                    var b = OrderItemDetail.newBuilder()
                            .setId(ProtoConversionUtils.formatLong(item.getId()))
                            .setSkuId(ProtoConversionUtils.formatLong(item.getSkuId()))
                            .setSkuName(item.getSkuName())
                            .setQuantity(ProtoConversionUtils.formatInt(item.getQuantity()))
                            .setUnitPrice(ProtoConversionUtils.formatLong(item.getUnitPrice()))
                            .setListPrice(ProtoConversionUtils.formatLong(item.getListPrice()))
                            .setActualPayAmount(ProtoConversionUtils.formatLong(item.getActualPayAmount()))
                            .setRefundStatus(item.getRefundStatus())
                            .setShareDiscount(ProtoConversionUtils.formatAmount(item.getShareDiscount()))
                            .setImageKey(item.getImageObjectKey() != null ? item.getImageObjectKey() : ""); // 49 §6.6：只填 key，Gateway 組 imageUrl
                    if (item.getProductId() != null) {
                        b.setProductId(ProtoConversionUtils.formatLong(item.getProductId()));
                    }
                    if (item.getProductName() != null) {
                        b.setProductName(item.getProductName());
                    }
                    return b.build();
                })
                .toList();
    }

    /** 將 orders.review JSONB 快照映射為 proto OrderReviewView（訂單層級單一；未評論時不設定，前端見 null）。 */
    private static void applyOrderReview(OrderDetailView.Builder b, ReviewSnapshot review) {
        if (review == null || review.getRating() == null) {
            return;
        }
        OrderReviewView.Builder rb = OrderReviewView.newBuilder()
                .setRating(ProtoConversionUtils.formatInt(review.getRating()))
                .setContent(review.getContent() != null ? review.getContent() : "")
                .setCreatedAt(review.getCreatedAt() != null ? review.getCreatedAt() : "");
        // proto image_urls 欄名沿用、值為 objectKey；Gateway 組顯示 URL（49 §6.2）
        if (review.getImageKeys() != null) {
            rb.addAllImageKeys(review.getImageKeys());
        }
        b.setReview(rb.build());
    }

    // ==================== 狀態摘要計算 ====================

    /** 付款截止時間顯示格式：yyyy/MM/dd HH:mm */
    private static final DateTimeFormatter DEADLINE_FMT = DateTimeFormatter.ofPattern("yyyy/MM/dd HH:mm");

    /**
     * 依 OrderStatus 自動選取對應的關鍵時間點
     */
    private static String resolveStatusTime(OrderEntity order) {
        String status = order.getStatus();
        if (status == null) return "";
        return switch (status) {
            case "0"  -> formatIso8601(order.getPaymentDeadline());  // PENDING_PAYMENT → 付款截止
            case "10" -> formatIso8601(order.getPayTime());          // PAID → 付款時間
            case "20" -> formatIso8601(order.getShipTime());         // SHIPPED → 出貨時間
            case "30" -> formatIso8601(order.getUpdatedAt());        // IN_TRANSIT → 開始配送時間
            case "32" -> formatIso8601(order.getUpdatedAt());        // WAITING_PICKUP → 到店時間
            // DELIVERED → 送達時間。曾寫成 confirmed_at，但那一欄只在 40→50 的轉換寫入
            // （completeDeliveredOrderByStatus），status 還是 40 的時候它恆為 NULL ⇒ 這一格
            // 其實一直走在 updated_at 上。delivered_at 才是這個狀態自己的時間欄。
            case "40" -> formatIso8601(order.getDeliveredAt() != null
                    ? order.getDeliveredAt() : order.getUpdatedAt());
            case "50" -> formatIso8601(order.getConfirmedAt());      // COMPLETED → 完成時間
            // CANCELLED → 取消時間。cancelled_at 於 V11 起由取消 CAS 同句寫入；更早的訂單
            // 無法回填，那些列仍退回 updated_at（會被後續寫入推走，但總比沒有時間好）。
            case "80" -> formatIso8601(order.getCancelledAt() != null
                    ? order.getCancelledAt() : order.getUpdatedAt());
            default   -> formatIso8601(order.getUpdatedAt());        // 其餘（退款等）
        };
    }

    /**
     * 依 OrderStatus + ShippingStatus 組合產生文案
     */
    private static String resolveStatusSummary(OrderEntity order) {
        String status = order.getStatus();
        String ss = order.getShippingStatus();
        if (status == null) return "";

        // 物流異常狀態優先判斷（不論 OrderStatus）
        if (ss != null) {
            if ("50".equals(ss)) return "物流異常，請聯繫客服處理。";
            if ("60".equals(ss) || "70".equals(ss)) return "退貨處理中。";
            if ("90".equals(ss)) return "物流需人工處理，請聯繫客服。";
        }

        return switch (status) {
            case "0" -> {  // PENDING_PAYMENT
                String deadline = order.getPaymentDeadline() != null
                        ? order.getPaymentDeadline().format(DEADLINE_FMT)
                        : "";
                yield deadline.isEmpty()
                        ? "請儘速完成付款，逾時訂單將自動取消。"
                        : "請於 " + deadline + " 前完成付款，逾時訂單將自動取消。";
            }
            case "10" -> "已完成付款，等待商家備貨出貨。";          // PAID
            case "20" -> "商家已寄出，等待物流攬件。";              // SHIPPED
            case "30" -> "您的包裹配送中，請留意收件。";            // IN_TRANSIT
            case "32" -> "包裹已到店，請儘速前往取貨。";            // WAITING_PICKUP
            case "40" -> "商品已送達，請確認收貨。";                // DELIVERED
            case "50" -> "訂單已完成，感謝您的購買！";              // COMPLETED
            case "60" -> "部分商品已退款處理。";                    // PARTIALLY_REFUNDED
            case "70" -> "全額退款已完成。";                        // FULLY_REFUNDED
            case "80" -> "訂單已取消。";                            // CANCELLED
            default   -> "";
        };
    }

    // ==================== 工具 ====================

    /**
     * 將 LocalDateTime(UTC) 轉為 ISO 8601 UTC 格式字串
     * 例：2024-02-14T08:00:00Z
     * 時區策略：DB 已統一儲存 UTC，直接格式化
     */
    private static String formatIso8601(LocalDateTime time) {
        return DateTimeUtils.formatIso8601(time);
    }

    /**
     * 組裝 CDN URL（cdnBaseUrl + objectKey）
     * objectKey 為 null 或空時回傳空字串
     */
    private static String buildCdnUrl(String cdnBaseUrl, String objectKey) {
        if (objectKey == null || objectKey.isEmpty()) {
            return "";
        }
        // 已是完整 URL（Seller Service 等上游已拼好 CDN 前綴）→ 直接回傳，避免重複拼接
        if (objectKey.startsWith("http://") || objectKey.startsWith("https://")) {
            return objectKey;
        }
        if (cdnBaseUrl == null || cdnBaseUrl.isEmpty()) {
            return objectKey;
        }
        String base = cdnBaseUrl.endsWith("/") ? cdnBaseUrl.substring(0, cdnBaseUrl.length() - 1) : cdnBaseUrl;
        String key = objectKey.startsWith("/") ? objectKey : "/" + objectKey;
        return base + key;
    }

    private static String firstNonBlank(String preferred, String fallback) {
        if (preferred != null && !preferred.isBlank()) {
            return preferred;
        }
        return fallback != null ? fallback : "";
    }
}
