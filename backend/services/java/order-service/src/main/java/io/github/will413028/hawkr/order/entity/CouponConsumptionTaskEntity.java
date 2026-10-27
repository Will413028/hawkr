package io.github.will413028.hawkr.order.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * 優惠券核銷任務表 Entity
 *
 * 用途：付款成功後非同步核銷優惠券，失敗時自動重試
 */
@Data
@TableName("coupon_consumption_tasks")
public class CouponConsumptionTaskEntity {

    @TableId(type = IdType.INPUT, value = "task_id")
    private Long taskId;

    private String orderNo;

    private String couponInstanceId;

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
