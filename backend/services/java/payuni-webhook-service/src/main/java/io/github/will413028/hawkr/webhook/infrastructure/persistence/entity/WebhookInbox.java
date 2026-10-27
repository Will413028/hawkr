package io.github.will413028.hawkr.webhook.infrastructure.persistence.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableField;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.typehandler.JsonbStringTypeHandler;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * Webhook 收件箱（去重 + 稽核）
 */
@Data
@TableName(value = "webhook_inbox", autoResultMap = true)
public class WebhookInbox {

    @TableId(type = IdType.AUTO)
    private Long id;

    private String provider;
    private String merId;
    private String webhookType;
    private String eventKey;
    private String status;
    private String verifyStatus;

    /** 原始加密信封（JSONB） */
    @TableField(typeHandler = JsonbStringTypeHandler.class)
    private String rawEnvelope;

    /** 解密後 payload（JSONB） */
    @TableField(typeHandler = JsonbStringTypeHandler.class)
    private String decryptedPayload;

    private LocalDateTime receivedAt;
    private LocalDateTime processedAt;
    private String lastError;
}
