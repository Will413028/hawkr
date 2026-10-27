package io.github.will413028.hawkr.order.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * 購物車清理重試任務表 Entity
 *
 * 用途：付款成功後非同步清理購物車，失敗時自動重試
 */
@Data
@TableName("cart_cleanup_retry_tasks")
public class CartCleanupRetryTaskEntity {

    @TableId(type = IdType.INPUT, value = "task_id")
    private Long taskId;

    private String orderNo;

    private String checkoutId;

    private Long buyerId;

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
