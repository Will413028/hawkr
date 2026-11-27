package io.github.will413028.hawkr.aftersales.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import io.github.will413028.hawkr.aftersales.config.AfterSalesConstants;
import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.entity.RefundTaskEntity;
import io.github.will413028.hawkr.aftersales.entity.SellerCancelRequestEntity;
import io.github.will413028.hawkr.aftersales.exception.AfterSalesBusinessException;
import io.github.will413028.hawkr.aftersales.grpc.client.OrderGrpcClient;
import io.github.will413028.hawkr.aftersales.mapper.RefundTaskMapper;
import io.github.will413028.hawkr.aftersales.mapper.SellerCancelRequestMapper;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.*;
import io.github.will413028.hawkr.common.utils.proto.ProtoConversionUtils;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.order.grpc.CancelOrderInternalResponse;
import io.github.will413028.hawkr.order.grpc.GetOrderForAfterSalesResponse;
import io.github.will413028.hawkr.order.grpc.OrderAfterSalesView;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.util.Map;
import java.util.Set;

/**
 * 賣家取消出貨服務。
 * 處理賣家主動取消訂單的流程，包含退款任務建立。
 */
@Service
@RequiredArgsConstructor
public class SellerCancelService {

    private static final ObsLogger log = ObsLogger.forClass(SellerCancelService.class);

    private final SellerCancelRequestMapper cancelMapper;
    private final RefundTaskMapper refundTaskMapper;
    private final OrderGrpcClient orderGrpcClient;
    private final AfterSaleAuditService auditService;
    private final AfterSalesProperties properties;
    private final AfterSalesEventPublisher afterSalesEventPublisher;

    /** 待付款訂單狀態 value */
    private static final String ORDER_STATUS_PENDING_PAYMENT = OrderStatus.PENDING_PAYMENT.value();

    /** 允許取消出貨的訂單狀態：待付款 / 已付款 / 待出貨 */
    private static final Set<String> CANCEL_ALLOWED_ORDER_STATUSES = Set.of(
            OrderStatus.PENDING_PAYMENT.value(), OrderStatus.PAID.value(), OrderStatus.PENDING_SHIPMENT.value());

