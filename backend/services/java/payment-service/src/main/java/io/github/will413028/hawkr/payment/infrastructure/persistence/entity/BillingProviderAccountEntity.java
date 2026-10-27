package io.github.will413028.hawkr.payment.infrastructure.persistence.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableField;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import io.github.will413028.hawkr.payment.infrastructure.persistence.typehandler.JsonbStringTypeHandler;
import java.time.LocalDateTime;
import lombok.Data;

/** Payment-owned provider account alias. Raw credentials are never persisted. */
@Data
@TableName(value = "billing_provider_accounts", autoResultMap = true)
public class BillingProviderAccountEntity {

    @TableId(type = IdType.INPUT)
    private Long id;
    private String provider;
    private String accountRef;
    private String environment;
    private String credentialRef;

    @TableField(typeHandler = JsonbStringTypeHandler.class)
    private String capabilities;

    private String status;
    private LocalDateTime createdAt;
    private LocalDateTime updatedAt;
}
