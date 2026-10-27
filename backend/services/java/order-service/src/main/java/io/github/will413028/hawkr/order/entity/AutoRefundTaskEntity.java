package io.github.will413028.hawkr.order.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * 自動退款任務表 Entity
 *
 * 用途：訂單已取消但收到付款事件時，自動退款
 */
@Data
@TableName("auto_refund_tasks")
public class AutoRefundTaskEntity {

    @TableId(type = IdType.INPUT, value = "task_id")
    private Long taskId;

    private String orderNo;

    private String transactionId;

    private Long amount;

    /** 付款方式（PaymentMethod value，用於 Worker 判斷退款路徑） */
    private String paymentMethod;

    private String status;

    private String lastError;

    private Integer retryCount;

    private Integer maxRetryCount;

    private LocalDateTime nextRetryTime;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;
}