    /**
     * 賣家取消出貨。
     *
     * @param storeId   賣家 store_id（Gateway sellerid resolver 解析後注入）
     * @param orderNo   訂單編號
     * @param reason    取消原因（OrderCancelReason 92xx 賣家觸發 value）
     * @param remark    取消備註（可選）
     */
    @Transactional
    public void cancelShipment(Long storeId, String orderNo, String reason, String remark) {
        CodeMetaValidator.requireValidValue(OrderCancelReason.class, reason, "reason");

        GetOrderForAfterSalesResponse orderResp = orderGrpcClient.getOrderForAfterSales(orderNo);
        OrderAfterSalesView order = orderResp.getData();

        Long orderStoreId = ProtoConversionUtils.parseLongRequired(order.getStoreId(), "storeId");
        if (!orderStoreId.equals(storeId)) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_NOT_OWNER);
        }

        if (!CANCEL_ALLOWED_ORDER_STATUSES.contains(order.getStatus())) {
            throw new AfterSalesBusinessException(GrpcResponseCode.CANCEL_NOT_ALLOWED);
        }

        Long existingCount = cancelMapper.selectCount(
                new LambdaQueryWrapper<SellerCancelRequestEntity>()
                        .eq(SellerCancelRequestEntity::getOrderNo, orderNo)
        );
        if (existingCount > 0) {
            throw new AfterSalesBusinessException(GrpcResponseCode.CANCEL_DUPLICATE);
        }

        Long buyerId = ProtoConversionUtils.parseLongRequired(order.getBuyerId(), "buyerId");
        boolean isUnpaid = ORDER_STATUS_PENDING_PAYMENT.equals(order.getStatus());
        Long refundAmount;
        if (isUnpaid) {
            refundAmount = 0L;
        } else {
            refundAmount = ProtoConversionUtils.parseLongRequired(order.getTotalActualAmount(), "totalActualAmount");
        }

        SellerCancelRequestEntity cancelEntity = new SellerCancelRequestEntity();
        cancelEntity.setId(TsidUtil.generate());
        cancelEntity.setOrderNo(orderNo);
        cancelEntity.setStoreId(orderStoreId);
        cancelEntity.setBuyerId(buyerId);
        cancelEntity.setReason(reason);
        cancelEntity.setRemark(remark);
        cancelEntity.setStatus(SellerCancelStatus.PENDING.value());
        cancelEntity.setRefundAmount(refundAmount);
        cancelEntity.setTransactionId(order.getTransactionId());
        cancelEntity.setPaymentMethod(order.getPaymentMethod());
        cancelEntity.setMerId(order.getMerId());
        cancelMapper.insert(cancelEntity);

        Long cancelId = cancelEntity.getId();

        try {
            orderGrpcClient.cancelOrderInternal(orderNo, reason, remark != null ? remark : "",
                    OrderCancelByRole.SELLER.value());
        } catch (io.grpc.StatusRuntimeException e) {
            log.error("SellerCancelService.cancelShipment.cancelOrderFailed",
                    "取消訂單失敗, orderNo=" + orderNo + ", " + e.getMessage());
            cancelMapper.markFailedFromPending(cancelId);
            throw io.github.will413028.hawkr.common.utils.grpc.GrpcClientErrorTranslator.translate(
                    e, AfterSalesBusinessException::new);
        }

        if (isUnpaid) {
            cancelMapper.markCompletedFromPending(cancelId);

            afterSalesEventPublisher.publishCancelCompleted(orderNo, cancelId);
        } else {
            RefundTaskEntity refundTask = new RefundTaskEntity();
            refundTask.setId(TsidUtil.generate());
            refundTask.setCancelId(cancelId);
            refundTask.setOrderNo(orderNo);
            refundTask.setTransactionId(order.getTransactionId());
            refundTask.setRefundAmount(refundAmount);
            refundTask.setPaymentMethod(order.getPaymentMethod());
            refundTask.setStatus(WorkerTaskStatus.PENDING.value());
            refundTask.setRetryCount(0);
            refundTask.setMaxRetryCount(properties.getRefund().getMaxRetryCount());
            refundTaskMapper.insert(refundTask);

            cancelMapper.markRefundProcessing(cancelId);
        }

        auditService.log(null, cancelId, storeId, AfterSaleAuditActor.SELLER.value(), AfterSaleAuditAction.CANCEL_SHIPMENT.value(),
                Map.of("orderNo", orderNo, "reason", reason, "refundAmount", refundAmount, "isUnpaid", isUnpaid));

        log.info("SellerCancelService.cancelShipment.success",
                "賣家取消出貨完成, cancelId=" + cancelId + ", refundAmount=" + refundAmount, "orderNo", orderNo);
    }

    /**
     * §5.9 系統自動取消出貨（賣家逾期未處理）。
     * 與 {@link #cancelShipment} 差異：跳過賣家身分驗證、使用 SYSTEM actor。
     *
     * @param orderNo 訂單編號
     */
    @Transactional
    public void cancelShipmentBySystem(String orderNo, OrderCancelReason cancelReason) {
        String reason = cancelReason.value();
        String remark = "系統自動取消：" + cancelReason.label();

        Long existingCount = cancelMapper.selectCount(
                new LambdaQueryWrapper<SellerCancelRequestEntity>()
                        .eq(SellerCancelRequestEntity::getOrderNo, orderNo)
        );
        if (existingCount > 0) {
            log.info("SellerCancelService.cancelShipmentBySystem.idempotent",
                    "已有取消記錄，跳過", "orderNo", orderNo);
            return;
        }

        GetOrderForAfterSalesResponse orderResp = orderGrpcClient.getOrderForAfterSales(orderNo);
        OrderAfterSalesView order = orderResp.getData();

        if (!CANCEL_ALLOWED_ORDER_STATUSES.contains(order.getStatus())) {
            log.info("SellerCancelService.cancelShipmentBySystem.skip",
                    "訂單狀態不允許取消, status=" + order.getStatus(), "orderNo", orderNo);
            return;
        }

        Long storeId = ProtoConversionUtils.parseLongRequired(order.getStoreId(), "storeId");
        Long buyerId = ProtoConversionUtils.parseLongRequired(order.getBuyerId(), "buyerId");
        boolean isUnpaid = ORDER_STATUS_PENDING_PAYMENT.equals(order.getStatus());
        Long refundAmount;
        if (isUnpaid) {
            refundAmount = 0L;
        } else {
            refundAmount = ProtoConversionUtils.parseLongRequired(order.getTotalActualAmount(), "totalActualAmount");
        }

        SellerCancelRequestEntity cancelEntity = new SellerCancelRequestEntity();
        cancelEntity.setId(TsidUtil.generate());
        cancelEntity.setOrderNo(orderNo);
        cancelEntity.setStoreId(storeId);
        cancelEntity.setBuyerId(buyerId);
        cancelEntity.setReason(reason);
        cancelEntity.setRemark(remark);
        cancelEntity.setStatus(SellerCancelStatus.PENDING.value());
        cancelEntity.setRefundAmount(refundAmount);
        cancelEntity.setTransactionId(order.getTransactionId());
        cancelEntity.setPaymentMethod(order.getPaymentMethod());
        cancelEntity.setMerId(order.getMerId());
        cancelMapper.insert(cancelEntity);

        Long cancelId = cancelEntity.getId();

        try {
            orderGrpcClient.cancelOrderInternal(orderNo, reason, remark, OrderCancelByRole.SYSTEM.value());
        } catch (io.grpc.StatusRuntimeException e) {
            log.error("SellerCancelService.cancelShipmentBySystem.cancelOrderFailed",
                    "取消訂單失敗, orderNo=" + orderNo + ", " + e.getMessage());
            cancelMapper.markFailedFromPending(cancelId);
            return;
        }

        if (isUnpaid) {
            cancelMapper.markCompletedFromPending(cancelId);
            afterSalesEventPublisher.publishCancelCompleted(orderNo, cancelId);
        } else {
            RefundTaskEntity refundTask = new RefundTaskEntity();
            refundTask.setId(TsidUtil.generate());
            refundTask.setCancelId(cancelId);
            refundTask.setOrderNo(orderNo);
            refundTask.setTransactionId(order.getTransactionId());
            refundTask.setRefundAmount(refundAmount);
            refundTask.setPaymentMethod(order.getPaymentMethod());
            refundTask.setStatus(WorkerTaskStatus.PENDING.value());
            refundTask.setRetryCount(0);
            refundTask.setMaxRetryCount(properties.getRefund().getMaxRetryCount());
            refundTaskMapper.insert(refundTask);

            cancelMapper.markRefundProcessing(cancelId);
        }

        auditService.log(null, cancelId, null, AfterSaleAuditActor.SYSTEM.value(),
                AfterSaleAuditAction.AUTO_CANCEL_PROCESSING_TIMEOUT.value(),
                Map.of("orderNo", orderNo, "reason", reason, "refundAmount", refundAmount, "isUnpaid", isUnpaid));

        log.info("SellerCancelService.cancelShipmentBySystem.success",
                "系統自動取消出貨完成, cancelId=" + cancelId + ", refundAmount=" + refundAmount, "orderNo", orderNo);
    }
}
