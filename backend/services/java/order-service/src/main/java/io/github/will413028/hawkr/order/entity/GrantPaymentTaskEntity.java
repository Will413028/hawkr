package io.github.will413028.hawkr.order.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * 撥款指示任務表 Entity
 * 確認收貨後非同步呼叫 Payment Service GrantPayment
 */
@Data
@TableName("grant_payment_tasks")
public class GrantPaymentTaskEntity {

    @TableId(type = IdType.INPUT)
    private Long taskId;

    private String orderNo;

    /** 確認收貨方式：MANUAL / AUTO */
    private String confirmType;

    /** 任務狀態：PENDING / PROCESSING / DONE / FAILED / MANUAL_INTERVENTION */
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
