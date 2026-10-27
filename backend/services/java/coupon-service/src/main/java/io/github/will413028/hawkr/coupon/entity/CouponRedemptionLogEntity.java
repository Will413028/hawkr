package io.github.will413028.hawkr.coupon.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.time.LocalDateTime;

@Data
@TableName("coupon_redemption_logs")
public class CouponRedemptionLogEntity {

    @TableId(type = IdType.AUTO)
    private Long logId;

    private Long instanceId;
    private Long templateId;
    private Long ownerBuyerId;
    private String action;
    private String orderNo;
    private Long operatorAccountId;
    private String traceId;
    private String requestId;
    private String detail;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;
}
