package io.github.will413028.hawkr.aftersales.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.time.LocalDateTime;

@Data
@TableName("aftersale_resolutions")
public class AfterSaleResolutionEntity {

    @TableId(type = IdType.INPUT)
    private Long id;

    private String caseNo;

    private String resolutionType;

    // §10 來源：SELLER（既有預設）/ PLATFORM（Admin 裁決產生）
    private String resolutionSource;

    private Long refundAmount;

    private Long couponAmount;

    private Long couponMinSpend;

    private String couponMessage;

    @TableField("coupon_template_id")
    private String couponTemplateId;

    private String rejectReason;

    private String rejectMessage;

    private String negotiationReason;

    private String returnShippingMethod;

    private String deductionReason;

    private String inspectionNote;

    private Long resolvedBy;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;
}
