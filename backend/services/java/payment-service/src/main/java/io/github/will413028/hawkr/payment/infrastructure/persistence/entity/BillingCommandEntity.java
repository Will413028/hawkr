package io.github.will413028.hawkr.payment.infrastructure.persistence.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableField;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import io.github.will413028.hawkr.payment.infrastructure.persistence.typehandler.UuidTypeHandler;
import java.time.LocalDateTime;
import java.util.UUID;
import lombok.Data;

/** Shared billing command idempotency row. */
@Data
@TableName(value = "billing_commands", autoResultMap = true)
public class BillingCommandEntity {

    @TableId(type = IdType.INPUT)
    @TableField(typeHandler = UuidTypeHandler.class)
    private UUID id;
    private String operation;
    private String subjectType;
    private String subjectId;
    private String idempotencyKey;
    private String requestHash;
    private String billingQuoteId;
    private Long resultSubscriptionId;
    private String status;
    private String errorCode;
    private String errorMessage;
    private String resultProviderSubscriptionId;
    private String resultRedirectUrl;
    private String resultMerTradeNo;
    private String resultStatus;
    private String actionReason;
    private LocalDateTime leaseExpiresAt;
    private LocalDateTime createdAt;
    private LocalDateTime updatedAt;
}
