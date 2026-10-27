package io.github.will413028.hawkr.coupon.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.time.LocalDateTime;

@Data
@TableName("coupon_instances")
public class CouponInstanceEntity {

    @TableId(type = IdType.INPUT)
    private Long instanceId;

    private Long templateId;
    private Long ownerBuyerId;
    private Long applicableStoreId;
    private String status;
    private String reservedOrderNo;
    private LocalDateTime reservedAt;
    private String usedOrderNo;
    private LocalDateTime usedAt;
    private LocalDateTime revokedAt;
    private String revokedReason;
    private LocalDateTime voidedAt;
    private String voidedReason;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;
}
