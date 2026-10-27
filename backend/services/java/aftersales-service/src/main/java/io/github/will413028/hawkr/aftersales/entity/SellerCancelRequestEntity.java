package io.github.will413028.hawkr.aftersales.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.time.LocalDateTime;

@Data
@TableName("seller_cancel_requests")
public class SellerCancelRequestEntity {

    @TableId(type = IdType.INPUT)
    private Long id;

    private String orderNo;

    private Long storeId;

    private Long buyerId;

    private String reason;

    private String remark;

    private String status;

    private Long refundAmount;

    private String transactionId;

    private String paymentMethod;

    private String merId;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;
}
