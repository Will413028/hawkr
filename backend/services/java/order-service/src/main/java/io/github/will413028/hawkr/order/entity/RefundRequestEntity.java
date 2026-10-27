package io.github.will413028.hawkr.order.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * 退款申請表 Entity
 */
@Data
@TableName("refund_requests")
public class RefundRequestEntity {

    @TableId(type = IdType.INPUT, value = "request_no")
    private String requestNo;

    private String orderNo;

    private Long orderItemId;

    private String reason;

    private String status;

    private Long auditAdminAccountId;

    private String auditRemark;

    private String selectedMethod;

    private Long refundAmount;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;
}
