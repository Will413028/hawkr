package io.github.will413028.hawkr.coupon.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.time.LocalDateTime;

@Data
@TableName("coupon_templates")
public class CouponTemplateEntity {

    @TableId(type = IdType.INPUT)
    private Long templateId;

    private String title;
    private String type;
    private Long applicableStoreId;
    private String issuerType;
    /**
     * 建立者 admin accountId（issuerType=ADMIN 專用；SELLER 模板為 null）—
     * V6 split from 通用欄位，見 docs/superpowers/specs/2026-08-06-coupon-issuer-identity-schema-split-design.md
     */
    private Long issuerAccountId;
    /**
     * 建立者 store_id（issuerType=SELLER 專用；ADMIN 模板為 null）—
     * V6 起不再是通用欄位。
     */
    private Long issuerStoreId;
    private Long thresholdAmount;
    private Long discountAmount;
    private Integer discountPercent;
    private Long maxDiscountAmount;
    private LocalDateTime startAt;
    private LocalDateTime endAt;
    private Integer usageLimitPerAccount;
    private Integer totalIssueLimit;
    private Integer issuedCount;
    private String targetType;
    private String status;
    private Boolean claimRequired;
    private LocalDateTime revokedAt;
    private String revokedReason;

    /** 因賣家停權連動停用時標記 TRUE；解除停權時僅精準還原此旗標為 TRUE 的模板（07 §5.20）。 */
    private Boolean disabledBySuspension;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;
}
