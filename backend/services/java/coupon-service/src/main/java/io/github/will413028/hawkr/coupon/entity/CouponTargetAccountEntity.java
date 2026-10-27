package io.github.will413028.hawkr.coupon.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.time.LocalDateTime;

@Data
@TableName("coupon_target_accounts")
public class CouponTargetAccountEntity {

    /**
     * 複合主鍵的第一部分。標記 @TableId 消除 MyBatis-Plus warning。
     * 實際操作全部使用自定義 SQL（CouponTargetAccountMapper），不使用 xxById 方法。
     */
    @TableId(type = IdType.INPUT)
    private Long templateId;
    private Long buyerId;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;
}
