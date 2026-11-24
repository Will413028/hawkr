package io.github.will413028.hawkr.order.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * 優惠券核銷任務表 Entity
 *
 * 用途：付款成功後非同步核銷優惠券、取消後釋放優惠券（task_type 區分），失敗時自動重試
 */
@Data
@TableName("coupon_consumption_tasks")
public class CouponConsumptionTaskEntity {

    @TableId(type = IdType.INPUT, value = "task_id")
    private Long taskId;

    private String orderNo;

    /** CONSUME（付款後核銷）或 RELEASE（以 order_no 釋券，coupon_instance_id 與 buyer_id 可空）。 */
    private String taskType;

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
