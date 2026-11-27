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
                .setPaymentMethod(order.getPaymentMethod() != null ? order.getPaymentMethod() : "")
                .setShippingStatus(order.getShippingStatus() != null
                        ? order.getShippingStatus() : "")
                .setShippingStatusDesc(order.getShippingStatusDesc() != null ? order.getShippingStatusDesc() : "")
                .setStatusTime(resolveStatusTime(order))
                .setStatusSummary(resolveStatusSummary(order))
                .setRecipientName(order.getRecipientName() != null ? order.getRecipientName() : "")
                .setShippingAddress(order.getShippingAddress() != null ? order.getShippingAddress() : "")
                .setBuyerId(ProtoConversionUtils.formatLong(order.getBuyerId()));

        if (isSellerView) {
            String liveLabelKey = liveShipment != null ? liveShipment.getLabelKey() : "";
            String livePartnerId = liveShipment != null ? liveShipment.getPartnerId() : "";
            String liveOdno = liveShipment != null ? liveShipment.getOdno() : "";
            String liveValidationNo = liveShipment != null ? liveShipment.getValidationNo() : "";

            builder.setLabelKey(firstNonBlank(liveLabelKey, order.getLabelUrl()))
                    .setLabelExpireAt(order.getLabelExpireAt() != null ? order.getLabelExpireAt().toString() : "")
                    .setPartnerId(firstNonBlank(livePartnerId, order.getPartnerId()))
                    .setOdno(firstNonBlank(liveOdno, order.getOdno()))
                    .setValidationNo(firstNonBlank(liveValidationNo, order.getValidationNo()));
        }

        if (isSellerView && liveShipment != null && liveShipment.getShipmentPhotoKeysCount() > 0) {
            builder.addAllShipmentPhotoKeys(liveShipment.getShipmentPhotoKeysList());
        } else if (order.getShipmentPhotoKeys() != null && !order.getShipmentPhotoKeys().isEmpty()) {
            builder.addAllShipmentPhotoKeys(order.getShipmentPhotoKeys());
        }

        if (sellerMeta != null) {
            builder.setSellerName(sellerMeta.getSellerName())
                    .setSellerAvatarKey(sellerMeta.getLogoKey())
                    .setStoreCode(sellerMeta.getStoreCode())
                    .setSellerCity(order.getSenderCity() != null ? order.getSenderCity() : "")
                    .setSellerDistrict(order.getSenderDistrict() != null ? order.getSenderDistrict() : "");
        }

        if (buyerDisplay != null) {
            builder.setBuyerNickname(buyerDisplay.getDisplayName())
                    .setBuyerAvatarKey(buyerDisplay.getAvatarKey());
        }

        if (order.getIsReviewed() != null && order.getIsReviewed()) {
            builder.setIsReviewed(true);
        }

        if (OrderStatus.COMPLETED.value().equals(order.getStatus())) {
            LocalDateTime completedAt = order.getConfirmedAt() != null
                    ? order.getConfirmedAt() : order.getCreatedAt();
            LocalDateTime deadline = completedAt.plusDays(REVIEW_WINDOW_DAYS);
            builder.setReviewDeadline(formatIso8601(deadline));
            builder.setCanReview(DateTimeUtils.nowUtc().isBefore(deadline));
        }

        if (order.getAutoCompleteAt() != null) {
            builder.setAutoCompleteAt(formatIso8601(order.getAutoCompleteAt()));
        }

        if (order.getCountdownPausedAt() != null) {
            builder.setCountdownPausedAt(formatIso8601(order.getCountdownPausedAt()));
        }

        if (order.getProcessingDeadline() != null) {
            builder.setProcessingDeadline(formatIso8601(order.getProcessingDeadline()));
        }

        if (OrderStatus.PENDING_PAYMENT.value().equals(order.getStatus()) && order.getPaymentDeadline() != null) {
            builder.setPaymentDeadline(formatIso8601(order.getPaymentDeadline()));
        }

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
                    .setSkuId(ProtoConversionUtils.formatUuid(item.getSkuId()))
                    .setSkuName(item.getSkuName())
                    .setQuantity(ProtoConversionUtils.formatInt(item.getQuantity()))
                    .setUnitPrice(ProtoConversionUtils.formatLong(item.getUnitPrice()))
                    .setListPrice(ProtoConversionUtils.formatLong(item.getListPrice()))
                    .setImageKey(item.getImageObjectKey() != null ? item.getImageObjectKey() : "");
            if (item.getProductId() != null) {
                itemBuilder.setProductId(ProtoConversionUtils.formatUuid(item.getProductId()));
            }
            if (item.getProductName() != null) {
                itemBuilder.setProductName(item.getProductName());
            }
            builder.addItems(itemBuilder.build());
        }
        return builder.build();
    }


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
                .setRecipientName(order.getRecipientName() != null ? order.getRecipientName() : "")
                .setRecipientPhone(order.getRecipientPhone() != null ? order.getRecipientPhone() : "")
                .setShippingAddress(order.getShippingAddress() != null ? order.getShippingAddress() : "")
                .setShippingNote(order.getShippingNote() != null ? order.getShippingNote() : "")
                .setOrderRemark(order.getOrderRemark() != null ? order.getOrderRemark() : "")
                .setCvsStoreId(order.getCvsStoreId() != null ? order.getCvsStoreId() : "")
                .setPickupStoreName(order.getCvsStoreName() != null ? order.getCvsStoreName() : "")
                .setShippingArea(order.getShippingArea() != null ? order.getShippingArea() : "")
                .setCouponInstanceId(order.getCouponInstanceId() != null ? order.getCouponInstanceId() : "")
                .setDiscountAmount(ProtoConversionUtils.formatAmount(order.getDiscountAmount()))
                .setShippingStatus(order.getShippingStatus() != null
                        ? order.getShippingStatus() : "")
                .setShippingStatusDesc(order.getShippingStatusDesc() != null ? order.getShippingStatusDesc() : "")
                .setLabelKey(order.getLabelUrl() != null ? order.getLabelUrl() : "")
                .setPaymentMethod(order.getPaymentMethod() != null ? order.getPaymentMethod() : "");

        if (shipment != null) {
            builder.setShippingStatus(shipment.getStatus());
            if (isSellerView) {
                builder.setLabelKey(shipment.getLabelKey())
                        .setPartnerId(shipment.getPartnerId())
                        .setOdno(shipment.getOdno())
                        .setValidationNo(shipment.getValidationNo());
            }
        }

        if (order.getShipmentPhotoKeys() != null && !order.getShipmentPhotoKeys().isEmpty()) {
            builder.addAllShipmentPhotoKeys(order.getShipmentPhotoKeys());
        }

        if (order.getReceiptPhotoKeys() != null && !order.getReceiptPhotoKeys().isEmpty()) {
            builder.addAllReceiptPhotoKeys(order.getReceiptPhotoKeys());
        }

        if (sellerMeta != null) {
            builder.setSellerName(sellerMeta.getSellerName())
                    .setSellerAvatarKey(sellerMeta.getLogoKey())
                    .setStoreCode(sellerMeta.getStoreCode())
                    .setSellerCity(order.getSenderCity() != null ? order.getSenderCity() : "")
                    .setSellerDistrict(order.getSenderDistrict() != null ? order.getSenderDistrict() : "");
        }

        if (buyerDisplay != null) {
            builder.setBuyerNickname(buyerDisplay.getDisplayName())
                    .setBuyerAvatarKey(buyerDisplay.getAvatarKey());
        }

        for (OrderItemEntity item : items) {
            OrderItemDetail.Builder itemBuilder = OrderItemDetail.newBuilder()
                    .setId(ProtoConversionUtils.formatLong(item.getId()))
                    .setSkuId(ProtoConversionUtils.formatUuid(item.getSkuId()))
                    .setSkuName(item.getSkuName())
                    .setQuantity(ProtoConversionUtils.formatInt(item.getQuantity()))
                    .setUnitPrice(ProtoConversionUtils.formatLong(item.getUnitPrice()))
                    .setListPrice(ProtoConversionUtils.formatLong(item.getListPrice()))
                    .setActualPayAmount(ProtoConversionUtils.formatLong(item.getActualPayAmount()))
                    .setRefundStatus(item.getRefundStatus())
                    .setShareDiscount(ProtoConversionUtils.formatAmount(item.getShareDiscount()))
                    .setImageKey(item.getImageObjectKey() != null ? item.getImageObjectKey() : "");
            if (item.getProductId() != null) {
                itemBuilder.setProductId(ProtoConversionUtils.formatUuid(item.getProductId()));
            }
            if (item.getProductName() != null) {
                itemBuilder.setProductName(item.getProductName());
            }
            builder.addItems(itemBuilder.build());
        }

        if (order.getIsReviewed() != null && order.getIsReviewed()) {
            builder.setIsReviewed(true);
        }

        if (OrderStatus.COMPLETED.value().equals(order.getStatus())) {
            LocalDateTime completedAt = order.getConfirmedAt() != null
                    ? order.getConfirmedAt() : order.getCreatedAt();
            LocalDateTime deadline = completedAt.plusDays(REVIEW_WINDOW_DAYS);
            builder.setReviewDeadline(formatIso8601(deadline));
            builder.setCanReview(DateTimeUtils.nowUtc().isBefore(deadline));
        }

        if (order.getAutoCompleteAt() != null) {
            builder.setAutoCompleteAt(formatIso8601(order.getAutoCompleteAt()));
        }

        if (order.getCountdownPausedAt() != null) {
            builder.setCountdownPausedAt(formatIso8601(order.getCountdownPausedAt()));
        }

        if (order.getProcessingDeadline() != null) {
            builder.setProcessingDeadline(formatIso8601(order.getProcessingDeadline()));
        }

        if (OrderStatus.PENDING_PAYMENT.value().equals(order.getStatus()) && order.getPaymentDeadline() != null) {
            builder.setPaymentDeadline(formatIso8601(order.getPaymentDeadline()));
        }

        if (order.getCancelReason() != null) {
            builder.setCancelReason(order.getCancelReason());
        }
        if (order.getCancelRemark() != null) {
            builder.setCancelRemark(order.getCancelRemark());
        }
        if (order.getCancelByRole() != null) {
            builder.setCancelByRole(order.getCancelByRole());
        }

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

        applyOrderReview(builder, order.getReview());

        return builder.build();
    }


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
                .setBuyerName(order.getRecipientName() != null ? order.getRecipientName() : "")
                .setBuyerPhone(order.getRecipientPhone() != null ? order.getRecipientPhone() : "")
                .setBuyerAddressDetail(order.getShippingAddress() != null ? order.getShippingAddress() : "");

        if (order.getShipmentPhotoKeys() != null && !order.getShipmentPhotoKeys().isEmpty()) {
            builder.addAllShipmentPhotoKeys(order.getShipmentPhotoKeys());
        }

        for (OrderItemEntity item : items) {
            builder.addItems(OrderItemForAfterSales.newBuilder()
                    .setId(ProtoConversionUtils.formatLong(item.getId()))
                    .setSkuId(ProtoConversionUtils.formatUuid(item.getSkuId()))
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


    public static PageMeta toPageMeta(int page, int size, long total) {
        int totalPages = size > 0 ? (int) Math.ceil((double) total / size) : 0;
        return PageMeta.newBuilder()
                .setCurrentPage(ProtoConversionUtils.formatInt(page))
                .setTotalPages(ProtoConversionUtils.formatInt(totalPages))
                .setTotalItems(ProtoConversionUtils.formatLong(total))
                .build();
    }


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
                        .setImageUrl(buildCdnUrl(cdnBaseUrl, item.getImageObjectKey()))
                        .setImageObjectKey(item.getImageObjectKey() != null ? item.getImageObjectKey() : "")
                        .setPrice(ProtoConversionUtils.formatLong(item.getUnitPrice()))
                        .setProductId(ProtoConversionUtils.formatUuid(item.getProductId()))
                        .build())
                .toList();
    }

    public static List<OrderItemDetail> toItemDetails(List<OrderItemEntity> items, String cdnBaseUrl) {
        return items.stream()
                .map(item -> {
                    var b = OrderItemDetail.newBuilder()
                            .setId(ProtoConversionUtils.formatLong(item.getId()))
                            .setSkuId(ProtoConversionUtils.formatUuid(item.getSkuId()))
                            .setSkuName(item.getSkuName())
                            .setQuantity(ProtoConversionUtils.formatInt(item.getQuantity()))
                            .setUnitPrice(ProtoConversionUtils.formatLong(item.getUnitPrice()))
                            .setListPrice(ProtoConversionUtils.formatLong(item.getListPrice()))
                            .setActualPayAmount(ProtoConversionUtils.formatLong(item.getActualPayAmount()))
                            .setRefundStatus(item.getRefundStatus())
                            .setShareDiscount(ProtoConversionUtils.formatAmount(item.getShareDiscount()))
                            .setImageKey(item.getImageObjectKey() != null ? item.getImageObjectKey() : "");
                    if (item.getProductId() != null) {
                        b.setProductId(ProtoConversionUtils.formatUuid(item.getProductId()));
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
        if (review.getImageKeys() != null) {
            rb.addAllImageKeys(review.getImageKeys());
        }
        b.setReview(rb.build());
    }


    /** 付款截止時間顯示格式：yyyy/MM/dd HH:mm */
    private static final DateTimeFormatter DEADLINE_FMT = DateTimeFormatter.ofPattern("yyyy/MM/dd HH:mm");

    /**
     * 依 OrderStatus 自動選取對應的關鍵時間點
     */
    private static String resolveStatusTime(OrderEntity order) {
        String status = order.getStatus();
        if (status == null) return "";
        return switch (status) {
            case "0"  -> formatIso8601(order.getPaymentDeadline());
            case "10" -> formatIso8601(order.getPayTime());
            case "20" -> formatIso8601(order.getShipTime());
            case "30" -> formatIso8601(order.getUpdatedAt());
            case "32" -> formatIso8601(order.getUpdatedAt());
            case "40" -> formatIso8601(order.getDeliveredAt() != null
                    ? order.getDeliveredAt() : order.getUpdatedAt());
            case "50" -> formatIso8601(order.getConfirmedAt());
            case "80" -> formatIso8601(order.getCancelledAt() != null
                    ? order.getCancelledAt() : order.getUpdatedAt());
            default   -> formatIso8601(order.getUpdatedAt());
        };
    }

    /**
     * 依 OrderStatus + ShippingStatus 組合產生文案
     */
    private static String resolveStatusSummary(OrderEntity order) {
        String status = order.getStatus();
        String ss = order.getShippingStatus();
        if (status == null) return "";

        if (ss != null) {
            if ("50".equals(ss)) return "物流異常，請聯繫客服處理。";
            if ("60".equals(ss) || "70".equals(ss)) return "退貨處理中。";
            if ("90".equals(ss)) return "物流需人工處理，請聯繫客服。";
        }

        return switch (status) {
            case "0" -> {
                String deadline = order.getPaymentDeadline() != null
                        ? order.getPaymentDeadline().format(DEADLINE_FMT)
                        : "";
                yield deadline.isEmpty()
                        ? "請儘速完成付款，逾時訂單將自動取消。"
                        : "請於 " + deadline + " 前完成付款，逾時訂單將自動取消。";
            }
            case "10" -> "已完成付款，等待商家備貨出貨。";
            case "20" -> "商家已寄出，等待物流攬件。";
            case "30" -> "您的包裹配送中，請留意收件。";
            case "32" -> "包裹已到店，請儘速前往取貨。";
            case "40" -> "商品已送達，請確認收貨。";
            case "50" -> "訂單已完成，感謝您的購買！";
            case "60" -> "部分商品已退款處理。";
            case "70" -> "全額退款已完成。";
            case "80" -> "訂單已取消。";
            default   -> "";
        };
    }


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
