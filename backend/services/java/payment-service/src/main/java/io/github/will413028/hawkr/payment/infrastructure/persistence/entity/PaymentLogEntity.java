package io.github.will413028.hawkr.payment.infrastructure.persistence.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * 支付日誌 Entity
 */
@Data
@TableName("payment_logs")
public class PaymentLogEntity {

    @TableId(type = IdType.AUTO)
    private Long id;

    /**
     * 交易編號
     */
    private String tradeNo;

    /**
     * 退款編號（選填）
     */
    private String refundNo;

    /**
     * 日誌類型
     */
    private String logType;

    /**
     * 日誌級別 (INFO, WARNING, ERROR)
     */
    private String logLevel;

    /**
     * 日誌內容
     */
    private String message;

    /**
     * 變更前狀態
     */
    private String fromStatus;

    /**
     * 變更後狀態
     */
    private String toStatus;

    /**
     * 觸發來源 (SYNC_API, WEBHOOK, TTL_CONSUMER, MANUAL)
     */
    private String triggerSource;

    /**
     * 衝突詳情（JSONB）
     */
    private String conflictDetail;

    /**
     * 操作人員
     */
    private String operatorId;

    /**
     * 操作原因
     */
    private String operatorReason;

    /**
     * 建立時間
     */
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;
}
