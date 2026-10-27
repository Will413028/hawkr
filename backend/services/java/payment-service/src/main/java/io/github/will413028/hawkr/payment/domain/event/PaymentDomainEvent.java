package io.github.will413028.hawkr.payment.domain.event;

import io.github.will413028.hawkr.common.utils.code.PaymentEventType;

import java.time.LocalDate;
import java.time.LocalDateTime;
import java.util.HashMap;
import java.util.Map;

/**
 * 支付領域事件基礎介面
 */
public sealed interface PaymentDomainEvent permits
        PaymentDomainEvent.PaymentSucceeded,
        PaymentDomainEvent.PaymentFailed,
        PaymentDomainEvent.PaymentExpired,
        PaymentDomainEvent.PaymentClosed,
        PaymentDomainEvent.PaymentConflict,
        PaymentDomainEvent.RefundSucceeded,
        PaymentDomainEvent.RefundFailed,
        PaymentDomainEvent.PaymentRescueRefundInitiated {

    /**
     * 事件類型
     */
    PaymentEventType eventType();

    /**
     * 聚合類型
     */
    String aggregateType();

    /**
     * 聚合 ID（交易編號或退款編號）
     */
    String aggregateId();

    /**
     * 事件 Payload
     */
    Map<String, Object> payload();

    /**
     * 事件發生時間
     */
    LocalDateTime occurredAt();

    // ==================== 支付相關事件 ====================

    /**
     * 付款成功事件
     */
    record PaymentSucceeded(
            String tradeNo,
            String orderId,
            String buyerId,
            long amount,
            String providerTradeNo,
            String paymentType,
            LocalDateTime payTime,
            LocalDateTime occurredAt
    ) implements PaymentDomainEvent {

        @Override
        public PaymentEventType eventType() {
            return PaymentEventType.PAYMENT_SUCCESS;
        }

        @Override
        public String aggregateType() {
            return PaymentAggregateType.PAYMENT;
        }

        @Override
        public String aggregateId() {
            return tradeNo;
        }

        @Override
        public Map<String, Object> payload() {
            return Map.of(
                    "trade_no", tradeNo,
                    "order_id", orderId,
                    "buyer_id", buyerId != null ? buyerId : "",
                    "amount", amount,
                    "provider_trade_no", providerTradeNo != null ? providerTradeNo : "",
                    "payment_type", paymentType != null ? paymentType : "",
                    "pay_time", payTime != null ? payTime.toString() : ""
            );
        }
    }

    /**
     * 付款失敗事件
     */
    record PaymentFailed(
            String tradeNo,
            String orderId,
            String buyerId,
            long amount,
            String providerMessage,
            LocalDateTime occurredAt
    ) implements PaymentDomainEvent {

        @Override
        public PaymentEventType eventType() {
            return PaymentEventType.PAYMENT_FAILED;
        }

        @Override
        public String aggregateType() {
            return PaymentAggregateType.PAYMENT;
        }

        @Override
        public String aggregateId() {
            return tradeNo;
        }

        @Override
        public Map<String, Object> payload() {
            return Map.of(
                    "trade_no", tradeNo,
                    "order_id", orderId,
                    "buyer_id", buyerId != null ? buyerId : "",
                    "amount", amount,
                    "provider_message", providerMessage != null ? providerMessage : ""
            );
        }
    }

    /**
     * 付款逾期事件
     */
    record PaymentExpired(
            String tradeNo,
            String orderId,
            String buyerId,
            long amount,
            LocalDateTime expireAt,
            LocalDateTime occurredAt
    ) implements PaymentDomainEvent {

        @Override
        public PaymentEventType eventType() {
            return PaymentEventType.PAYMENT_EXPIRED;
        }

        @Override
        public String aggregateType() {
            return PaymentAggregateType.PAYMENT;
        }

        @Override
        public String aggregateId() {
            return tradeNo;
        }

        @Override
        public Map<String, Object> payload() {
            return Map.of(
                    "trade_no", tradeNo,
                    "order_id", orderId,
                    "buyer_id", buyerId != null ? buyerId : "",
                    "amount", amount,
                    "expire_at", expireAt != null ? expireAt.toString() : ""
            );
        }
    }

    /**
     * 交易關閉事件
     */
    record PaymentClosed(
            String tradeNo,
            String orderId,
            String buyerId,
            long amount,
            LocalDateTime closedAt,
            LocalDateTime occurredAt
    ) implements PaymentDomainEvent {

        @Override
        public PaymentEventType eventType() {
            return PaymentEventType.PAYMENT_CLOSED;
        }

        @Override
        public String aggregateType() {
            return PaymentAggregateType.PAYMENT;
        }

        @Override
        public String aggregateId() {
            return tradeNo;
        }

        @Override
        public Map<String, Object> payload() {
            return Map.of(
                    "trade_no", tradeNo,
                    "order_id", orderId,
                    "buyer_id", buyerId != null ? buyerId : "",
                    "amount", amount,
                    "closed_at", closedAt != null ? closedAt.toString() : ""
            );
        }
    }

    /**
     * 狀態衝突事件
     */
    record PaymentConflict(
            String tradeNo,
            String orderId,
            String buyerId,
            long amount,
            String syncResult,
            String webhookResult,
            LocalDateTime occurredAt
    ) implements PaymentDomainEvent {

        @Override
        public PaymentEventType eventType() {
            return PaymentEventType.PAYMENT_CONFLICT;
        }

        @Override
        public String aggregateType() {
            return PaymentAggregateType.PAYMENT;
        }

        @Override
        public String aggregateId() {
            return tradeNo;
        }

        @Override
        public Map<String, Object> payload() {
            return Map.of(
                    "trade_no", tradeNo,
                    "order_id", orderId,
                    "buyer_id", buyerId != null ? buyerId : "",
                    "amount", amount,
                    "sync_result", syncResult != null ? syncResult : "",
                    "webhook_result", webhookResult != null ? webhookResult : ""
            );
        }
    }

    // ==================== 退款相關事件 ====================

    /**
     * 退款成功事件
     */
    record RefundSucceeded(
            String refundNo,
            String tradeNo,
            String orderId,
            long amount,
            String providerRefundNo,
            String payuniStatus,          // PayUni 原始狀態碼（sync: response.Status；async webhook: inner RefundStatus）
            String payuniMessage,         // PayUni 原始訊息
            LocalDate estimatedRefundDate, // ATM sync success：PayUni EstimatedRefundDate（僅 ATM 且仍等 webhook 時有值）
            LocalDate bankDate,           // ATM async RefundStatus=1：webhook BankDate
            LocalDateTime completedAt,
            LocalDateTime occurredAt
    ) implements PaymentDomainEvent {

        @Override
        public PaymentEventType eventType() {
            return PaymentEventType.REFUND_SUCCEEDED;
        }

        @Override
        public String aggregateType() {
            return PaymentAggregateType.REFUND;
        }

        @Override
        public String aggregateId() {
            return refundNo;
        }

        // Payload 遵循 docs/開發規範/API 設計規範.md camelCase；收斂規範 §4.4 單一欄位雙階段寫入規則。
        @Override
        public Map<String, Object> payload() {
            Map<String, Object> map = new HashMap<>();
            map.put("refundNo", refundNo);
            map.put("tradeNo", tradeNo);
            map.put("orderId", orderId);
            map.put("amount", amount);
            map.put("providerRefundNo", providerRefundNo != null ? providerRefundNo : "");
            map.put("payuniStatus", payuniStatus != null ? payuniStatus : "");
            map.put("payuniMessage", payuniMessage != null ? payuniMessage : "");
            map.put("estimatedRefundDate", estimatedRefundDate != null ? estimatedRefundDate.toString() : "");
            map.put("bankDate", bankDate != null ? bankDate.toString() : "");
            map.put("completedAt", completedAt != null ? completedAt.toString() : "");
            map.put("occurredAt", occurredAt != null ? occurredAt.toString() : "");
            return map;
        }
    }

    /**
     * 退款失敗事件
     */
    record RefundFailed(
            String refundNo,
            String tradeNo,
            String orderId,
            long amount,
            String payuniStatus,          // PayUni 錯誤碼（sync: response.Status；async webhook: inner RefundStatus=2）
            String payuniMessage,         // PayUni 原始錯誤訊息（欄位原名 providerMessage，改為對齊 aftersales snapshot 命名）
            LocalDateTime occurredAt
    ) implements PaymentDomainEvent {

        @Override
        public PaymentEventType eventType() {
            return PaymentEventType.REFUND_FAILED;
        }

        @Override
        public String aggregateType() {
            return PaymentAggregateType.REFUND;
        }

        @Override
        public String aggregateId() {
            return refundNo;
        }

        // Payload 遵循 camelCase；不包含 status 欄位，consumer 依 routing key 判斷成敗。
        @Override
        public Map<String, Object> payload() {
            Map<String, Object> map = new HashMap<>();
            map.put("refundNo", refundNo);
            map.put("tradeNo", tradeNo);
            map.put("orderId", orderId);
            map.put("amount", amount);
            map.put("payuniStatus", payuniStatus != null ? payuniStatus : "");
            map.put("payuniMessage", payuniMessage != null ? payuniMessage : "");
            map.put("occurredAt", occurredAt != null ? occurredAt.toString() : "");
            return map;
        }
    }

    // ==================== L2 補救路徑事件 ====================

    /**
     * Rescue 退款已發起：reconcile 在 grace 視窗內偵測到 EXPIRED 但 PayUni 已成功，自動觸發退款
     */
    record PaymentRescueRefundInitiated(
            String tradeNo,
            String orderId,
            String merId,
            String providerTradeNo,
            long amount,
            String paymentType,
            String rescueReason,
            LocalDateTime occurredAt
    ) implements PaymentDomainEvent {

        @Override
        public PaymentEventType eventType() {
            return PaymentEventType.PAYMENT_RESCUE_REFUND_INITIATED;
        }

        @Override
        public String aggregateType() {
            return PaymentAggregateType.PAYMENT;
        }

        @Override
        public String aggregateId() {
            return tradeNo;
        }

        @Override
        public Map<String, Object> payload() {
            // 用 HashMap 而非 Map.of 因為 Map.of 不允許 null value，
            // mer_id / provider_trade_no 在某些場景可能為 null
            java.util.HashMap<String, Object> m = new java.util.HashMap<>();
            m.put("trade_no", tradeNo);
            m.put("order_id", orderId);
            m.put("mer_id", merId != null ? merId : "");
            m.put("provider_trade_no", providerTradeNo != null ? providerTradeNo : "");
            m.put("amount", amount);
            m.put("payment_type", paymentType != null ? paymentType : "");
            m.put("rescue_reason", rescueReason != null ? rescueReason : "");
            return m;
        }
    }
}
