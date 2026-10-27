package io.github.will413028.hawkr.payment.infrastructure.persistence.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.time.LocalDate;
import java.time.LocalDateTime;

/**
 * 退款交易 Entity
 */
@Data
@TableName("payment_refunds")
public class RefundTransactionEntity {

    @TableId(type = IdType.INPUT)
    private Long id;

    /**
     * 系統退款編號 (RFD{yyyyMMdd}{HHmmss}{random4})
     */
    private String refundNo;

    /**
     * 對應 payment_transactions(id) 的 PK，§2A.3.6 FK 改指 PK
     */
    private Long paymentId;

    /**
     * 原交易編號（保留供查詢/顯示，不再 UNIQUE）
     */
    private String tradeNo;

    /**
     * 訂單編號
     */
    private String orderId;

    /**
     * 退款金額（新台幣元）
     */
    private Long amount;

    /**
     * 退款狀態 (0:REQUESTED, 1:PROCESSING, 2:SUCCEEDED, 3:FAILED)
     */
    private String status;

    /**
     * 退款原因
     */
    private String reason;

    /**
     * 冪等鍵（AfterSales 傳入 case_no 或 cancel_id）
     */
    private String idempotencyKey;

    /**
     * PayUni 退款編號
     */
    private String providerRefundNo;

    /**
     * PayUni 退款狀態
     */
    private String providerStatus;

    /**
     * PayUni 回傳訊息
     */
    private String providerMessage;

    /**
     * ATM sync success 時 PayUni 回報的預計扣款日（收斂規範 §3.4 里程碑）
     * 欄位型別由原 provider_estimated_refund_at TIMESTAMPTZ 改為 DATE + 改名
     */
    private LocalDate estimatedRefundDate;

    /**
     * ATM async webhook RefundStatus=1 時的實際到帳日
     */
    private LocalDate bankDate;

    /**
     * 退款完成時間
     */
    private LocalDateTime completedAt;

    /**
     * 建立時間
     */
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    /**
     * 更新時間
     */
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;

    /**
     * 版本控制
     */
    @Version
    private Integer version;
}
